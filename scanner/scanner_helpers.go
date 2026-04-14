package scanner

import (
	"fmt"
	"strings"

	"github.com/gophish/gophish/pkg/network"
)

// buildScannerArgs constructs CLI arguments for each ProjectDiscovery tool.
func buildScannerArgs(toolName, target, ifaceName string, extra []string) []string {
	var args []string
	switch toolName {
	case "subfinder":
		args = []string{"subfinder", "-d", target, "-json", "-silent"}
	case "httpx":
		args = []string{"httpx", "-u", target, "-json", "-silent"}
	case "nuclei":
		args = []string{"nuclei", "-u", target, "-json", "-silent"}
	case "naabu":
		args = []string{"naabu", "-host", target, "-json", "-silent"}
		if ifaceName != "" {
			args = append(args, "-interface", ifaceName)
		}
	case "dnsx":
		args = []string{"dnsx", "-d", target, "-json", "-silent"}
	case "katana":
		args = []string{"katana", "-u", target, "-json", "-silent"}
	case "tlsx":
		args = []string{"tlsx", "-u", target, "-json", "-silent"}
	case "uncover":
		args = []string{"uncover", "-q", target, "-json"}
	case "asnmap":
		args = []string{"asnmap", "-a", target, "-json"}
	case "interactsh-client":
		args = []string{"interactsh-client", "-json"}
	case "assetfinder":
		args = []string{"assetfinder", "-subs", target}
	case "cloudlist":
		args = []string{"cloudlist", "-provider", "aws,gcp,azure,digitalocean", "-json"}
	default:
		args = []string{toolName, "-u", target, "-json", "-silent"}
	}
	return append(args, extra...)
}

// extractTarget extracts the target from tool-specific JSON output.
func extractTarget(toolName string, obj map[string]interface{}) string {
	switch toolName {
	case "subfinder":
		if host, ok := obj["host"].(string); ok { return host }
	case "httpx":
		if url, ok := obj["url"].(string); ok { return url }
	case "naabu":
		if host, ok := obj["host"].(string); ok { return host }
	case "dnsx":
		if host, ok := obj["host"].(string); ok { return host }
	case "interactsh-client":
		if fullReq, ok := obj["full_request"].(string); ok && fullReq != "" { return fullReq }
		if data, ok := obj["data"].(string); ok && data != "" { return data }
	case "cloudlist":
		if artifact, ok := obj["artifact"].(string); ok && artifact != "" { return artifact }
	default:
		if host, ok := obj["host"].(string); ok { return host }
		if url, ok := obj["url"].(string); ok { return url }
	}
	return ""
}

// extractString extracts a (possibly nested) string from a map.
func extractString(obj map[string]interface{}, key string) string {
	if strings.Contains(key, ".") {
		parts := strings.Split(key, ".")
		var current interface{} = obj
		for _, part := range parts {
			m, ok := current.(map[string]interface{})
			if !ok { return "" }
			current, ok = m[part]
			if !ok { return "" }
		}
		if s, ok := current.(string); ok { return s }
		return ""
	}
	if v, ok := obj[key].(string); ok { return v }
	return ""
}

// hostToArgs converts []string to httpx/nuclei -u flags.
func hostToArgs(hosts []string) []string {
	var args []string
	for _, h := range hosts {
		if h != "" {
			args = append(args, "-u", h)
		}
	}
	return args
}

// ensureInterfaceForScan validates availability and state of network interfaces.
func ensureInterfaceForScan(toolName, target, ifaceName string) error {
	if ifaceName == "" { return nil }
	ifaces, err := network.ListInterfaces()
	if err != nil { return fmt.Errorf("listing interfaces failed: %w", err) }
	
	foundUp := false
	for _, iface := range ifaces {
		if iface.Name == ifaceName && iface.IsUp {
			foundUp = true
			break
		}
	}
	if !foundUp { return fmt.Errorf("selected interface %s is not active", ifaceName) }
	
	if ifaceName == "tun0" {
		_, _, connected, err := network.GetActiveTUNIP()
		if err != nil { return fmt.Errorf("tun0 verification failed: %w", err) }
		if !connected { return fmt.Errorf("tun0 selected but no reverse tunnel agent is connected") }
	}
	return nil
}
