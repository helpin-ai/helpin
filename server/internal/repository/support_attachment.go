package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportAttachmentRepository handles DB operations for support attachments.
type SupportAttachmentRepository struct {
	db *gorm.DB
}

// NewSupportAttachmentRepository creates a new SupportAttachmentRepository.
func NewSupportAttachmentRepository(db *gorm.DB) *SupportAttachmentRepository {
	return &SupportAttachmentRepository{db: db}
}

// Create inserts a support attachment record.
func (r *SupportAttachmentRepository) Create(ctx context.Context, attachment *model.SupportAttachment) error {
	if err := r.db.WithContext(ctx).Create(attachment).Error; err != nil {
		return fmt.Errorf("create support attachment: %w", err)
	}
	return nil
}

// GetByID returns a support attachment by its ID.
func (r *SupportAttachmentRepository) GetByID(ctx context.Context, id string) (*model.SupportAttachment, error) {
	var attachment model.SupportAttachment
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&attachment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get support attachment: %w", err)
	}
	return &attachment, nil
}

// ConfirmUpload marks a support attachment as uploaded.
func (r *SupportAttachmentRepository) ConfirmUpload(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Model(&model.SupportAttachment{}).
		Where("id = ?", id).
		Update("is_uploaded", true)
	if result.Error != nil {
		return fmt.Errorf("confirm support attachment upload: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("support attachment not found")
	}
	return nil
}

// UpdateStorageKey sets the storage key and public URL on a support attachment.
func (r *SupportAttachmentRepository) UpdateStorageKey(ctx context.Context, id, storageKey, publicURL string) error {
	result := r.db.WithContext(ctx).
		Model(&model.SupportAttachment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"storage_key": storageKey,
			"public_url":  publicURL,
		})
	if result.Error != nil {
		return fmt.Errorf("update support attachment storage key: %w", result.Error)
	}
	return nil
}

// LinkToMessage sets the message_id on the given attachment IDs.
func (r *SupportAttachmentRepository) LinkToMessage(ctx context.Context, attachmentIDs []string, messageID string) error {
	if len(attachmentIDs) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Model(&model.SupportAttachment{}).
		Where("id IN ? AND is_uploaded = true", attachmentIDs).
		Update("message_id", messageID).Error; err != nil {
		return fmt.Errorf("link support attachments to message: %w", err)
	}
	return nil
}

// ListByMessageIDs returns uploaded attachments for a batch of message IDs.
func (r *SupportAttachmentRepository) ListByMessageIDs(ctx context.Context, messageIDs []string) ([]model.SupportAttachment, error) {
	if len(messageIDs) == 0 {
		return nil, nil
	}
	var attachments []model.SupportAttachment
	if err := r.db.WithContext(ctx).
		Where("message_id IN ? AND is_uploaded = true", messageIDs).
		Order("created_at ASC").
		Find(&attachments).Error; err != nil {
		return nil, fmt.Errorf("list support attachments by message IDs: %w", err)
	}
	return attachments, nil
}

// Delete removes a support attachment record.
func (r *SupportAttachmentRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.SupportAttachment{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete support attachment: %w", err)
	}
	return nil
}
