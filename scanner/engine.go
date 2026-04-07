package scanner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gophish/gophish/pkg/network"
	"github.com/gorilla/websocket"
)

// ── WebSocket Support for Live Scanner Logs ────────────────────────────────────

// ScannerLogHub manages WebSocket connections for streaming live scan logs to the UI.
type ScannerLogHub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan string
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

var logHub *ScannerLogHub

// InitScannerHub initializes the WebSocket hub for scan logs.
func InitScannerHub() {
	logHub = &ScannerLogHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan string, 512),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
	go logHub.run()
}

// run is the main loop that distributes messages to all connected clients.
func (h *ScannerLogHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[scanner] ws client connected (%d total)", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			delete(h.clients, client)
			h.mu.Unlock()
			client.Close()
			log.Printf("[scanner] ws client disconnected (%d remaining)", len(h.clients))

		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				client.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := client.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
					h.mu.RUnlock()
					go func(c *websocket.Conn) {
						h.unregister <- c
					}(client)
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

// ScannerWSHandler handles incoming WebSocket connections for live scan logs.
func ScannerWSHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}
	logHub.register <- conn

	// Keep connection alive by reading frames (not used, just prevent disconnect)
	go func() {
		defer func() { logHub.unregister <- conn }()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

// ── Scanner Engine State ──────────────────────────────────────────────────────

// ScanState tracks active scanning operations.
type ScanState struct {
	Running bool
	Tool    string
	Target  string
	Started time.Time
	mu      sync.RWMutex
}

var scanState = &ScanState{}

// GetScanState returns the global scan state tracker.
func GetScanState() *ScanState {
	return scanState
}

// IsScanRunning returns true if a scan is currently in progress.
func (s *ScanState) IsScanRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Running
}

// AcquireLock attempts to start a scan. Returns an error if already running.
func (s *ScanState) AcquireLock(tool, target string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Running {
		return fmt.Errorf("scan already in progress (tool: %s, target: %s)", s.Tool, s.Target)
	}
	s.Running = true
	s.Tool = tool
	s.Target = target
	s.Started = time.Now()
	return nil
}

// ReleaseLock marks the scan as complete.
func (s *ScanState) ReleaseLock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Running = false
}

// Status returns a copy of the current scan state.
func (s *ScanState) Status() (bool, string, string, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Running, s.Tool, s.Target, s.Started
}

// ── Scanner Engine ───────────────────────────────────────────────────────────

// emitLog broadcasts a log message to all WebSocket clients and console.
func emitLog(msg string) {
	log.Println(msg)
	if logHub != nil {
		select {
		case logHub.broadcast <- msg:
		default:
			// Channel full; drop to prevent blocking
		}
	}
}

// RunScannerTool executes a single ProjectDiscovery tool asynchronously.
// ifaceName optionally binds the scan to a specific network interface (e.g. "tun0").
// When ifaceName is non-empty, the interface flag is injected into the tool CLI args
// (supported: naabu). For tools that use the OS routing table (nuclei, httpx), a
// route-existence warning is emitted but execution is not blocked.
func RunScannerTool(toolName, target, ifaceName string, extraFlags []string) error {
	if err := scanState.AcquireLock(toolName, target); err != nil {
		return err
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				emitLog(fmt.Sprintf("[FATAL] Scanner panic in %s: %v", toolName, r))
				log.Printf("[scanner] panic recovered: %v", r)
			}
			scanState.ReleaseLock()
		}()
		if ifaceName != "" {
			verifyRouteBeforeScan(target, ifaceName)
		}
		args := buildScannerArgs(toolName, target, ifaceName, extraFlags)
		emitLog(fmt.Sprintf("[VANTAGE] ▶ Starting %s on target=%s (iface=%s)", strings.ToUpper(toolName), target, ifaceName))
		runAndStreamTool(context.Background(), toolName, args)
		emitLog(fmt.Sprintf("[VANTAGE] ✔ %s finished", strings.ToUpper(toolName)))
	}()

	return nil
}

// RunDiscovery chains: subfinder → httpx → nuclei
// ifaceName optionally routes traffic through a specific interface for naabu.
func RunDiscovery(target, ifaceName string) error {
	if err := scanState.AcquireLock("discovery", target); err != nil {
		return err
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				emitLog(fmt.Sprintf("[FATAL] Discovery chain panic: %v", r))
				log.Printf("[scanner] discovery panic recovered: %v", r)
			}
			scanState.ReleaseLock()
		}()
		emitLog("[VANTAGE] ═══ DISCOVERY MODE ══════════════════════════")

		// Phase 1: Subdomain enumeration
		emitLog("[VANTAGE] Phase 1 — Subdomain Enumeration (subfinder)")
		subdomains := collectTargets(context.Background(), "subfinder",
			[]string{"subfinder", "-d", target, "-json", "-silent"})
		if len(subdomains) == 0 {
			subdomains = []string{target}
			emitLog("[VANTAGE] No subdomains found, using original target")
		} else {
			emitLog(fmt.Sprintf("[VANTAGE] ✓ Found %d subdomains", len(subdomains)))
		}

		// Phase 2: HTTP probing
		emitLog("[VANTAGE] Phase 2 — HTTP Service Discovery (httpx)")
		aliveArgs := append([]string{"httpx", "-json", "-silent"}, hostToArgs(subdomains)...)
		alive := collectTargets(context.Background(), "httpx", aliveArgs)
		if len(alive) == 0 {
			alive = subdomains
			emitLog("[VANTAGE] No alive HTTP services found, falling back to subdomains")
		} else {
			emitLog(fmt.Sprintf("[VANTAGE] ✓ Found %d alive hosts", len(alive)))
		}

		// Phase 3: Vulnerability scan
		emitLog(fmt.Sprintf("[VANTAGE] Phase 3 — Vulnerability Scan (nuclei) - %d targets", len(alive)))
		for _, host := range alive {
			host = strings.TrimSpace(host)
			if host != "" {
				args := []string{"nuclei", "-u", host, "-json", "-silent"}
				runAndStreamTool(context.Background(), "nuclei", args)
			}
		}

		emitLog("[VANTAGE] ═══ DISCOVERY COMPLETE ════════════════════════")
	}()

	return nil
}

// ── Internal Helpers ───────────────────────────────────────────────────────────

// runAndStreamTool executes a command and streams output to WebSocket + logs.
func runAndStreamTool(ctx context.Context, toolName string, args []string) {
	if len(args) == 0 {
		return
	}

	emitLog(fmt.Sprintf("[CMD] %s", strings.Join(args, " ")))
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		emitLog(fmt.Sprintf("[ERROR] stdout pipe: %v", err))
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		emitLog(fmt.Sprintf("[ERROR] stderr pipe: %v", err))
		return
	}

	if err := cmd.Start(); err != nil {
		emitLog(fmt.Sprintf("[ERROR] start %s: %v", args[0], err))
		return
	}

	var wg sync.WaitGroup

	// Stream stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 64*1024)
		for scanner.Scan() {
			line := scanner.Text()
			emitLog(fmt.Sprintf("[%s] %s", strings.ToUpper(toolName), line))
		}
	}()

	// Stream stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			emitLog(fmt.Sprintf("[%s:stderr] %s", strings.ToUpper(toolName), line))
		}
	}()

	wg.Wait()
	if err := cmd.Wait(); err != nil {
		emitLog(fmt.Sprintf("[WARN] %s exited: %v", args[0], err))
	}
}

// collectTargets runs a command and collects "target" fields from JSON output.
func collectTargets(ctx context.Context, parseAs string, args []string) []string {
	var targets []string
	var mu sync.Mutex

	if len(args) == 0 {
		return targets
	}

	emitLog(fmt.Sprintf("[CMD] %s", strings.Join(args, " ")))
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		emitLog(fmt.Sprintf("[ERROR] %v", err))
		return targets
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		emitLog(fmt.Sprintf("[ERROR] %v", err))
		return targets
	}

	if err := cmd.Start(); err != nil {
		emitLog(fmt.Sprintf("[ERROR] start %s: %v", args[0], err))
		return targets
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 64*1024)
		for scanner.Scan() {
			line := scanner.Text()
			emitLog(fmt.Sprintf("[%s] %s", strings.ToUpper(parseAs), line))

			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(line), &obj); err == nil {
				if target := extractTarget(parseAs, obj); target != "" {
					mu.Lock()
					targets = append(targets, target)
					mu.Unlock()
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			emitLog(fmt.Sprintf("[%s:stderr] %s", strings.ToUpper(parseAs), scanner.Text()))
		}
	}()

	wg.Wait()
	cmd.Wait()
	return targets
}

// extractTarget extracts the target from tool-specific JSON output.
func extractTarget(toolName string, obj map[string]interface{}) string {
	switch toolName {
	case "subfinder":
		if host, ok := obj["host"].(string); ok {
			return host
		}
	case "httpx":
		if url, ok := obj["url"].(string); ok {
			return url
		}
	case "naabu":
		if host, ok := obj["host"].(string); ok {
			return host
		}
	case "dnsx":
		if host, ok := obj["host"].(string); ok {
			return host
		}
	case "interactsh-client":
		// interactsh JSON: { "timestamp", "full_request", "data" }
		if fullReq, ok := obj["full_request"].(string); ok && fullReq != "" {
			return fullReq
		}
		if data, ok := obj["data"].(string); ok && data != "" {
			return data
		}
	case "cloudlist":
		// cloudlist JSON: { "artifact", "tag", "provider" }
		if artifact, ok := obj["artifact"].(string); ok && artifact != "" {
			return artifact
		}
	default:
		if host, ok := obj["host"].(string); ok {
			return host
		}
		if url, ok := obj["url"].(string); ok {
			return url
		}
	}
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

// buildScannerArgs constructs CLI arguments for each ProjectDiscovery tool.
// ifaceName, when non-empty, injects -interface for tools that support it (naabu).
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
		// cloudlist requires comma-separated provider config: -provider provider1,provider2
		args = []string{"cloudlist", "-provider", "aws,gcp,azure,digitalocean", "-json"}
	default:
		args = []string{toolName, "-u", target, "-json", "-silent"}
	}
	return append(args, extra...)
}

// verifyRouteBeforeScan checks whether a route to the target exists via the
// selected interface. On failure it emits a log warning but does NOT abort
// the scan — the operator may have set up routing out-of-band.
func verifyRouteBeforeScan(target, ifaceName string) {
	// Strip CIDR notation to get a routable IP for the check
	ip := target
	if idx := strings.Index(target, "/"); idx != -1 {
		ip = target[:idx]
	}
	if err := network.VerifyRoute(ip, ifaceName); err != nil {
		emitLog(fmt.Sprintf(
			"[WARN] No confirmed route to %s via interface %s — scan will use OS default routing. Error: %v",
			target, ifaceName, err,
		))
	} else {
		emitLog(fmt.Sprintf("[INFO] Route verified: %s → %s", target, ifaceName))
	}
}
