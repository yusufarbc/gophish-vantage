package scanner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/notifier"
)

// DefaultExecutor implements ToolExecutor with hardening and OOM-safe streaming.
type DefaultExecutor struct {
	Persister ResultPersister
}

// Execute runs a tool and streams its stdout line-by-line.
// OOM protection: bufio.Scanner with a 1MB buffer — mandatory for nuclei outputs.
// Process safety: exec.CommandContext ensures the process is killed on ctx cancellation.
func (e *DefaultExecutor) Execute(ctx context.Context, userID int64, toolName, target, ifaceName string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no arguments provided for tool: %s", toolName)
	}

	emitLog(fmt.Sprintf("[CMD] %s", strings.Join(args, " ")))

	toolPath, err := exec.LookPath(args[0])
	if err != nil {
		emitLog(fmt.Sprintf("[ERROR] tool not found in PATH: %s", args[0]))
		return fmt.Errorf("tool not found in PATH: %s", args[0])
	}

	cmd := exec.CommandContext(ctx, toolPath, args[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe failed for %s: %w", toolName, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe failed for %s: %w", toolName, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", toolName, err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// ── Stdout: OOM-safe line streaming ──────────────────────────────────────
	// Uses bufio.Scanner with a 1MB line buffer. This is the ONLY safe way to
	// handle nuclei's massive JSONL outputs. json.NewDecoder is NOT used here
	// because it buffers aggressively and can cause OOM on large result sets.
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB per line — nuclei safe
		for sc.Scan() {
			line := sc.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			emitLog(fmt.Sprintf("[%s] %s", strings.ToUpper(toolName), line))
			if e.Persister != nil {
				_ = e.Persister.PersistFinding(userID, toolName, target, ifaceName, line)
			}
		}
		if err := sc.Err(); err != nil && err != io.EOF {
			emitLog(fmt.Sprintf("[%s:stdout] scanner error: %v", strings.ToUpper(toolName), err))
		}
	}()

	// ── Stderr: standard line drain (no persistence needed) ──────────────────
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			line := sc.Text()
			if strings.TrimSpace(line) != "" {
				emitLog(fmt.Sprintf("[%s:stderr] %s", strings.ToUpper(toolName), line))
			}
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			// Context cancelled — kill the process group cleanly
			_ = killProcessGroup(cmd.Process.Pid)
			return ctx.Err()
		}
		// Non-zero exit is common for recon tools (no results found = exit 1), treat as warning
		emitLog(fmt.Sprintf("[%s:warn] process exited non-zero: %v", strings.ToUpper(toolName), err))
	}
	return nil
}

// Collect runs a tool and returns extracted target strings for pipeline chaining.
// OOM protection: bufio.Scanner with 1MB buffer — identical to Execute.
// This method is used when a stage's output feeds the NEXT stage as inputs.
func (e *DefaultExecutor) Collect(ctx context.Context, userID int64, parseAs, target, ifaceName string, args []string) ([]string, error) {
	var targets []string
	var mu sync.Mutex

	if len(args) == 0 {
		return targets, nil
	}

	emitLog(fmt.Sprintf("[CMD] %s", strings.Join(args, " ")))

	toolPath, err := exec.LookPath(args[0])
	if err != nil {
		return nil, fmt.Errorf("tool not found: %s", args[0])
	}

	cmd := exec.CommandContext(ctx, toolPath, args[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe failed for %s: %w", parseAs, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe failed for %s: %w", parseAs, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start %s: %w", parseAs, err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// ── Stdout: OOM-safe JSONL parsing for target extraction ─────────────────
	// We use bufio.Scanner here (not json.NewDecoder) to ensure we never buffer
	// the entire stream. Each line is individually parsed from a 1MB byte buffer.
	go func() {
		defer wg.Done()

		tool, toolFound := DefaultRegistry.Get(parseAs)

		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB per line — OOM safe
		for sc.Scan() {
			line := sc.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			emitLog(fmt.Sprintf("[%s] %s", strings.ToUpper(parseAs), line))

			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(line), &obj); err != nil {
				continue // Skip malformed JSON lines silently
			}

			// Extract the target string using the registered tool's logic
			var extracted string
			if toolFound {
				extracted = tool.ExtractTarget(obj)
			} else {
				extracted = extractTarget(parseAs, obj)
			}

			if extracted != "" {
				mu.Lock()
				targets = append(targets, extracted)
				mu.Unlock()

				if e.Persister != nil {
					_ = e.Persister.PersistDiscoveredTarget(userID, extracted, parseAs)
					_ = e.Persister.PersistFinding(userID, parseAs, target, ifaceName, line)
				}
			}
		}
		if err := sc.Err(); err != nil && err != io.EOF {
			emitLog(fmt.Sprintf("[%s:stdout] collector scanner error: %v", strings.ToUpper(parseAs), err))
		}
	}()

	// ── Stderr drain ──────────────────────────────────────────────────────────
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			line := sc.Text()
			if strings.TrimSpace(line) != "" {
				emitLog(fmt.Sprintf("[%s:stderr] %s", strings.ToUpper(parseAs), line))
			}
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			_ = killProcessGroup(cmd.Process.Pid)
			return targets, ctx.Err()
		}
		// Non-zero exits are acceptable for recon tools with no results
		emitLog(fmt.Sprintf("[%s:warn] collector exited non-zero: %v", strings.ToUpper(parseAs), err))
	}
	return targets, nil
}

// ── GormPersister ─────────────────────────────────────────────────────────────

// GormPersister implements ResultPersister using the models package and GORM/SQLite.
type GormPersister struct{}

// PersistFinding parses a raw JSONL line from any PD tool and writes it to the
// vantage_findings table. Enriches the Finding with severity, template ID, and name
// extracted from the nested JSON structure specific to each tool.
func (p *GormPersister) PersistFinding(userID int64, toolName, scanTarget, ifaceName, line string) error {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return fmt.Errorf("PersistFinding: malformed JSON from %s: %w", toolName, err)
	}

	// Resolve target from tool-specific JSON field
	tool, toolFound := DefaultRegistry.Get(toolName)
	var target string
	if toolFound {
		target = tool.ExtractTarget(obj)
	}
	if target == "" {
		target = extractTarget(toolName, obj)
	}
	if target == "" {
		target = scanTarget
	}

	// Severity — nuclei provides it; other tools default to "info"
	severity := extractString(obj, "severity")
	if severity == "" {
		// Nuclei nests severity inside "info" block
		if info, ok := obj["info"].(map[string]interface{}); ok {
			severity = extractString(info, "severity")
		}
	}
	if severity == "" {
		if strings.EqualFold(toolName, "nuclei") {
			severity = "medium"
		} else {
			severity = "info"
		}
	}

	// Template ID — nuclei-specific
	templateID := extractString(obj, "template-id")
	if templateID == "" {
		templateID = extractString(obj, "template")
	}

	// Finding name — nuclei nests it; other tools use tool name
	name := ""
	if info, ok := obj["info"].(map[string]interface{}); ok {
		name = extractString(info, "name")
	}
	if name == "" {
		name = extractString(obj, "name")
	}
	if name == "" {
		name = strings.ToUpper(toolName) + " finding"
	}

	err := models.UpsertFindingFromTool(userID, toolName, severity, name, target, line, templateID, ifaceName)
	if err == nil {
		notifier.SendAlert(toolName, severity, name, target)
	}

	// If this is a discovery tool, also upsert the target into vantage_targets
	switch strings.ToLower(toolName) {
	case "subfinder", "dnsx", "naabu", "uncover":
		_ = models.UpsertDiscoveredTarget(userID, target, toolName)
	}

	return err
}

// PersistDiscoveredTarget stores a newly found asset into vantage_targets.
func (p *GormPersister) PersistDiscoveredTarget(userID int64, target, source string) error {
	return models.UpsertDiscoveredTarget(userID, target, source)
}
