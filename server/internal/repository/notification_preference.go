package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// NotificationPreferenceRepository handles notification preference operations.
type NotificationPreferenceRepository struct {
	db *gorm.DB
}

// NewNotificationPreferenceRepository creates a new notification preference repository.
func NewNotificationPreferenceRepository(db *gorm.DB) *NotificationPreferenceRepository {
	return &NotificationPreferenceRepository{db: db}
}

// Get fetches a user's notification preferences for a workspace.
// Returns a default preference if none exists.
func (r *NotificationPreferenceRepository) Get(ctx context.Context, userID, workspaceID string) (*model.NotificationPreference, error) {
	var pref model.NotificationPreference
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND workspace_id = ?", userID, workspaceID).
		First(&pref).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &model.NotificationPreference{
				UserID:               userID,
				WorkspaceID:          workspaceID,
				EmailEnabled:         true,
				EmailDigestFrequency: "daily",
				EmailDigestTime:      "09:00",
				EmailDigestDay:       1,
				Timezone:             "UTC",
				ChannelPreferences:   model.JSONB{},
				BadgeMode:            "all",
			}, nil
		}
		return nil, fmt.Errorf("get notification preference: %w", err)
	}
	return &pref, nil
}

// Upsert creates or updates notification preferences.
func (r *NotificationPreferenceRepository) Upsert(ctx context.Context, pref *model.NotificationPreference) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "workspace_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"do_not_disturb", "dnd_until", "email_enabled",
				"email_digest_frequency", "email_digest_time", "email_digest_day",
				"timezone", "channel_preferences", "badge_mode", "updated_at",
			}),
		}).
		Create(pref).Error
}

// ShouldNotify checks if a user should receive a notification for a given event type and channel.
func (r *NotificationPreferenceRepository) ShouldNotify(ctx context.Context, userID, workspaceID, eventType, channel string) (bool, error) {
	pref, err := r.Get(ctx, userID, workspaceID)
	if err != nil {
		return false, err
	}

	// DND check
	if pref.DoNotDisturb {
		return false, nil
	}

	// Check channel preferences
	if pref.ChannelPreferences != nil {
		if eventPref, ok := pref.ChannelPreferences[eventType]; ok {
			if channelMap, ok := eventPref.(map[string]interface{}); ok {
				if enabled, ok := channelMap[channel]; ok {
					if b, ok := enabled.(bool); ok {
						return b, nil
					}
				}
			}
		}
	}

	// Default: in_app is always on, email follows EmailEnabled
	if channel == "in_app" {
		return true, nil
	}
	if channel == "email" {
		return pref.EmailEnabled, nil
	}

	return true, nil
}
