package repository

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// portalEligibleConversation excludes conversations that must never reach a
// customer portal. Queries alias support_conversations as conv.
const portalEligibleConversation = `conv.status <> '` + model.SupportConversationStatusSpam +
	`' AND conv.channel <> 'internal' AND conv.source <> 'internal'`

// portalVisibleConversation is the single customer-visibility rule for a
// conversation aliased as conv.
const portalVisibleConversation = `conv.portal_visible = true AND ` + portalEligibleConversation

// portalContinuityGuard rechecks stored ownership against the conversation's
// current customer and verified widget evidence. Conversation emails are
// compared as lower(customer_email) to use the existing inbox email index.
// It needs conv and identity aliases and two model.IdentityTrustVerified
// arguments.
const portalContinuityGuard = `lower(conv.customer_email) = lower(trim(identity.email)) AND
	((conv.channel <> 'widget' AND conv.source <> 'widget') OR
	(EXISTS (SELECT 1 FROM support_widget_sessions AS session WHERE session.workspace_id = conv.workspace_id
		AND session.conversation_id = conv.id AND session.identity_trust = ? AND session.identity_verified_at IS NOT NULL
		AND lower(trim(session.customer_email)) = lower(trim(identity.email)))
	AND NOT EXISTS (SELECT 1 FROM support_widget_sessions AS other WHERE other.workspace_id = conv.workspace_id
		AND other.conversation_id = conv.id AND other.identity_trust = ? AND other.identity_verified_at IS NOT NULL
		AND lower(trim(other.customer_email)) <> lower(trim(identity.email)))))`

// CustomerPortalRepository persists portal identities, sessions, intake tokens
// and request references. Requests are projections of support_conversations.
type CustomerPortalRepository struct{ db *gorm.DB }

// NewCustomerPortalRepository creates a portal repository.
func NewCustomerPortalRepository(db *gorm.DB) *CustomerPortalRepository {
	return &CustomerPortalRepository{db: db}
}

// WithTx returns a repository bound to tx.
func (r *CustomerPortalRepository) WithTx(tx *gorm.DB) *CustomerPortalRepository {
	return &CustomerPortalRepository{db: tx}
}

// PortalRequestDraft describes a new portal request. Identity is nil for an
// anonymous intake request, which stays hidden until its address is confirmed.
type PortalRequestDraft struct {
	WorkspaceID     string
	Identity        *model.SupportPortalIdentity
	Email           string
	DisplayName     *string
	Subject         string
	Description     string
	MailboxID       *string
	OwnerID         *string
	CRMContactID    *string
	FlowState       *string
	AttachmentIDs   []string
	UploadSessionID string
	DispatchAIMode  string
}

// PortalLinkCounts summarizes recently issued sign-in links.
type PortalLinkCounts struct {
	ForEmail     int64
	ForWorkspace int64
	LastForEmail *time.Time
}

// WorkspaceBySlug finds a workspace by its public slug.
func (r *CustomerPortalRepository) WorkspaceBySlug(ctx context.Context, slug string) (*model.Workspace, error) {
	var ws model.Workspace
	if err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&ws).Error; err != nil {
		return nil, err
	}
	return &ws, nil
}

// CreateIntakeSession stores an anonymous intake capability.
func (r *CustomerPortalRepository) CreateIntakeSession(ctx context.Context, session *model.PortalIntakeSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

// FindIntakeSession returns an unused, unexpired intake session.
func (r *CustomerPortalRepository) FindIntakeSession(ctx context.Context, workspaceID, hash string, now time.Time) (*model.PortalIntakeSession, error) {
	var session model.PortalIntakeSession
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND token_hash = ? AND used_at IS NULL AND expires_at > ?", workspaceID, hash, now).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// ConsumeIntakeSession locks the intake session, runs create, and marks the
// session used in one transaction. A failed create leaves the session usable.
func (r *CustomerPortalRepository) ConsumeIntakeSession(ctx context.Context, workspaceID, hash string, now time.Time, create func(*gorm.DB, *model.PortalIntakeSession) error) error {
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

// CreateRequest commits the inbox conversation, first customer message,
// attachment links and audit trail together. An identified request also gets
// its opaque portal reference; an anonymous one returns a nil projection.
func (r *CustomerPortalRepository) CreateRequest(ctx context.Context, draft PortalRequestDraft) (*model.SupportConversation, *model.SupportPortalRequest, error) {
	email := strings.ToLower(strings.TrimSpace(draft.Email))
	conversation := &model.SupportConversation{
		ID: uuid.NewString(), WorkspaceID: draft.WorkspaceID, Subject: draft.Subject,
		Status: "open", Priority: "medium", Channel: "portal", Source: "portal",
		CustomerName: draft.DisplayName, CustomerEmail: &email, PortalVisible: draft.Identity != nil,
		MailboxID: draft.MailboxID, AssignedUserID: draft.OwnerID, CRMContactID: draft.CRMContactID, FlowState: draft.FlowState,
	}
	var reference string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := NewSupportConversationRepository(tx).Create(ctx, conversation); err != nil {
			return err
		}
		message := &model.SupportMessage{
			ID: uuid.NewString(), WorkspaceID: draft.WorkspaceID, ConversationID: conversation.ID,
			SenderType: "customer", SenderDisplayName: draft.DisplayName,
			MessageType: "reply", Content: draft.Description,
		}
		portalChannel := "portal"
		message.ViaChannel = &portalChannel
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		if draft.DispatchAIMode != "" {
			if err := EnqueuePortalAIDispatch(ctx, tx, draft.WorkspaceID, conversation.ID, message.ID, draft.DispatchAIMode); err != nil {
				return err
			}
		}
		if len(draft.AttachmentIDs) > 0 {
			if err := NewSupportAttachmentRepository(tx).LinkPortalAttachments(ctx, draft.AttachmentIDs, draft.WorkspaceID, draft.UploadSessionID, "", conversation.ID, message.ID); err != nil {
				return err
			}
		}
		if err := projectFirstPortalMessage(tx, conversation, message); err != nil {
			return err
		}
		var identityID *string
		if draft.Identity != nil {
			identityID = &draft.Identity.ID
			ref, err := createPortalReference(tx, draft.WorkspaceID, conversation.ID, draft.Identity.ID)
			if err != nil {
				return err
			}
			reference = ref
		}
		metadata := "{}"
		if draft.Identity == nil {
			metadata = `{"intake":"anonymous"}`
		}
		if err := createPortalAudit(tx, conversation, identityID, model.SupportPortalAuditRequestCreated, metadata); err != nil {
			return err
		}
		for _, attachmentID := range draft.AttachmentIDs {
			attachmentMetadata, err := json.Marshal(map[string]string{"attachment_id": attachmentID})
			if err != nil {
				return err
			}
			if err := createPortalAudit(tx, conversation, identityID, model.SupportPortalAuditAttachmentUploaded, string(attachmentMetadata)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if draft.Identity == nil {
		return conversation, nil, nil
	}
	return conversation, &model.SupportPortalRequest{Reference: reference, Subject: draft.Subject, Status: "active", CreatedAt: conversation.CreatedAt, UpdatedAt: conversation.UpdatedAt, LastActivityAt: conversation.LastPublicMessageAt}, nil
}

func projectFirstPortalMessage(tx *gorm.DB, conversation *model.SupportConversation, message *model.SupportMessage) error {
	conversation.ApplyMessageProjection(*message)
	return tx.Model(conversation).Updates(map[string]any{
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
	}).Error
}

func createPortalAudit(tx *gorm.DB, conversation *model.SupportConversation, identityID *string, eventType, metadata string) error {
	return tx.Create(&model.SupportPortalAuditEvent{
		ID: uuid.NewString(), WorkspaceID: conversation.WorkspaceID, ConversationID: conversation.ID,
		PortalIdentityID: identityID, ActorType: model.SupportPortalActorCustomer,
		EventType: eventType, Metadata: metadata, OccurredAt: time.Now().UTC(),
	}).Error
}

// createPortalReference creates the opaque reference for a conversation unless
// one already exists. It returns the stored reference either way.
func createPortalReference(tx *gorm.DB, workspaceID, conversationID, identityID string) (string, error) {
	reference, err := newPortalReference()
	if err != nil {
		return "", err
	}
	ref := model.SupportPortalRequestReference{ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: conversationID, PortalIdentityID: identityID, Reference: reference}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ref).Error; err != nil {
		return "", err
	}
	var stored model.SupportPortalRequestReference
	if err := tx.Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).First(&stored).Error; err != nil {
		return "", err
	}
	if stored.PortalIdentityID != identityID {
		slog.Warn("portal continuity identity conflict", "workspace_id", workspaceID, "conversation_id", conversationID, "identity_id", identityID, "existing_identity_id", stored.PortalIdentityID)
	}
	return stored.Reference, nil
}

func newPortalReference() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("read portal reference entropy: %w", err)
	}
	return "req_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)), nil
}

// AssociateVerifiedEmail claims portal-eligible conversations after email
// verification. Email conversations need a confirmed primary recipient;
// widget conversations need matching verified widget provenance. Anonymous
// portal intake is never claimed here; only its own confirmation link can
// publish it (see ClaimIntakeRequest).
func (r *CustomerPortalRepository) AssociateVerifiedEmail(tx *gorm.DB, workspaceID, email, identityID string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	// An explicit visibility decision (including an opt-out) is never overwritten.
	if err := tx.Exec(`UPDATE support_conversations AS conv SET portal_visible = true
		WHERE conv.workspace_id = ? AND conv.portal_visible = false
		AND conv.portal_visibility_changed_at IS NULL AND `+portalEligibleConversation+`
		AND lower(conv.customer_email) = ?
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
				AND lower(trim(other.customer_email)) <> ?)))`,
		workspaceID, email, model.IdentityTrustVerified, email, model.IdentityTrustVerified, email).Error; err != nil {
		return err
	}
	var ambiguousIDs []string
	if err := tx.Table("support_conversations AS conv").
		Where("conv.workspace_id = ? AND conv.portal_visible = true AND conv.channel = ? AND lower(conv.customer_email) = ?", workspaceID, "widget", email).
		Where(`EXISTS (SELECT 1 FROM support_widget_sessions AS session
			WHERE session.workspace_id = conv.workspace_id AND session.conversation_id = conv.id
			AND session.identity_trust = ? AND session.identity_verified_at IS NOT NULL
			AND lower(trim(session.customer_email)) <> ?)`, model.IdentityTrustVerified, email).
		Pluck("conv.id", &ambiguousIDs).Error; err != nil {
		return err
	}
	for _, id := range ambiguousIDs {
		slog.Warn("portal continuity conflicting widget evidence", "workspace_id", workspaceID, "conversation_id", id, "identity_id", identityID)
	}
	var ids []string
	if err := tx.Table("support_conversations AS conv").
		Joins("JOIN support_portal_identities AS identity ON identity.workspace_id = conv.workspace_id AND identity.id = ?", identityID).
		Where("conv.workspace_id = ? AND lower(conv.customer_email) = ?", workspaceID, email).
		Where(portalVisibleConversation).
		Where(portalContinuityGuard, model.IdentityTrustVerified, model.IdentityTrustVerified).
		Where("NOT EXISTS (SELECT 1 FROM support_portal_request_references AS refs WHERE refs.workspace_id = conv.workspace_id AND refs.conversation_id = conv.id)").
		Pluck("conv.id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := createPortalReference(tx, workspaceID, id, identityID); err != nil {
			return err
		}
	}
	return nil
}

// ClaimIntakeRequest publishes one anonymous intake request to the identity
// that just proved ownership of the submitted address.
func (r *CustomerPortalRepository) ClaimIntakeRequest(tx *gorm.DB, workspaceID, conversationID, identityID, dispatchAIMode string) error {
	var identity model.SupportPortalIdentity
	if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, identityID).First(&identity).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	result := tx.Exec(`UPDATE support_conversations AS conv SET portal_visible = true, portal_visibility_changed_at = ?
		WHERE conv.workspace_id = ? AND conv.id = ? AND conv.portal_visible = false
		AND conv.portal_visibility_changed_at IS NULL AND conv.channel = 'portal' AND conv.source = 'portal'
		AND `+portalEligibleConversation+` AND lower(conv.customer_email) = ?`,
		now, workspaceID, conversationID, strings.ToLower(strings.TrimSpace(identity.Email)))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// Already claimed, moved to spam, or hidden by a teammate: nothing to publish.
		return nil
	}
	if _, err := createPortalReference(tx, workspaceID, conversationID, identityID); err != nil {
		return err
	}
	conversation := &model.SupportConversation{ID: conversationID, WorkspaceID: workspaceID}
	if err := createPortalAudit(tx, conversation, &identityID, model.SupportPortalAuditVisibilityChanged, `{"reason":"intake_confirmed"}`); err != nil {
		return err
	}
	if dispatchAIMode == "" {
		return nil
	}
	var first struct{ ID string }
	if err := tx.Raw(`SELECT msg.id FROM support_messages AS msg
		JOIN support_conversations AS conv ON conv.id = msg.conversation_id
		WHERE conv.workspace_id = ? AND conv.id = ? AND conv.status = 'open'
		AND conv.assigned_user_id IS NULL AND conv.opened_by_user_id IS NULL
		AND (conv.human_takeover IS NULL OR conv.human_takeover = false)
		AND msg.sender_type = 'customer' AND msg.message_type = 'reply'
		AND NOT EXISTS (SELECT 1 FROM support_messages AS reply
			WHERE reply.conversation_id = conv.id AND reply.sender_type <> 'customer'
			AND reply.message_type = 'reply' AND reply.is_internal = false)
		ORDER BY msg.created_at ASC LIMIT 1`, workspaceID, conversationID).Scan(&first).Error; err != nil {
		return err
	}
	if first.ID != "" {
		return EnqueuePortalAIDispatch(tx.Statement.Context, tx, workspaceID, conversationID, first.ID, dispatchAIMode)
	}
	return nil
}

func (r *CustomerPortalRepository) ownedRequests(ctx context.Context, workspaceID, identityID string) *gorm.DB {
	return r.db.WithContext(ctx).Table("support_portal_request_references AS refs").
		Joins("JOIN support_conversations AS conv ON conv.id = refs.conversation_id AND conv.workspace_id = refs.workspace_id").
		Joins("JOIN support_portal_identities AS identity ON identity.id = refs.portal_identity_id AND identity.workspace_id = refs.workspace_id").
		Where("refs.workspace_id = ? AND refs.portal_identity_id = ?", workspaceID, identityID).
		Where(portalVisibleConversation).
		Where(portalContinuityGuard, model.IdentityTrustVerified, model.IdentityTrustVerified)
}

// ListRequests returns the identity's visible requests, newest activity first.
func (r *CustomerPortalRepository) ListRequests(ctx context.Context, workspaceID, identityID, status string) ([]model.SupportPortalRequest, error) {
	var requests []model.SupportPortalRequest
	query := r.ownedRequests(ctx, workspaceID, identityID).
		Select("refs.reference, conv.subject, conv.status, conv.created_at, conv.last_public_message_at, conv.resolved_at")
	switch status {
	case "active":
		query = query.Where("conv.status NOT IN ?", []string{model.SupportConversationStatusWaitingOnCustomer, "waiting", model.SupportConversationStatusResolved, "closed"})
	case model.SupportConversationStatusWaitingOnCustomer:
		query = query.Where("conv.status IN ?", []string{model.SupportConversationStatusWaitingOnCustomer, "waiting"})
	case model.SupportConversationStatusResolved:
		query = query.Where("conv.status IN ?", []string{model.SupportConversationStatusResolved, "closed"})
	}
	if err := query.Order("COALESCE(conv.last_public_message_at, conv.created_at) DESC").Scan(&requests).Error; err != nil {
		return nil, err
	}
	for i := range requests {
		if requests[i].LastPublicMessageAt != nil {
			requests[i].LastActivityAt = requests[i].LastPublicMessageAt
		} else {
			requests[i].LastActivityAt = &requests[i].CreatedAt
		}
		requests[i].Status = model.PortalRequestStatus(requests[i].Status)
	}
	return requests, nil
}

// FindRequest resolves a reference the identity may read, or nil.
func (r *CustomerPortalRepository) FindRequest(ctx context.Context, workspaceID, identityID, reference string) (*model.SupportConversation, error) {
	var conversation model.SupportConversation
	err := r.ownedRequests(ctx, workspaceID, identityID).Select("conv.*").
		Where("refs.reference = ?", reference).Take(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

// ReferenceForCustomer returns the reference owned by the conversation's
// current customer address, or "" when none has been claimed yet.
func (r *CustomerPortalRepository) ReferenceForCustomer(ctx context.Context, workspaceID, conversationID, email string) (string, error) {
	var references []string
	err := r.db.WithContext(ctx).Table("support_portal_request_references AS refs").
		Joins("JOIN support_portal_identities AS identity ON identity.id = refs.portal_identity_id AND identity.workspace_id = refs.workspace_id").
		Where("refs.workspace_id = ? AND refs.conversation_id = ? AND lower(identity.email) = lower(?)", workspaceID, conversationID, strings.TrimSpace(email)).
		Limit(1).Pluck("refs.reference", &references).Error
	if err != nil || len(references) == 0 {
		return "", err
	}
	return references[0], nil
}

// ReconcileRequests reruns continuity for an identity.
func (r *CustomerPortalRepository) ReconcileRequests(ctx context.Context, workspaceID, identityID string) error {
	var identity model.SupportPortalIdentity
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, identityID).First(&identity).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.AssociateVerifiedEmail(tx, workspaceID, identity.Email, identityID)
	})
}

// MarkSessionReconciled claims the session's next reconciliation slot. It
// reports false when another request reconciled it within interval.
func (r *CustomerPortalRepository) MarkSessionReconciled(ctx context.Context, sessionID string, now time.Time, interval time.Duration) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.PortalSession{}).
		Where("id = ? AND (reconciled_at IS NULL OR reconciled_at <= ?)", sessionID, now.Add(-interval)).
		Update("reconciled_at", now)
	return result.RowsAffected == 1, result.Error
}

// CreateLink stores a sign-in link digest.
func (r *CustomerPortalRepository) CreateLink(ctx context.Context, link *model.PortalMagicLink) error {
	return r.db.WithContext(ctx).Create(link).Error
}

// LinkCountsSince counts sign-in links issued since the given time.
func (r *CustomerPortalRepository) LinkCountsSince(ctx context.Context, workspaceID, email string, since time.Time) (PortalLinkCounts, error) {
	var counts PortalLinkCounts
	var last struct{ CreatedAt time.Time }
	db := r.db.WithContext(ctx).Model(&model.PortalMagicLink{})
	if err := db.Where("workspace_id = ? AND email = ? AND created_at > ?", workspaceID, email, since).Count(&counts.ForEmail).Error; err != nil {
		return counts, err
	}
	if counts.ForEmail > 0 {
		if err := r.db.WithContext(ctx).Model(&model.PortalMagicLink{}).Select("created_at").
			Where("workspace_id = ? AND email = ?", workspaceID, email).Order("created_at DESC").Take(&last).Error; err != nil {
			return counts, err
		}
		counts.LastForEmail = &last.CreatedAt
	}
	err := r.db.WithContext(ctx).Model(&model.PortalMagicLink{}).
		Where("workspace_id = ? AND created_at > ?", workspaceID, since).Count(&counts.ForWorkspace).Error
	return counts, err
}

// ConsumeLink locks the link and creates a session in the same transaction. A
// failed identity lookup or insert rolls back consumption, allowing a retry.
func (r *CustomerPortalRepository) ConsumeLink(ctx context.Context, workspaceID, hash string, now time.Time, create func(*gorm.DB, *model.PortalMagicLink) (*model.PortalSession, error)) (*model.PortalSession, error) {
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

// FindSession returns an unrevoked, unexpired session.
func (r *CustomerPortalRepository) FindSession(ctx context.Context, workspaceID, hash string, now time.Time) (*model.PortalSession, error) {
	var session model.PortalSession
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND token_hash = ? AND revoked_at IS NULL AND expires_at > ?", workspaceID, hash, now).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// FindSessionIdentity loads the session's identity within the workspace.
func (r *CustomerPortalRepository) FindSessionIdentity(ctx context.Context, workspaceID, identityID string, identity *model.SupportPortalIdentity) error {
	return r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, identityID).First(identity).Error
}

// RevokeSession revokes a session by token digest.
func (r *CustomerPortalRepository) RevokeSession(ctx context.Context, workspaceID, hash string, now time.Time) error {
	return r.db.WithContext(ctx).Model(new(model.PortalSession)).Where("workspace_id = ? AND token_hash = ? AND revoked_at IS NULL", workspaceID, hash).Update("revoked_at", now).Error
}
