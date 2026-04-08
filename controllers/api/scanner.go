package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gophish/gophish/config"
	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/notifier"
	"github.com/gophish/gophish/reporting"
	"github.com/gophish/gophish/scanner"
	"github.com/gorilla/mux"
)

// ScanRequest is the JSON body for POST /api/scanner/scan
type ScanRequest struct {
	TaskName      string   `json:"task_name,omitempty"`
	Target        string   `json:"target"`
	Tool          string   `json:"tool"`
	EnabledTools  []string `json:"enabled_tools,omitempty"`
	Parallel      bool     `json:"parallel,omitempty"`
	Flags         []string `json:"flags,omitempty"`
	DiscoveryMode bool     `json:"discovery_mode,omitempty"`
	Interface     string   `json:"interface,omitempty"` // optional: bind scan to this network interface (e.g. "tun0")
}

// ScanResponse indicates scan was accepted (HTTP 202)
type ScanResponse struct {
	Message string `json:"message"`
	Target  string `json:"target"`
	Mode    string `json:"mode"`
}

// StatusResponse indicates current scanner state
type StatusResponse struct {
	Running bool   `json:"running"`
	Tool    string `json:"tool,omitempty"`
	Target  string `json:"target,omitempty"`
}

// ── POST /api/scanner/start ───────────────────────────────────────────────────

// StartScan initiates a ProjectDiscovery tool scan asynchronously.
// Returns 202 Accepted immediately; scan runs in background.
// WebSocket clients connected to /ws/scanner/logs receive live output.
func (as *Server) StartScan(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON body"}, http.StatusBadRequest)
		return
	}

	// Validation
	req.Target = strings.TrimSpace(req.Target)
	if req.Target == "" {
		JSONResponse(w, models.Response{Success: false, Message: "target is required"}, http.StatusBadRequest)
		return
	}

	uid := ctx.Get(r, "user_id").(int64)
	mode := "single"
	if req.DiscoveryMode {
		mode = "discovery"
	} else if len(req.EnabledTools) > 0 {
		mode = "task"
	}
	tools := req.EnabledTools
	if len(tools) == 0 && req.Tool != "" {
		tools = []string{req.Tool}
	}
	scanRecord, err := models.CreateScanTask(uid, req.TaskName, req.Target, req.Interface, mode, tools)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}

	// Dispatch scan
	var scanErr error
	if req.DiscoveryMode {
		scanErr = scanner.RunDiscovery(uid, scanRecord.ID, req.Target, req.Interface)
	} else if len(req.EnabledTools) > 0 {
		scanErr = scanner.RunTask(uid, scanRecord.ID, req.Target, req.Interface, req.EnabledTools, req.Parallel, req.Flags)
	} else {
		if req.Tool == "" {
			req.Tool = "nuclei"
		}
		scanErr = scanner.RunScannerTool(uid, scanRecord.ID, req.Tool, req.Target, req.Interface, req.Flags)
	}

	if scanErr != nil {
		JSONResponse(w, models.Response{Success: false, Message: scanErr.Error()}, http.StatusConflict)
		return
	}

	mode = req.Tool
	if req.DiscoveryMode {
		mode = "discovery (subfinder → httpx → nuclei)"
	} else if len(req.EnabledTools) > 0 {
		mode = "task"
	}

	response := ScanResponse{
		Message: "scan started",
		Target:  req.Target,
		Mode:    mode,
	}

	json.NewEncoder(w).Encode(response)
}

// ── POST /api/scanner/stop/:id ──────────────────────────────────────────────

// StopScanHandler terminates an active scan.
func (as *Server) StopScanHandler(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id64, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "invalid scan id"}, http.StatusBadRequest)
		return
	}

	if err := scanner.StopScan(uint(id64)); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusNotFound)
		return
	}

	JSONResponse(w, models.Response{Success: true, Message: "scan termination signal sent"}, http.StatusOK)
}

// ── GET /api/scanner/status ──────────────────────────────────────────────────

// ScanStatus returns the current state of the scanner (whether a scan is running).
func (as *Server) ScanStatus(w http.ResponseWriter, r *http.Request) {
	state := scanner.GetScanState()
	running, tool, target, _ := state.Status()
	response := StatusResponse{
		Running: running,
		Tool:    tool,
		Target:  target,
	}
	JSONResponse(w, response, http.StatusOK)
}

// ── Findings API ──────────────────────────────────────────────────────────────

// GetFindings returns all vulnerability findings stored in Vantage.
// Supports filtering by severity and tool.
// GET /api/scanner/findings?severity=high&tool=nuclei&limit=100
func (as *Server) GetFindings(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	severity := strings.TrimSpace(r.URL.Query().Get("severity"))
	tool := strings.TrimSpace(r.URL.Query().Get("tool"))
	limit := models.ParseLimit(r.URL.Query().Get("limit"), 500)
	findings, err := models.GetFindingsForUser(uid, severity, tool, limit)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, findings, http.StatusOK)
}

// DeleteFinding removes a single finding by ID.
// DELETE /api/scanner/findings/:id
func (as *Server) DeleteFinding(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	idStr := mux.Vars(r)["id"]
	id64, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "invalid finding id"}, http.StatusBadRequest)
		return
	}
	if err := models.DeleteFindingForUser(uid, uint(id64)); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{
		Success: true,
		Message: "finding deleted",
	}, http.StatusOK)
}

// ClearFindings truncates the findings table (destructive).
// DELETE /api/scanner/findings
func (as *Server) ClearFindings(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	if err := models.ClearFindingsForUser(uid); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{
		Success: true,
		Message: "all findings cleared",
	}, http.StatusOK)
}

// GetStats returns severity breakdown of findings.
// GET /api/scanner/stats
func (as *Server) GetStats(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	stats, err := models.GetFindingStats(uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, stats, http.StatusOK)
}

// ListTasks returns recent task-centric scan jobs.
// GET /api/scanner/tasks?limit=50
func (as *Server) ListTasks(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	limit := models.ParseLimit(r.URL.Query().Get("limit"), 50)
	tasks, err := models.ListScanTasks(uid, limit)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, tasks, http.StatusOK)
}

// DownloadScanReport generates a PDF report for a scan and streams it to the client.
// GET /api/v1/scanner/report/{id}
func (as *Server) DownloadScanReport(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	idStr := mux.Vars(r)["id"]
	id64, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "invalid scan id"}, http.StatusBadRequest)
		return
	}

	reportBytes, err := reporting.GenerateScanReport(uint(id64), uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "failed to generate report: " + err.Error()}, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=vantage_report_%d.pdf", id64))
	w.Header().Set("Content-Length", strconv.Itoa(len(reportBytes)))
	w.Write(reportBytes)
}

// GetNotificationSettings returns the current notification configuration.
// GET /api/v1/settings/notifications
func (as *Server) GetNotificationSettings(w http.ResponseWriter, r *http.Request) {
	// Need to access the global config
	// Since we don't have a direct reference to config.Config here, we retrieve it from the models package or a global.
	// However, gophish's config is loaded in gophish.go.
	// For now, we'll return the state from the notifier package if it's initialized.
	JSONResponse(w, config.GetConfig().Notifications, http.StatusOK)
}

// PostNotificationSettings updates the notification configuration.
// POST /api/v1/settings/notifications
func (as *Server) PostNotificationSettings(w http.ResponseWriter, r *http.Request) {
	var req config.NotificationConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "invalid request body"}, http.StatusBadRequest)
		return
	}

	c := config.GetConfig()
	c.Notifications = &req
	
	// Update the notifier package state
	notifier.Setup(c.Notifications)
	
	log.Infof("Notification settings updated by user")
	JSONResponse(w, models.Response{Success: true, Message: "notification settings updated"}, http.StatusOK)
}
