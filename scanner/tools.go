package scanner

import "fmt"

// ── Phase 1a: OSINT — Passive Subdomain Enumeration ──────────────────────────
// Subfinder only does PASSIVE enumeration. No active DNS probing.

type SubfinderTool struct{}

func (t *SubfinderTool) Name() string { return "subfinder" }
func (t *SubfinderTool) BuildArgs(target, ifaceName string, extra []string) []string {
	// -d: domain, -json: JSONL output, -silent: no banner, -all: use all sources
	args := []string{"subfinder", "-d", target, "-json", "-silent", "-all"}
	return append(args, extra...)
}
func (t *SubfinderTool) ExtractTarget(obj map[string]interface{}) string {
	// Subfinder outputs: {"host":"sub.example.com","source":"crtsh",...}
	if host, ok := obj["host"].(string); ok && host != "" {
		return host
	}
	return ""
}
func (t *SubfinderTool) SupportsInterface() bool { return false }
func (t *SubfinderTool) IsJSONLOutput() bool     { return true }

// ── Phase 1b: OSINT — DNS Resolution & Wildcard Filtering ────────────────────
// DNSx resolves discovered subdomains and filters wildcards to remove noise.

type DNSxTool struct{}

func (t *DNSxTool) Name() string { return "dnsx" }
func (t *DNSxTool) BuildArgs(target, ifaceName string, extra []string) []string {
	// -d: domain, -json: JSONL, -silent: no banner, -wd: wildcard filtering
	// NOTE: In pipeline mode, target may be a file passed via -l instead of -d.
	args := []string{"dnsx", "-d", target, "-json", "-silent", "-wd", target}
	return append(args, extra...)
}
func (t *DNSxTool) ExtractTarget(obj map[string]interface{}) string {
	// DNSx outputs: {"host":"sub.example.com","resolver":["8.8.8.8:53"],...}
	if host, ok := obj["host"].(string); ok && host != "" {
		return host
	}
	return ""
}
func (t *DNSxTool) SupportsInterface() bool { return false }
func (t *DNSxTool) IsJSONLOutput() bool     { return true }

// ── Phase 2: Network & Port Scanning ─────────────────────────────────────────
// Naabu supports L3 interface routing (e.g., tun0 for Chisel tunnels).

type NaabuTool struct{}

func (t *NaabuTool) Name() string { return "naabu" }
func (t *NaabuTool) BuildArgs(target, ifaceName string, extra []string) []string {
	// -host: target, -json: JSONL, -silent: no banner, -top-ports: common ports
	args := []string{"naabu", "-host", target, "-json", "-silent", "-top-ports", "1000"}
	if ifaceName != "" {
		// -interface flag routes outbound packets through the specified interface (e.g., tun0)
		args = append(args, "-interface", ifaceName)
	}
	return append(args, extra...)
}
func (t *NaabuTool) ExtractTarget(obj map[string]interface{}) string {
	// Naabu outputs: {"host":"1.2.3.4","port":443,...}
	// Return host:port so httpx can receive a fully-qualified probe target.
	host, hok := obj["host"].(string)
	if !hok || host == "" {
		return ""
	}
	if port, pok := obj["port"].(float64); pok && port > 0 {
		return fmt.Sprintf("%s:%d", host, int(port))
	}
	return host
}
func (t *NaabuTool) SupportsInterface() bool { return true }  // L3 tun0 support
func (t *NaabuTool) IsJSONLOutput() bool     { return true }

// ── Phase 3a: Surface Mapping — HTTP Probing ──────────────────────────────────
// Httpx probes live HTTP/HTTPS services on the discovered host:port targets.

type HttpxTool struct{}

func (t *HttpxTool) Name() string { return "httpx" }
func (t *HttpxTool) BuildArgs(target, ifaceName string, extra []string) []string {
	// -u: single target url, -json: JSONL, -silent: no banner
	// -tech-detect: fingerprint tech stack, -status-code: include status codes
	args := []string{"httpx", "-u", target, "-json", "-silent", "-tech-detect", "-status-code"}
	return append(args, extra...)
}
func (t *HttpxTool) ExtractTarget(obj map[string]interface{}) string {
	// Httpx outputs: {"url":"https://sub.example.com","status_code":200,...}
	if url, ok := obj["url"].(string); ok && url != "" {
		return url
	}
	return ""
}
func (t *HttpxTool) SupportsInterface() bool { return false }
func (t *HttpxTool) IsJSONLOutput() bool     { return true }

// ── Phase 3b: Surface Mapping — TLS/SSL Analysis ─────────────────────────────
// TLSx analyzes TLS certificates for each live HTTPS host.

type TLSxTool struct{}

func (t *TLSxTool) Name() string { return "tlsx" }
func (t *TLSxTool) BuildArgs(target, ifaceName string, extra []string) []string {
	// -u: target, -json: JSONL, -silent: no banner, -san: extract Subject Alternative Names
	args := []string{"tlsx", "-u", target, "-json", "-silent", "-san"}
	return append(args, extra...)
}
func (t *TLSxTool) ExtractTarget(obj map[string]interface{}) string {
	// TLSx outputs: {"host":"sub.example.com","port":"443","subject_cn":"...",...}
	if host, ok := obj["host"].(string); ok && host != "" {
		return host
	}
	return ""
}
func (t *TLSxTool) SupportsInterface() bool { return false }
func (t *TLSxTool) IsJSONLOutput() bool     { return true }

// ── Phase 4: Crawling & Spidering ────────────────────────────────────────────
// Katana crawls live HTTP services to discover endpoints and JS files.

type KatanaTool struct{}

func (t *KatanaTool) Name() string { return "katana" }
func (t *KatanaTool) BuildArgs(target, ifaceName string, extra []string) []string {
	// -u: target URL, -json: JSONL, -silent: no banner, -jc: parse JS, -d: depth 3
	args := []string{"katana", "-u", target, "-json", "-silent", "-jc", "-d", "3"}
	return append(args, extra...)
}
func (t *KatanaTool) ExtractTarget(obj map[string]interface{}) string {
	// Katana outputs nested: {"request":{"url":"..."},"response":{"status_code":200},...}
	if req, ok := obj["request"].(map[string]interface{}); ok {
		if url, ok := req["url"].(string); ok && url != "" {
			return url
		}
	}
	// Fallback to top-level url if present
	if url, ok := obj["url"].(string); ok && url != "" {
		return url
	}
	return ""
}
func (t *KatanaTool) SupportsInterface() bool { return false }
func (t *KatanaTool) IsJSONLOutput() bool     { return true }

// ── Phase 5: Vulnerability Scanning ──────────────────────────────────────────
// Nuclei scans with templates. Outputs MASSIVE JSONL files — OOM protection
// via bufio.Scanner with a 1MB line buffer is MANDATORY in the executor.

type NucleiTool struct{}

func (t *NucleiTool) Name() string { return "nuclei" }
func (t *NucleiTool) BuildArgs(target, ifaceName string, extra []string) []string {
	// -u: single target, -json: JSONL, -silent: no banner, -severity: all levels
	// WARNING: Do NOT use -o flag; stream stdout directly to avoid huge temp files.
	args := []string{"nuclei", "-u", target, "-json", "-silent", "-severity", "critical,high,medium,low,info"}
	return append(args, extra...)
}
func (t *NucleiTool) ExtractTarget(obj map[string]interface{}) string {
	// Nuclei outputs: {"matched-at":"https://host/path","info":{"severity":"high"},...}
	if matched, ok := obj["matched-at"].(string); ok && matched != "" {
		return matched
	}
	// Fallback to host field
	if host, ok := obj["host"].(string); ok && host != "" {
		return host
	}
	return ""
}
func (t *NucleiTool) SupportsInterface() bool { return false }
// IsJSONLOutput returns true — WARNING: Nuclei can emit 100k+ lines per scan.
// The executor MUST use bufio.Scanner (1MB buffer), never json.NewDecoder here.
func (t *NucleiTool) IsJSONLOutput() bool { return true }
