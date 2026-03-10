package service

import (
	"context"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// UserNotificationSettingsService manages account-level notification settings.
type UserNotificationSettingsService struct {
	repo   *repository.UserNotificationSettingsRepository
	logger *slog.Logger
}

// NewUserNotificationSettingsService creates a new service.
func NewUserNotificationSettingsService(repo *repository.UserNotificationSettingsRepository) *UserNotificationSettingsService {
	return &UserNotificationSettingsService{
		repo:   repo,
		logger: slog.Default().With("service", "user_notification_settings"),
	}
}

// Get returns account-level notification settings for a user.
func (s *UserNotificationSettingsService) Get(ctx context.Context, userID string) (*model.UserNotificationSettings, error) {
	return s.repo.Get(ctx, userID)
}

// Update applies partial updates to account-level notification settings.
func (s *UserNotificationSettingsService) Update(ctx context.Context, userID string, req model.UpdateUserNotificationSettingsRequest) (*model.UserNotificationSettings, error) {
	settings, err := s.repo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}

	settings.UserID = userID

	if req.EmailEnabled != nil {
		settings.EmailEnabled = *req.EmailEnabled
	}
	if req.EmailDigestFrequency != nil {
		settings.EmailDigestFrequency = *req.EmailDigestFrequency
	}
	if req.EmailDigestTime != nil {
		settings.EmailDigestTime = *req.EmailDigestTime
	}
	if req.EmailDigestDay != nil {
		settings.EmailDigestDay = *req.EmailDigestDay
	}
	if req.DoNotDisturb != nil {
		settings.DoNotDisturb = *req.DoNotDisturb
	}
	if req.DNDUntil != nil {
		settings.DNDUntil = req.DNDUntil
	}
	if req.BadgeMode != nil {
		settings.BadgeMode = *req.BadgeMode
	}
	if req.Timezone != nil {
		settings.Timezone = *req.Timezone
	}

	if err := s.repo.Upsert(ctx, settings); err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "account notification settings updated", "user_id", userID)
	return settings, nil
}
