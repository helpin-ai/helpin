package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type SupportTagRepository struct {
	db *gorm.DB
}

func NewSupportTagRepository(db *gorm.DB) *SupportTagRepository {
	return &SupportTagRepository{db: db}
}

func (r *SupportTagRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.SupportTag, error) {
	var tags []model.SupportTag
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("LOWER(name) ASC").
		Find(&tags).Error; err != nil {
		return nil, fmt.Errorf("list support tags: %w", err)
	}
	return tags, nil
}

func (r *SupportTagRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.SupportTag, error) {
	var tag model.SupportTag
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		First(&tag).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get support tag: %w", err)
	}
	return &tag, nil
}

func (r *SupportTagRepository) GetByName(ctx context.Context, workspaceID, name string) (*model.SupportTag, error) {
	var tag model.SupportTag
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND LOWER(name) = LOWER(?)", workspaceID, strings.TrimSpace(name)).
		First(&tag).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get support tag by name: %w", err)
	}
	return &tag, nil
}

func (r *SupportTagRepository) Create(ctx context.Context, tag *model.SupportTag) error {
	if err := r.db.WithContext(ctx).Create(tag).Error; err != nil {
		return fmt.Errorf("create support tag: %w", err)
	}
	return nil
}

func (r *SupportTagRepository) Update(ctx context.Context, tag *model.SupportTag) error {
	if err := r.db.WithContext(ctx).Save(tag).Error; err != nil {
		return fmt.Errorf("update support tag: %w", err)
	}
	return nil
}

func (r *SupportTagRepository) Delete(ctx context.Context, workspaceID, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tag_id = ?", id).Delete(&model.SupportConversationTag{}).Error; err != nil {
			return fmt.Errorf("delete support conversation tag links: %w", err)
		}
		if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, id).Delete(&model.SupportTag{}).Error; err != nil {
			return fmt.Errorf("delete support tag: %w", err)
		}
		return nil
	})
}

func (r *SupportTagRepository) AddConversationTag(ctx context.Context, workspaceID, conversationID, tagID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.SupportConversation{}).
			Where("workspace_id = ? AND id = ?", workspaceID, conversationID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("check support conversation: %w", err)
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(&model.SupportTag{}).
			Where("workspace_id = ? AND id = ?", workspaceID, tagID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("check support tag: %w", err)
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		link := model.SupportConversationTag{ConversationID: conversationID, TagID: tagID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
			return fmt.Errorf("add support conversation tag: %w", err)
		}
		return nil
	})
}

func (r *SupportTagRepository) RemoveConversationTag(ctx context.Context, workspaceID, conversationID, tagID string) error {
	err := r.db.WithContext(ctx).
		Where(`conversation_id = ? AND tag_id = ? AND EXISTS (
			SELECT 1 FROM support_conversations sc
			WHERE sc.id = support_conversation_tags.conversation_id
			  AND sc.workspace_id = ?
		)`, conversationID, tagID, workspaceID).
		Delete(&model.SupportConversationTag{}).Error
	if err != nil {
		return fmt.Errorf("remove support conversation tag: %w", err)
	}
	return nil
}

func (r *SupportTagRepository) ListByConversationIDs(ctx context.Context, workspaceID string, conversationIDs []string) (map[string][]model.SupportTag, error) {
	result := make(map[string][]model.SupportTag, len(conversationIDs))
	if len(conversationIDs) == 0 {
		return result, nil
	}
	type row struct {
		ConversationID string  `gorm:"column:conversation_id"`
		ID             string  `gorm:"column:id"`
		WorkspaceID    string  `gorm:"column:workspace_id"`
		Name           string  `gorm:"column:name"`
		Color          *string `gorm:"column:color"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("support_tags").
		Select("support_tags.*, support_conversation_tags.conversation_id").
		Joins("JOIN support_conversation_tags ON support_conversation_tags.tag_id = support_tags.id").
		Where("support_tags.workspace_id = ? AND support_conversation_tags.conversation_id IN ?", workspaceID, conversationIDs).
		Order("LOWER(support_tags.name) ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list support tags by conversation: %w", err)
	}
	for _, item := range rows {
		tag := model.SupportTag{
			ID:          item.ID,
			WorkspaceID: item.WorkspaceID,
			Name:        item.Name,
			Color:       item.Color,
		}
		result[item.ConversationID] = append(result[item.ConversationID], tag)
	}
	return result, nil
}
