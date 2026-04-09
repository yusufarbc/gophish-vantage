package api

import (
	"net/http"

	mid "github.com/gophish/gophish/middleware"
	"github.com/gophish/gophish/middleware/ratelimit"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/worker"
	"github.com/gorilla/mux"
)

// ServerOption is an option to apply to the API server.
type ServerOption func(*Server)

// Server represents the routes and functionality of the Gophish API.
// It's not a server in the traditional sense, in that it isn't started and
// stopped. Rather, it's meant to be used as an http.Handler in the
// AdminServer.
type Server struct {
	handler http.Handler
	worker  worker.Worker
	limiter *ratelimit.PostLimiter
}

// NewServer returns a new instance of the API handler with the provided
// options applied.
func NewServer(options ...ServerOption) *Server {
	defaultWorker, _ := worker.New()
	defaultLimiter := ratelimit.NewPostLimiter()
	as := &Server{
		worker:  defaultWorker,
		limiter: defaultLimiter,
	}
	for _, opt := range options {
		opt(as)
	}
	as.registerRoutes()
	return as
}

// WithWorker is an option that sets the background worker.
func WithWorker(w worker.Worker) ServerOption {
	return func(as *Server) {
		as.worker = w
	}
}

func WithLimiter(limiter *ratelimit.PostLimiter) ServerOption {
	return func(as *Server) {
		as.limiter = limiter
	}
}

func (as *Server) registerRoutes() {
	root := mux.NewRouter()
	root = root.StrictSlash(true)
	router := root.PathPrefix("/api/").Subrouter()
	router.Use(mid.RequireAPIKey)
	router.Use(mid.EnforceViewOnly)
	router.HandleFunc("/imap/", as.IMAPServer)
	router.HandleFunc("/imap/validate", as.IMAPServerValidate)
	router.HandleFunc("/reset", as.Reset)
	router.HandleFunc("/campaigns/", as.Campaigns)
	router.HandleFunc("/campaigns/summary", as.CampaignsSummary)
	router.HandleFunc("/campaigns/{id:[0-9]+}", as.Campaign)
	router.HandleFunc("/campaigns/{id:[0-9]+}/results", as.CampaignResults)
	router.HandleFunc("/campaigns/{id:[0-9]+}/summary", as.CampaignSummary)
	router.HandleFunc("/campaigns/{id:[0-9]+}/complete", as.CampaignComplete)
	router.HandleFunc("/groups/", as.Groups)
	router.HandleFunc("/groups/summary", as.GroupsSummary)
	router.HandleFunc("/groups/{id:[0-9]+}", as.Group)
	router.HandleFunc("/groups/{id:[0-9]+}/summary", as.GroupSummary)
	router.HandleFunc("/templates/", as.Templates)
	router.HandleFunc("/templates/{id:[0-9]+}", as.Template)
	router.HandleFunc("/pages/", as.Pages)
	router.HandleFunc("/pages/{id:[0-9]+}", as.Page)
	router.HandleFunc("/smtp/", as.SendingProfiles)
	router.HandleFunc("/smtp/{id:[0-9]+}", as.SendingProfile)
	router.HandleFunc("/users/", mid.Use(as.Users, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/users/{id:[0-9]+}", mid.Use(as.User))
	router.HandleFunc("/util/send_test_email", as.SendTestEmail)
	router.HandleFunc("/import/group", as.ImportGroup)
	router.HandleFunc("/import/email", as.ImportEmail)
	router.HandleFunc("/import/site", as.ImportSite)
	router.HandleFunc("/webhooks/", mid.Use(as.Webhooks, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/webhooks/{id:[0-9]+}/validate", mid.Use(as.ValidateWebhook, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/webhooks/{id:[0-9]+}", mid.Use(as.Webhook, mid.RequirePermission(models.PermissionModifySystem)))

	// ── Scanner / Vulnerability Management Routes ────────────────────────────────
	router.HandleFunc("/scanner/start", as.StartScan).Methods("POST")
	router.HandleFunc("/scanner/status", as.ScanStatus).Methods("GET")
	router.HandleFunc("/scanner/findings", as.GetFindings).Methods("GET")
	router.HandleFunc("/scanner/findings/{id:[0-9]+}", as.DeleteFinding).Methods("DELETE")
	router.HandleFunc("/scanner/findings", as.ClearFindings).Methods("DELETE")
	router.HandleFunc("/scanner/stats", as.GetStats).Methods("GET")
	router.HandleFunc("/scanner/tasks", as.ListTasks).Methods("GET")
	router.HandleFunc("/scanner/tasks/{id:[0-9]+}", as.DeleteScanTask).Methods("DELETE")
	router.HandleFunc("/scanner/stop/{id:[0-9]+}", as.StopScanHandler).Methods("POST")
	router.HandleFunc("/scanner/stress/start", as.StartStressTest).Methods("POST")

	// ── Vantage v1 API — Network & Tunnel Management ──────────────────────────
	// Versioned subrouter: /api/v1/
	// Uses the same auth middleware (RequireAPIKey) as the parent router.
	v1 := root.PathPrefix("/api/v1/").Subrouter()
	v1.Use(mid.RequireAPIKey)
	v1.HandleFunc("/interfaces", as.ListInterfaces).Methods("GET")
	v1.HandleFunc("/tunnel/status", as.TunnelStatus).Methods("GET")
	v1.HandleFunc("/tunnel/start", as.StartTunnelServer).Methods("POST")
	v1.HandleFunc("/tunnel/stop", as.StopTunnelServer).Methods("POST")
	v1.HandleFunc("/tunnel/route", as.AddRoute).Methods("POST")
	v1.HandleFunc("/targets", as.ListTargets).Methods("GET")
	v1.HandleFunc("/targets/import", as.ImportTargets).Methods("POST")
	v1.HandleFunc("/mail/queue", as.MailQueueSummary).Methods("GET")
	v1.HandleFunc("/scanner/report/{id:[0-9]+}", as.DownloadScanReport).Methods("GET")
	v1.HandleFunc("/settings/notifications", as.GetNotificationSettings).Methods("GET")
	v1.HandleFunc("/settings/notifications", as.PostNotificationSettings).Methods("POST")

	as.handler = root
}

func (as *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	as.handler.ServeHTTP(w, r)
}
