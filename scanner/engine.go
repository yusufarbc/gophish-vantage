package scanner

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gophish/gophish/models"
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
		broadcast:  make(chan string, 2048),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[FATAL] ScannerLogHub panic: %v", r)
			}
		}()
		logHub.run()
	}()
}

func (h *ScannerLogHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			delete(h.clients, client)
			h.mu.Unlock()
			client.Close()
		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				client.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := client.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
					h.mu.RUnlock()
					go func(c *websocket.Conn) {
						defer func() { recover() }()
						h.unregister <- c
					}(client)
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

func ScannerWSHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}
	logHub.register <- conn
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[scanner] ws connection handler panic: %v", r)
			}
			logHub.unregister <- conn
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

// ── Scanner Engine State ──────────────────────────────────────────────────────

type ScanState struct {
	Running bool
	Tool    string
	Target  string
	Started time.Time
	mu      sync.RWMutex
}

var scanState = &ScanState{}

func GetScanState() *ScanState { return scanState }

func (s *ScanState) IsScanRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Running
}

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

func (s *ScanState) Status() (bool, string, string, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Running, s.Tool, s.Target, s.Started
}

func (s *ScanState) ReleaseLock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Running = false
}

// ── Active Scan Context Management ───────────────────────────────────────────

var (
	activeScans   = make(map[uint]context.CancelFunc)
	activeScansMu sync.Mutex
)

func RegisterScan(scanID uint, cancel context.CancelFunc) {
	activeScansMu.Lock()
	defer activeScansMu.Unlock()
	activeScans[scanID] = cancel
}

func UnregisterScan(scanID uint) {
	activeScansMu.Lock()
	defer activeScansMu.Unlock()
	delete(activeScans, scanID)
}

func StopScan(scanID uint) error {
	activeScansMu.Lock()
	cancel, ok := activeScans[scanID]
	activeScansMu.Unlock()
	if !ok { return fmt.Errorf("no active scan found for ID %d", scanID) }
	cancel()
	UnregisterScan(scanID)
	_ = models.UpdateScanTaskProgress(scanID, "stopped", 0)
	emitLog(fmt.Sprintf("[VANTAGE] !! Scan %d stopped by user", scanID))
	return nil
}

func emitLog(msg string) {
	log.Println(msg)
	if logHub != nil {
		select {
		case logHub.broadcast <- msg:
		default:
		}
	}
}

// ── VantageScanService Implementation ────────────────────────────────────────

type VantageScanService struct {
	Executor ToolExecutor
	State    *ScanState
}

var DefaultScanService ScanService

func InitDefaultService() {
	DefaultScanService = &VantageScanService{
		Executor: &DefaultExecutor{Persister: &GormPersister{}},
		State:    scanState,
	}
}

func (s *VantageScanService) RunScannerTool(userID int64, scanID uint, toolName, target, ifaceName string, extraFlags []string) error {
	if err := ensureInterfaceForScan(toolName, target, ifaceName); err != nil { return err }
	if err := s.State.AcquireLock(toolName, target); err != nil { return err }
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Scanner panic", "tool", toolName, "error", r, "target", target)
				emitLog(fmt.Sprintf("[FATAL] Scanner panic in %s: %v", toolName, r))
			}
			_ = models.UpdateScanTaskProgress(scanID, "done", 100)
			s.State.ReleaseLock()
		}()

		_ = models.UpdateScanTaskProgress(scanID, "running", 20)
		args := buildScannerArgs(toolName, target, ifaceName, extraFlags)
		emitLog(fmt.Sprintf("[VANTAGE] ▶ Starting %s on target=%s", strings.ToUpper(toolName), target))

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		RegisterScan(scanID, cancel)
		defer UnregisterScan(scanID)

		if err := s.Executor.Execute(ctx, userID, toolName, target, ifaceName, args); err != nil {
			emitLog(fmt.Sprintf("[ERROR] %s failed: %v", toolName, err))
			_ = models.UpdateScanTaskProgress(scanID, "failed", 0)
			return
		}
		
		_ = models.UpdateScanTaskProgress(scanID, "running", 90)
		emitLog(fmt.Sprintf("[VANTAGE] ✔ %s finished", strings.ToUpper(toolName)))
	}()
	return nil
}

func (s *VantageScanService) RunDiscovery(userID int64, scanID uint, target, ifaceName string) error {
	if err := ensureInterfaceForScan("discovery", target, ifaceName); err != nil { return err }
	if err := s.State.AcquireLock("discovery", target); err != nil { return err }

	go func() {
		defer func() {
			if r := recover(); r != nil {
				emitLog(fmt.Sprintf("[FATAL] Discovery panic: %v", r))
			}
			_ = models.UpdateScanTaskProgress(scanID, "done", 100)
			s.State.ReleaseLock()
		}()

		emitLog("[VANTAGE] ═══ DISCOVERY MODE ════════════")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		RegisterScan(scanID, cancel)
		defer UnregisterScan(scanID)

		_ = models.UpdateScanTaskProgress(scanID, "running", 10)
		emitLog("[VANTAGE] Phase 1 — Subdomain Discovery")
		subArgs := []string{"subfinder", "-d", target, "-json", "-silent"}
		subdomains, _ := s.Executor.Collect(ctx, userID, "subfinder", target, ifaceName, subArgs)
		if ctx.Err() != nil { return }
		if len(subdomains) == 0 { subdomains = []string{target} }

		_ = models.UpdateScanTaskProgress(scanID, "running", 40)
		emitLog("[VANTAGE] Phase 2 — HTTP Probing")
		aliveArgs := append([]string{"httpx", "-json", "-silent"}, hostToArgs(subdomains)...)
		alive, _ := s.Executor.Collect(ctx, userID, "httpx", target, ifaceName, aliveArgs)
		if ctx.Err() != nil { return }

		_ = models.UpdateScanTaskProgress(scanID, "running", 70)
		emitLog(fmt.Sprintf("[VANTAGE] Phase 3 — Vulnerability Scanning (%d targets)", len(alive)))
		for _, host := range alive {
			if ctx.Err() != nil { break }
			args := []string{"nuclei", "-u", host, "-json", "-silent"}
			_ = s.Executor.Execute(ctx, userID, "nuclei", host, ifaceName, args)
		}
		emitLog("[VANTAGE] ═══ DISCOVERY COMPLETE ════════")
	}()
	return nil
}

func (s *VantageScanService) RunTask(userID int64, scanID uint, target, ifaceName string, tools []string, parallel bool, extraFlags []string) error {
	if err := ensureInterfaceForScan("task", target, ifaceName); err != nil { return err }
	if err := s.State.AcquireLock("task", target); err != nil { return err }

	go func() {
		defer func() {
			if r := recover(); r != nil {
				emitLog(fmt.Sprintf("[FATAL] Task panic: %v", r))
			}
			_ = models.UpdateScanTaskProgress(scanID, "done", 100)
			s.State.ReleaseLock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		RegisterScan(scanID, cancel)
		defer UnregisterScan(scanID)

		if parallel {
			var wg sync.WaitGroup
			for _, tool := range tools {
				if ctx.Err() != nil { break }
				wg.Add(1)
				go func(t string) {
					defer wg.Done()
					args := buildScannerArgs(t, target, ifaceName, extraFlags)
					_ = s.Executor.Execute(ctx, userID, t, target, ifaceName, args)
				}(tool)
			}
			wg.Wait()
		} else {
			for i, tool := range tools {
				if ctx.Err() != nil { break }
				prog := 10 + (i * 80 / len(tools))
				_ = models.UpdateScanTaskProgress(scanID, "running", prog)
				args := buildScannerArgs(tool, target, ifaceName, extraFlags)
				_ = s.Executor.Execute(ctx, userID, tool, target, ifaceName, args)
			}
		}
	}()
	return nil
}

// ── Legacy Entry Points for Backward Compatibility ───────────────────────────

func RunScannerTool(userID int64, scanID uint, toolName, target, ifaceName string, extraFlags []string) error {
	if DefaultScanService == nil { InitDefaultService() }
	return DefaultScanService.RunScannerTool(userID, scanID, toolName, target, ifaceName, extraFlags)
}

func RunDiscovery(userID int64, scanID uint, target, ifaceName string) error {
	if DefaultScanService == nil { InitDefaultService() }
	return DefaultScanService.RunDiscovery(userID, scanID, target, ifaceName)
}

func RunTask(userID int64, scanID uint, target, ifaceName string, tools []string, parallel bool, extraFlags []string) error {
	if DefaultScanService == nil { InitDefaultService() }
	return DefaultScanService.RunTask(userID, scanID, target, ifaceName, tools, parallel, extraFlags)
}
