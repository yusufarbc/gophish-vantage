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

// DefaultExecutor implements ToolExecutor with hardening and streaming JSON support.
type DefaultExecutor struct {
	Persister ResultPersister
}

func (e *DefaultExecutor) Execute(ctx context.Context, userID int64, toolName, target, ifaceName string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no arguments provided")
	}

	emitLog(fmt.Sprintf("[CMD] %s", strings.Join(args, " ")))

	toolPath, err := exec.LookPath(args[0])
	if err != nil {
		emitLog(fmt.Sprintf("[ERROR] tool not found in PATH: %s", args[0]))
		return err
	}

	cmd := exec.CommandContext(ctx, toolPath, args[1:]...)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", toolName, err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Stream stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for large JSON lines
		for scanner.Scan() {
			line := scanner.Text()
			emitLog(fmt.Sprintf("[%s] %s", strings.ToUpper(toolName), line))
			if e.Persister != nil {
				_ = e.Persister.PersistFinding(userID, toolName, target, ifaceName, line)
			}
		}
	}()

	// Stream stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			emitLog(fmt.Sprintf("[%s:stderr] %s", strings.ToUpper(toolName), scanner.Text()))
		}
	}()

	wg.Wait()
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			_ = killProcessGroup(cmd.Process.Pid)
			return ctx.Err()
		}
		return err
	}
	return nil
}

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
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		decoder := json.NewDecoder(stdout)
		for {
			var raw json.RawMessage
			if err := decoder.Decode(&raw); err != nil {
				if err == io.EOF { break }
				continue
			}
			line := string(raw)
			emitLog(fmt.Sprintf("[%s] %s", strings.ToUpper(parseAs), line))

			var obj map[string]interface{}
			if err := json.Unmarshal(raw, &obj); err == nil {
				if t := extractTarget(parseAs, obj); t != "" {
					mu.Lock()
					targets = append(targets, t)
					mu.Unlock()
					if e.Persister != nil {
						_ = e.Persister.PersistDiscoveredTarget(userID, t, parseAs)
						_ = e.Persister.PersistFinding(userID, parseAs, target, ifaceName, line)
					}
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			emitLog(fmt.Sprintf("[%s:stderr] %s", strings.ToUpper(parseAs), scanner.Text()))
		}
	}()

	wg.Wait()
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			_ = killProcessGroup(cmd.Process.Pid)
			return targets, ctx.Err()
		}
	}
	return targets, nil
}

// GormPersister implements ResultPersister using models package.
type GormPersister struct{}

func (p *GormPersister) PersistFinding(userID int64, toolName, scanTarget, ifaceName, line string) error {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return err
	}
	target := extractTarget(toolName, obj)
	if target == "" { target = scanTarget }

	severity := extractString(obj, "severity")
	templateID := extractString(obj, "template-id")
	if templateID == "" { templateID = extractString(obj, "template") }
	
	name := extractString(obj, "info.name")
	if name == "" { name = strings.ToUpper(toolName) }

	if strings.EqualFold(toolName, "nuclei") && severity == "" {
		severity = "medium"
	} else if severity == "" {
		severity = "info"
	}

	err := models.UpsertFindingFromTool(userID, toolName, severity, name, target, line, templateID, ifaceName)
	if err == nil {
		notifier.SendAlert(toolName, severity, name, target)
	}
	
	// If it's discovery tool, also upsert as target
	if strings.EqualFold(toolName, "subfinder") || strings.EqualFold(toolName, "uncover") {
		_ = models.UpsertDiscoveredTarget(userID, target, toolName)
	}
	return err
}

func (p *GormPersister) PersistDiscoveredTarget(userID int64, target, source string) error {
	return models.UpsertDiscoveredTarget(userID, target, source)
}
