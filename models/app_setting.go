package models

import "time"

// AppSetting stores small persistent key/value settings in SQLite.
// It is intentionally generic to avoid adding new tables for every tiny feature.
type AppSetting struct {
	Key       string    `gorm:"primaryKey;size:128" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}
