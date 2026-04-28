package scanner

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/pkg/network"
)

// buildScannerArgs constructs the CLI argument slice for a given PD tool.
// It delegates to the Tool registry when possible so that argument logic
// stays exclusively in tools.go, not duplicated here.
// buildScannerArgs constructs the CLI argument slice for a given PD tool.
// It delegates to the Tool registry when possible so that argument logic
// stays exclusively in tools.go, not duplicated here.
func buildScannerArgs(toolName, target, ifaceName string, opts models.ScanOptions) []string {
	tool, ok := DefaultRegistry.Get(toolName)
	if ok {
		return tool.BuildArgs(target, ifaceName, opts)
	}

	// Fallback for any unlisted tools — generic best-effort invocation
	return []string{toolName, "-u", target, "-json", "-silent"}
}

// deduplicateTargets removes duplicate entries from a string slice.
func deduplicateTargets(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, exists := seen[v]; !exists {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

// targetsToArgs generates a slice of repeated flags for a list of targets.
// Example: targetsToArgs("-u", ["a", "b"]) -> ["-u", "a", "-u", "b"]
func targetsToArgs(flag string, targets []string) []string {
	args := make([]string, 0, len(targets)*2)
	for _, t := range targets {
		if t != "" {
			args = append(args, flag, t)
		}
	}
	return args
}

// hostToArgs is a convenience alias for targetsToArgs("-u", hosts).
func hostToArgs(hosts []string) []string {
	return targetsToArgs("-u", hosts)
}

// targetsToListArgs is a convenience alias for targetsToArgs("-u", hosts).
func targetsToListArgs(hosts []string) []string {
	return targetsToArgs("-u", hosts)
}

// extractTarget extracts the primary target identifier from a tool's JSONL output.
// This is the fallback for unregistered tools. Registered tools should use
// their Tool.ExtractTarget() method via the registry instead.
func extractTarget(toolName string, obj map[string]interface{}) string {
	switch strings.ToLower(toolName) {
	case "subfinder":
		if host, ok := obj["host"].(string); ok && host != "" {
			return host
		}
	case "dnsx":
		if host, ok := obj["host"].(string); ok && host != "" {
			return host
		}
	case "naabu":
		// Return host:port to allow httpx to probe specific open ports
		host, _ := obj["host"].(string)
		if host == "" {
			return ""
		}
		if port, ok := obj["port"].(float64); ok && port > 0 {
			return fmt.Sprintf("%s:%d", host, int(port))
		}
		return host
	case "httpx", "http-probe":
		if url, ok := obj["url"].(string); ok && url != "" {
			return url
		}
	case "tlsx":
		if host, ok := obj["host"].(string); ok && host != "" {
			return host
		}
	case "katana":
		if req, ok := obj["request"].(map[string]interface{}); ok {
			if url, ok := req["url"].(string); ok && url != "" {
				return url
			}
		}
		if url, ok := obj["url"].(string); ok && url != "" {
			return url
		}
	case "nuclei":
		if matched, ok := obj["matched-at"].(string); ok && matched != "" {
			return matched
		}
		if host, ok := obj["host"].(string); ok && host != "" {
			return host
		}
	case "uncover":
		if ip, ok := obj["ip"].(string); ok && ip != "" {
			return ip
		}
	case "interactsh-client":
		if fullReq, ok := obj["full_request"].(string); ok && fullReq != "" {
			return fullReq
		}
		if data, ok := obj["data"].(string); ok && data != "" {
			return data
		}
	case "cloudlist":
		if artifact, ok := obj["artifact"].(string); ok && artifact != "" {
			return artifact
		}
	}

	// Generic fallback — try common target field names in order of likelihood
	for _, key := range []string{"host", "url", "target", "ip", "domain"} {
		if val, ok := obj[key].(string); ok && val != "" {
			return val
		}
	}
	return ""
}

// extractString extracts a string value from a map with optional dot-notation nesting.
// Example: extractString(obj, "info.severity") navigates obj["info"]["severity"].
func extractString(obj map[string]interface{}, key string) string {
	if strings.Contains(key, ".") {
		parts := strings.Split(key, ".")
		var current interface{} = obj
		for _, part := range parts {
			m, ok := current.(map[string]interface{})
			if !ok {
				return ""
			}
			current, ok = m[part]
			if !ok {
				return ""
			}
		}
		if s, ok := current.(string); ok {
			return s
		}
		return ""
	}
	if v, ok := obj[key].(string); ok {
		return v
	}
	return ""
}

// ensureInterfaceForScan validates that the requested network interface is active.
// For tun0, it additionally confirms that a Chisel reverse tunnel agent is connected
// and autonomously adds routes for CIDR targets to ensure traffic flows through the tunnel.
func ensureInterfaceForScan(toolName, target, ifaceName string) error {
	if ifaceName == "" {
		return nil // No interface specified — use system default routing
	}

	ifaces, err := network.ListInterfaces()
	if err != nil {
		return fmt.Errorf("cannot list network interfaces: %w", err)
	}

	foundUp := false
	for _, iface := range ifaces {
		if iface.Name == ifaceName && iface.IsUp {
			foundUp = true
			break
		}
	}
	if !foundUp {
		return fmt.Errorf("selected interface %q is not active or does not exist", ifaceName)
	}

	// Extra validation & autonomous routing for tun0 (Chisel L3 tunnel)
	if ifaceName == "tun0" {
		_, _, connected, err := network.GetActiveTUNIP()
		if err != nil {
			return fmt.Errorf("tun0 verification failed: %w", err)
		}
		if !connected {
			return fmt.Errorf("tun0 selected but no active Chisel reverse tunnel agent is connected")
		}

		// Autonomous Route Injection: If target is a CIDR, ensure the route exists
		// Note: We ignore errors from 'ip route add' because it might already exist
		if strings.Contains(target, "/") {
			subnets := strings.Split(target, "\n")
			for _, s := range subnets {
				s = strings.TrimSpace(s)
				if strings.Contains(s, "/") {
					_ = exec.Command("ip", "route", "add", s, "dev", "tun0").Run()
				}
			}
		}
	}
	return nil
}
