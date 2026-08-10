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

// ListByWorkspaceUser returns the user's unarchived chats, most recently active first.
func (r *DockChatRepository) ListByWorkspaceUser(ctx context.Context, workspaceID, userID string, limit int, before *time.Time, beforeID string) ([]model.DockChat, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND user_id = ? AND archived_at IS NULL", workspaceID, userID)
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

// TouchLastMessage bumps the chat's last activity timestamp.
func (r *DockChatRepository) TouchLastMessage(ctx context.Context, workspaceID, id string, at time.Time) error {
	return r.Update(ctx, workspaceID, id, map[string]interface{}{"last_message_at": at})
}
