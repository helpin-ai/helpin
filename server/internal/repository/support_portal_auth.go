package repository

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PortalAuthRepository struct{ db *gorm.DB }

func NewPortalAuthRepository(db *gorm.DB) *PortalAuthRepository { return &PortalAuthRepository{db: db} }

// CreateRequest commits the inbox conversation, customer message and portal reference together.
func (r *PortalAuthRepository) CreateRequest(ctx context.Context, workspaceID string, identity *model.SupportPortalIdentity, subject, description, reference string, mailboxID, ownerID, crmContactID, flowState *string) (*model.SupportConversation, *model.SupportPortalRequest, error) {
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
		return tx.Create(&model.SupportPortalRequestReference{
			ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: conversation.ID,
			PortalIdentityID: identity.ID, Reference: reference,
		}).Error
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

// AssociateVerifiedEmail only claims customer-visible conversations backed by
// the verified address, or by a CRM contact with that address and no conflicting
// customer email. Existing references are never transferred between identities.
func (r *PortalAuthRepository) AssociateVerifiedEmail(tx *gorm.DB, workspaceID, email, identityID string) error {
	contactIDs := tx.Model(&model.CRMContact{}).Select("id").Where("workspace_id = ? AND lower(email) = ?", workspaceID, strings.ToLower(email))
	var ids []string
	if err := tx.Model(&model.SupportConversation{}).
		Where("workspace_id = ? AND portal_visible = true AND status <> ? AND channel <> ? AND source <> ? AND deleted_at IS NULL", workspaceID, model.SupportConversationStatusSpam, "internal", "internal").
		Where("lower(customer_email) = ? OR (crm_contact_id IN (?) AND (customer_email IS NULL OR customer_email = ''))", strings.ToLower(email), contactIDs).
		Pluck("id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
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
		Where("refs.workspace_id = ? AND refs.portal_identity_id = ? AND conv.portal_visible = true AND conv.status <> ? AND conv.channel <> ? AND conv.source <> ? AND conv.deleted_at IS NULL", workspaceID, identityID, model.SupportConversationStatusSpam, "internal", "internal")
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
		Where("refs.workspace_id = ? AND refs.portal_identity_id = ? AND refs.reference = ? AND conv.portal_visible = true AND conv.status <> ? AND conv.channel <> ? AND conv.source <> ? AND conv.deleted_at IS NULL", workspaceID, identityID, reference, model.SupportConversationStatusSpam, "internal", "internal").First(&conversation).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &conversation, err
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
