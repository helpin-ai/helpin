package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMAttachmentRepository handles DB operations for attachments.
type PMAttachmentRepository struct {
	db *gorm.DB
}

// NewPMAttachmentRepository creates a new PMAttachmentRepository.
func NewPMAttachmentRepository(db *gorm.DB) *PMAttachmentRepository {
	return &PMAttachmentRepository{db: db}
}

// Create inserts an attachment record.
func (r *PMAttachmentRepository) Create(ctx context.Context, attachment *model.PMAttachment) error {
	if err := r.db.WithContext(ctx).Create(attachment).Error; err != nil {
		return fmt.Errorf("create attachment: %w", err)
	}
	return nil
}

// GetByID returns an attachment by its ID.
func (r *PMAttachmentRepository) GetByID(ctx context.Context, id string) (*model.PMAttachment, error) {
	var attachment model.PMAttachment
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&attachment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get attachment: %w", err)
	}
	return &attachment, nil
}

// List returns uploaded attachments for a given entity.
func (r *PMAttachmentRepository) List(ctx context.Context, entityType, entityID string) ([]model.PMAttachment, error) {
	var attachments []model.PMAttachment
	if err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ? AND is_uploaded = true", entityType, entityID).
		Order("created_at ASC").
		Find(&attachments).Error; err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	return attachments, nil
}

// ConfirmUpload marks an attachment as uploaded.
func (r *PMAttachmentRepository) ConfirmUpload(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Model(&model.PMAttachment{}).
		Where("id = ?", id).
		Update("is_uploaded", true)
	if result.Error != nil {
		return fmt.Errorf("confirm upload: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("attachment not found")
	}
	return nil
}

// UpdateStorageKey sets the storage key on an attachment.
func (r *PMAttachmentRepository) UpdateStorageKey(ctx context.Context, id, storageKey string) error {
	result := r.db.WithContext(ctx).
		Model(&model.PMAttachment{}).
		Where("id = ?", id).
		Update("storage_key", storageKey)
	if result.Error != nil {
		return fmt.Errorf("update storage key: %w", result.Error)
	}
	return nil
}

// ListByWorkspace returns all uploaded attachments for a workspace with their storage keys.
func (r *PMAttachmentRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.PMAttachment, error) {
	var attachments []model.PMAttachment
	if err := r.db.WithContext(ctx).
		Select("id, storage_key").
		Where("workspace_id = ? AND is_uploaded = true AND storage_key != ''", workspaceID).
		Find(&attachments).Error; err != nil {
		return nil, fmt.Errorf("list workspace attachments: %w", err)
	}
	return attachments, nil
}

// ReassignToEntity updates attachments to point to a target entity.
func (r *PMAttachmentRepository) ReassignToEntity(ctx context.Context, attachmentIDs []string, entityType, entityID string) error {
	if len(attachmentIDs) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Model(&model.PMAttachment{}).
		Where("id IN ?", attachmentIDs).
		Updates(map[string]interface{}{
			"entity_type": entityType,
			"entity_id":   entityID,
		}).Error; err != nil {
		return fmt.Errorf("reassign attachments: %w", err)
	}
	return nil
}

// Delete removes an attachment record.
func (r *PMAttachmentRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMAttachment{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}
	return nil
}
