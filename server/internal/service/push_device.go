package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PushDeviceRepo is the subset of push device persistence the service needs.
// Defined consumer-side so tests can supply a fake implementation.
type PushDeviceRepo interface {
	UpsertByToken(ctx context.Context, device *model.PushDevice) error
	DeleteByToken(ctx context.Context, userID, token string) error
}

var validPushPlatforms = map[string]bool{
	"ios":     true,
	"android": true,
}

// PushDeviceService manages mobile push notification device registrations.
type PushDeviceService struct {
	repo   PushDeviceRepo
	logger *slog.Logger
}

// NewPushDeviceService creates a new service.
func NewPushDeviceService(repo PushDeviceRepo) *PushDeviceService {
	return &PushDeviceService{
		repo:   repo,
		logger: slog.Default().With("service", "push_device"),
	}
}

// Register validates and upserts a push device registration for userID.
// Re-registering an existing token moves the device to the current user,
// since phones change owners/accounts.
func (s *PushDeviceService) Register(ctx context.Context, userID string, req model.RegisterPushDeviceRequest) (*model.PushDevice, error) {
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if !validPushPlatforms[platform] {
		return nil, fmt.Errorf("invalid platform %q: must be ios or android", req.Platform)
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	device := &model.PushDevice{
		UserID:     userID,
		Platform:   platform,
		Token:      token,
		AppVersion: strings.TrimSpace(req.AppVersion),
		LastSeenAt: time.Now(),
	}

	if err := s.repo.UpsertByToken(ctx, device); err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "push device registered", "user_id", userID, "platform", platform)
	return device, nil
}

// Unregister removes a device registration, scoped to the calling user so a
// user may only unregister their own device.
func (s *PushDeviceService) Unregister(ctx context.Context, userID, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is required")
	}

	if err := s.repo.DeleteByToken(ctx, userID, token); err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "push device unregistered", "user_id", userID)
	return nil
}
