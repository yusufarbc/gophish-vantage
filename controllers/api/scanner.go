package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/scanner"
)

// ScanRequest is the JSON body for POST /api/scanner/scan
type ScanRequest struct {
	Target        string   `json:"target"`
	Tool          string   `json:"tool"`
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
		JSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	// Validation
	req.Target = strings.TrimSpace(req.Target)
	if req.Target == "" {
		JSONError(w, "target is required", http.StatusBadRequest)
		return
	}

	// Dispatch scan
	var scanErr error
	if req.DiscoveryMode {
		scanErr = scanner.RunDiscovery(req.Target, req.Interface)
	} else {
		if req.Tool == "" {
			req.Tool = "nuclei"
		}
		scanErr = scanner.RunScannerTool(req.Tool, req.Target, req.Interface, req.Flags)
	}

	if scanErr != nil {
		JSONError(w, scanErr.Error(), http.StatusConflict)
		return
	}

	mode := req.Tool
	if req.DiscoveryMode {
		mode = "discovery (subfinder → httpx → nuclei)"
	}

	response := ScanResponse{
		Message: "scan started",
		Target:  req.Target,
		Mode:    mode,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)
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
	// TODO: Query the Vantage database (vantage.db) for findings
	// For now, return placeholder
	findings := []models.Finding{}
	JSONResponse(w, findings, http.StatusOK)
}

// DeleteFinding removes a single finding by ID.
// DELETE /api/scanner/findings/:id
func (as *Server) DeleteFinding(w http.ResponseWriter, r *http.Request) {
	// TODO: Delete from Vantage findings table
	JSONResponse(w, models.Response{
		Success: true,
		Message: "finding deleted",
	}, http.StatusOK)
}

// ClearFindings truncates the findings table (destructive).
// DELETE /api/scanner/findings
func (as *Server) ClearFindings(w http.ResponseWriter, r *http.Request) {
	// TODO: Clear vantage findings table
	JSONResponse(w, models.Response{
		Success: true,
		Message: "all findings cleared",
	}, http.StatusOK)
}

// GetStats returns severity breakdown of findings.
// GET /api/scanner/stats
func (as *Server) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]int64{
		"total":    0,
		"critical": 0,
		"high":     0,
		"medium":   0,
		"low":      0,
		"info":     0,
	}
	// TODO: Query Vantage findings to compute stats
	JSONResponse(w, stats, http.StatusOK)
}
