package repository

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PortalAuthRepository struct{ db *gorm.DB }

func NewPortalAuthRepository(db *gorm.DB) *PortalAuthRepository { return &PortalAuthRepository{db: db} }

func (r *PortalAuthRepository) WithTx(tx *gorm.DB) *PortalAuthRepository {
	return &PortalAuthRepository{db: tx}
}

func (r *PortalAuthRepository) CreateIntakeSession(ctx context.Context, session *model.PortalIntakeSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *PortalAuthRepository) FindIntakeSession(ctx context.Context, workspaceID, hash string, now time.Time) (*model.PortalIntakeSession, error) {
	var session model.PortalIntakeSession
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND token_hash = ? AND used_at IS NULL AND expires_at > ?", workspaceID, hash, now).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *PortalAuthRepository) ConsumeIntakeSession(ctx context.Context, workspaceID, hash string, now time.Time, create func(*gorm.DB, *model.PortalIntakeSession) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session model.PortalIntakeSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND token_hash = ? AND used_at IS NULL AND expires_at > ?", workspaceID, hash, now).First(&session).Error; err != nil {
			return err
		}
		if err := create(tx, &session); err != nil {
			return err
		}
		return tx.Model(&session).Update("used_at", now).Error
	})
}

// CreateRequest commits the inbox conversation, customer message and portal reference together.
func (r *PortalAuthRepository) CreateRequest(ctx context.Context, workspaceID string, identity *model.SupportPortalIdentity, subject, description, reference string, mailboxID, ownerID, crmContactID, flowState *string, attachmentIDs []string, sessionID string) (*model.SupportConversation, *model.SupportPortalRequest, error) {
	conversation := &model.SupportConversation{
		ID: uuid.NewString(), WorkspaceID: workspaceID, Subject: subject,
		Status: "open", Priority: "medium", Channel: "portal", Source: "portal",
		CustomerName: identity.DisplayName, CustomerEmail: &identity.Email, PortalVisible: true,
		MailboxID: mailboxID, AssignedUserID: ownerID, CRMContactID: crmContactID, FlowState: flowState,
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		conversationRepo := NewSupportConversationRepository(tx)
		if err := conversationRepo.Create(ctx, conversation); err != nil {
			return err
		}
		message := &model.SupportMessage{
			ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: conversation.ID,
			SenderType: "customer", SenderDisplayName: identity.DisplayName,
			MessageType: "reply", Content: description,
		}
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		if len(attachmentIDs) > 0 {
			if err := NewSupportAttachmentRepository(tx).LinkPortalAttachments(ctx, attachmentIDs, workspaceID, sessionID, "", conversation.ID, message.ID); err != nil {
				return err
			}
		}
		conversation.ApplyMessageProjection(*message)
		if err := tx.Model(conversation).Updates(map[string]any{
			"list_last_message_id":              conversation.ListLastMessageID,
			"list_last_message_at":              conversation.ListLastMessageAt,
			"list_last_message_preview":         conversation.ListLastMessagePreview,
			"list_last_message_is_internal":     conversation.ListLastMessageIsInternal,
			"last_public_message_at":            conversation.LastPublicMessageAt,
			"last_public_message_id":            conversation.LastPublicMessageID,
			"last_public_sender_type":           conversation.LastPublicSenderType,
			"last_public_sender_display_name":   conversation.LastPublicSenderDisplayName,
			"last_customer_message_id":          conversation.LastCustomerMessageID,
			"last_customer_message_at":          conversation.LastCustomerMessageAt,
			"unanswered_customer_message_count": conversation.UnansweredCustomerMessageCount,
			"customer_awaiting_response":        conversation.CustomerAwaitingResponse,
			"needs_human_reply":                 conversation.NeedsHumanReply,
			"support_state_version":             conversation.SupportStateVersion,
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.SupportPortalRequestReference{
			ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: conversation.ID,
			PortalIdentityID: identity.ID, Reference: reference,
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.SupportPortalAuditEvent{
			ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: conversation.ID,
			PortalIdentityID: &identity.ID, ActorType: model.SupportPortalActorCustomer,
			EventType: model.SupportPortalAuditRequestCreated, Metadata: "{}", OccurredAt: time.Now().UTC(),
		}).Error; err != nil {
			return err
		}
		for _, attachmentID := range attachmentIDs {
			metadata, _ := json.Marshal(map[string]string{"attachment_id": attachmentID})
			if err := tx.Create(&model.SupportPortalAuditEvent{
				ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: conversation.ID,
				PortalIdentityID: &identity.ID, ActorType: model.SupportPortalActorCustomer,
				EventType: model.SupportPortalAuditAttachmentUploaded, Metadata: string(metadata), OccurredAt: time.Now().UTC(),
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return conversation, &model.SupportPortalRequest{Reference: reference, Subject: subject, Status: "active", CreatedAt: conversation.CreatedAt, UpdatedAt: conversation.UpdatedAt, LastActivityAt: conversation.LastPublicMessageAt}, nil
}

func (r *PortalAuthRepository) WorkspaceBySlug(ctx context.Context, slug string) (*model.Workspace, error) {
	var ws model.Workspace
	if err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&ws).Error; err != nil {
		return nil, err
	}
	return &ws, nil
}

func (r *PortalAuthRepository) WorkspaceByID(ctx context.Context, id string) (*model.Workspace, error) {
	var ws model.Workspace
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&ws).Error; err != nil {
		return nil, err
	}
	return &ws, nil
}

// AssociateVerifiedEmail claims portal-visible conversations after email verification.
// Widget-originated conversations also require matching verified widget provenance.
func (r *PortalAuthRepository) AssociateVerifiedEmail(tx *gorm.DB, workspaceID, email, identityID string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	// A verified mailbox owner may claim older customer conversations. An explicit
	// visibility decision (including an opt-out) is never overwritten here.
	if err := tx.Exec(`UPDATE support_conversations AS conv SET portal_visible = true
		WHERE conv.workspace_id = ? AND conv.portal_visible = false
		AND conv.portal_visibility_changed_at IS NULL AND conv.deleted_at IS NULL
		AND conv.status <> ? AND conv.channel <> 'internal' AND conv.source <> 'internal'
		AND lower(trim(conv.customer_email)) = ?
		AND ((conv.channel = 'email' AND conv.source = 'email'
			AND COALESCE(conv.primary_recipient_state, 'confirmed') = 'confirmed')
		OR ((conv.channel = 'widget' OR conv.source = 'widget')
			AND EXISTS (SELECT 1 FROM support_widget_sessions AS session
				WHERE session.workspace_id = conv.workspace_id AND session.conversation_id = conv.id
				AND session.identity_trust = ? AND session.identity_verified_at IS NOT NULL
				AND lower(trim(session.customer_email)) = ?)
			AND NOT EXISTS (SELECT 1 FROM support_widget_sessions AS other
				WHERE other.workspace_id = conv.workspace_id AND other.conversation_id = conv.id
				AND other.identity_trust = ? AND other.identity_verified_at IS NOT NULL
				AND lower(trim(other.customer_email)) <> ?)))`, workspaceID, model.SupportConversationStatusSpam,
		email, model.IdentityTrustVerified, email, model.IdentityTrustVerified, email).Error; err != nil {
		return err
	}
	var ambiguousIDs []string
	if err := tx.Model(&model.SupportConversation{}).
		Where("workspace_id = ? AND portal_visible = true AND channel = ? AND deleted_at IS NULL AND lower(trim(customer_email)) = ?", workspaceID, "widget", email).
		Where(`EXISTS (SELECT 1 FROM support_widget_sessions AS session
			WHERE session.workspace_id = support_conversations.workspace_id AND session.conversation_id = support_conversations.id
			AND session.identity_trust = ? AND session.identity_verified_at IS NOT NULL
			AND lower(trim(session.customer_email)) <> ?)`, model.IdentityTrustVerified, email).
		Pluck("id", &ambiguousIDs).Error; err != nil {
		return err
	}
	for _, id := range ambiguousIDs {
		slog.Warn("portal continuity conflicting widget evidence", "workspace_id", workspaceID, "conversation_id", id, "identity_id", identityID)
	}
	var ids []string
	if err := tx.Model(&model.SupportConversation{}).
		Where("workspace_id = ? AND portal_visible = true AND status <> ? AND channel <> ? AND source <> ? AND deleted_at IS NULL", workspaceID, model.SupportConversationStatusSpam, "internal", "internal").
		Where("lower(trim(customer_email)) = ?", email).
		Where(`(channel <> ? AND source <> ?) OR EXISTS (
			SELECT 1 FROM support_widget_sessions AS session
			WHERE session.workspace_id = support_conversations.workspace_id
			AND session.conversation_id = support_conversations.id
			AND session.identity_trust = ? AND session.identity_verified_at IS NOT NULL
			AND lower(trim(session.customer_email)) = ?
		) AND NOT EXISTS (
			SELECT 1 FROM support_widget_sessions AS other
			WHERE other.workspace_id = support_conversations.workspace_id
			AND other.conversation_id = support_conversations.id
			AND other.identity_trust = ? AND other.identity_verified_at IS NOT NULL
			AND lower(trim(other.customer_email)) <> ?
		)`, "widget", "widget", model.IdentityTrustVerified, email, model.IdentityTrustVerified, email).
		Pluck("id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		var existing model.SupportPortalRequestReference
		err := tx.Where("workspace_id = ? AND conversation_id = ?", workspaceID, id).First(&existing).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == nil {
			if existing.PortalIdentityID != identityID {
				slog.Warn("portal continuity identity conflict", "workspace_id", workspaceID, "conversation_id", id, "identity_id", identityID, "existing_identity_id", existing.PortalIdentityID)
			}
			continue
		}
		ref := model.SupportPortalRequestReference{ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: id, PortalIdentityID: identityID, Reference: uuid.NewString()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ref).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *PortalAuthRepository) ListRequests(ctx context.Context, workspaceID, identityID, status string) ([]model.SupportPortalRequest, error) {
	var requests []model.SupportPortalRequest
	query := r.db.WithContext(ctx).Table("support_portal_request_references AS refs").
		Select("refs.reference, conv.subject, conv.status, conv.created_at, conv.last_public_message_at, conv.resolved_at").
		Joins("JOIN support_conversations AS conv ON conv.id = refs.conversation_id AND conv.workspace_id = refs.workspace_id").
		Joins("JOIN support_portal_identities AS identity ON identity.id = refs.portal_identity_id AND identity.workspace_id = refs.workspace_id").
		Where("refs.workspace_id = ? AND refs.portal_identity_id = ? AND conv.portal_visible = true AND conv.status <> ? AND conv.channel <> ? AND conv.source <> ? AND conv.deleted_at IS NULL", workspaceID, identityID, model.SupportConversationStatusSpam, "internal", "internal").
		Where(portalContinuityGuard, model.IdentityTrustVerified, model.IdentityTrustVerified)
	switch status {
	case "active":
		query = query.Where("conv.status NOT IN ?", []string{model.SupportConversationStatusWaitingOnCustomer, "waiting", model.SupportConversationStatusResolved, "closed"})
	case model.SupportConversationStatusWaitingOnCustomer:
		query = query.Where("conv.status IN ?", []string{model.SupportConversationStatusWaitingOnCustomer, "waiting"})
	case model.SupportConversationStatusResolved:
		query = query.Where("conv.status IN ?", []string{model.SupportConversationStatusResolved, "closed"})
	}
	err := query.Order("COALESCE(conv.last_public_message_at, conv.created_at) DESC").Scan(&requests).Error
	for i := range requests {
		if requests[i].LastPublicMessageAt != nil {
			requests[i].LastActivityAt = requests[i].LastPublicMessageAt
		} else {
			requests[i].LastActivityAt = &requests[i].CreatedAt
		}
		requests[i].Status = model.PortalRequestStatus(requests[i].Status)
	}
	return requests, err
}

func (r *PortalAuthRepository) FindRequest(ctx context.Context, workspaceID, identityID, reference string) (*model.SupportConversation, error) {
	var conversation model.SupportConversation
	err := r.db.WithContext(ctx).Table("support_conversations AS conv").Select("conv.*").
		Joins("JOIN support_portal_request_references AS refs ON refs.conversation_id = conv.id AND refs.workspace_id = conv.workspace_id").
		Joins("JOIN support_portal_identities AS identity ON identity.id = refs.portal_identity_id AND identity.workspace_id = refs.workspace_id").
		Where("refs.workspace_id = ? AND refs.portal_identity_id = ? AND refs.reference = ? AND conv.portal_visible = true AND conv.status <> ? AND conv.channel <> ? AND conv.source <> ? AND conv.deleted_at IS NULL", workspaceID, identityID, reference, model.SupportConversationStatusSpam, "internal", "internal").
		Where(portalContinuityGuard, model.IdentityTrustVerified, model.IdentityTrustVerified).First(&conversation).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &conversation, err
}

// Recheck stored ownership against current customer and verified widget evidence.
const portalContinuityGuard = `lower(trim(conv.customer_email)) = lower(trim(identity.email)) AND
	((conv.channel <> 'widget' AND conv.source <> 'widget') OR
	(EXISTS (SELECT 1 FROM support_widget_sessions AS session WHERE session.workspace_id = conv.workspace_id
		AND session.conversation_id = conv.id AND session.identity_trust = ? AND session.identity_verified_at IS NOT NULL
		AND lower(trim(session.customer_email)) = lower(trim(identity.email)))
	AND NOT EXISTS (SELECT 1 FROM support_widget_sessions AS other WHERE other.workspace_id = conv.workspace_id
		AND other.conversation_id = conv.id AND other.identity_trust = ? AND other.identity_verified_at IS NOT NULL
		AND lower(trim(other.customer_email)) <> lower(trim(identity.email)))))`

func (r *PortalAuthRepository) ReconcileRequests(ctx context.Context, workspaceID, identityID string) error {
	var identity model.SupportPortalIdentity
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, identityID).First(&identity).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.AssociateVerifiedEmail(tx, workspaceID, identity.Email, identityID)
	})
}

func (r *PortalAuthRepository) CreateLink(ctx context.Context, link *model.PortalMagicLink) error {
	return r.db.WithContext(ctx).Create(link).Error
}

// ConsumeLink locks the link and creates a session in the same transaction. A
// failed identity lookup or insert rolls back consumption, allowing a retry.
func (r *PortalAuthRepository) ConsumeLink(ctx context.Context, workspaceID, hash string, now time.Time, create func(*gorm.DB, *model.PortalMagicLink) (*model.PortalSession, error)) (*model.PortalSession, error) {
	var session *model.PortalSession
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var link model.PortalMagicLink
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND token_hash = ? AND used_at IS NULL AND expires_at > ?", workspaceID, hash, now).First(&link).Error; err != nil {
			return err
		}
		var err error
		session, err = create(tx, &link)
		if err != nil {
			return err
		}
		if err = tx.Create(session).Error; err != nil {
			return err
		}
		return tx.Model(&link).Update("used_at", now).Error
	})
	return session, err
}

func (r *PortalAuthRepository) FindSession(ctx context.Context, workspaceID, hash string, now time.Time) (*model.PortalSession, error) {
	var session model.PortalSession
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND token_hash = ? AND revoked_at IS NULL AND expires_at > ?", workspaceID, hash, now).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}
func (r *PortalAuthRepository) FindSessionIdentity(ctx context.Context, workspaceID, identityID string, identity *model.SupportPortalIdentity) error {
	return r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, identityID).First(identity).Error
}
func (r *PortalAuthRepository) RevokeSession(ctx context.Context, workspaceID, hash string, now time.Time) error {
	return r.db.WithContext(ctx).Model(new(model.PortalSession)).Where("workspace_id = ? AND token_hash = ? AND revoked_at IS NULL", workspaceID, hash).Update("revoked_at", now).Error
}
