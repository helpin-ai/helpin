package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

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

// Get fetches a user's workspace-level notification preferences (team_id IS NULL).
// Returns a default preference if none exists.
func (r *NotificationPreferenceRepository) Get(ctx context.Context, userID, workspaceID string) (*model.NotificationPreference, error) {
	var pref model.NotificationPreference
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND workspace_id = ? AND team_id IS NULL", userID, workspaceID).
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

// GetForTeam fetches a user's team-level notification preferences.
// Returns nil if no team-level override exists.
func (r *NotificationPreferenceRepository) GetForTeam(ctx context.Context, userID, workspaceID, teamID string) (*model.NotificationPreference, error) {
	var pref model.NotificationPreference
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND workspace_id = ? AND team_id = ?", userID, workspaceID, teamID).
		First(&pref).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get team notification preference: %w", err)
	}
	return &pref, nil
}

// Upsert creates or updates notification preferences.
func (r *NotificationPreferenceRepository) Upsert(ctx context.Context, pref *model.NotificationPreference) error {
	if pref.ID != "" {
		return r.db.WithContext(ctx).Save(pref).Error
	}

	// For new records, check if one already exists
	var existing model.NotificationPreference
	query := r.db.WithContext(ctx).Where("user_id = ? AND workspace_id = ?", pref.UserID, pref.WorkspaceID)
	if pref.TeamID != nil {
		query = query.Where("team_id = ?", *pref.TeamID)
	} else {
		query = query.Where("team_id IS NULL")
	}

	err := query.First(&existing).Error
	if err == nil {
		// Update existing
		pref.ID = existing.ID
		return r.db.WithContext(ctx).Save(pref).Error
	}
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(pref).Error
	}
	return fmt.Errorf("upsert notification preference: %w", err)
}

// DeleteForTeam removes a team-level notification preference (reset to workspace defaults).
func (r *NotificationPreferenceRepository) DeleteForTeam(ctx context.Context, userID, workspaceID, teamID string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND workspace_id = ? AND team_id = ?", userID, workspaceID, teamID).
		Delete(&model.NotificationPreference{}).Error
}

// resolveChannelPref checks a preference's channel_preferences for a category+channel value.
// Returns (value, found).
func resolveChannelPref(pref *model.NotificationPreference, category, channel string) (bool, bool) {
	if pref == nil || pref.ChannelPreferences == nil {
		return false, false
	}
	catPref, ok := pref.ChannelPreferences[category]
	if !ok {
		return false, false
	}
	channelMap, ok := catPref.(map[string]any)
	if !ok {
		return false, false
	}
	enabled, ok := channelMap[channel]
	if !ok {
		return false, false
	}
	b, ok := enabled.(bool)
	if !ok {
		return false, false
	}
	return b, true
}

// ShouldNotify checks if a user should receive a notification for a given event type and channel.
// It checks team-level preferences first (if teamID is provided), then falls back to workspace-level.
func (r *NotificationPreferenceRepository) ShouldNotify(ctx context.Context, userID, workspaceID, eventType, channel, teamID string) (bool, error) {
	wsPref, err := r.Get(ctx, userID, workspaceID)
	if err != nil {
		return false, err
	}

	// DND check (workspace-level only)
	if wsPref.DoNotDisturb {
		return false, nil
	}

	// Resolve category from event type
	category := model.EventTypeToCategory[eventType]

	// Check team-level preference first
	if teamID != "" && category != "" {
		teamPref, err := r.GetForTeam(ctx, userID, workspaceID, teamID)
		if err != nil {
			// Log but don't fail — fall through to workspace-level
			_ = err
		}
		if teamPref != nil {
			if val, found := resolveChannelPref(teamPref, category, channel); found {
				return val, nil
			}
		}
	}

	// Check workspace-level category preference
	if category != "" {
		if val, found := resolveChannelPref(wsPref, category, channel); found {
			return val, nil
		}
	}

	// Legacy: check by exact event type (backwards compatibility)
	if val, found := resolveChannelPref(wsPref, eventType, channel); found {
		return val, nil
	}

	// Default: in_app is always on, email follows EmailEnabled
	if channel == "in_app" {
		return true, nil
	}
	if channel == "email" {
		return wsPref.EmailEnabled, nil
	}

	return true, nil
}
