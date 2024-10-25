package db

import (
	"gorm.io/gorm"
	"strings"
	"time"
)

const defaultLanguage = "EN"

type (
	SchemaInfo struct {
		Version uint
	}
	User struct {
		gorm.Model
		TelegramChatId int64           `gorm:"uniqueIndex"`
		Language       string          `gorm:"default:EN;not null"`
		Rewards        []TrackedReward `gorm:"constraint:OnDelete:CASCADE;"`
	}
	TrackedReward struct {
		gorm.Model
		UserID         uint  `gorm:"uniqueIndex:reward_per_user"`
		RewardId       int64 `gorm:"uniqueIndex:reward_per_user"`
		IsMissing      bool  `gorm:"default:false;not null"`
		AvailableSince *time.Time
		LastNotified   *time.Time
	}
)

func (u *User) BeforeSave(tx *gorm.DB) error {
	if u.Language == "" {
		u.Language = defaultLanguage
	}

	u.Language = strings.ToUpper(u.Language)
	return nil
}
