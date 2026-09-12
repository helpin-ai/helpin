package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// SupportPortalRepository owns portal-only persistence. It intentionally
// references support_conversations instead of introducing a ticket table.
type SupportPortalRepository struct{ db *gorm.DB }

func NewSupportPortalRepository(db *gorm.DB) *SupportPortalRepository {
	return &SupportPortalRepository{db: db}
}

func (r *SupportPortalRepository) FindIdentityByEmail(ctx context.Context, workspaceID, email string) (*model.SupportPortalIdentity, error) {
	var identity model.SupportPortalIdentity
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND email = ?", workspaceID, strings.ToLower(strings.TrimSpace(email))).First(&identity).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find portal identity: %w", err)
	}
	return &identity, nil
}

func (r *SupportPortalRepository) CreateIdentity(ctx context.Context, identity *model.SupportPortalIdentity) error {
	identity.Email = strings.ToLower(strings.TrimSpace(identity.Email))
	if err := r.db.WithContext(ctx).Create(identity).Error; err != nil {
		return fmt.Errorf("create portal identity: %w", err)
	}
	return nil
}

func (r *SupportPortalRepository) FindReference(ctx context.Context, workspaceID, conversationID string) (*model.SupportPortalRequestReference, error) {
	var reference model.SupportPortalRequestReference
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).First(&reference).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find portal reference: %w", err)
	}
	return &reference, nil
}

func (r *SupportPortalRepository) FindReferenceForIdentity(ctx context.Context, workspaceID, reference, identityID string) (*model.SupportPortalRequestReference, error) {
	var item model.SupportPortalRequestReference
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND reference = ? AND portal_identity_id = ?", workspaceID, reference, identityID).First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find portal request reference: %w", err)
	}
	return &item, nil
}

func (r *SupportPortalRepository) CreateReference(ctx context.Context, reference *model.SupportPortalRequestReference) error {
	if err := r.db.WithContext(ctx).Create(reference).Error; err != nil {
		return fmt.Errorf("create portal request reference: %w", err)
	}
	return nil
}

func (r *SupportPortalRepository) FindVisibleConversation(ctx context.Context, workspaceID, conversationID string) (*model.SupportConversation, error) {
	var conversation model.SupportConversation
	err := r.db.WithContext(ctx).Where(`workspace_id = ? AND id = ? AND portal_visible = true AND status <> ? AND channel <> ? AND source <> ? AND deleted_at IS NULL`, workspaceID, conversationID, model.SupportConversationStatusSpam, "internal", "internal").First(&conversation).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find visible portal conversation: %w", err)
	}
	return &conversation, nil
}

func (r *SupportPortalRepository) CreateAuditEvent(ctx context.Context, event *model.SupportPortalAuditEvent) error {
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("create portal audit event: %w", err)
	}
	return nil
}

func (r *SupportPortalRepository) ListAuditEvents(ctx context.Context, workspaceID, conversationID string) ([]model.SupportPortalAuditEvent, error) {
	var events []model.SupportPortalAuditEvent
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).Order("occurred_at DESC").Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list portal audit events: %w", err)
	}
	return events, nil
}
