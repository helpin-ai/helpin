package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	testEmailMinInterval = 30 * time.Second
	testEmailWindow      = time.Hour
	testEmailPerWindow   = 5
	testEmailSubject     = "Helpin test email"
	// testEmailSendFailed is the only failure text returned or stored; raw mail
	// server responses can contain hostnames or account details.
	testEmailSendFailed = "The mail server did not accept the message. Check the SMTP settings and the API logs."
)

var (
	// ErrTestEmailRateLimited rejects test emails sent too often by one user.
	ErrTestEmailRateLimited = errors.New("too many test emails; wait a minute and try again")
	// ErrTestEmailNoRecipient rejects accounts without an email address.
	ErrTestEmailNoRecipient = errors.New("your account has no email address")
)

// TestEmailSender delivers the test message through the application mail sender.
type TestEmailSender interface {
	SendEmail(to, subject, htmlBody, textBody string) error
}

// capabilityUserStore resolves the requesting user's own address.
type capabilityUserStore interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
}

type testEmailSender struct {
	sender  TestEmailSender
	users   capabilityUserStore
	limiter *testEmailLimiter
	// state reports live application email settings when they can change at
	// runtime (settings saved in the app); nil uses the static config.
	state func(context.Context) (bool, string)
}

// SetTestEmail enables SendTestEmail. A nil sender reports that application
// email is not configured.
func (s *CapabilityService) SetTestEmail(sender TestEmailSender, users capabilityUserStore) *CapabilityService {
	var state func(context.Context) (bool, string)
	if s.email != nil {
		state = s.email.state
	}
	s.email = &testEmailSender{sender: sender, users: users, limiter: newTestEmailLimiter(time.Now), state: state}
	return s
}

// SetAppEmailState makes email_outbound follow application email settings
// that can change without a restart. state returns whether email is
// configured and a fingerprint of its non-secret settings.
func (s *CapabilityService) SetAppEmailState(state func(context.Context) (bool, string)) *CapabilityService {
	if s.email == nil {
		s.email = &testEmailSender{limiter: newTestEmailLimiter(time.Now)}
	}
	s.email.state = state
	return s
}

// appEmail returns whether application email is configured and its
// configuration fingerprint.
func (s *CapabilityService) appEmail(ctx context.Context) (bool, string) {
	if s.email != nil && s.email.state != nil {
		return s.email.state(ctx)
	}
	return s.cfg.AppEmailConfigured, s.cfg.AppEmailFingerprint
}

// appEmailEditable reports whether application email can be set up in the app.
func (s *CapabilityService) appEmailEditable() bool {
	return s.email != nil && s.email.state != nil
}

// SendTestEmail sends a short message to the requesting user's own address and
// records the outcome for the email_outbound capability. Recipients are never
// caller-supplied, so the endpoint cannot be used to send mail to others.
func (s *CapabilityService) SendTestEmail(ctx context.Context, workspaceID, userID string) (model.TestEmailResult, error) {
	configured, fingerprint := s.appEmail(ctx)
	if s.email == nil || s.email.sender == nil || !configured {
		return model.TestEmailResult{OK: false, Error: "Application email is not configured on this server."}, nil
	}
	if !s.email.limiter.allow(userID) {
		return model.TestEmailResult{}, ErrTestEmailRateLimited
	}
	user, err := s.email.users.GetByID(ctx, userID)
	if err != nil {
		return model.TestEmailResult{}, err
	}
	if user == nil || strings.TrimSpace(user.Email) == "" {
		return model.TestEmailResult{}, ErrTestEmailNoRecipient
	}
	recipient := strings.TrimSpace(user.Email)
	text := "This is a test email from your Helpin installation.\n\nApplication email is working: invitations, password resets and notifications can be delivered.\n"
	body := "<p>This is a test email from your Helpin installation.</p><p>Application email is working: invitations, password resets and notifications can be delivered.</p>"
	sendErr := s.email.sender.SendEmail(recipient, testEmailSubject, body, text)
	check := &model.InstanceCapabilityCheck{
		Key: model.CapabilityKeyEmailOutbound, OK: sendErr == nil, ConfigFingerprint: fingerprint,
		CheckedBy: &userID, CheckedAt: time.Now().UTC(),
	}
	result := model.TestEmailResult{OK: sendErr == nil, Recipient: recipient}
	if sendErr != nil {
		slog.ErrorContext(ctx, "test email delivery failed", "error", sendErr, "workspace_id", workspaceID, "user_id", userID)
		failure := testEmailSendFailed
		check.Error = &failure
		result.Error = failure
	} else {
		slog.InfoContext(ctx, "test email sent", "workspace_id", workspaceID, "user_id", userID)
	}
	if err := s.evidence.RecordCheck(ctx, check); err != nil {
		slog.ErrorContext(ctx, "record test email result failed", "error", err, "workspace_id", workspaceID)
	}
	return result, nil
}

// testEmailLimiter bounds test emails per user in this process: one per
// testEmailMinInterval and testEmailPerWindow per testEmailWindow.
type testEmailLimiter struct {
	mu   sync.Mutex
	now  func() time.Time
	sent map[string][]time.Time
}

func newTestEmailLimiter(now func() time.Time) *testEmailLimiter {
	return &testEmailLimiter{now: now, sent: make(map[string][]time.Time)}
}

func (l *testEmailLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	recent := l.sent[key][:0]
	for _, at := range l.sent[key] {
		if now.Sub(at) < testEmailWindow {
			recent = append(recent, at)
		}
	}
	if len(recent) >= testEmailPerWindow || (len(recent) > 0 && now.Sub(recent[len(recent)-1]) < testEmailMinInterval) {
		l.sent[key] = recent
		return false
	}
	l.sent[key] = append(recent, now)
	// Drop idle users so the map cannot grow without bound.
	for user, times := range l.sent {
		if len(times) == 0 || now.Sub(times[len(times)-1]) >= testEmailWindow {
			delete(l.sent, user)
		}
	}
	return true
}
