package worker

import (
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/scanner"
)

// VantageWorker handles periodic tasks specific to the Vantage Security Hub.
// Primarily, it manages the execution of scheduled vulnerability scans.
type VantageWorker struct{}

// NewVantageWorker returns a new instance of the VantageWorker.
func NewVantageWorker() *VantageWorker {
	return &VantageWorker{}
}

// Start launches the background worker loop.
// It checks for scheduled tasks every 30 seconds.
func (vw *VantageWorker) Start() {
	log.Info("Vantage Background Worker Started - Monitoring Scheduled Scans")
	
	// Check every 30 seconds for scheduled scans
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for range ticker.C {
			vw.processScheduledScans()
		}
	}()
}

// processScheduledScans queries the database for scans that are ready to run.
func (vw *VantageWorker) processScheduledScans() {
	now := time.Now().UTC()
	scans, err := models.GetScheduledScanTasks(now)
	if err != nil {
		log.Errorf("Worker: Failed to query scheduled scans: %v", err)
		return
	}

	if len(scans) > 0 {
		log.Infof("Worker: Found %d scheduled scans ready for execution", len(scans))
	}

	for _, s := range scans {
		log.Infof("Worker: Triggering scheduled scan ID=%d (Target: %s, Mode: %s)", s.ID, s.Target, s.Mode)
		
		// Reset ScheduledAt to prevent double triggers if status update fails
		// and update status to "queued" before starting
		err := models.UpdateScanTaskProgress(s.ID, "queued", 0)
		if err != nil {
			log.Errorf("Worker: Failed to update status for scan %d: %v", s.ID, err)
			continue
		}

		// Dispatch based on mode
		go func(task models.Scan) {
			var scanErr error
			tools := task.GetToolList()
			
			switch task.Mode {
			case "discovery":
				scanErr = scanner.RunDiscovery(task.UserID, task.ID, task.Target, task.OutboundInterface)
			case "task":
				// For scheduled tasks, we default to non-parallel unless we add more metadata later
				scanErr = scanner.RunTask(task.UserID, task.ID, task.Target, task.OutboundInterface, tools, false, []string{})
			default:
				tool := "nuclei"
				if len(tools) > 0 {
					tool = tools[0]
				}
				scanErr = scanner.RunScannerTool(task.UserID, task.ID, tool, task.Target, task.OutboundInterface, []string{})
			}

			if scanErr != nil {
				log.Errorf("Worker: Failed to start scan %d: %v", task.ID, scanErr)
				models.UpdateScanTaskProgress(task.ID, "error", 0)
			}
		}(s)
	}
}
