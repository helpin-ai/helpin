package repository

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

type CommandBarChatRepository struct {
	db *gorm.DB
}

func NewCommandBarChatRepository(db *gorm.DB) *CommandBarChatRepository {
	return &CommandBarChatRepository{db: db}
}

func (r *CommandBarChatRepository) CreateThread(ctx context.Context, thread *model.CommandBarThread) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return r.db.WithContext(ctx).Create(thread).Error
}

func (r *CommandBarChatRepository) GetThread(ctx context.Context, workspaceID, id string) (*model.CommandBarThread, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	var thread model.CommandBarThread
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		First(&thread).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &thread, nil
}

func (r *CommandBarChatRepository) ListRecentThreads(ctx context.Context, workspaceID, actorID string, limit int) ([]model.CommandBarThread, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	q := r.db.WithContext(ctx).
		Where("workspace_id = ? AND status = ?", workspaceID, model.CommandBarThreadStatusOpen)
	if actorID == "" {
		q = q.Where("actor_id IS NULL")
	} else {
		q = q.Where("actor_id = ?", actorID)
	}
	var threads []model.CommandBarThread
	if err := q.Order("updated_at DESC").Limit(limit).Find(&threads).Error; err != nil {
		return nil, err
	}
	return threads, nil
}

func (r *CommandBarChatRepository) CreateMessage(ctx context.Context, message *model.CommandBarMessage) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		updatedAt := message.CreatedAt
		if updatedAt.IsZero() {
			updatedAt = time.Now()
		}
		return tx.Model(&model.CommandBarThread{}).
			Where("workspace_id = ? AND id = ?", message.WorkspaceID, message.ThreadID).
			Update("updated_at", updatedAt).
			Error
	})
}

func (r *CommandBarChatRepository) GetMessage(ctx context.Context, workspaceID, id string) (*model.CommandBarMessage, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	var message model.CommandBarMessage
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		First(&message).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *CommandBarChatRepository) ListRecentMessages(ctx context.Context, workspaceID, threadID string, limit int) ([]model.CommandBarMessage, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var messages []model.CommandBarMessage
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND thread_id = ?", workspaceID, threadID).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error; err != nil {
		return nil, err
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}
