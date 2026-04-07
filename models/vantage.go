package models

import "time"

// ── Vulnerability Scanning Models ─────────────────────────────────────────────

// Scan represents a vulnerability scanning session.
// It tracks execution history and metadata for performed scans.
type Scan struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Target    string    `gorm:"not null;index" json:"target"`
	ToolName  string    `gorm:"index" json:"tool_name"`
	Mode      string    `json:"mode"` // "single" | "discovery" | "bulk"
	Status    string    `gorm:"default:'running'" json:"status"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Relationships
	Findings []Finding `gorm:"foreignKey:ScanID" json:"findings,omitempty"`
}

// Finding represents a single vulnerability finding or asset discovered
// by ProjectDiscovery tools. This is the unified model for all tool outputs.
//
// Example sources:
// - nuclei: CVE/template-based detection (high confidence)
// - subfinder: Subdomain enumeration (reconnaissance)
// - httpx: Alive HTTP service on host:port
// - naabu: Open network port
// - dnsx: DNS resolution record
// - katana: Web endpoint crawled from site
// - tlsx: TLS certificate information
// - uncover: Cloud/search engine results
// - asn-map: ASN/CIDR research
type Finding struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ScanID    uint      `gorm:"index" json:"scan_id,omitempty"` // FK → Scan.ID
	ToolName  string    `gorm:"not null;index" json:"tool_name"`
	Severity  string    `gorm:"not null;index;default:'info'" json:"severity"`
	Name      string    `json:"name"`           // Human-readable finding name
	Target    string    `gorm:"not null;index" json:"target"`
	Detail    string    `json:"detail"`         // Extra context: IP, port, URL path, etc
	TemplateID string   `json:"template_id"`    // For nuclei: template that triggered
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	Scan *Scan `gorm:"foreignKey:ScanID" json:"scan,omitempty"`
}

// ── Phishing Campaign Extension ───────────────────────────────────────────────

// The standard Campaign, Group, Template, SMTP, etc. models are used unchanged.
// This section is for additional fields that Vantage might add to the schema.
// Future: Consider adding a VantageMetadata field to Campaign for extra tracking.

// VantageRiskScoring holds risk scores across Gophish campaigns and PD findings.
// This is a computed model, not persisted to the main Campaign table.
type VantageRiskScoring struct {
	CampaignID  int       `json:"campaign_id"`
	TargetName  string    `json:"target_name"`
	Severity    string    `json:"severity"` // computed from linked findings
	PhishScore  float64   `json:"phish_score"`   // % of targets opened email or clicked link
	VulnScore   float64   `json:"vuln_score"`    // count of critical findings on target
	RiskLevel   string    `json:"risk_level"`    // "critical" | "high" | "medium" | "low"
	LastUpdated time.Time `json:"last_updated"`
}

// ── Network Configuration ───────────────────────────────────────────────────────

// UserNetworkConfig stores user's preferred network interface for scanning.
// Allows switching between default, Tailscale, VPN, or other interfaces.
type UserNetworkConfig struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID          uint      `gorm:"not null;uniqueIndex" json:"user_id"`      // FK → User.ID
	PreferredInterface string  `gorm:"default:'default'" json:"preferred_interface"` // "default" | "tailscale0" | "tun0"
	// Example: PreferredInterface could be:
	// - "default" (use system default routing)
	// - "tailscale0" (route through Tailscale VPN)
	// - "tun0" (custom VPN tunnel)
	// - "eth1" (secondary network interface)
	AllowedInterfaces []string `gorm:"serializer:json" json:"allowed_interfaces"` // JSON array of available options
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
