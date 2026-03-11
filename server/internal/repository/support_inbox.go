package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportTicketRepository handles DB operations for support tickets.
type SupportTicketRepository struct {
	db *gorm.DB
}

// NewSupportTicketRepository creates a new SupportTicketRepository.
func NewSupportTicketRepository(db *gorm.DB) *SupportTicketRepository {
	return &SupportTicketRepository{db: db}
}

// List returns tickets with optional filters and pagination.
func (r *SupportTicketRepository) List(ctx context.Context, workspaceID string, status string, priority string, pagination model.PMPagination) ([]model.SupportTicket, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.SupportTicket{}).Where("workspace_id = ?", workspaceID)

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count tickets: %w", err)
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

	var tickets []model.SupportTicket
	if err := query.Order("updated_at DESC").Offset(offset).Limit(perPage).Find(&tickets).Error; err != nil {
		return nil, 0, fmt.Errorf("list tickets: %w", err)
	}
	return tickets, total, nil
}

// GetByID returns a single ticket.
func (r *SupportTicketRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.SupportTicket, error) {
	var ticket model.SupportTicket
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&ticket).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get ticket: %w", err)
	}
	return &ticket, nil
}

// ListByIDs returns tickets by ID for a workspace.
func (r *SupportTicketRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.SupportTicket, error) {
	if len(ids) == 0 {
		return []model.SupportTicket{}, nil
	}

	var tickets []model.SupportTicket
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Order("updated_at DESC").
		Find(&tickets).Error; err != nil {
		return nil, fmt.Errorf("list tickets by ids: %w", err)
	}
	return tickets, nil
}

// Create creates a new ticket with auto-allocated display_id.
func (r *SupportTicketRepository) Create(ctx context.Context, ticket *model.SupportTicket) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Advisory lock on workspace to prevent duplicate display_id under concurrency.
		tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", ticket.WorkspaceID)

		var maxDisplayID int
		tx.Model(&model.SupportTicket{}).Where("workspace_id = ?", ticket.WorkspaceID).
			Select("COALESCE(MAX(display_id), 0)").Scan(&maxDisplayID)
		ticket.DisplayID = maxDisplayID + 1

		if err := tx.Create(ticket).Error; err != nil {
			return fmt.Errorf("create ticket: %w", err)
		}
		return nil
	})
}

// Update saves a ticket.
func (r *SupportTicketRepository) Update(ctx context.Context, ticket *model.SupportTicket) error {
	if err := r.db.WithContext(ctx).Save(ticket).Error; err != nil {
		return fmt.Errorf("update ticket: %w", err)
	}
	return nil
}

// ListByLinkedStoryIDs returns tickets linked to any of the provided stories.
func (r *SupportTicketRepository) ListByLinkedStoryIDs(ctx context.Context, workspaceID string, storyIDs []string) ([]model.SupportTicket, error) {
	if len(storyIDs) == 0 {
		return []model.SupportTicket{}, nil
	}

	var tickets []model.SupportTicket
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND linked_story_id IN ?", workspaceID, storyIDs).
		Order("updated_at DESC").
		Find(&tickets).Error; err != nil {
		return nil, fmt.Errorf("list tickets by linked stories: %w", err)
	}
	return tickets, nil
}

// ListByContact returns tickets linked to a CRM contact.
func (r *SupportTicketRepository) ListByContact(ctx context.Context, workspaceID, contactID string, pagination model.PMPagination) ([]model.SupportTicket, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.SupportTicket{}).
		Where("workspace_id = ? AND crm_contact_id = ?", workspaceID, contactID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count contact tickets: %w", err)
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

	var tickets []model.SupportTicket
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&tickets).Error; err != nil {
		return nil, 0, fmt.Errorf("list contact tickets: %w", err)
	}
	return tickets, total, nil
}

// SupportMessageRepository handles DB operations for support messages.
type SupportMessageRepository struct {
	db *gorm.DB
}

// NewSupportMessageRepository creates a new SupportMessageRepository.
func NewSupportMessageRepository(db *gorm.DB) *SupportMessageRepository {
	return &SupportMessageRepository{db: db}
}

// ListByTicket returns messages for a conversation (queries conversation_id, falls back to ticket_id for legacy data).
func (r *SupportMessageRepository) ListByTicket(ctx context.Context, workspaceID, ticketID string, includeInternal bool) ([]model.SupportMessage, error) {
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND (conversation_id = ? OR ticket_id = ?)", workspaceID, ticketID, ticketID)
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
	return nil
}

// WidgetInstallationRepository handles widget installations.
type WidgetInstallationRepository struct {
	db *gorm.DB
}

// NewWidgetInstallationRepository creates a new WidgetInstallationRepository.
func NewWidgetInstallationRepository(db *gorm.DB) *WidgetInstallationRepository {
	return &WidgetInstallationRepository{db: db}
}

// GetByWorkspace returns the widget installation for a workspace.
func (r *WidgetInstallationRepository) GetByWorkspace(ctx context.Context, workspaceID string) (*model.SupportWidgetInstallation, error) {
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
func (r *WidgetInstallationRepository) GetByWidgetKey(ctx context.Context, widgetKey string) (*model.SupportWidgetInstallation, error) {
	var inst model.SupportWidgetInstallation
	if err := r.db.WithContext(ctx).Where("widget_key = ? AND active = true", widgetKey).First(&inst).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get widget by key: %w", err)
	}
	return &inst, nil
}

// Create creates a new installation.
func (r *WidgetInstallationRepository) Create(ctx context.Context, inst *model.SupportWidgetInstallation) error {
	if err := r.db.WithContext(ctx).Create(inst).Error; err != nil {
		return fmt.Errorf("create widget installation: %w", err)
	}
	return nil
}

// Update saves an installation.
func (r *WidgetInstallationRepository) Update(ctx context.Context, inst *model.SupportWidgetInstallation) error {
	if err := r.db.WithContext(ctx).Save(inst).Error; err != nil {
		return fmt.Errorf("update widget installation: %w", err)
	}
	return nil
}

// WidgetSessionRepository handles widget sessions.
type WidgetSessionRepository struct {
	db *gorm.DB
}

// NewWidgetSessionRepository creates a new WidgetSessionRepository.
func NewWidgetSessionRepository(db *gorm.DB) *WidgetSessionRepository {
	return &WidgetSessionRepository{db: db}
}

// GetByToken returns a session by token.
func (r *WidgetSessionRepository) GetByToken(ctx context.Context, token string) (*model.SupportWidgetSession, error) {
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
func (r *WidgetSessionRepository) Create(ctx context.Context, session *model.SupportWidgetSession) error {
	if err := r.db.WithContext(ctx).Create(session).Error; err != nil {
		return fmt.Errorf("create widget session: %w", err)
	}
	return nil
}

// Update saves a session.
func (r *WidgetSessionRepository) Update(ctx context.Context, session *model.SupportWidgetSession) error {
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
	query := r.db.WithContext(ctx).Model(&model.SupportConversation{}).Where("workspace_id = ?", workspaceID)

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
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

	var conversations []model.SupportConversation
	if err := query.Order("updated_at DESC").Offset(offset).Limit(perPage).Find(&conversations).Error; err != nil {
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
