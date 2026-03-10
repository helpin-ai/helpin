package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

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
	if settings.ID != "" {
		return r.db.WithContext(ctx).Save(settings).Error
	}

	var existing model.UserNotificationSettings
	err := r.db.WithContext(ctx).Where("user_id = ?", settings.UserID).First(&existing).Error
	if err == nil {
		settings.ID = existing.ID
		return r.db.WithContext(ctx).Save(settings).Error
	}
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(settings).Error
	}
	return fmt.Errorf("upsert user notification settings: %w", err)
}
