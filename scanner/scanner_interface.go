package scanner

import (
	"context"
)

// ScanService defines the core orchestration logic for different types of scans.
type ScanService interface {
	RunScannerTool(userID int64, scanID uint, toolName, target, ifaceName string, extraFlags []string) error
	RunDiscovery(userID int64, scanID uint, target, ifaceName string) error
	RunTask(userID int64, scanID uint, target, ifaceName string, tools []string, parallel bool, extraFlags []string) error
}

// ToolExecutor handles the low-level execution of a single ProjectDiscovery tool.
type ToolExecutor interface {
	Execute(ctx context.Context, userID int64, toolName, target, ifaceName string, args []string) error
	Collect(ctx context.Context, userID int64, parseAs, target, ifaceName string, args []string) ([]string, error)
}

// ResultPersister defines how scan results are saved to the database.
type ResultPersister interface {
	PersistFinding(userID int64, toolName, target, ifaceName, line string) error
	PersistDiscoveredTarget(userID int64, target, source string) error
}
