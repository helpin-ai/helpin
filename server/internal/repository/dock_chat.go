package repository

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// DockChatRepository persists dock chats.
type DockChatRepository struct {
	db *gorm.DB
}

// NewDockChatRepository creates a DockChatRepository.
func NewDockChatRepository(db *gorm.DB) *DockChatRepository {
	return &DockChatRepository{db: db}
}

// Create inserts a new dock chat.
func (r *DockChatRepository) Create(ctx context.Context, chat *model.DockChat) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return r.db.WithContext(ctx).Create(chat).Error
}

// GetByID returns a chat by workspace and id, or nil when not found.
func (r *DockChatRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.DockChat, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	var chat model.DockChat
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		First(&chat).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &chat, nil
}

// ListBySupportConversation returns unarchived chats associated with a support
// conversation, most recently active first. The service applies visibility
// authorization before selecting a chat for the requester.
func (r *DockChatRepository) ListBySupportConversation(ctx context.Context, workspaceID, conversationID string) ([]model.DockChat, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	var chats []model.DockChat
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND support_conversation_id = ? AND archived_at IS NULL", workspaceID, conversationID).
		Order("COALESCE(last_message_at, created_at) DESC").
		Order("id DESC").
		Find(&chats).Error
	if err != nil {
		return nil, err
	}
	return chats, nil
}

// ListVisible returns the requester's private chats plus chats shared with the
// workspace or with a module the requester can access.
func (r *DockChatRepository) ListVisible(ctx context.Context, workspaceID, userID string, modules []model.ModuleID, limit int, before *time.Time, beforeID string) ([]model.DockChat, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND archived_at IS NULL", workspaceID).
		Where("TRIM(COALESCE(title, '')) <> '' OR last_message_at IS NOT NULL OR active_run_id IS NOT NULL")
	visibility := r.db.Where("user_id = ?", userID).
		Or("visibility = ?", model.DockChatVisibilityWorkspace)
	if len(modules) > 0 {
		visibility = visibility.Or("visibility = ? AND module_id IN ?", model.DockChatVisibilityModule, modules)
	}
	query = query.Where(visibility)
	if before != nil {
		query = query.Where(
			"((COALESCE(last_message_at, created_at) < ?) OR (COALESCE(last_message_at, created_at) = ? AND id < ?))",
			*before, *before, beforeID,
		)
	}
	var chats []model.DockChat
	err := query.
		Order("COALESCE(last_message_at, created_at) DESC").
		Order("id DESC").
		Limit(limit).
		Find(&chats).Error
	if err != nil {
		return nil, err
	}
	return chats, nil
}

// Update applies partial updates to a chat.
func (r *DockChatRepository) Update(ctx context.Context, workspaceID, id string, updates map[string]interface{}) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&model.DockChat{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Updates(updates).Error
}

// SetActiveRun points the chat at its current backing run.
func (r *DockChatRepository) SetActiveRun(ctx context.Context, workspaceID, id, runID string) error {
	return r.Update(ctx, workspaceID, id, map[string]interface{}{"active_run_id": runID})
}

// SetTitleIfEmpty assigns an automatic title without overwriting a user rename.
func (r *DockChatRepository) SetTitleIfEmpty(ctx context.Context, workspaceID, id, title string) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return r.db.WithContext(ctx).
		Model(&model.DockChat{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Where("TRIM(COALESCE(title, '')) = ''").
		Update("title", title).Error
}

// TouchLastMessage bumps the chat's last activity timestamp.
func (r *DockChatRepository) TouchLastMessage(ctx context.Context, workspaceID, id string, at time.Time) error {
	return r.Update(ctx, workspaceID, id, map[string]interface{}{"last_message_at": at})
}

// WithTurnLock serializes settings transitions and message admission across API
// replicas without holding a row lock during runtime callbacks.
func (r *DockChatRepository) WithTurnLock(ctx context.Context, workspaceID, chatID string, fn func() error) error {
	if r.db.Dialector.Name() != "postgres" {
		return fn()
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "dock-turn:"+workspaceID+":"+chatID).Error; err != nil {
			return err
		}
		return fn()
	})
}
