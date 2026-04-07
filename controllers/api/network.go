package api

import (
	"encoding/json"
	"net/http"

	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/pkg/network"
)

// ── Response Types ─────────────────────────────────────────────────────────────

// InterfaceListResponse is the JSON response for GET /api/v1/interfaces
type InterfaceListResponse struct {
	Interfaces []network.NetworkInterface `json:"interfaces"`
}

// TunnelStatusResponse is the JSON response for GET /api/v1/tunnel/status
type TunnelStatusResponse struct {
	ServerRunning  bool   `json:"server_running"`
	AgentConnected bool   `json:"agent_connected"`
	TunInterface   string `json:"tun_interface,omitempty"`
	TunIP          string `json:"tun_ip,omitempty"`
}

// RouteRequest is the JSON body for POST /api/v1/tunnel/route
type RouteRequest struct {
	CIDR      string `json:"cidr"`
	Interface string `json:"interface"`
}

// ── Handlers ───────────────────────────────────────────────────────────────────

// ListInterfaces returns all active network interfaces on the host.
// GET /api/v1/interfaces
//
// Example response:
//
//	{
//	  "interfaces": [
//	    { "name": "eth0", "addresses": ["10.0.0.2/24"], "is_up": true, "is_tun": false },
//	    { "name": "tun0", "addresses": ["10.100.0.2/24"], "is_up": true, "is_tun": true }
//	  ]
//	}
func (as *Server) ListInterfaces(w http.ResponseWriter, r *http.Request) {
	ifaces, err := network.ListInterfaces()
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "failed to list interfaces: " + err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, InterfaceListResponse{Interfaces: ifaces}, http.StatusOK)
}

// TunnelStatus returns the current state of the Chisel tunnel server and
// whether a remote agent is connected (detected via a TUN interface presence).
// GET /api/v1/tunnel/status
func (as *Server) TunnelStatus(w http.ResponseWriter, r *http.Request) {
	mgr := network.GlobalTunnelManager()
	ifaceName, ip, connected, err := mgr.AgentConnected()
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "tunnel status check failed: " + err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, TunnelStatusResponse{
		ServerRunning:  mgr.IsRunning(),
		AgentConnected: connected,
		TunInterface:   ifaceName,
		TunIP:          ip,
	}, http.StatusOK)
}

// StartTunnelServer starts the Chisel reverse-tunnel server subprocess.
// POST /api/v1/tunnel/start
// Idempotent: returns 200 if already running.
func (as *Server) StartTunnelServer(w http.ResponseWriter, r *http.Request) {
	mgr := network.GlobalTunnelManager()
	if err := mgr.Start(); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "failed to start tunnel server: " + err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "tunnel server started"}, http.StatusOK)
}

// StopTunnelServer stops the Chisel reverse-tunnel server subprocess.
// POST /api/v1/tunnel/stop
func (as *Server) StopTunnelServer(w http.ResponseWriter, r *http.Request) {
	mgr := network.GlobalTunnelManager()
	if err := mgr.Stop(); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "failed to stop tunnel server: " + err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "tunnel server stopped"}, http.StatusOK)
}

// AddRoute adds a host route through a specific network interface.
// POST /api/v1/tunnel/route
// Body: { "cidr": "192.168.1.0/24", "interface": "tun0" }
func (as *Server) AddRoute(w http.ResponseWriter, r *http.Request) {
	var req RouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "invalid JSON body"}, http.StatusBadRequest)
		return
	}
	if req.CIDR == "" || req.Interface == "" {
		JSONResponse(w, models.Response{Success: false, Message: "cidr and interface are required"}, http.StatusBadRequest)
		return
	}
	if err := network.SetupRoute(req.CIDR, req.Interface); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "failed to add route: " + err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, map[string]string{
		"message":   "route added",
		"cidr":      req.CIDR,
		"interface": req.Interface,
	}, http.StatusOK)
}
