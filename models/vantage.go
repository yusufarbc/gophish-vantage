package models

import "time"

// ── Vulnerability Scanning Models ─────────────────────────────────────────────
// PHASE 2: DATABASE INDEXING FOR ENTERPRISE PERFORMANCE (Sub-millisecond queries)

// Scan represents a vulnerability scanning session.
// It tracks execution history and metadata for performed scans.
type Scan struct {
	ID                uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID            int64      `gorm:"not null;index:idx_scan_user" json:"user_id"`
	Name              string     `gorm:"size:255;index" json:"name"`
	Target            string     `gorm:"not null;index:idx_scan_target" json:"target"` // High-frequency query
	ToolName          string     `gorm:"index:idx_scan_tool" json:"tool_name"`
	EnabledTools      JSONList   `gorm:"type:text" json:"enabled_tools"`
	OutboundInterface string     `gorm:"size:64;index" json:"outbound_interface"`
	Mode              string     `gorm:"index" json:"mode"`
	Status            string     `gorm:"default:'queued';index:idx_scan_status" json:"status"`
	Progress          int        `gorm:"default:0" json:"progress"`
	ScheduledAt       *time.Time `gorm:"index:idx_scan_schedule" json:"scheduled_at,omitempty"`
	CreatedAt         time.Time  `gorm:"autoCreateTime;index:idx_scan_created" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime" json:"updated_at"`

	Findings []Finding `gorm:"foreignKey:ScanID;constraint:OnDelete:CASCADE" json:"findings,omitempty"`
}

// Finding represents a single vulnerability finding or asset discovered
// by ProjectDiscovery tools. This is the unified model for all tool outputs.
type Finding struct {
	ID                uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID            int64     `gorm:"not null;index:idx_finding_user" json:"user_id"`
	ScanID            uint      `gorm:"index:idx_finding_scan" json:"scan_id,omitempty"` // FK Index
	ToolName          string    `gorm:"not null;index:idx_finding_tool" json:"tool_name"`
	Severity          string    `gorm:"not null;index:idx_finding_severity" json:"severity"` // Frequent UI filter
	Name              string    `gorm:"index" json:"name"`
	Target            string    `gorm:"not null;index:idx_finding_target" json:"target"` // Search filter
	Detail            string    `json:"detail"`
	TemplateID        string    `gorm:"index:idx_finding_template" json:"template_id"`
	OutboundInterface string    `gorm:"index;size:64" json:"outbound_interface"`
	CreatedAt         time.Time `gorm:"autoCreateTime;index:idx_finding_created" json:"created_at"`

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
