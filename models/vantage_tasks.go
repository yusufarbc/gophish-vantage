package models

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
)

// ToolList is a JSON-backed list of enabled scanner tools per task.
type ToolList []string

func (t ToolList) Value() (driver.Value, error) {
	if len(t) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal([]string(t))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (t *ToolList) Scan(value interface{}) error {
	if value == nil {
		*t = ToolList{}
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		*t = ToolList{}
		return nil
	}
	if len(raw) == 0 {
		*t = ToolList{}
		return nil
	}
	var parsed []string
	if err := json.Unmarshal(raw, &parsed); err != nil {
		*t = ToolList{}
		return nil
	}
	*t = ToolList(parsed)
	return nil
}

func CreateScanTask(uid int64, name, target, iface, mode string, tools []string) (Scan, error) {
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
	s := Scan{
		UserID:            uid,
		Name:              strings.TrimSpace(name),
		Target:            strings.TrimSpace(target),
		ToolName:          "task",
		EnabledTools:      ToolList(clean),
		OutboundInterface: strings.TrimSpace(iface),
		Mode:              strings.TrimSpace(mode),
		Status:            "queued",
		Progress:          0,
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
