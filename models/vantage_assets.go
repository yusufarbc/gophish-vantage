package models

import (
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
)

var (
	errTargetValueRequired = errors.New("target value is required")
	ipv4Regex  = regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|$)){4}$`)
	domainRegex = regexp.MustCompile(`^(?i)([a-z0-9-]+\.)+[a-z]{2,}$`)
)

// TargetAsset stores managed scan scope for Vantage.
// Table name is intentionally separated from Gophish's existing targets table.
type TargetAsset struct {
	ID               uint       `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	UserID           int64      `gorm:"not null;index;unique_index:idx_vantage_target_user_value" json:"user_id"`
	Value            string     `gorm:"not null;size:512;unique_index:idx_vantage_target_user_value" json:"value"`
	InputType        string     `gorm:"not null;size:32;index" json:"input_type"`
	Source           string     `gorm:"size:32;index" json:"source"`
	Tag              string     `gorm:"size:64;index" json:"tag"`
	IsInternal       bool       `gorm:"index" json:"is_internal"`
	LastScanned      *time.Time `json:"last_scanned,omitempty"`
	Availability     string     `gorm:"size:16;default:'unknown';index" json:"availability"`
	OperatingSystem  string     `gorm:"size:128" json:"operating_system"`
	OutboundInterface string    `gorm:"size:64;index" json:"outbound_interface"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (TargetAsset) TableName() string {
	return "vantage_targets"
}

// UpsertTargetAsset inserts or updates a managed target asset.
func UpsertTargetAsset(uid int64, rawValue, source, tag, explicitType, iface string) (TargetAsset, error) {
	v := normalizeAssetValue(rawValue)
	asset := TargetAsset{}
	if v == "" {
		return asset, errTargetValueRequired
	}

	if explicitType == "" {
		explicitType = DetectInputType(v)
	}
	internal := isInternalTarget(v)

	err := db.Where("user_id = ? AND value = ?", uid, v).First(&asset).Error
	if err == gorm.ErrRecordNotFound {
		asset = TargetAsset{
			UserID:            uid,
			Value:             v,
			InputType:         explicitType,
			Source:            source,
			Tag:               tag,
			IsInternal:        internal,
			Availability:      "unknown",
			OutboundInterface: iface,
		}
		return asset, db.Create(&asset).Error
	}
	if err != nil {
		return asset, err
	}

	asset.InputType = explicitType
	if source != "" {
		asset.Source = source
	}
	if tag != "" {
		asset.Tag = tag
	}
	asset.IsInternal = internal
	if iface != "" {
		asset.OutboundInterface = iface
	}
	return asset, db.Save(&asset).Error
}

// UpsertDiscoveredTarget stores assets discovered by recon tools.
func UpsertDiscoveredTarget(uid int64, value, source string) error {
	_, err := UpsertTargetAsset(uid, value, source, "discovered", "", "")
	return err
}

// ImportTargetsFromInput parses mixed user input and upserts all entries.
func ImportTargetsFromInput(uid int64, input, source string) ([]TargetAsset, error) {
	parts := splitAssetInput(input)
	assets := make([]TargetAsset, 0, len(parts))
	for _, p := range parts {
		a, err := UpsertTargetAsset(uid, p, source, "manual", "", "")
		if err != nil {
			continue
		}
		assets = append(assets, a)
	}
	return assets, nil
}

func splitAssetInput(input string) []string {
	r := strings.NewReplacer(",", "\n", ";", "\n", "\t", "\n")
	raw := strings.Split(r.Replace(input), "\n")
	seen := map[string]bool{}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		n := normalizeAssetValue(v)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

// DetectInputType classifies input into ip, cidr, domain, org.
func DetectInputType(v string) string {
	v = normalizeAssetValue(v)
	if v == "" {
		return "unknown"
	}
	if strings.Contains(v, "/") {
		if _, _, err := net.ParseCIDR(v); err == nil {
			return "cidr"
		}
	}
	if ip := net.ParseIP(v); ip != nil || ipv4Regex.MatchString(v) {
		return "ip"
	}
	if domainRegex.MatchString(v) {
		return "domain"
	}
	return "organization"
}

func normalizeAssetValue(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func isInternalTarget(v string) bool {
	v = normalizeAssetValue(v)
	if v == "" {
		return false
	}
	if strings.Contains(v, "/") {
		ip, _, err := net.ParseCIDR(v)
		if err == nil {
			return isPrivateIP(ip)
		}
	}
	if ip := net.ParseIP(v); ip != nil {
		return isPrivateIP(ip)
	}
	return false
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	privateBlocks := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8"}
	for _, cidr := range privateBlocks {
		_, b, _ := net.ParseCIDR(cidr)
		if b.Contains(ip) {
			return true
		}
	}
	return false
}

// ListTargetAssets returns managed targets with optional filters.
func ListTargetAssets(uid int64, scope, search string, limit int) ([]TargetAsset, error) {
	if limit <= 0 {
		limit = 500
	}
	q := db.Where("user_id = ?", uid)
	if scope == "internal" {
		q = q.Where("is_internal = ?", true)
	} else if scope == "external" {
		q = q.Where("is_internal = ?", false)
	}
	search = strings.TrimSpace(search)
	if search != "" {
		q = q.Where("value LIKE ?", "%"+search+"%")
	}
	var assets []TargetAsset
	err := q.Order("updated_at desc").Limit(limit).Find(&assets).Error
	return assets, err
}

// UpdateTargetScanState updates runtime status after each scan.
func UpdateTargetScanState(uid int64, value, liveStatus, osName, iface string) error {
	v := normalizeAssetValue(value)
	if v == "" {
		return nil
	}
	now := time.Now()
	updates := map[string]interface{}{
		"last_scanned":       now,
		"availability":       strings.ToLower(strings.TrimSpace(liveStatus)),
		"operating_system":   strings.TrimSpace(osName),
		"outbound_interface": strings.TrimSpace(iface),
	}
	return db.Model(&TargetAsset{}).
		Where("user_id = ? AND value = ?", uid, v).
		Updates(updates).Error
}

// GetFindingsForUser returns persisted findings with optional filters.
func GetFindingsForUser(uid int64, severity, tool string, limit int) ([]Finding, error) {
	if limit <= 0 {
		limit = 500
	}
	q := db.Where("user_id = ?", uid)
	if severity != "" {
		q = q.Where("severity = ?", strings.ToLower(severity))
	}
	if tool != "" {
		q = q.Where("tool_name = ?", strings.ToLower(tool))
	}
	var findings []Finding
	err := q.Order("created_at desc").Limit(limit).Find(&findings).Error
	return findings, err
}

func DeleteFindingForUser(uid int64, id uint) error {
	return db.Where("id = ? AND user_id = ?", id, uid).Delete(&Finding{}).Error
}

func ClearFindingsForUser(uid int64) error {
	return db.Where("user_id = ?", uid).Delete(&Finding{}).Error
}

// GetFindingStats returns aggregate severities for dashboard cards.
func GetFindingStats(uid int64) (map[string]int64, error) {
	stats := map[string]int64{
		"total":    0,
		"critical": 0,
		"high":     0,
		"medium":   0,
		"low":      0,
		"info":     0,
	}
	var total int64
	if err := db.Model(&Finding{}).Where("user_id = ?", uid).Count(&total).Error; err != nil {
		return stats, err
	}
	stats["total"] = total

	for _, sev := range []string{"critical", "high", "medium", "low", "info"} {
		var sevCount int64
		if err := db.Model(&Finding{}).Where("user_id = ? AND severity = ?", uid, sev).Count(&sevCount).Error; err != nil {
			return stats, err
		}
		stats[sev] = sevCount
	}
	return stats, nil
}

// UpsertFindingFromTool stores tool outputs as findings when meaningful.
func UpsertFindingFromTool(uid int64, toolName, severity, name, target, detail, templateID, iface string) error {
	toolName = strings.ToLower(strings.TrimSpace(toolName))
	if toolName == "" {
		return nil
	}
	target = normalizeAssetValue(target)
	if target == "" {
		return nil
	}
	if severity == "" {
		severity = "info"
	}
	f := Finding{
		UserID:            uid,
		ToolName:          toolName,
		Severity:          strings.ToLower(severity),
		Name:              strings.TrimSpace(name),
		Target:            target,
		Detail:            strings.TrimSpace(detail),
		TemplateID:        strings.TrimSpace(templateID),
		OutboundInterface: strings.TrimSpace(iface),
	}
	if f.Name == "" {
		f.Name = toolName + " finding"
	}
	if err := db.Create(&f).Error; err != nil {
		return err
	}
	return UpdateTargetScanState(uid, target, "live", "", iface)
}

// MailQueueSummary is an operational summary for postfix-like queue visibility.
type MailQueueSummary struct {
	Sent     int64 `json:"sent"`
	Deferred int64 `json:"deferred"`
	Queued   int64 `json:"queued"`
}

func GetMailQueueSummary(uid int64) (MailQueueSummary, error) {
	var m MailQueueSummary
	if err := db.Model(&Result{}).Where("user_id = ? AND status = ?", uid, EventSent).Count(&m.Sent).Error; err != nil {
		return m, err
	}
	if err := db.Model(&Result{}).Where("user_id = ? AND status IN (?)", uid, []string{StatusRetry, Error, StatusSending}).Count(&m.Deferred).Error; err != nil {
		return m, err
	}
	if err := db.Model(&Result{}).Where("user_id = ? AND status IN (?)", uid, []string{StatusQueued, StatusScheduled}).Count(&m.Queued).Error; err != nil {
		return m, err
	}
	return m, nil
}

// ParseLimit safely parses ?limit= query params.
func ParseLimit(v string, fallback int) int {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}