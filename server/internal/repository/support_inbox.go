package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportMessageRepository handles DB operations for support messages.
type SupportMessageRepository struct {
	db *gorm.DB
}

// NewSupportMessageRepository creates a new SupportMessageRepository.
func NewSupportMessageRepository(db *gorm.DB) *SupportMessageRepository {
	return &SupportMessageRepository{db: db}
}

// ListByConversation returns messages for a conversation.
func (r *SupportMessageRepository) ListByConversation(ctx context.Context, workspaceID, conversationID string, includeInternal bool) ([]model.SupportMessage, error) {
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID)
	if !includeInternal {
		query = query.Where("is_internal = false")
	}
	var messages []model.SupportMessage
	if err := query.Order("created_at ASC").Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	return messages, nil
}

// Create creates a new message.
func (r *SupportMessageRepository) Create(ctx context.Context, message *model.SupportMessage) error {
	if err := r.db.WithContext(ctx).Create(message).Error; err != nil {
		return fmt.Errorf("create message: %w", err)
	}
	// Bump parent conversation's updated_at so it moves to top of inbox list
	if message.ConversationID != "" {
		r.db.WithContext(ctx).
			Model(&model.SupportConversation{}).
			Where("id = ?", message.ConversationID).
			Update("updated_at", time.Now())
	}
	return nil
}

// GetByID returns a support message by ID.
func (r *SupportMessageRepository) GetByID(ctx context.Context, id string) (*model.SupportMessage, error) {
	var message model.SupportMessage
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&message).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get message: %w", err)
	}
	return &message, nil
}

// GetByIDs returns support messages by ID ordered by created_at.
func (r *SupportMessageRepository) GetByIDs(ctx context.Context, ids []string) ([]model.SupportMessage, error) {
	if len(ids) == 0 {
		return []model.SupportMessage{}, nil
	}
	var messages []model.SupportMessage
	if err := r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("get messages by ids: %w", err)
	}
	return messages, nil
}

// UpdateEmailNotifiedAt stamps email_notified_at for the provided message IDs.
func (r *SupportMessageRepository) UpdateEmailNotifiedAt(ctx context.Context, ids []string, notifiedAt time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportMessage{}).
		Where("id IN ?", ids).
		Update("email_notified_at", notifiedAt).Error; err != nil {
		return fmt.Errorf("update email_notified_at: %w", err)
	}
	return nil
}

// UpdateEmailReadAt stamps email_read_at for the provided message IDs.
func (r *SupportMessageRepository) UpdateEmailReadAt(ctx context.Context, ids []string, readAt time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportMessage{}).
		Where("id IN ?", ids).
		Update("email_read_at", readAt).Error; err != nil {
		return fmt.Errorf("update email_read_at: %w", err)
	}
	return nil
}

// DB returns the underlying *gorm.DB for transaction support.
func (r *SupportMessageRepository) DB() *gorm.DB {
	return r.db
}

// WithTx returns a new SupportMessageRepository using the given transaction.
func (r *SupportMessageRepository) WithTx(tx *gorm.DB) *SupportMessageRepository {
	return &SupportMessageRepository{db: tx}
}

// SupportInboxInstallationRepository handles widget installations.
type SupportInboxInstallationRepository struct {
	db *gorm.DB
}

// NewSupportInboxInstallationRepository creates a new SupportInboxInstallationRepository.
func NewSupportInboxInstallationRepository(db *gorm.DB) *SupportInboxInstallationRepository {
	return &SupportInboxInstallationRepository{db: db}
}

// GetByWorkspace returns the widget installation for a workspace.
func (r *SupportInboxInstallationRepository) GetByWorkspace(ctx context.Context, workspaceID string) (*model.SupportWidgetInstallation, error) {
	var inst model.SupportWidgetInstallation
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&inst).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get widget installation: %w", err)
	}
	return &inst, nil
}

// GetByWidgetKey returns the installation by public widget key.
func (r *SupportInboxInstallationRepository) GetByWidgetKey(ctx context.Context, widgetKey string) (*model.SupportWidgetInstallation, error) {
	var inst model.SupportWidgetInstallation
	if err := r.db.WithContext(ctx).Where("widget_key = ? AND active = true", widgetKey).First(&inst).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get widget by key: %w", err)
	}
	return &inst, nil
}

// GetByID returns the installation by its UUID.
func (r *SupportInboxInstallationRepository) GetByID(ctx context.Context, id string) (*model.SupportWidgetInstallation, error) {
	var inst model.SupportWidgetInstallation
	if err := r.db.WithContext(ctx).Where("id = ? AND active = true", id).First(&inst).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get widget by id: %w", err)
	}
	return &inst, nil
}

// Create creates a new installation.
func (r *SupportInboxInstallationRepository) Create(ctx context.Context, inst *model.SupportWidgetInstallation) error {
	if err := r.db.WithContext(ctx).Create(inst).Error; err != nil {
		return fmt.Errorf("create widget installation: %w", err)
	}
	return nil
}

// Update saves an installation.
func (r *SupportInboxInstallationRepository) Update(ctx context.Context, inst *model.SupportWidgetInstallation) error {
	if err := r.db.WithContext(ctx).Save(inst).Error; err != nil {
		return fmt.Errorf("update widget installation: %w", err)
	}
	return nil
}

// ListAllActive returns all active widget installations across all workspaces.
func (r *SupportInboxInstallationRepository) ListAllActive(ctx context.Context) ([]model.SupportWidgetInstallation, error) {
	var installations []model.SupportWidgetInstallation
	if err := r.db.WithContext(ctx).Where("active = true").Find(&installations).Error; err != nil {
		return nil, fmt.Errorf("list active installations: %w", err)
	}
	return installations, nil
}

// RegenerateKeys updates just the widget_key and secret_key columns.
func (r *SupportInboxInstallationRepository) RegenerateKeys(ctx context.Context, id, widgetKey, secretKey string) error {
	if err := r.db.WithContext(ctx).Model(&model.SupportWidgetInstallation{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"widget_key": widgetKey, "secret_key": secretKey}).Error; err != nil {
		return fmt.Errorf("regenerate widget keys: %w", err)
	}
	return nil
}

// SupportInboxSessionRepository handles widget sessions.
type SupportInboxSessionRepository struct {
	db *gorm.DB
}

// NewSupportInboxSessionRepository creates a new SupportInboxSessionRepository.
func NewSupportInboxSessionRepository(db *gorm.DB) *SupportInboxSessionRepository {
	return &SupportInboxSessionRepository{db: db}
}

// GetByToken returns a session by token.
func (r *SupportInboxSessionRepository) GetByToken(ctx context.Context, token string) (*model.SupportWidgetSession, error) {
	var session model.SupportWidgetSession
	if err := r.db.WithContext(ctx).Where("session_token = ?", token).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get widget session: %w", err)
	}
	return &session, nil
}

// Create creates a new session.
func (r *SupportInboxSessionRepository) Create(ctx context.Context, session *model.SupportWidgetSession) error {
	values := map[string]interface{}{
		"workspace_id":    session.WorkspaceID,
		"conversation_id": session.ConversationID,
		"session_token":   session.SessionToken,
		"anonymous_id":    session.AnonymousID,
		"is_anonymous":    session.IsAnonymous,
		"customer_name":   session.CustomerName,
		"customer_email":  session.CustomerEmail,
		"customer_phone":  session.CustomerPhone,
		"user_agent":      session.UserAgent,
		"last_page_url":   session.LastPageURL,
		"timezone":        session.Timezone,
		"locale":          session.Locale,
		"revoked_at":      session.RevokedAt,
		"expires_at":      session.ExpiresAt,
	}
	if session.ID != "" {
		values["id"] = session.ID
	}
	if !session.CreatedAt.IsZero() {
		values["created_at"] = session.CreatedAt
	}

	if err := r.db.WithContext(ctx).Model(&model.SupportWidgetSession{}).Create(values).Error; err != nil {
		return fmt.Errorf("create widget session: %w", err)
	}

	created, err := r.GetByToken(ctx, session.SessionToken)
	if err != nil {
		return err
	}
	if created == nil {
		return fmt.Errorf("create widget session: session not found after insert")
	}
	*session = *created
	return nil
}

// Update saves a session.
func (r *SupportInboxSessionRepository) Update(ctx context.Context, session *model.SupportWidgetSession) error {
	if err := r.db.WithContext(ctx).Save(session).Error; err != nil {
		return fmt.Errorf("update widget session: %w", err)
	}
	return nil
}

// UpdatePageURL updates only the last_page_url on a session by token in a single query.
func (r *SupportInboxSessionRepository) UpdatePageURL(ctx context.Context, sessionToken, url string) error {
	result := r.db.WithContext(ctx).
		Model(&model.SupportWidgetSession{}).
		Where("session_token = ? AND revoked_at IS NULL", sessionToken).
		Update("last_page_url", url)
	if result.Error != nil {
		return fmt.Errorf("update session page url: %w", result.Error)
	}
	return nil
}

// GetLatestByConversation returns the most recent session for a conversation.
func (r *SupportInboxSessionRepository) GetLatestByConversation(ctx context.Context, workspaceID, conversationID string) (*model.SupportWidgetSession, error) {
	var session model.SupportWidgetSession
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).
		Order("created_at DESC").
		First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest session by conversation: %w", err)
	}
	return &session, nil
}

// GetLatestByAnonymousID returns the most recent session for an anonymous visitor.
func (r *SupportInboxSessionRepository) GetLatestByAnonymousID(ctx context.Context, workspaceID, anonymousID string) (*model.SupportWidgetSession, error) {
	var session model.SupportWidgetSession
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND anonymous_id = ?", workspaceID, anonymousID).
		Order("created_at DESC").
		First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest session by anonymous_id: %w", err)
	}
	return &session, nil
}

// SupportConversationRepository handles DB operations for support conversations.
type SupportConversationRepository struct {
	db *gorm.DB
}

// NewSupportConversationRepository creates a new SupportConversationRepository.
func NewSupportConversationRepository(db *gorm.DB) *SupportConversationRepository {
	return &SupportConversationRepository{db: db}
}

func isElevatedSupportRole(role string) bool {
	return role == model.RoleOwner || role == model.RoleAdmin
}

func (r *SupportConversationRepository) epochExpr() string {
	if r.db != nil && r.db.Dialector.Name() == "sqlite" {
		return "'1970-01-01 00:00:00'"
	}
	return "'1970-01-01'::timestamptz"
}

func (r *SupportConversationRepository) nowExpr() string {
	if r.db != nil && r.db.Dialector.Name() == "sqlite" {
		return "CURRENT_TIMESTAMP"
	}
	return "NOW()"
}

func (r *SupportConversationRepository) textPrefixExpr(column string, limit int) string {
	if r.db != nil && r.db.Dialector.Name() == "sqlite" {
		return fmt.Sprintf("substr(%s, 1, %d)", column, limit)
	}
	return fmt.Sprintf("LEFT(%s, %d)", column, limit)
}

func (r *SupportConversationRepository) maybeAcquireWorkspaceDisplayIDLock(tx *gorm.DB, workspaceID string) {
	if tx == nil || tx.Dialector.Name() != "postgres" {
		return
	}
	tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", workspaceID)
}

// List returns conversations with optional filters and pagination.
func (r *SupportConversationRepository) List(ctx context.Context, workspaceID string, status string, priority string, pagination model.PMPagination, workspaceMemberID, role string, mailboxID *string, aiState ...string) ([]model.SupportConversation, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.SupportConversation{}).Where("support_conversations.workspace_id = ?", workspaceID)
	base = r.applyMailboxAccess(base, workspaceMemberID, role)
	base = r.applyMailboxScope(base, mailboxID)

	if status != "" {
		base = base.Where("status = ?", status)
	}
	if priority != "" {
		base = base.Where("priority = ?", priority)
	}
	// AI state filter: "any" = ai_state IS NOT NULL, specific value = exact match
	if len(aiState) > 0 && aiState[0] != "" {
		if aiState[0] == "any" {
			base = base.Where("ai_state IS NOT NULL")
		} else {
			base = base.Where("ai_state = ?", aiState[0])
		}
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count conversations: %w", err)
	}

	page := pagination.Page
	perPage := pagination.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	// Fresh query for fetch — Count() taints the SELECT clause
	fetch := r.db.WithContext(ctx).Table("support_conversations").Where("support_conversations.workspace_id = ?", workspaceID)
	fetch = r.applyMailboxAccess(fetch, workspaceMemberID, role)
	fetch = r.applyMailboxScope(fetch, mailboxID)
	if status != "" {
		fetch = fetch.Where("support_conversations.status = ?", status)
	}
	if priority != "" {
		fetch = fetch.Where("support_conversations.priority = ?", priority)
	}
	if len(aiState) > 0 && aiState[0] != "" {
		if aiState[0] == "any" {
			fetch = fetch.Where("support_conversations.ai_state IS NOT NULL")
		} else {
			fetch = fetch.Where("support_conversations.ai_state = ?", aiState[0])
		}
	}

	var conversations []model.SupportConversation
	if err := fetch.
		Joins("LEFT JOIN support_mailboxes sm ON sm.id = support_conversations.mailbox_id").
		Select(fmt.Sprintf(`support_conversations.*, (
			SELECT CASE WHEN m.is_internal THEN 'Note: ' || %s ELSE %s END
			FROM support_messages m
			WHERE m.conversation_id = support_conversations.id
			ORDER BY m.created_at DESC LIMIT 1
		) AS last_message,
		(SELECT COUNT(*)
			FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.is_internal = false
			  AND sm.sender_type = 'customer'
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(support_conversations.team_last_seen_at, %s)
		) AS unread_count,
		sm.name AS mailbox_name,
		sm.handle AS mailbox_handle,
		sm.icon AS mailbox_icon`, r.textPrefixExpr("m.content", 100), r.textPrefixExpr("m.content", 100), r.epochExpr())).
		Order("support_conversations.updated_at DESC").Offset(offset).Limit(perPage).Find(&conversations).Error; err != nil {
		return nil, 0, fmt.Errorf("list conversations: %w", err)
	}
	return conversations, total, nil
}

// GetByID returns a single conversation.
func (r *SupportConversationRepository) GetByID(ctx context.Context, workspaceID, id, workspaceMemberID, role string) (*model.SupportConversation, error) {
	var conversation model.SupportConversation
	query := r.db.WithContext(ctx).
		Table("support_conversations").
		Joins("LEFT JOIN support_mailboxes sm ON sm.id = support_conversations.mailbox_id").
		Where("support_conversations.workspace_id = ? AND support_conversations.id = ?", workspaceID, id)
	query = r.applyMailboxAccess(query, workspaceMemberID, role)
	if err := query.Select("support_conversations.*, sm.name AS mailbox_name, sm.handle AS mailbox_handle, sm.icon AS mailbox_icon").First(&conversation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	return &conversation, nil
}

// Create creates a new conversation with auto-allocated display_id.
func (r *SupportConversationRepository) Create(ctx context.Context, conversation *model.SupportConversation) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Advisory lock on workspace to prevent duplicate display_id under concurrency.
		r.maybeAcquireWorkspaceDisplayIDLock(tx, conversation.WorkspaceID)

		var maxDisplayID int
		tx.Model(&model.SupportConversation{}).Where("workspace_id = ?", conversation.WorkspaceID).
			Select("COALESCE(MAX(display_id), 0)").Scan(&maxDisplayID)
		conversation.DisplayID = maxDisplayID + 1

		if err := tx.Create(conversation).Error; err != nil {
			return fmt.Errorf("create conversation: %w", err)
		}
		return nil
	})
}

// Update saves a conversation.
func (r *SupportConversationRepository) Update(ctx context.Context, conversation *model.SupportConversation) error {
	if err := r.db.WithContext(ctx).Save(conversation).Error; err != nil {
		return fmt.Errorf("update conversation: %w", err)
	}
	return nil
}

// UpdateFields updates specific fields on a conversation by ID and workspace.
func (r *SupportConversationRepository) UpdateFields(ctx context.Context, workspaceID, conversationID string, fields map[string]any) error {
	if err := r.db.WithContext(ctx).
		Model(&model.SupportConversation{}).
		Where("id = ? AND workspace_id = ?", conversationID, workspaceID).
		Updates(fields).Error; err != nil {
		return fmt.Errorf("update conversation fields: %w", err)
	}
	return nil
}

// UpdateSubject updates only the subject of a conversation.
func (r *SupportConversationRepository) UpdateSubject(ctx context.Context, id, subject string) error {
	if err := r.db.WithContext(ctx).Model(&model.SupportConversation{}).Where("id = ?", id).Update("subject", subject).Error; err != nil {
		return fmt.Errorf("update conversation subject: %w", err)
	}
	return nil
}

// ListConversationIDsWithMentions returns conversation IDs where the given user was mentioned in internal notes.
func (r *SupportConversationRepository) ListConversationIDsWithMentions(ctx context.Context, workspaceID, userID string) ([]string, error) {
	filterJSON, _ := json.Marshal(map[string][]string{"mentioned_user_ids": {userID}})
	var ids []string
	if err := r.db.WithContext(ctx).
		Table("support_messages").
		Where("workspace_id = ? AND is_internal = true AND metadata::jsonb @> ?::jsonb", workspaceID, string(filterJSON)).
		Distinct().
		Pluck("conversation_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list conversations with mentions: %w", err)
	}
	return ids, nil
}

// ListByIDs returns conversations by ID for a workspace.
func (r *SupportConversationRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string, workspaceMemberID, role string) ([]model.SupportConversation, error) {
	if len(ids) == 0 {
		return []model.SupportConversation{}, nil
	}

	var conversations []model.SupportConversation
	query := r.db.WithContext(ctx).
		Table("support_conversations").
		Joins("LEFT JOIN support_mailboxes sm ON sm.id = support_conversations.mailbox_id").
		Where("support_conversations.workspace_id = ? AND support_conversations.id IN ?", workspaceID, ids)
	query = r.applyMailboxAccess(query, workspaceMemberID, role)
	if err := query.
		Select("support_conversations.*, sm.name AS mailbox_name, sm.handle AS mailbox_handle, sm.icon AS mailbox_icon").
		Order("updated_at DESC").
		Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("list conversations by ids: %w", err)
	}
	return conversations, nil
}

// ListByLinkedStoryIDs returns conversations linked to any of the provided stories.
func (r *SupportConversationRepository) ListByLinkedStoryIDs(ctx context.Context, workspaceID string, storyIDs []string) ([]model.SupportConversation, error) {
	if len(storyIDs) == 0 {
		return []model.SupportConversation{}, nil
	}

	var conversations []model.SupportConversation
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND linked_story_id IN ?", workspaceID, storyIDs).
		Order("updated_at DESC").
		Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("list conversations by linked stories: %w", err)
	}
	return conversations, nil
}

// ListByContact returns conversations linked to a CRM contact.
func (r *SupportConversationRepository) ListByContact(ctx context.Context, workspaceID, contactID string, pagination model.PMPagination) ([]model.SupportConversation, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.SupportConversation{}).
		Where("workspace_id = ? AND crm_contact_id = ?", workspaceID, contactID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count contact conversations: %w", err)
	}

	page := pagination.Page
	perPage := pagination.PerPage
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	var conversations []model.SupportConversation
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&conversations).Error; err != nil {
		return nil, 0, fmt.Errorf("list contact conversations: %w", err)
	}
	return conversations, total, nil
}

// ListByAnonymousID returns conversations for a visitor by anonymous_id.
func (r *SupportConversationRepository) ListByAnonymousID(ctx context.Context, workspaceID, anonymousID string) ([]model.SupportConversation, error) {
	var conversations []model.SupportConversation
	if err := r.db.WithContext(ctx).
		Select(fmt.Sprintf(`support_conversations.*,
		(SELECT content FROM support_messages WHERE support_messages.conversation_id = support_conversations.id AND support_messages.is_internal = false ORDER BY created_at DESC LIMIT 1) AS last_message,
		(SELECT COUNT(*)
			FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.is_internal = false
			  AND sm.sender_type IN ('user', 'agent', 'ai')
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(support_conversations.contact_last_seen_at, %s)
		) AS unread_count`, r.epochExpr())).
		Where("workspace_id = ? AND anonymous_id = ?", workspaceID, anonymousID).
		Order("updated_at DESC").
		Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("list conversations by anonymous_id: %w", err)
	}
	return conversations, nil
}

// MarkInternalRead sets team_last_seen_at = NOW() if unread messages exist beyond the current cursor.
// Uses raw SQL to avoid GORM's autoUpdateTime touching updated_at (which would re-sort the conversation).
func (r *SupportConversationRepository) MarkInternalRead(ctx context.Context, conversationID string) error {
	result := r.db.WithContext(ctx).Exec(fmt.Sprintf(`
		UPDATE support_conversations
		SET team_last_seen_at = %s
		WHERE id = ?
		  AND EXISTS (
			SELECT 1 FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.is_internal = false
			  AND sm.sender_type = 'customer'
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(support_conversations.team_last_seen_at, %s)
		  )
	`, r.nowExpr(), r.epochExpr()), conversationID)
	if result.Error != nil {
		return fmt.Errorf("mark internal read: %w", result.Error)
	}
	return nil
}

// MarkUnread resets team_last_seen_at to epoch so the conversation appears unread again.
// Uses raw SQL to avoid GORM's autoUpdateTime touching updated_at.
func (r *SupportConversationRepository) MarkUnread(ctx context.Context, conversationID string) error {
	result := r.db.WithContext(ctx).Exec(fmt.Sprintf(`
		UPDATE support_conversations
		SET team_last_seen_at = %s
		WHERE id = ?
	`, r.epochExpr()), conversationID)
	if result.Error != nil {
		return fmt.Errorf("mark unread: %w", result.Error)
	}
	return nil
}

// Delete permanently removes a conversation and its messages.
func (r *SupportConversationRepository) Delete(ctx context.Context, workspaceID, conversationID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).Delete(&model.SupportMessage{}).Error; err != nil {
			return fmt.Errorf("delete conversation messages: %w", err)
		}
		if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, conversationID).Delete(&model.SupportConversation{}).Error; err != nil {
			return fmt.Errorf("delete conversation: %w", err)
		}
		return nil
	})
}

// MarkContactRead sets contact_last_seen_at = NOW() if unread messages exist beyond the current cursor.
// Uses raw SQL to avoid GORM's autoUpdateTime touching updated_at (which would re-sort the conversation).
func (r *SupportConversationRepository) MarkContactRead(ctx context.Context, conversationID string) error {
	result := r.db.WithContext(ctx).Exec(fmt.Sprintf(`
		UPDATE support_conversations
		SET contact_last_seen_at = %s
		WHERE id = ?
		  AND EXISTS (
			SELECT 1 FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.is_internal = false
			  AND sm.sender_type IN ('user', 'agent', 'ai')
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(support_conversations.contact_last_seen_at, %s)
		  )
	`, r.nowExpr(), r.epochExpr()), conversationID)
	if result.Error != nil {
		return fmt.Errorf("mark contact read: %w", result.Error)
	}
	return nil
}

// GetUnreadStats returns aggregate unread conversation counts for an accessible inbox scope.
func (r *SupportConversationRepository) GetUnreadStats(ctx context.Context, workspaceID, userID, workspaceMemberID, role string, mailboxID *string) (model.UnreadStats, error) {
	var stats model.UnreadStats
	baseQuery := `
		SELECT
			COUNT(*) FILTER (
				WHERE (
					SELECT COUNT(*)
					FROM support_messages sm
					WHERE sm.conversation_id = sc.id
					  AND sm.is_internal = false
					  AND sm.sender_type = 'customer'
					  AND sm.message_type = 'reply'
					  AND sm.created_at > COALESCE(sc.team_last_seen_at, %s)
				) > 0
				  AND (sc.ai_state IS NULL OR sc.ai_state = 'escalated')
			) AS total,
			COUNT(*) FILTER (
				WHERE (
					SELECT COUNT(*)
					FROM support_messages sm
					WHERE sm.conversation_id = sc.id
					  AND sm.is_internal = false
					  AND sm.sender_type = 'customer'
					  AND sm.message_type = 'reply'
					  AND sm.created_at > COALESCE(sc.team_last_seen_at, %s)
				) > 0
				  AND (sc.ai_state IS NULL OR sc.ai_state = 'escalated')
				  AND sc.opened_by_user_id = ?
			) AS my_inbox,
			COUNT(*) FILTER (
				WHERE (
					SELECT COUNT(*)
					FROM support_messages sm
					WHERE sm.conversation_id = sc.id
					  AND sm.is_internal = false
					  AND sm.sender_type = 'customer'
					  AND sm.message_type = 'reply'
					  AND sm.created_at > COALESCE(sc.team_last_seen_at, %s)
				) > 0
				  AND (sc.ai_state IS NULL OR sc.ai_state = 'escalated')
				  AND sc.assigned_agent_id IS NULL
				  AND sc.opened_by_user_id IS NULL
			) AS unassigned
		FROM support_conversations sc
		WHERE sc.workspace_id = ?
		  AND sc.status != 'closed'
	`

	args := []any{userID, workspaceID}
	if mailboxID != nil {
		if *mailboxID == "" {
			baseQuery += " AND sc.mailbox_id IS NULL"
		} else {
			baseQuery += " AND sc.mailbox_id = ?"
			args = append(args, *mailboxID)
		}
	}
	if !isElevatedSupportRole(role) {
		baseQuery += ` AND (
			sc.mailbox_id IS NULL
			OR sc.mailbox_id IN (
				SELECT sm.id
				FROM support_mailboxes sm
				WHERE sm.active = true
				  AND ` + supportMailboxAccessCondition("sm") + `
			)
		)`
		args = append(args, workspaceMemberID, workspaceMemberID)
	}

	err := r.db.WithContext(ctx).Raw(fmt.Sprintf(baseQuery, r.epochExpr(), r.epochExpr(), r.epochExpr()), args...).Scan(&stats).Error
	if err != nil {
		return stats, fmt.Errorf("get unread stats: %w", err)
	}
	return stats, nil
}

func (r *SupportConversationRepository) applyMailboxScope(query *gorm.DB, mailboxID *string) *gorm.DB {
	if mailboxID == nil {
		return query
	}
	if strings.TrimSpace(*mailboxID) == "" {
		return query.Where("support_conversations.mailbox_id IS NULL")
	}
	return query.Where("support_conversations.mailbox_id = ?", strings.TrimSpace(*mailboxID))
}

func (r *SupportConversationRepository) applyMailboxAccess(query *gorm.DB, workspaceMemberID, role string) *gorm.DB {
	if isElevatedSupportRole(role) {
		return query
	}
	return query.Where(`
		(
			support_conversations.mailbox_id IS NULL
			OR support_conversations.mailbox_id IN (
				SELECT sm.id
				FROM support_mailboxes sm
				WHERE sm.active = true
				  AND `+supportMailboxAccessCondition("sm")+`
			)
		)
	`, workspaceMemberID, workspaceMemberID)
}

// UpdateIdentityByAnonymousID batch-updates all anonymous conversations for a visitor
// with the provided email, name, and CRM contact ID. Returns the IDs of updated conversations.
func (r *SupportConversationRepository) UpdateIdentityByAnonymousID(ctx context.Context, workspaceID, anonymousID, email, name string, crmContactID *string) ([]string, error) {
	// Build the update map
	updates := map[string]interface{}{
		"customer_email": email,
		"customer_name":  name,
	}

	query := r.db.WithContext(ctx).
		Model(&model.SupportConversation{}).
		Where("workspace_id = ? AND anonymous_id = ? AND (customer_email IS NULL OR customer_email = '')", workspaceID, anonymousID)

	// First, get the IDs of conversations that will be updated
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&model.SupportConversation{}).
		Where("workspace_id = ? AND anonymous_id = ? AND (customer_email IS NULL OR customer_email = '')", workspaceID, anonymousID).
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("find anonymous conversations: %w", err)
	}

	if len(ids) == 0 {
		return nil, nil
	}

	// Apply CRM contact ID only where not already set
	if crmContactID != nil {
		// Use raw SQL to handle COALESCE for crm_contact_id
		if err := r.db.WithContext(ctx).Exec(
			"UPDATE support_conversations SET customer_email = ?, customer_name = ?, crm_contact_id = COALESCE(crm_contact_id, ?) WHERE id IN ? AND (customer_email IS NULL OR customer_email = '')",
			email, name, *crmContactID, ids,
		).Error; err != nil {
			return nil, fmt.Errorf("backfill conversation identity: %w", err)
		}
	} else {
		if err := query.Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("backfill conversation identity: %w", err)
		}
	}

	return ids, nil
}

// UpdateSessionsByAnonymousID batch-updates all anonymous sessions for a visitor
// with the provided email and name.
func (r *SupportInboxSessionRepository) UpdateSessionsByAnonymousID(ctx context.Context, workspaceID, anonymousID, email, name string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.SupportWidgetSession{}).
		Where("workspace_id = ? AND anonymous_id = ? AND is_anonymous = true", workspaceID, anonymousID).
		Updates(map[string]interface{}{
			"customer_email": email,
			"customer_name":  name,
			"is_anonymous":   false,
		}).Error; err != nil {
		return fmt.Errorf("backfill session identity: %w", err)
	}
	return nil
}

// DB returns the underlying *gorm.DB for transaction support.
func (r *SupportConversationRepository) DB() *gorm.DB {
	return r.db
}

// WithTx returns a new SupportConversationRepository using the given transaction.
func (r *SupportConversationRepository) WithTx(tx *gorm.DB) *SupportConversationRepository {
	return &SupportConversationRepository{db: tx}
}

// WithTx returns a new SupportInboxSessionRepository using the given transaction.
func (r *SupportInboxSessionRepository) WithTx(tx *gorm.DB) *SupportInboxSessionRepository {
	return &SupportInboxSessionRepository{db: tx}
}

// WithTx returns a new CRMContactRepository using the given transaction.
func (r *CRMContactRepository) WithTx(tx *gorm.DB) *CRMContactRepository {
	return &CRMContactRepository{db: tx}
}

// SupportCannedResponseRepository handles canned responses.
type SupportCannedResponseRepository struct {
	db *gorm.DB
}

// NewSupportCannedResponseRepository creates a new SupportCannedResponseRepository.
func NewSupportCannedResponseRepository(db *gorm.DB) *SupportCannedResponseRepository {
	return &SupportCannedResponseRepository{db: db}
}

// List returns canned responses for a workspace.
func (r *SupportCannedResponseRepository) List(ctx context.Context, workspaceID string) ([]model.SupportCannedResponse, error) {
	var responses []model.SupportCannedResponse
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("title ASC").
		Find(&responses).Error; err != nil {
		return nil, fmt.Errorf("list canned responses: %w", err)
	}
	return responses, nil
}

// Search returns canned responses matching a query.
func (r *SupportCannedResponseRepository) Search(ctx context.Context, workspaceID, query string) ([]model.SupportCannedResponse, error) {
	// Escape LIKE wildcards in the user-provided search term.
	escaped := escapeLike(query)
	pattern := "%" + escaped + "%"

	var responses []model.SupportCannedResponse
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Where("short_code LIKE ? OR title LIKE ? OR content LIKE ?", pattern, pattern, pattern).
		Order("short_code ASC").
		Limit(10).
		Find(&responses).Error; err != nil {
		return nil, fmt.Errorf("search canned responses: %w", err)
	}
	return responses, nil
}

// escapeLike escapes LIKE pattern special characters (%, _, \).
func escapeLike(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
}

// GetByID returns a single canned response.
func (r *SupportCannedResponseRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.SupportCannedResponse, error) {
	var response model.SupportCannedResponse
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&response).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get canned response: %w", err)
	}
	return &response, nil
}

// Create creates a new canned response.
func (r *SupportCannedResponseRepository) Create(ctx context.Context, response *model.SupportCannedResponse) error {
	if err := r.db.WithContext(ctx).Create(response).Error; err != nil {
		return fmt.Errorf("create canned response: %w", err)
	}
	return nil
}

// Update saves a canned response.
func (r *SupportCannedResponseRepository) Update(ctx context.Context, response *model.SupportCannedResponse) error {
	if err := r.db.WithContext(ctx).Save(response).Error; err != nil {
		return fmt.Errorf("update canned response: %w", err)
	}
	return nil
}

// Delete removes a canned response.
func (r *SupportCannedResponseRepository) Delete(ctx context.Context, workspaceID, id string) error {
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).Delete(&model.SupportCannedResponse{}).Error; err != nil {
		return fmt.Errorf("delete canned response: %w", err)
	}
	return nil
}
