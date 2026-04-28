package models

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
	"time"
)

// ScanOptions holds granular configurations for each tool in the Vantage engine.
type ScanOptions struct {
	NucleiTags       []string `json:"nuclei_tags"`
	NucleiSeverities []string `json:"nuclei_severities"`
	NaabuPorts       string   `json:"naabu_ports"`
	SubfinderActive  bool     `json:"subfinder_active"`
	HttpxTech        bool     `json:"httpx_tech"`
	KatanaDepth      int      `json:"katana_depth"`
	Parallel         bool     `json:"parallel"`
}

// ScanRequest is the incoming payload from the UI Scan Wizard.
type ScanRequest struct {
	Name      string      `json:"name"`
	Target    string      `json:"target"`
	Interface string      `json:"interface"`
	Tools     []string    `json:"tools"`
	Options   ScanOptions `json:"options"`
	Schedule  string      `json:"schedule"`
}

// JSONList is a JSON-backed string list for scanner tools or settings.
type JSONList string

func (t JSONList) Value() (driver.Value, error) {
	if t == "" {
		return "[]", nil
	}
	return string(t), nil
}

func (t *JSONList) Scan(value interface{}) error {
	if value == nil {
		*t = ""
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*t = JSONList(string(v))
	case string:
		*t = JSONList(v)
	default:
		*t = ""
	}
	return nil
}

func (t JSONList) MarshalJSON() ([]byte, error) {
	if t == "" {
		return []byte("[]"), nil
	}
	return []byte(t), nil
}

func (t *JSONList) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*t = ""
		return nil
	}
	*t = JSONList(string(b))
	return nil
}

func CreateScanTask(uid int64, name, target, iface, mode string, tools []string, scheduledAt *time.Time) (Scan, error) {
	clean := make([]string, 0, len(tools))
	seen := map[string]bool{}
	for _, tool := range tools {
		t := strings.ToLower(strings.TrimSpace(tool))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		clean = append(clean, t)
	}
	toolJSON, _ := json.Marshal(clean)
	s := Scan{
		UserID:            uid,
		Name:              strings.TrimSpace(name),
		Target:            strings.TrimSpace(target),
		ToolName:          "task",
		EnabledTools:      JSONList(string(toolJSON)),
		OutboundInterface: strings.TrimSpace(iface),
		Mode:              strings.TrimSpace(mode),
		Status:            "queued",
		Progress:          0,
		ScheduledAt:       scheduledAt,
	}
	if scheduledAt != nil {
		s.Status = "scheduled"
	}
	if s.Name == "" {
		s.Name = "Task: " + s.Target
	}
	return s, db.Create(&s).Error
}

func ListScanTasks(uid int64, limit int) ([]Scan, error) {
	if limit <= 0 {
		limit = 50
	}
	var scans []Scan
	err := db.Where("user_id = ?", uid).Order("created_at desc").Limit(limit).Find(&scans).Error
	return scans, err
}

func GetScanTask(uid int64, id uint) (Scan, error) {
	var s Scan
	err := db.Where("id = ? AND user_id = ?", id, uid).First(&s).Error
	return s, err
}

func GetFindingsForScan(uid int64, scanID uint) ([]Finding, error) {
	var findings []Finding
	err := db.Where("scan_id = ? AND user_id = ?", scanID, uid).Order("severity asc").Find(&findings).Error
	return findings, err
}

func GetScanFindingStats(uid int64, id uint) (map[string]int64, error) {
	stats := map[string]int64{
		"critical": 0,
		"high":     0,
		"medium":   0,
		"low":      0,
		"info":     0,
	}
	for _, sev := range []string{"critical", "high", "medium", "low", "info"} {
		var count int64
		db.Model(&Finding{}).Where("scan_id = ? AND user_id = ? AND severity = ?", id, uid, sev).Count(&count)
		stats[sev] = count
	}
	return stats, nil
}

func UpdateScanTaskProgress(scanID uint, status string, progress int) error {
	if scanID == 0 {
		return nil
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	return db.Model(&Scan{}).Where("id = ?", scanID).Updates(map[string]interface{}{
		"status":   strings.TrimSpace(status),
		"progress": progress,
	}).Error
}

func GetScheduledScanTasks(now time.Time) ([]Scan, error) {
	var scans []Scan
	err := db.Where("status = ? AND scheduled_at <= ?", "scheduled", now).Find(&scans).Error
	return scans, err
}

func (s *Scan) GetToolList() []string {
	var tools []string
	if s.EnabledTools == "" {
		return tools
	}
	json.Unmarshal([]byte(s.EnabledTools), &tools)
	return tools
}

// UpdateScanOptions persists the serialized ScanOptions JSON for a scan task.
func UpdateScanOptions(id uint, options string) error {
	return db.Model(&Scan{}).Where("id = ?", id).Update("options", options).Error
}

func DeleteScanTask(uid int64, id uint) error {
	tx := db.Begin()
	if err := tx.Where("scan_id = ? AND user_id = ?", id, uid).Delete(&Finding{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Where("id = ? AND user_id = ?", id, uid).Delete(&Scan{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}
