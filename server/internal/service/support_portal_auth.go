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
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrPortalAuthInvalid = errors.New("invalid portal authentication")

var ErrPortalIntakeDisabled = errors.New("portal intake is disabled")
var ErrPortalRequestInvalid = errors.New("subject and description are required")

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
	_, settings, err := s.inbox.GetInstallation(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, ErrPortalAuthInvalid
	}
	return map[string]any{"enabled": true, "requests_only": settings.PortalRequestsOnly, "intake_enabled": settings.PortalIntakeEnabled, "file_uploads_enabled": settings.FileUploadsEnabled, "branding": map[string]string{"name": "Support portal"}}, nil
}

var ErrPortalAttachmentsUnavailable = errors.New("portal file uploads are disabled")
var ErrPortalAttachmentsInvalid = errors.New("one or more attachments are unavailable")

func (s *PortalAuthService) SessionForToken(ctx context.Context, workspaceID, secret string) (*model.PortalSession, error) {
	if _, err := s.Validate(ctx, workspaceID, secret); err != nil {
		return nil, err
	}
	session, err := s.repo.FindSession(ctx, workspaceID, portalHash(secret), time.Now())
	if err != nil {
		return nil, ErrPortalAuthInvalid
	}
	return session, nil
}

func (s *PortalAuthService) AttachmentSession(ctx context.Context, workspaceID, secret string) (*model.PortalSession, error) {
	session, err := s.SessionForToken(ctx, workspaceID, secret)
	if err != nil {
		return nil, err
	}
	_, settings, err := s.inbox.GetInstallation(ctx, workspaceID)
	if err != nil || settings == nil || !settings.FileUploadsEnabled || s.inbox.attachmentService == nil {
		return nil, ErrPortalAttachmentsUnavailable
	}
	return session, nil
}

func (s *PortalAuthService) UploadAttachment(ctx context.Context, workspaceID, identityID, sessionID, reference string, req model.CreateSupportAttachmentRequest) (*model.SupportAttachmentResponse, error) {
	conversationID := ""
	if reference != "" {
		conv, err := s.repo.FindRequest(ctx, workspaceID, identityID, reference)
		if err != nil || conv == nil || conv.AnonymizedAt != nil {
			return nil, ErrPortalRequestNotFound
		}
		conversationID = conv.ID
	}
	return s.inbox.attachmentService.Create(ctx, req, workspaceID, conversationID, "customer", nil, &sessionID)
}

func (s *PortalAuthService) ConfirmAttachment(ctx context.Context, workspaceID, identityID, sessionID, reference, attachmentID string) error {
	conversationID := ""
	if reference != "" {
		conv, err := s.repo.FindRequest(ctx, workspaceID, identityID, reference)
		if err != nil || conv == nil || conv.AnonymizedAt != nil {
			return ErrPortalAttachmentsInvalid
		}
		conversationID = conv.ID
	}
	if !s.inbox.attachmentService.PortalAttachmentOwned(ctx, attachmentID, workspaceID, sessionID, conversationID) {
		return ErrPortalAttachmentsInvalid
	}
	return s.inbox.attachmentService.ConfirmUpload(ctx, attachmentID, "customer", nil, &sessionID)
}

func (s *PortalAuthService) CreateRequest(ctx context.Context, workspaceID string, identity *model.SupportPortalIdentity, subject, description string, attachmentIDs []string, sessionID string) (*model.SupportPortalRequest, error) {
	if identity == nil || identity.WorkspaceID != workspaceID {
		return nil, ErrPortalAuthInvalid
	}
	_, settings, err := s.inbox.GetInstallation(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if settings == nil || !settings.PortalEnabled || !settings.PortalIntakeEnabled {
		return nil, ErrPortalIntakeDisabled
	}
	subject, description = strings.TrimSpace(subject), strings.TrimSpace(description)
	if subject == "" || description == "" || len([]rune(subject)) > 200 {
		return nil, ErrPortalRequestInvalid
	}
	if len(attachmentIDs) > 0 {
		if !settings.FileUploadsEnabled || s.inbox.attachmentService == nil {
			return nil, ErrPortalAttachmentsUnavailable
		}
		if err := s.inbox.attachmentService.ValidatePortalAttachments(ctx, attachmentIDs, workspaceID, sessionID, ""); err != nil {
			return nil, ErrPortalAttachmentsInvalid
		}
	}
	reference, err := newPortalReference()
	if err != nil {
		return nil, err
	}
	mailboxID, mailbox, err := s.inbox.maybeApplyMailboxRoutingForChannel(ctx, workspaceID, nil, true, "portal")
	if err != nil {
		return nil, err
	}
	ownerID, flowState, err := s.inbox.determineMailboxOwner(ctx, workspaceID, mailbox, nil)
	if err != nil {
		return nil, err
	}
	var contactID *string
	if s.inbox.contactRepo != nil {
		contactID = s.inbox.matchOrCreateCRMContact(ctx, workspaceID, &identity.Email, identity.DisplayName)
	}
	conversation, request, err := s.repo.CreateRequest(ctx, workspaceID, identity, subject, description, reference, mailboxID, ownerID, contactID, &flowState, attachmentIDs, sessionID)
	if err != nil {
		return nil, err
	}
	s.inbox.wsPublisher.Publish(websocket.Event{
		Action: "created", Entity: "support_conversation", EntityID: conversation.ID, WorkspaceID: workspaceID,
	})
	if s.inbox.triageService != nil {
		if err := s.inbox.triageService.HydrateConversation(ctx, conversation); err != nil {
			slog.ErrorContext(ctx, "hydrate portal conversation triage", "error", err, "workspace_id", workspaceID, "conversation_id", conversation.ID)
		}
	}
	return request, nil
}

func (s *PortalAuthService) Requests(ctx context.Context, workspaceID, identityID, status string) ([]model.SupportPortalRequest, error) {
	if err := s.repo.ReconcileRequests(ctx, workspaceID, identityID); err != nil {
		return nil, err
	}
	return s.repo.ListRequests(ctx, workspaceID, identityID, status)
}

var ErrPortalReplyUnavailable = errors.New("this request cannot receive replies")

type PortalMessage struct {
	ID          string                   `json:"id"`
	Content     string                   `json:"content"`
	SenderType  string                   `json:"sender_type"`
	SenderName  *string                  `json:"sender_name,omitempty"`
	ViaChannel  string                   `json:"via_channel,omitempty"`
	Attachments []model.WidgetAttachment `json:"attachments,omitempty"`
	CreatedAt   time.Time                `json:"created_at"`
}

type PortalRequestDetail struct {
	Reference      string          `json:"reference"`
	Subject        string          `json:"subject"`
	Status         string          `json:"status"`
	LastActivityAt time.Time       `json:"last_activity_at"`
	CanReply       bool            `json:"can_reply"`
	Messages       []PortalMessage `json:"messages"`
}

func (s *PortalAuthService) RequestDetail(ctx context.Context, workspaceID, identityID, reference string) (*PortalRequestDetail, error) {
	conv, err := s.repo.FindRequest(ctx, workspaceID, identityID, reference)
	if err != nil || conv == nil {
		return nil, err
	}
	messages, err := s.inbox.ListConversationMessages(ctx, workspaceID, conv.ID, false)
	if err != nil {
		return nil, err
	}
	detail := &PortalRequestDetail{Reference: reference, Subject: conv.Subject, Status: model.PortalRequestStatus(conv.Status), LastActivityAt: conv.CreatedAt, CanReply: conv.AnonymizedAt == nil, Messages: []PortalMessage{}}
	for i := range messages {
		msg := &messages[i]
		if !portalMessageVisible(msg) {
			continue
		}
		public := PortalMessage{ID: msg.ID, Content: msg.Content, SenderType: msg.SenderType, SenderName: msg.SenderDisplayName, CreatedAt: msg.CreatedAt}
		if msg.ViaChannel != nil {
			public.ViaChannel = *msg.ViaChannel
		}
		for _, attachment := range msg.Attachments {
			if attachment.URL == "" || attachment.ProcessingStatus != "" {
				continue
			}
			public.Attachments = append(public.Attachments, model.WidgetAttachment{ID: attachment.ID, FileName: attachment.FileName, FileType: attachment.FileType, FileSize: attachment.FileSize, URL: attachment.URL})
		}
		detail.Messages = append(detail.Messages, public)
		if msg.CreatedAt.After(detail.LastActivityAt) {
			detail.LastActivityAt = msg.CreatedAt
		}
	}
	return detail, nil
}

func portalMessageVisible(msg *model.SupportMessage) bool {
	return !msg.IsInternal && msg.DeliveryMode() != model.SupportDeliveryEmailOnly && msg.MessageType == "reply" &&
		(msg.SenderType == "customer" || msg.SenderType == "user" || msg.SenderType == "agent" || msg.SenderType == "ai")
}

func (s *PortalAuthService) Reply(ctx context.Context, workspaceID string, identity *model.SupportPortalIdentity, reference, content string, attachmentIDs []string, sessionID string) error {
	conv, err := s.repo.FindRequest(ctx, workspaceID, identity.ID, reference)
	if err != nil {
		return err
	}
	if conv == nil {
		return ErrPortalRequestNotFound
	}
	if conv.AnonymizedAt != nil {
		return ErrPortalReplyUnavailable
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("reply is required")
	}
	if len(attachmentIDs) > 0 {
		_, settings, settingsErr := s.inbox.GetInstallation(ctx, workspaceID)
		if settingsErr != nil || settings == nil || !settings.FileUploadsEnabled || s.inbox.attachmentService == nil {
			return ErrPortalAttachmentsUnavailable
		}
		if err := s.inbox.attachmentService.ValidatePortalAttachments(ctx, attachmentIDs, workspaceID, sessionID, conv.ID); err != nil {
			return ErrPortalAttachmentsInvalid
		}
	}
	msg, err := s.inbox.CreateConversationMessage(context.WithValue(ctx, portalReplySourceKey{}, true), workspaceID, conv.ID, model.CreateMessageRequest{Content: content, MessageType: "reply"}, "customer", nil, nil, identity.DisplayName)
	if err == nil && len(attachmentIDs) > 0 {
		if err = s.inbox.attachmentService.LinkPortalAttachments(ctx, attachmentIDs, workspaceID, sessionID, conv.ID, msg.ID); err != nil {
			return ErrPortalAttachmentsInvalid
		}
	}
	return err
}

type portalReplySourceKey struct{}

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
