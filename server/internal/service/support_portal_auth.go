package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrPortalAuthInvalid = errors.New("invalid portal authentication")

type PortalAuthService struct {
	repo    *repository.PortalAuthRepository
	inbox   *SupportInboxService
	sender  email.AppSender
	baseURL string
}

func NewPortalAuthService(repo *repository.PortalAuthRepository, inbox *SupportInboxService, sender email.AppSender, baseURL string) *PortalAuthService {
	return &PortalAuthService{repo: repo, inbox: inbox, sender: sender, baseURL: baseURL}
}
func portalSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
func portalHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
func (s *PortalAuthService) enabled(ctx context.Context, workspaceID string) bool {
	if _, err := uuid.Parse(workspaceID); err != nil {
		return false
	}
	_, settings, err := s.inbox.GetInstallation(ctx, workspaceID)
	if err != nil {
		slog.ErrorContext(ctx, "portal settings lookup failed", "workspace_id", workspaceID, "error", err)
		return false
	}
	return settings != nil && settings.PortalEnabled
}

func (s *PortalAuthService) WorkspaceID(ctx context.Context, slug string) (string, error) {
	ws, err := s.repo.WorkspaceBySlug(ctx, slug)
	if err != nil {
		return "", ErrPortalAuthInvalid
	}
	if !s.enabled(ctx, ws.ID) {
		return "", ErrPortalAuthInvalid
	}
	return ws.ID, nil
}

func (s *PortalAuthService) Configuration(ctx context.Context, workspaceID string) (map[string]any, error) {
	if !s.enabled(ctx, workspaceID) {
		return nil, ErrPortalAuthInvalid
	}
	return map[string]any{"enabled": true, "requests_only": true, "intake_enabled": false, "branding": map[string]string{"name": "Support portal"}}, nil
}

func (s *PortalAuthService) Requests(ctx context.Context, workspaceID, identityID, status string) ([]model.SupportPortalRequest, error) {
	return s.repo.ListRequests(ctx, workspaceID, identityID, status)
}

// RequestLink deliberately returns the same result for unknown workspaces and
// emails. No identity is created until the mailbox owner proves possession.
func (s *PortalAuthService) RequestLink(ctx context.Context, workspaceID, address string) {
	parsed, err := mail.ParseAddress(strings.TrimSpace(address))
	if err != nil || parsed.Address != strings.TrimSpace(address) || !s.enabled(ctx, workspaceID) || s.sender == nil {
		return
	}
	address = strings.ToLower(parsed.Address)
	secret, err := portalSecret()
	if err != nil {
		slog.ErrorContext(ctx, "portal link generation failed", "error", err)
		return
	}
	link := &model.PortalMagicLink{ID: uuid.NewString(), WorkspaceID: workspaceID, Email: address, TokenHash: portalHash(secret), ExpiresAt: time.Now().Add(15 * time.Minute)}
	if err := s.repo.CreateLink(ctx, link); err != nil {
		slog.ErrorContext(ctx, "portal link persistence failed", "workspace_id", workspaceID, "error", err)
		return
	}
	ws, err := s.repo.WorkspaceByID(ctx, workspaceID)
	if err != nil {
		slog.ErrorContext(ctx, "portal workspace lookup failed", "workspace_id", workspaceID, "error", err)
		return
	}
	target := strings.TrimRight(s.baseURL, "/") + "/portal/" + url.PathEscape(ws.Slug) + "/callback?token=" + url.QueryEscape(secret)
	if err := s.sender.SendEmail(address, "Sign in to your support portal", "<p>Use this link to sign in (valid for 15 minutes): <a href=\""+html.EscapeString(target)+"\">Sign in</a></p>", "Sign in to your support portal (valid for 15 minutes): "+target); err != nil {
		slog.ErrorContext(ctx, "portal link delivery failed", "workspace_id", workspaceID, "error", err)
	}
}

func (s *PortalAuthService) Exchange(ctx context.Context, workspaceID, secret string) (string, *model.SupportPortalIdentity, error) {
	if !s.enabled(ctx, workspaceID) || len(secret) != 64 {
		return "", nil, ErrPortalAuthInvalid
	}
	if _, err := hex.DecodeString(secret); err != nil {
		return "", nil, ErrPortalAuthInvalid
	}
	sessionSecret, err := portalSecret()
	if err != nil {
		return "", nil, err
	}
	var identity model.SupportPortalIdentity
	_, err = s.repo.ConsumeLink(ctx, workspaceID, portalHash(secret), time.Now(), func(tx *gorm.DB, link *model.PortalMagicLink) (*model.PortalSession, error) {
		candidate := model.SupportPortalIdentity{ID: uuid.NewString(), WorkspaceID: workspaceID, Email: link.Email}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "workspace_id"}, {Name: "email"}}, DoNothing: true}).Create(&candidate).Error; err != nil {
			return nil, err
		}
		if err := tx.Where("workspace_id = ? AND email = ?", workspaceID, link.Email).First(&identity).Error; err != nil {
			return nil, err
		}
		if err := s.repo.AssociateVerifiedEmail(tx, workspaceID, link.Email, identity.ID); err != nil {
			return nil, err
		}
		return &model.PortalSession{ID: uuid.NewString(), WorkspaceID: workspaceID, IdentityID: identity.ID, TokenHash: portalHash(sessionSecret), ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}, nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, ErrPortalAuthInvalid
		}
		return "", nil, fmt.Errorf("exchange portal link: %w", err)
	}
	return sessionSecret, &identity, nil
}
func (s *PortalAuthService) Validate(ctx context.Context, workspaceID, secret string) (*model.SupportPortalIdentity, error) {
	if !s.enabled(ctx, workspaceID) || len(secret) != 64 {
		return nil, ErrPortalAuthInvalid
	}
	if _, err := hex.DecodeString(secret); err != nil {
		return nil, ErrPortalAuthInvalid
	}
	session, err := s.repo.FindSession(ctx, workspaceID, portalHash(secret), time.Now())
	if err != nil {
		return nil, ErrPortalAuthInvalid
	}
	// Identity lookup remains scoped to the verified workspace.
	var identity model.SupportPortalIdentity
	if err := s.repo.FindSessionIdentity(ctx, workspaceID, session.IdentityID, &identity); err != nil {
		return nil, ErrPortalAuthInvalid
	}
	return &identity, nil
}
func (s *PortalAuthService) Logout(ctx context.Context, workspaceID, secret string) {
	if len(secret) == 64 {
		if err := s.repo.RevokeSession(ctx, workspaceID, portalHash(secret), time.Now()); err != nil {
			slog.ErrorContext(ctx, "portal session revocation failed", "workspace_id", workspaceID, "error", err)
		}
	}
}
