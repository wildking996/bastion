package database

import (
	"bastion/models"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// GetSetting returns a persisted key/value setting.
// ok is false when the key does not exist.
func GetSetting(key string) (value string, ok bool, err error) {
	if DB == nil {
		return "", false, errors.New("database not initialized")
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return "", false, errors.New("empty setting key")
	}

	var s models.AppSetting
	if err := DB.First(&s, "key = ?", key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return s.Value, true, nil
}

// SetSetting persists a key/value setting.
func SetSetting(key, value string) error {
	if DB == nil {
		return errors.New("database not initialized")
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("empty setting key")
	}

	value = strings.TrimSpace(value)
	return DB.Save(&models.AppSetting{Key: key, Value: value}).Error
}

// DeleteSetting removes a persisted setting if it exists.
func DeleteSetting(key string) error {
	if DB == nil {
		return errors.New("database not initialized")
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("empty setting key")
	}

	return DB.Where("key = ?", key).Delete(&models.AppSetting{}).Error
}
