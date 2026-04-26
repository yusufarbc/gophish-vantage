package models

// CheckDBHealth returns true if the database connection is alive.
// Used by the /api/health endpoint.
func CheckDBHealth() bool {
	if db == nil {
		return false
	}
	return db.DB().Ping() == nil
}

// GetQueuedMailCount returns the count of mail logs that are pending delivery.
func GetQueuedMailCount() (int, error) {
	var count int
	err := db.Model(&MailLog{}).Where("processing = ?", false).Count(&count).Error
	return count, err
}
