package models

import "time"

// ── Vulnerability Scanning Models ─────────────────────────────────────────────
// PHASE 2: DATABASE INDEXING FOR ENTERPRISE PERFORMANCE (Sub-millisecond queries)

// Scan represents a vulnerability scanning session.
// It tracks execution history and metadata for performed scans.
type Scan struct {
	ID               uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           int64     `gorm:"not null;index" json:"user_id"`
	Name             string    `gorm:"size:255;index" json:"name"`
	Target           string    `gorm:"not null;index:idx_user_target" json:"target"` // Composite index with UserID
	ToolName         string    `gorm:"index" json:"tool_name"`
	EnabledTools     JSONList  `gorm:"type:text" json:"enabled_tools"`
	OutboundInterface string   `gorm:"size:64;index" json:"outbound_interface"`
	Mode             string    `gorm:"index" json:"mode"` // "single" | "discovery" | "task"
	Status           string    `gorm:"default:'queued';index:idx_user_status" json:"status"` // Composite index with UserID
	Progress         int       `gorm:"default:0" json:"progress"`
	CreatedAt        time.Time `gorm:"autoCreateTime;index" json:"created_at"` // Index for time-based queries
	UpdatedAt        time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Relationships
	Findings []Finding `gorm:"foreignKey:ScanID;constraint:OnDelete:CASCADE" json:"findings,omitempty"`
}

// Finding represents a single vulnerability finding or asset discovered
// by ProjectDiscovery tools. This is the unified model for all tool outputs.
type Finding struct {
	ID                uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID            int64     `gorm:"not null;index" json:"user_id"`
	ScanID            uint      `gorm:"index:idx_scan_severity;index" json:"scan_id,omitempty"` // FK → Scan.ID, composite index with Severity
	ToolName          string    `gorm:"not null;index" json:"tool_name"`
	Severity          string    `gorm:"not null;index:idx_scan_severity;index:idx_sev_target" json:"severity"` // Multiple indexes for filtering
	Name              string    `gorm:"index" json:"name"`
	Target            string    `gorm:"not null;index:idx_sev_target;index:idx_user_target" json:"target"` // Composite indexes
	Detail            string    `json:"detail"`
	TemplateID        string    `gorm:"index" json:"template_id"`
	OutboundInterface string    `gorm:"index;size:64" json:"outbound_interface"`
	CreatedAt         time.Time `gorm:"autoCreateTime;index" json:"created_at"` // Index for time range queries

	// Relationships
	Scan *Scan `gorm:"foreignKey:ScanID;constraint:OnDelete:CASCADE" json:"scan,omitempty"`
}

// ── Phishing Campaign Extension ───────────────────────────────────────────────

// VantageRiskScoring holds risk scores across Gophish campaigns and PD findings.
// This is a computed model, not persisted to the main Campaign table.
type VantageRiskScoring struct {
	CampaignID  int       `json:"campaign_id"`
	TargetName  string    `json:"target_name"`
	Severity    string    `json:"severity"`
	PhishScore  float64   `json:"phish_score"`
	VulnScore   float64   `json:"vuln_score"`
	RiskLevel   string    `json:"risk_level"`
	LastUpdated time.Time `json:"last_updated"`
}

// ── Network Configuration ───────────────────────────────────────────────────────

// UserNetworkConfig stores user's preferred network interface for scanning.
// Allows switching between default, Tailscale, VPN, or other interfaces.
type UserNetworkConfig struct {
	ID                uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID            uint      `gorm:"not null;uniqueIndex" json:"user_id"`
	PreferredInterface string    `gorm:"default:'default'" json:"preferred_interface"`
	AllowedInterfaces  JSONList  `gorm:"type:text" json:"allowed_interfaces"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
