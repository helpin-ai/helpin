package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// UserNotificationSettingsRepository handles account-level notification settings.
type UserNotificationSettingsRepository struct {
	db *gorm.DB
}

// NewUserNotificationSettingsRepository creates a new repository.
func NewUserNotificationSettingsRepository(db *gorm.DB) *UserNotificationSettingsRepository {
	return &UserNotificationSettingsRepository{db: db}
}

// Get fetches a user's account-level notification settings.
// Returns default settings if no row exists.
func (r *UserNotificationSettingsRepository) Get(ctx context.Context, userID string) (*model.UserNotificationSettings, error) {
	var settings model.UserNotificationSettings
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&settings).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &model.UserNotificationSettings{
				UserID:               userID,
				EmailEnabled:         true,
				EmailDigestFrequency: "daily",
				EmailDigestTime:      "09:00",
				EmailDigestDay:       1,
				BadgeMode:            "all",
				Timezone:             "UTC",
			}, nil
		}
		return nil, fmt.Errorf("get user notification settings: %w", err)
	}
	return &settings, nil
}

// Upsert creates or updates account-level notification settings.
func (r *UserNotificationSettingsRepository) Upsert(ctx context.Context, settings *model.UserNotificationSettings) error {
	// Maps preserve explicit zero values; GORM struct Create applies default tags to them.
	now := time.Now()
	values := map[string]any{
		"user_id": settings.UserID, "email_enabled": settings.EmailEnabled,
		"email_digest_frequency": settings.EmailDigestFrequency, "email_digest_time": settings.EmailDigestTime,
		"email_digest_day": settings.EmailDigestDay, "do_not_disturb": settings.DoNotDisturb,
		"dnd_until": settings.DNDUntil, "badge_mode": settings.BadgeMode, "timezone": settings.Timezone,
		"created_at": now, "updated_at": now,
	}
	if err := r.db.WithContext(ctx).Model(&model.UserNotificationSettings{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"email_enabled", "email_digest_frequency", "email_digest_time", "email_digest_day", "do_not_disturb", "dnd_until", "badge_mode", "timezone", "updated_at"}),
	}).Create(values).Error; err != nil {
		return fmt.Errorf("upsert user notification settings: %w", err)
	}
	return r.db.WithContext(ctx).Where("user_id = ?", settings.UserID).First(settings).Error
}

// InitializeTimezone records defaults only once; concurrent requests cannot replace saved preferences.
func (r *UserNotificationSettingsRepository) InitializeTimezone(ctx context.Context, userID, timezone string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.UserNotificationSettings{}).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoNothing: true}).Create(map[string]any{
		"user_id": userID, "timezone": timezone, "email_enabled": true, "email_digest_frequency": "daily", "email_digest_time": "09:00", "email_digest_day": 1, "do_not_disturb": false, "badge_mode": "all", "created_at": now, "updated_at": now,
	}).Error
}
