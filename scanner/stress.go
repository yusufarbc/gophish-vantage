package scanner

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gophish/gophish/pkg/network"
)

// StressRequest defines infrastructure stress-test parameters.
type StressRequest struct {
	Tool      string `json:"tool"`
	TargetURL string `json:"target_url"`
	Duration  string `json:"duration"`
	Rate      int    `json:"rate"`
	Interface string `json:"interface"`
}

func RunStressTest(req StressRequest) error {
	tool := strings.ToLower(strings.TrimSpace(req.Tool))
	if tool == "" {
		tool = "vegeta"
	}
	if req.TargetURL == "" {
		return fmt.Errorf("target_url is required")
	}
	if req.Duration == "" {
		req.Duration = "60s"
	}
	if req.Rate <= 0 {
		req.Rate = 100
	}
	if req.Interface != "" {
		ifaces, err := network.ListInterfaces()
		if err != nil {
			return err
		}
		active := false
		for _, itf := range ifaces {
			if itf.Name == req.Interface && itf.IsUp {
				active = true
				break
			}
		}
		if !active {
			return fmt.Errorf("interface %s is not active", req.Interface)
		}
		host := stressTargetHost(req.TargetURL)
		if host != "" {
			if ip := resolveStressHostIP(host); ip != "" {
				if err := network.VerifyRoute(ip, req.Interface); err != nil {
					return fmt.Errorf("no route to stress target via %s: %w", req.Interface, err)
				}
			}
		}
	}

	if err := scanState.AcquireLock("stress-"+tool, req.TargetURL); err != nil {
		return err
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				emitLog(fmt.Sprintf("[FATAL] Stress test panic: %v", r))
			}
			scanState.ReleaseLock()
		}()
		emitLog(fmt.Sprintf("[RESILIENCE] ▶ Stress test started tool=%s target=%s duration=%s rate=%d iface=%s", tool, req.TargetURL, req.Duration, req.Rate, req.Interface))

		ctx := context.Background()
		cmd := buildStressCommand(ctx, tool, req)
		if cmd == nil {
			emitLog("[RESILIENCE] unsupported stress tool")
			return
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			emitLog(fmt.Sprintf("[RESILIENCE] stdout pipe error: %v", err))
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			emitLog(fmt.Sprintf("[RESILIENCE] stderr pipe error: %v", err))
			return
		}
		if err := cmd.Start(); err != nil {
			emitLog(fmt.Sprintf("[RESILIENCE] start error: %v", err))
			return
		}

		var wg sync.WaitGroup
		start := time.Now()
		wg.Add(2)
		go func() {
			defer wg.Done()
			s := bufio.NewScanner(stdout)
			for s.Scan() {
				line := s.Text()
				emitLog("[STRESS] " + line)
				emitLatencyFromLine(line, start)
			}
		}()
		go func() {
			defer wg.Done()
			s := bufio.NewScanner(stderr)
			for s.Scan() {
				line := s.Text()
				emitLog("[STRESS:stderr] " + line)
				emitLatencyFromLine(line, start)
			}
		}()
		wg.Wait()
		if err := cmd.Wait(); err != nil {
			emitLog(fmt.Sprintf("[RESILIENCE] stress test ended with error: %v", err))
		} else {
			emitLog("[RESILIENCE] ✔ Stress test completed")
		}
	}()

	return nil
}

func stressTargetHost(targetURL string) string {
	u, err := url.Parse(strings.TrimSpace(targetURL))
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func resolveStressHostIP(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return ""
	}
	return ips[0].String()
}

func emitLatencyFromLine(line string, start time.Time) {
	lower := strings.ToLower(line)
	if !strings.Contains(lower, "latency") && !strings.Contains(lower, "avg") {
		return
	}
	num := firstNumericToken(lower)
	if num == "" {
		return
	}
	emitLog(fmt.Sprintf("[STRESS_METRIC] {\"t\":%d,\"latency_ms\":%s}", int(time.Since(start).Seconds()), num))
}

func firstNumericToken(v string) string {
	for _, token := range strings.Fields(v) {
		t := strings.Trim(token, ",:msµs[]{}")
		if _, err := strconv.ParseFloat(t, 64); err == nil {
			return t
		}
	}
	return ""
}

func buildStressCommand(ctx context.Context, tool string, req StressRequest) *exec.Cmd {
	var cmd *exec.Cmd
	switch tool {
	case "bombardier":
		cmd = exec.CommandContext(ctx, "bombardier", "-d", req.Duration, "-r", strconv.Itoa(req.Rate), "-c", "32", req.TargetURL)
	case "hey":
		// Approximate by requests = rate * durationSeconds
		durationSeconds := 60
		if d, err := time.ParseDuration(req.Duration); err == nil {
			durationSeconds = int(d.Seconds())
		}
		requests := req.Rate * durationSeconds
		cmd = exec.CommandContext(ctx, "hey", "-n", strconv.Itoa(requests), "-c", "32", req.TargetURL)
	default:
		// vegeta wrapper through shell to keep attack/report pipeline simple.
		bashCmd := fmt.Sprintf("echo 'GET %s' | vegeta attack -duration=%s -rate=%d | vegeta report", req.TargetURL, req.Duration, req.Rate)
		cmd = exec.CommandContext(ctx, "bash", "-lc", bashCmd)
	}

	if cmd != nil {
		path, err := exec.LookPath(cmd.Args[0])
		if err != nil {
			emitLog(fmt.Sprintf("[ERROR] stress tool not found in PATH: %s", cmd.Args[0]))
			return nil
		}
		cmd.Path = path
	}
	return cmd
}
