package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrPortalRequestNotFound        = errors.New("portal request not found")
	ErrPortalConversationNotVisible = errors.New("conversation is not portal-visible")
)

// SupportPortalService creates and reads portal projections from canonical
// conversations. Its APIs accept a portal identity explicitly so future auth
// adapters cannot accidentally turn an opaque reference into authorization.
type SupportPortalService struct {
	repo *repository.SupportPortalRepository
}

func NewSupportPortalService(repo *repository.SupportPortalRepository) *SupportPortalService {
	return &SupportPortalService{repo: repo}
}

func (s *SupportPortalService) FindOrCreateIdentity(ctx context.Context, workspaceID, email string, displayName *string) (*model.SupportPortalIdentity, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if workspaceID == "" || email == "" {
		return nil, fmt.Errorf("workspace_id and email are required")
	}
	identity, err := s.repo.FindIdentityByEmail(ctx, workspaceID, email)
	if err != nil || identity != nil {
		return identity, err
	}
	identity = &model.SupportPortalIdentity{ID: uuid.NewString(), WorkspaceID: workspaceID, Email: email, DisplayName: displayName}
	if err := s.repo.CreateIdentity(ctx, identity); err != nil {
		// A concurrent first login may have created the identity. Return the
		// canonical row instead of treating a stable email as an error.
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return s.repo.FindIdentityByEmail(ctx, workspaceID, email)
		}
		return nil, err
	}
	return identity, nil
}

// EnsureRequestReference returns the stable opaque reference for a visible
// conversation. It never exposes a conversation ID and cannot create a
// reference for internal, spam, deleted, or hidden conversations.
func (s *SupportPortalService) EnsureRequestReference(ctx context.Context, workspaceID, conversationID, portalIdentityID string) (*model.SupportPortalRequestReference, error) {
	if strings.TrimSpace(portalIdentityID) == "" {
		return nil, fmt.Errorf("portal_identity_id is required")
	}
	conversation, err := s.repo.FindVisibleConversation(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, ErrPortalConversationNotVisible
	}
	if existing, err := s.repo.FindReference(ctx, workspaceID, conversationID); err != nil || existing != nil {
		if existing != nil && existing.PortalIdentityID != portalIdentityID {
			return nil, ErrPortalRequestNotFound
		}
		return existing, err
	}

	for attempts := 0; attempts < 3; attempts++ {
		reference, err := newPortalReference()
		if err != nil {
			return nil, err
		}
		item := &model.SupportPortalRequestReference{ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: conversationID, PortalIdentityID: portalIdentityID, Reference: reference}
		if err := s.repo.CreateReference(ctx, item); err == nil {
			if err := s.RecordAuditEvent(ctx, workspaceID, conversationID, portalIdentityID, model.SupportPortalActorSystem, nil, model.SupportPortalAuditRequestCreated, "{}"); err != nil {
				return nil, err
			}
			return item, nil
		}
	}
	return nil, fmt.Errorf("generate unique portal request reference")
}

// GetRequestProjection authorizes the reference against the portal identity
// before deriving a minimal customer-safe request response.
func (s *SupportPortalService) GetRequestProjection(ctx context.Context, workspaceID, reference, portalIdentityID string) (*model.SupportPortalRequest, error) {
	item, err := s.repo.FindReferenceForIdentity(ctx, workspaceID, reference, portalIdentityID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrPortalRequestNotFound
	}
	conversation, err := s.repo.FindVisibleConversation(ctx, workspaceID, item.ConversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, ErrPortalRequestNotFound
	}
	return projectPortalRequest(*conversation, item.Reference), nil
}

func (s *SupportPortalService) RecordAuditEvent(ctx context.Context, workspaceID, conversationID, portalIdentityID, actorType string, actorUserID *string, eventType, metadata string) error {
	if workspaceID == "" || conversationID == "" || eventType == "" {
		return fmt.Errorf("workspace_id, conversation_id, and event_type are required")
	}
	var identityID *string
	if portalIdentityID != "" {
		identityID = &portalIdentityID
	}
	if strings.TrimSpace(metadata) == "" {
		metadata = "{}"
	}
	return s.repo.CreateAuditEvent(ctx, &model.SupportPortalAuditEvent{ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: conversationID, PortalIdentityID: identityID, ActorType: actorType, ActorUserID: actorUserID, EventType: eventType, Metadata: metadata, OccurredAt: time.Now().UTC()})
}

func projectPortalRequest(conversation model.SupportConversation, reference string) *model.SupportPortalRequest {
	lastActivity := conversation.ListLastActivityAt
	if lastActivity == nil {
		lastActivity = conversation.LastPublicMessageAt
	}
	return &model.SupportPortalRequest{Reference: reference, Subject: conversation.Subject, Status: model.PortalRequestStatus(conversation.Status), CreatedAt: conversation.CreatedAt, UpdatedAt: conversation.UpdatedAt, LastActivityAt: lastActivity, ResolvedAt: conversation.ResolvedAt}
}

func newPortalReference() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("read portal reference entropy: %w", err)
	}
	return "req_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)), nil
}
