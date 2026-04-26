package api

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/pkg/network"
)

// HealthStatus represents the structured JSON response for /api/health.
type HealthStatus struct {
	Status      string    `json:"status"`         // "ok" or "degraded"
	Timestamp   time.Time `json:"timestamp"`
	Uptime      string    `json:"uptime"`
	DB          DBHealth  `json:"database"`
	Network     NetHealth `json:"network"`
	MailQueue   MailHealth `json:"mail_queue"`
	GoVersion   string    `json:"go_version"`
	GoroutineCount int    `json:"goroutines"`
}

// DBHealth holds the database connectivity state.
type DBHealth struct {
	Connected bool   `json:"connected"`
	Driver    string `json:"driver"`
	WALMode   bool   `json:"wal_mode"`
}

// NetHealth holds the active network interface state.
type NetHealth struct {
	ActiveInterface string `json:"active_interface"` // "eth0", "tun0", etc.
	TunnelConnected bool   `json:"tunnel_connected"`
	TunnelIP        string `json:"tunnel_ip,omitempty"`
}

// MailHealth holds the queued email count.
type MailHealth struct {
	Queued int `json:"queued"`
}

var startTime = time.Now()

// HealthCheck handles GET /api/health — no authentication required so that
// external monitoring tools (UptimeRobot, Grafana) can query it freely.
// It is registered on a separate, un-authenticated subrouter in server.go.
func (as *Server) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:         "ok",
		Timestamp:      time.Now().UTC(),
		Uptime:         time.Since(startTime).Round(time.Second).String(),
		GoVersion:      runtime.Version(),
		GoroutineCount: runtime.NumGoroutine(),
	}

	// ── Database health ─────────────────────────────────────────────────────
	dbOk := models.CheckDBHealth()
	status.DB = DBHealth{
		Connected: dbOk,
		Driver:    "sqlite3",
		WALMode:   true, // enforced in models.Setup()
	}
	if !dbOk {
		status.Status = "degraded"
	}

	// ── Network / Tunnel health ──────────────────────────────────────────────
	tunnelActive := network.GlobalTunnelManager().IsRunning()
	ifaceName, tunIP, tunConnected, err := network.GlobalTunnelManager().AgentConnected()
	
	if !tunnelActive {
		// Tunnel engine is down
		status.Network = NetHealth{
			ActiveInterface: "eth0",
			TunnelConnected: false,
		}
	} else if err != nil || !tunConnected {
		// No tunnel — determine primary interface name (eth0 or similar)
		ifaces, _ := network.ListInterfaces()
		primary := "eth0"
		for _, iface := range ifaces {
			if iface.IsUp && !iface.IsVirtual && !iface.IsTUN && len(iface.Addresses) > 0 {
				primary = iface.Name
				break
			}
		}
		status.Network = NetHealth{
			ActiveInterface: primary,
			TunnelConnected: false,
		}
	} else {
		status.Network = NetHealth{
			ActiveInterface: ifaceName,
			TunnelConnected: true,
			TunnelIP:        tunIP,
		}
	}

	// ── Mail queue health ────────────────────────────────────────────────────
	queued, err := models.GetQueuedMailCount()
	if err != nil {
		queued = -1
	}
	status.MailQueue = MailHealth{Queued: queued}

	httpStatus := http.StatusOK
	if status.Status == "degraded" {
		httpStatus = http.StatusServiceUnavailable
	}

	JSONResponse(w, status, httpStatus)
}
