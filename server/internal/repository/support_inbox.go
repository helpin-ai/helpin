package repository

import (
	"context"
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
	if err := r.db.WithContext(ctx).Create(session).Error; err != nil {
		return fmt.Errorf("create widget session: %w", err)
	}
	return nil
}

// Update saves a session.
func (r *SupportInboxSessionRepository) Update(ctx context.Context, session *model.SupportWidgetSession) error {
	if err := r.db.WithContext(ctx).Save(session).Error; err != nil {
		return fmt.Errorf("update widget session: %w", err)
	}
	return nil
}

// SupportConversationRepository handles DB operations for support conversations.
type SupportConversationRepository struct {
	db *gorm.DB
}

// NewSupportConversationRepository creates a new SupportConversationRepository.
func NewSupportConversationRepository(db *gorm.DB) *SupportConversationRepository {
	return &SupportConversationRepository{db: db}
}

// List returns conversations with optional filters and pagination.
func (r *SupportConversationRepository) List(ctx context.Context, workspaceID string, status string, priority string, pagination model.PMPagination) ([]model.SupportConversation, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.SupportConversation{}).Where("workspace_id = ?", workspaceID)

	if status != "" {
		base = base.Where("status = ?", status)
	}
	if priority != "" {
		base = base.Where("priority = ?", priority)
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
	fetch := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if status != "" {
		fetch = fetch.Where("status = ?", status)
	}
	if priority != "" {
		fetch = fetch.Where("priority = ?", priority)
	}

	var conversations []model.SupportConversation
	if err := fetch.
		Select(`support_conversations.*, (
			SELECT CASE WHEN m.is_internal THEN 'Note: ' || LEFT(m.content, 100) ELSE LEFT(m.content, 100) END
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
			  AND sm.created_at > COALESCE(support_conversations.team_last_seen_at, '1970-01-01'::timestamptz)
		) AS unread_count`).
		Order("updated_at DESC").Offset(offset).Limit(perPage).Find(&conversations).Error; err != nil {
		return nil, 0, fmt.Errorf("list conversations: %w", err)
	}
	return conversations, total, nil
}

// GetByID returns a single conversation.
func (r *SupportConversationRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.SupportConversation, error) {
	var conversation model.SupportConversation
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&conversation).Error; err != nil {
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
		tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", conversation.WorkspaceID)

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

// UpdateSubject updates only the subject of a conversation.
func (r *SupportConversationRepository) UpdateSubject(ctx context.Context, id, subject string) error {
	if err := r.db.WithContext(ctx).Model(&model.SupportConversation{}).Where("id = ?", id).Update("subject", subject).Error; err != nil {
		return fmt.Errorf("update conversation subject: %w", err)
	}
	return nil
}

// ListByIDs returns conversations by ID for a workspace.
func (r *SupportConversationRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.SupportConversation, error) {
	if len(ids) == 0 {
		return []model.SupportConversation{}, nil
	}

	var conversations []model.SupportConversation
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
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
		Select(`support_conversations.*,
		(SELECT content FROM support_messages WHERE support_messages.conversation_id = support_conversations.id AND support_messages.is_internal = false ORDER BY created_at DESC LIMIT 1) AS last_message,
		(SELECT COUNT(*)
			FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.is_internal = false
			  AND sm.sender_type IN ('user', 'agent', 'ai')
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(support_conversations.contact_last_seen_at, '1970-01-01'::timestamptz)
		) AS unread_count`).
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
	result := r.db.WithContext(ctx).Exec(`
		UPDATE support_conversations
		SET team_last_seen_at = NOW()
		WHERE id = ?
		  AND EXISTS (
			SELECT 1 FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.is_internal = false
			  AND sm.sender_type = 'customer'
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(support_conversations.team_last_seen_at, '1970-01-01'::timestamptz)
		  )
	`, conversationID)
	if result.Error != nil {
		return fmt.Errorf("mark internal read: %w", result.Error)
	}
	return nil
}

// MarkContactRead sets contact_last_seen_at = NOW() if unread messages exist beyond the current cursor.
// Uses raw SQL to avoid GORM's autoUpdateTime touching updated_at (which would re-sort the conversation).
func (r *SupportConversationRepository) MarkContactRead(ctx context.Context, conversationID string) error {
	result := r.db.WithContext(ctx).Exec(`
		UPDATE support_conversations
		SET contact_last_seen_at = NOW()
		WHERE id = ?
		  AND EXISTS (
			SELECT 1 FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.is_internal = false
			  AND sm.sender_type IN ('user', 'agent', 'ai')
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(support_conversations.contact_last_seen_at, '1970-01-01'::timestamptz)
		  )
	`, conversationID)
	if result.Error != nil {
		return fmt.Errorf("mark contact read: %w", result.Error)
	}
	return nil
}

// GetUnreadStats returns aggregate unread conversation counts for a workspace.
func (r *SupportConversationRepository) GetUnreadStats(ctx context.Context, workspaceID, userID string) (model.UnreadStats, error) {
	var stats model.UnreadStats
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*) FILTER (WHERE u.unread > 0) AS total,
			COUNT(*) FILTER (WHERE u.unread > 0 AND sc.opened_by_user_id = ?) AS my_inbox,
			COUNT(*) FILTER (WHERE u.unread > 0 AND sc.assigned_agent_id IS NULL) AS unassigned
		FROM support_conversations sc
		CROSS JOIN LATERAL (
			SELECT COUNT(*) AS unread
			FROM support_messages sm
			WHERE sm.conversation_id = sc.id
			  AND sm.is_internal = false
			  AND sm.sender_type = 'customer'
			  AND sm.message_type = 'reply'
			  AND sm.created_at > COALESCE(sc.team_last_seen_at, '1970-01-01'::timestamptz)
		) u
		WHERE sc.workspace_id = ?
		  AND sc.status != 'closed'
	`, userID, workspaceID).Scan(&stats).Error
	if err != nil {
		return stats, fmt.Errorf("get unread stats: %w", err)
	}
	return stats, nil
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
