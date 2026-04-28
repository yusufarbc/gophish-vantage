package scanner

import (
	"fmt"
	"strings"

	"github.com/gophish/gophish/models"
)

// ── Phase 1a: OSINT — Passive Subdomain Enumeration ──────────────────────────
type SubfinderTool struct{}

func (t *SubfinderTool) Name() string { return "subfinder" }
func (t *SubfinderTool) BuildArgs(target, ifaceName string, opts models.ScanOptions) []string {
	args := []string{"subfinder", "-d", target, "-json", "-silent"}
	if opts.SubfinderActive {
		args = append(args, "-active")
	} else {
		args = append(args, "-all") // Use all passive sources
	}
	return args
}
func (t *SubfinderTool) ExtractTarget(obj map[string]interface{}) string {
	if host, ok := obj["host"].(string); ok && host != "" {
		return host
	}
	return ""
}
func (t *SubfinderTool) SupportsInterface() bool { return false }
func (t *SubfinderTool) IsJSONLOutput() bool     { return true }

// ── Phase 1b: OSINT — DNS Resolution ──────────────────────────────────────────
type DNSxTool struct{}

func (t *DNSxTool) Name() string { return "dnsx" }
func (t *DNSxTool) BuildArgs(target, ifaceName string, opts models.ScanOptions) []string {
	return []string{"dnsx", "-d", target, "-json", "-silent", "-wd", target}
}
func (t *DNSxTool) ExtractTarget(obj map[string]interface{}) string {
	if host, ok := obj["host"].(string); ok && host != "" {
		return host
	}
	return ""
}
func (t *DNSxTool) SupportsInterface() bool { return false }
func (t *DNSxTool) IsJSONLOutput() bool     { return true }

// ── Phase 2: Network & Port Scanning ─────────────────────────────────────────
type NaabuTool struct{}

func (t *NaabuTool) Name() string { return "naabu" }
func (t *NaabuTool) BuildArgs(target, ifaceName string, opts models.ScanOptions) []string {
	args := []string{"naabu", "-host", target, "-json", "-silent"}
	
	switch opts.NaabuPorts {
	case "top100":
		args = append(args, "-top-ports", "100")
	case "full":
		args = append(args, "-p", "-")
	default:
		args = append(args, "-top-ports", "1000")
	}

	if ifaceName != "" {
		args = append(args, "-interface", ifaceName)
	}
	return args
}
func (t *NaabuTool) ExtractTarget(obj map[string]interface{}) string {
	host, hok := obj["host"].(string)
	if !hok || host == "" {
		return ""
	}
	if port, pok := obj["port"].(float64); pok && port > 0 {
		return fmt.Sprintf("%s:%d", host, int(port))
	}
	return host
}
func (t *NaabuTool) SupportsInterface() bool { return true }
func (t *NaabuTool) IsJSONLOutput() bool     { return true }

// ── Phase 3a: Surface Mapping — HTTP Probing ──────────────────────────────────
type HttpxTool struct{}

func (t *HttpxTool) Name() string { return "httpx" }
func (t *HttpxTool) BuildArgs(target, ifaceName string, opts models.ScanOptions) []string {
	args := []string{"httpx", "-u", target, "-json", "-silent", "-status-code"}
	if opts.HttpxTech {
		args = append(args, "-tech-detect")
	}
	return args
}
func (t *HttpxTool) ExtractTarget(obj map[string]interface{}) string {
	if url, ok := obj["url"].(string); ok && url != "" {
		return url
	}
	return ""
}
func (t *HttpxTool) SupportsInterface() bool { return false }
func (t *HttpxTool) IsJSONLOutput() bool     { return true }

// ── Phase 3b: TLS/SSL Analysis ─────────────────────────────────────────────
type TLSxTool struct{}

func (t *TLSxTool) Name() string { return "tlsx" }
func (t *TLSxTool) BuildArgs(target, ifaceName string, opts models.ScanOptions) []string {
	return []string{"tlsx", "-u", target, "-json", "-silent", "-san"}
}
func (t *TLSxTool) ExtractTarget(obj map[string]interface{}) string {
	if host, ok := obj["host"].(string); ok && host != "" {
		return host
	}
	return ""
}
func (t *TLSxTool) SupportsInterface() bool { return false }
func (t *TLSxTool) IsJSONLOutput() bool     { return true }

// ── Phase 4: Crawling & Spidering ────────────────────────────────────────────
type KatanaTool struct{}

func (t *KatanaTool) Name() string { return "katana" }
func (t *KatanaTool) BuildArgs(target, ifaceName string, opts models.ScanOptions) []string {
	depth := 3
	if opts.KatanaDepth > 0 {
		depth = opts.KatanaDepth
	}
	return []string{"katana", "-u", target, "-json", "-silent", "-jc", "-d", fmt.Sprintf("%d", depth)}
}
func (t *KatanaTool) ExtractTarget(obj map[string]interface{}) string {
	if req, ok := obj["request"].(map[string]interface{}); ok {
		if url, ok := req["url"].(string); ok && url != "" {
			return url
		}
	}
	if url, ok := obj["url"].(string); ok && url != "" {
		return url
	}
	return ""
}
func (t *KatanaTool) SupportsInterface() bool { return false }
func (t *KatanaTool) IsJSONLOutput() bool     { return true }

// ── Phase 5: Vulnerability Scanning ──────────────────────────────────────────
type NucleiTool struct{}

func (t *NucleiTool) Name() string { return "nuclei" }
func (t *NucleiTool) BuildArgs(target, ifaceName string, opts models.ScanOptions) []string {
	args := []string{"nuclei", "-u", target, "-json", "-silent"}
	
	if len(opts.NucleiTags) > 0 {
		args = append(args, "-tags", strings.Join(opts.NucleiTags, ","))
	}
	
	if len(opts.NucleiSeverities) > 0 {
		args = append(args, "-severity", strings.Join(opts.NucleiSeverities, ","))
	} else {
		args = append(args, "-severity", "critical,high,medium,low,info")
	}
	
	return args
}
func (t *NucleiTool) ExtractTarget(obj map[string]interface{}) string {
	if matched, ok := obj["matched-at"].(string); ok && matched != "" {
		return matched
	}
	if host, ok := obj["host"].(string); ok && host != "" {
		return host
	}
	return ""
}
func (t *NucleiTool) SupportsInterface() bool { return false }
func (t *NucleiTool) IsJSONLOutput() bool     { return true }

// ── Phase 6: OSINT & Other Tools ─────────────────────────────────────────────

type UncoverTool struct{}
func (t *UncoverTool) Name() string { return "uncover" }
func (t *UncoverTool) BuildArgs(target, ifaceName string, opts models.ScanOptions) []string {
	return []string{"uncover", "-q", target, "-json", "-silent"}
}
func (t *UncoverTool) ExtractTarget(obj map[string]interface{}) string {
	if ip, ok := obj["ip"].(string); ok && ip != "" { return ip }
	return ""
}
func (t *UncoverTool) SupportsInterface() bool { return false }
func (t *UncoverTool) IsJSONLOutput() bool     { return true }

type CloudlistTool struct{}
func (t *CloudlistTool) Name() string { return "cloudlist" }
func (t *CloudlistTool) BuildArgs(target, ifaceName string, opts models.ScanOptions) []string {
	return []string{"cloudlist", "-json", "-silent"}
}
func (t *CloudlistTool) ExtractTarget(obj map[string]interface{}) string {
	if artifact, ok := obj["artifact"].(string); ok && artifact != "" { return artifact }
	return ""
}
func (t *CloudlistTool) SupportsInterface() bool { return false }
func (t *CloudlistTool) IsJSONLOutput() bool     { return true }

type ASNMapTool struct{}
func (t *ASNMapTool) Name() string { return "asnmap" }
func (t *ASNMapTool) BuildArgs(target, ifaceName string, opts models.ScanOptions) []string {
	return []string{"asnmap", "-a", target, "-json", "-silent"}
}
func (t *ASNMapTool) ExtractTarget(obj map[string]interface{}) string {
	if ip, ok := obj["ip"].(string); ok && ip != "" { return ip }
	return ""
}
func (t *ASNMapTool) SupportsInterface() bool { return false }
func (t *ASNMapTool) IsJSONLOutput() bool     { return true }

