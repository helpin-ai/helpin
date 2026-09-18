package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DemoConfig describes the public read-only demo login. The feature is off
// while ViewerEmail is empty.
type DemoConfig struct {
	// ViewerEmail is the shared viewer account visitors are signed in as.
	ViewerEmail string
	// RequireEmail makes the visitor's own email mandatory.
	RequireEmail bool
	// LeadWebhookURL, when set, receives a JSON POST per captured visitor email.
	LeadWebhookURL string
}

const demoLeadWebhookTimeout = 5 * time.Second

// ConfigureDemo enables or disables the demo login.
func (s *AuthService) ConfigureDemo(cfg DemoConfig) {
	cfg.ViewerEmail = normalizeAuthEmail(cfg.ViewerEmail)
	cfg.LeadWebhookURL = strings.TrimSpace(cfg.LeadWebhookURL)
	s.demo = cfg
	if s.demoHTTPClient == nil {
		s.demoHTTPClient = &http.Client{Timeout: demoLeadWebhookTimeout}
	}
}

// DemoEnabled reports whether POST /api/auth/demo is available.
func (s *AuthService) DemoEnabled() bool {
	return s.demo.ViewerEmail != ""
}

// DemoRequiresEmail reports whether visitors must supply an email.
func (s *AuthService) DemoRequiresEmail() bool {
	return s.DemoEnabled() && s.demo.RequireEmail
}

// IsDemoUser reports whether the email belongs to the shared demo viewer.
func (s *AuthService) IsDemoUser(email string) bool {
	return s.DemoEnabled() && normalizeAuthEmail(email) == s.demo.ViewerEmail
}

// DemoSignin issues a token pair for the shared demo viewer account. The
// visitor never authenticates; the account itself must be a plain viewer.
func (s *AuthService) DemoSignin(ctx context.Context, req model.DemoSigninRequest) (*model.SigninResponse, error) {
	if !s.DemoEnabled() {
		return nil, ErrDemoDisabled
	}

	visitorEmail := normalizeAuthEmail(req.Email)
	if visitorEmail == "" && s.demo.RequireEmail {
		return nil, fmt.Errorf("%w: email is required", ErrBadRequest)
	}
	if visitorEmail != "" && !looksLikeEmail(visitorEmail) {
		return nil, fmt.Errorf("%w: email is invalid", ErrBadRequest)
	}

	user, err := s.userRepo.GetByEmail(ctx, s.demo.ViewerEmail)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load demo viewer", "email", s.demo.ViewerEmail, "error", err)
		return nil, fmt.Errorf("find demo user: %w", err)
	}
	if user == nil {
		s.logger.ErrorContext(ctx, "demo viewer account does not exist", "email", s.demo.ViewerEmail)
		return nil, ErrDemoDisabled
	}
	if user.IsPlatformAdmin || user.TOTPVerified {
		s.logger.ErrorContext(ctx, "demo viewer account must not be a platform admin or use 2fa", "user_id", user.ID)
		return nil, ErrDemoDisabled
	}

	accessToken, refreshToken, err := s.generateTokenPairForUser(user, false, false)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to generate demo tokens", "user_id", user.ID, "error", err)
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	if visitorEmail != "" && s.demo.LeadWebhookURL != "" {
		go s.postDemoLead(visitorEmail)
	}

	s.logger.InfoContext(ctx, "demo viewer signed in", "user_id", user.ID, "lead_captured", visitorEmail != "")

	profile := toUserProfile(user)
	return &model.SigninResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         &profile,
	}, nil
}

func (s *AuthService) postDemoLead(email string) {
	payload, err := json.Marshal(map[string]string{
		"email":       email,
		"source":      "demo",
		"captured_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), demoLeadWebhookTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.demo.LeadWebhookURL, bytes.NewReader(payload))
	if err != nil {
		s.logger.Warn("demo lead webhook request build failed", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.demoHTTPClient.Do(req)
	if err != nil {
		s.logger.Warn("demo lead webhook failed", "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		s.logger.Warn("demo lead webhook rejected", "status", resp.StatusCode)
	}
}

func looksLikeEmail(v string) bool {
	at := strings.Index(v, "@")
	return at > 0 && at < len(v)-1 && !strings.ContainsAny(v, " \t\r\n") && strings.Contains(v[at+1:], ".")
}
