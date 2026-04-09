package scanner

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ToolExecConfig configures timeout for tool execution
type ToolExecConfig struct {
	Timeout time.Duration
}

// DefaultToolConfigs defines timeout for each tool
var DefaultToolConfigs = map[string]ToolExecConfig{
	"subfinder":  {Timeout: 5 * time.Minute},
	"httpx":      {Timeout: 10 * time.Minute},
	"nuclei":     {Timeout: 30 * time.Minute},
	"naabu":      {Timeout: 15 * time.Minute},
	"dnsx":       {Timeout: 5 * time.Minute},
	"katana":     {Timeout: 10 * time.Minute},
	"tlsx":       {Timeout: 5 * time.Minute},
	"asnmap":     {Timeout: 3 * time.Minute},
	"uncover":    {Timeout: 5 * time.Minute},
	"interactsh": {Timeout: 5 * time.Minute},
	"chisel":     {Timeout: 300 * time.Second},
}

// ExecuteTool runs a tool with timeout context
func ExecuteTool(toolName string, args []string) (string, error) {
	cfg, exists := DefaultToolConfigs[toolName]
	if !exists {
		cfg = ToolExecConfig{Timeout: 5 * time.Minute}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, toolName, args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("[TIMEOUT] %s exceeded %v timeout\n", toolName, cfg.Timeout)
			return "", fmt.Errorf("timeout after %v", cfg.Timeout)
		}
		log.Printf("[ERROR] %s execution failed: %v\n", toolName, err)
		return "", fmt.Errorf("execution failed: %w", err)
	}

	log.Printf("[OK] %s completed\n", toolName)
	return string(output), nil
}

// KillProcessTree kills a process and its children
func KillProcessTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	if runtime.GOOS == "windows" {
		killCmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		if err := killCmd.Run(); err != nil {
			log.Printf("[WARN] Failed to kill process tree: %v\n", err)
		}
		return
	}

	// Unix: use killpg
	if cmd.ProcessState != nil && !cmd.ProcessState.Exited() {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		time.Sleep(500 * time.Millisecond)
		if !cmd.ProcessState.Exited() {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		log.Printf("[OK] Process tree killed\n")
	}
}

// GetToolTimeout retrieves timeout from environment or uses default
func GetToolTimeout(tool string) time.Duration {
	timeoutStr := os.Getenv(strings.ToUpper(tool) + "_TIMEOUT")
	if timeoutStr != "" {
		if d, err := time.ParseDuration(timeoutStr); err == nil {
			return d
		}
	}

	if cfg, exists := DefaultToolConfigs[tool]; exists {
		return cfg.Timeout
	}

	return 5 * time.Minute
}
