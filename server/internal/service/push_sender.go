package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"firebase.google.com/go/v4/messaging"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PushNotification is a platform-agnostic push message payload delivered to
// a single device via FCMClient.
type PushNotification struct {
	Title string
	Body  string
	// Data carries structured payload fields consumed by the mobile client's
	// tap handler, e.g. "workspace_slug", "conversation_id", "deep_link".
	Data map[string]string
}

// FCMClient sends a single push message to a device token. Implemented by
// firebaseFCMClient (production, backed by Firebase Cloud Messaging) and by
// fakes in tests.
type FCMClient interface {
	Send(ctx context.Context, token string, n PushNotification) error
}

// ErrUnregisteredDevice is returned (wrapped) by an FCMClient.Send
// implementation when the receiving device is no longer registered with the
// push provider (app uninstalled, token rotated, etc). PushSenderService
// deletes the offending device registration whenever it sees this error, so
// callers must wrap their provider-specific "unregistered" error with this
// sentinel (see firebaseFCMClient.Send) for cleanup to kick in.
var ErrUnregisteredDevice = errors.New("push: device unregistered")

// PushSenderRepo is the subset of push device persistence PushSenderService
// needs. Defined consumer-side so tests can supply a fake implementation.
type PushSenderRepo interface {
	ListByUserIDs(ctx context.Context, userIDs []string) ([]model.PushDevice, error)
	DeleteByTokenAny(ctx context.Context, token string) error
}

// PushSenderService fans out a push notification to every device registered
// to a set of users. It is nil-safe end to end: a nil *PushSenderService, or
// one constructed with a nil FCMClient (e.g. because FCM_SERVICE_ACCOUNT_JSON
// is unset), makes NotifyUsers a no-op so push delivery degrades gracefully
// wherever it isn't configured, without requiring callers to nil-check.
type PushSenderService struct {
	repo   PushSenderRepo
	client FCMClient
	logger *slog.Logger
}

// NewPushSenderService creates a new service. client may be nil when push is
// not configured; NotifyUsers becomes a no-op in that case.
func NewPushSenderService(repo PushSenderRepo, client FCMClient) *PushSenderService {
	return &PushSenderService{
		repo:   repo,
		client: client,
		logger: slog.Default().With("service", "push_sender"),
	}
}

// NotifyUsers sends n to every device registered to any of userIDs.
// Per-device send failures are logged and do not stop the fan-out to
// remaining devices; a device the provider reports as unregistered is
// deleted so future sends don't keep retrying a dead token. Never logs
// device tokens.
func (s *PushSenderService) NotifyUsers(ctx context.Context, userIDs []string, n PushNotification) {
	if s == nil || s.client == nil || s.repo == nil || len(userIDs) == 0 {
		return
	}

	devices, err := s.repo.ListByUserIDs(ctx, userIDs)
	if err != nil {
		s.logger.ErrorContext(ctx, "list push devices failed", "error", err)
		return
	}

	for _, device := range devices {
		if err := s.client.Send(ctx, device.Token, n); err != nil {
			if errors.Is(err, ErrUnregisteredDevice) {
				s.logger.WarnContext(ctx, "push device unregistered, removing registration",
					"user_id", device.UserID,
					"platform", device.Platform,
				)
				if delErr := s.repo.DeleteByTokenAny(ctx, device.Token); delErr != nil {
					s.logger.ErrorContext(ctx, "delete unregistered push device failed",
						"error", delErr,
						"user_id", device.UserID,
					)
				}
				continue
			}
			s.logger.ErrorContext(ctx, "push send failed",
				"error", err,
				"user_id", device.UserID,
				"platform", device.Platform,
			)
			continue
		}
	}
}

// firebaseFCMClient adapts firebase.google.com/go/v4/messaging.Client to the
// FCMClient interface.
type firebaseFCMClient struct {
	client *messaging.Client
}

// NewFirebaseFCMClient wraps a Firebase Cloud Messaging client for use as an
// FCMClient. Returns nil (typed as FCMClient) is the caller's responsibility
// when FCM is not configured — this constructor always assumes client is
// non-nil.
func NewFirebaseFCMClient(client *messaging.Client) FCMClient {
	return &firebaseFCMClient{client: client}
}

func (f *firebaseFCMClient) Send(ctx context.Context, token string, n PushNotification) error {
	msg := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: n.Title,
			Body:  n.Body,
		},
		Data: n.Data,
	}
	if _, err := f.client.Send(ctx, msg); err != nil {
		if messaging.IsUnregistered(err) {
			return fmt.Errorf("fcm send: %w: %w", ErrUnregisteredDevice, err)
		}
		return fmt.Errorf("fcm send: %w", err)
	}
	return nil
}
