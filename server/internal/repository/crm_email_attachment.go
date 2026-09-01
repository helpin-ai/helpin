package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

type CRMEmailAttachmentRepository struct{ db *gorm.DB }

func NewCRMEmailAttachmentRepository(db *gorm.DB) *CRMEmailAttachmentRepository {
	return &CRMEmailAttachmentRepository{db: db}
}

func (r *CRMEmailAttachmentRepository) Create(ctx context.Context, attachment *model.CRMEmailAttachment) error {
	if err := r.db.WithContext(ctx).Create(attachment).Error; err != nil {
		return fmt.Errorf("create CRM email attachment: %w", err)
	}
	return nil
}

func (r *CRMEmailAttachmentRepository) UpdateStorageKey(ctx context.Context, id, storageKey string) error {
	return r.db.WithContext(ctx).Model(&model.CRMEmailAttachment{}).Where("id = ?", id).Update("storage_key", storageKey).Error
}

func (r *CRMEmailAttachmentRepository) Get(ctx context.Context, workspaceID, id string) (*model.CRMEmailAttachment, error) {
	var attachment model.CRMEmailAttachment
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&attachment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get CRM email attachment: %w", err)
	}
	return &attachment, nil
}

func (r *CRMEmailAttachmentRepository) Confirm(ctx context.Context, workspaceID, id, userID string) error {
	result := r.db.WithContext(ctx).Model(&model.CRMEmailAttachment{}).
		Where("workspace_id = ? AND id = ? AND uploaded_by_id = ? AND message_id IS NULL", workspaceID, id, userID).
		Update("is_uploaded", true)
	if result.Error != nil {
		return fmt.Errorf("confirm CRM email attachment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("attachment not found")
	}
	return nil
}

func (r *CRMEmailAttachmentRepository) Delete(ctx context.Context, workspaceID, id, userID string) error {
	result := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ? AND uploaded_by_id = ? AND message_id IS NULL", workspaceID, id, userID).Delete(&model.CRMEmailAttachment{})
	if result.Error != nil {
		return fmt.Errorf("delete CRM email attachment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("attachment not found")
	}
	return nil
}

func (r *CRMEmailAttachmentRepository) ListDraft(ctx context.Context, workspaceID, draftID, userID string, ids []string) ([]model.CRMEmailAttachment, error) {
	var attachments []model.CRMEmailAttachment
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND draft_id = ? AND uploaded_by_id = ? AND id IN ? AND is_uploaded = true AND message_id IS NULL", workspaceID, draftID, userID, ids).
		Order("created_at ASC").Find(&attachments).Error
	if err != nil {
		return nil, fmt.Errorf("list CRM email draft attachments: %w", err)
	}
	return attachments, nil
}

func (r *CRMEmailAttachmentRepository) Link(ctx context.Context, workspaceID, draftID, userID, messageID string, ids []string) error {
	result := r.db.WithContext(ctx).Model(&model.CRMEmailAttachment{}).
		Where("workspace_id = ? AND draft_id = ? AND uploaded_by_id = ? AND id IN ? AND is_uploaded = true AND message_id IS NULL", workspaceID, draftID, userID, ids).
		Update("message_id", messageID)
	if result.Error != nil {
		return fmt.Errorf("link CRM email attachments: %w", result.Error)
	}
	if result.RowsAffected != int64(len(ids)) {
		return fmt.Errorf("one or more attachments are unavailable")
	}
	return nil
}

func (r *CRMEmailAttachmentRepository) ListByMessageIDs(ctx context.Context, workspaceID string, messageIDs []string) ([]model.CRMEmailAttachment, error) {
	var attachments []model.CRMEmailAttachment
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND message_id IN ?", workspaceID, messageIDs).Order("created_at ASC").Find(&attachments).Error
	if err != nil {
		return nil, fmt.Errorf("list CRM email message attachments: %w", err)
	}
	return attachments, nil
}

func (r *CRMEmailAttachmentRepository) ListStale(ctx context.Context, before time.Time) ([]model.CRMEmailAttachment, error) {
	var attachments []model.CRMEmailAttachment
	err := r.db.WithContext(ctx).Where("message_id IS NULL AND created_at < ?", before).Find(&attachments).Error
	return attachments, err
}
