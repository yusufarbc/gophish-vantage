package scanner

import (
	"context"
	"sync"
)

// ── Tool Interface ────────────────────────────────────────────────────────────

// Tool is the core interface that every ProjectDiscovery tool adapter must implement.
// The execution engine is completely agnostic to tool-specific logic — all tool
// knowledge lives exclusively in the implementing structs in tools.go.
type Tool interface {
	// Name returns the binary name used to look up the executable (e.g., "nuclei").
	Name() string

	// BuildArgs constructs the full CLI argument list, including the binary name as args[0].
	// The engine calls this; no tool-specific arg logic belongs in the engine.
	BuildArgs(target, ifaceName string, extra []string) []string

	// ExtractTarget pulls the primary target identifier from a tool's JSON output object.
	// Returns "" if the field is absent or not a string.
	ExtractTarget(obj map[string]interface{}) string

	// SupportsInterface reports whether this tool accepts a network interface flag (e.g., naabu -interface).
	// Only tools that route traffic at L3 should return true.
	SupportsInterface() bool

	// IsJSONLOutput reports whether the tool emits one JSON object per line.
	// When true, the executor MUST use bufio.Scanner (1 MB line buffer) instead of
	// json.NewDecoder to guarantee OOM safety on massive nuclei outputs.
	IsJSONLOutput() bool
}

// ── Tool Registry ─────────────────────────────────────────────────────────────

// ToolRegistry is a thread-safe store of registered Tool implementations.
// All supported ProjectDiscovery tools are registered at package init time.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// DefaultRegistry is the global singleton registry, initialised with all supported tools.
var DefaultRegistry = newDefaultRegistry()

func newDefaultRegistry() *ToolRegistry {
	r := &ToolRegistry{tools: make(map[string]Tool)}
	// Register in pipeline execution order.
	r.Register(new(SubfinderTool)) // Phase 1 – OSINT / Asset Discovery
	r.Register(new(DNSxTool))      // Phase 1 – DNS Resolution
	r.Register(new(NaabuTool))     // Phase 2 – Port Scanning (L3 tun0 support)
	r.Register(new(HttpxTool))     // Phase 3 – HTTP Probing
	r.Register(new(TLSxTool))      // Phase 3 – TLS/SSL Analysis
	r.Register(new(KatanaTool))    // Phase 4 – Crawling & Spidering
	r.Register(new(NucleiTool))    // Phase 5 – Vulnerability Scanning (OOM-critical)
	return r
}

// Register adds or replaces a Tool in the registry. Safe for concurrent use.
func (r *ToolRegistry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Get returns a Tool by name and a boolean indicating whether it was found.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Names returns a slice of all registered tool names.
func (r *ToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	return names
}

// ── ScanService Interface ─────────────────────────────────────────────────────

// ScanService defines the core orchestration logic for different types of scans.
type ScanService interface {
	RunScannerTool(userID int64, scanID uint, toolName, target, ifaceName string, extraFlags []string) error
	RunDiscovery(userID int64, scanID uint, target, ifaceName string) error
	RunTask(userID int64, scanID uint, target, ifaceName string, tools []string, parallel bool, extraFlags []string) error
}

// ── ToolExecutor Interface ────────────────────────────────────────────────────

// ToolExecutor handles the low-level execution of a single ProjectDiscovery tool.
type ToolExecutor interface {
	Execute(ctx context.Context, userID int64, toolName, target, ifaceName string, args []string) error
	Collect(ctx context.Context, userID int64, parseAs, target, ifaceName string, args []string) ([]string, error)
}

// ── ResultPersister Interface ─────────────────────────────────────────────────

// ResultPersister defines how scan results are saved to the database.
type ResultPersister interface {
	PersistFinding(userID int64, toolName, target, ifaceName, line string) error
	PersistDiscoveredTarget(userID int64, target, source string) error
}
