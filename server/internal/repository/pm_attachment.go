package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMAttachmentRepository handles DB operations for attachments.
type PMAttachmentRepository struct {
	db *gorm.DB
}

// PMAttachmentClone pairs a source attachment with the cloned target record.
type PMAttachmentClone struct {
	Source model.PMAttachment
	Clone  model.PMAttachment
}

// NewPMAttachmentRepository creates a new PMAttachmentRepository.
func NewPMAttachmentRepository(db *gorm.DB) *PMAttachmentRepository {
	return &PMAttachmentRepository{db: db}
}

// DB returns the underlying database handle for transaction orchestration.
func (r *PMAttachmentRepository) DB() *gorm.DB {
	return r.db
}

// WithTx returns a repository bound to the provided transaction.
func (r *PMAttachmentRepository) WithTx(tx *gorm.DB) *PMAttachmentRepository {
	return NewPMAttachmentRepository(tx)
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
	uniqueIDs := make([]string, 0, len(attachmentIDs))
	seen := make(map[string]struct{}, len(attachmentIDs))
	for _, id := range attachmentIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	var matchedCount int64
	if err := r.db.WithContext(ctx).
		Model(&model.PMAttachment{}).
		Where("id IN ?", uniqueIDs).
		Count(&matchedCount).Error; err != nil {
		return fmt.Errorf("reassign attachments: count attachments: %w", err)
	}
	if matchedCount != int64(len(uniqueIDs)) {
		return fmt.Errorf("reassign attachments: matched %d of %d attachments", matchedCount, len(uniqueIDs))
	}
	result := r.db.WithContext(ctx).
		Model(&model.PMAttachment{}).
		Where("id IN ?", uniqueIDs).
		Updates(map[string]interface{}{
			"entity_type": entityType,
			"entity_id":   entityID,
		})
	if result.Error != nil {
		return fmt.Errorf("reassign attachments: %w", result.Error)
	}
	if result.RowsAffected != int64(len(uniqueIDs)) {
		return fmt.Errorf("reassign attachments: updated %d of %d attachments", result.RowsAffected, len(uniqueIDs))
	}
	return nil
}

// CloneUploadedFromEntityToEntity creates new attachment records for a target entity
// while reusing the already-uploaded storage objects from the source entity.
func (r *PMAttachmentRepository) CloneUploadedFromEntityToEntity(ctx context.Context, sourceEntityType, sourceEntityID, targetEntityType, targetEntityID string) ([]model.PMAttachment, error) {
	pairs, err := r.CloneUploadedFromEntityToEntityWithSources(ctx, sourceEntityType, sourceEntityID, targetEntityType, targetEntityID)
	if err != nil {
		return nil, err
	}
	cloned := make([]model.PMAttachment, 0, len(pairs))
	for _, pair := range pairs {
		cloned = append(cloned, pair.Clone)
	}
	return cloned, nil
}

// CloneUploadedFromEntityToEntityWithSources creates new attachment records for a target
// entity and returns each source attachment paired with its clone.
func (r *PMAttachmentRepository) CloneUploadedFromEntityToEntityWithSources(ctx context.Context, sourceEntityType, sourceEntityID, targetEntityType, targetEntityID string) ([]PMAttachmentClone, error) {
	sourceAttachments, err := r.List(ctx, sourceEntityType, sourceEntityID)
	if err != nil {
		return nil, err
	}
	return r.cloneUploadedAttachmentsToEntity(ctx, sourceAttachments, targetEntityType, targetEntityID)
}

// CloneUploadedByIDToEntity creates target attachment records from uploaded source IDs,
// regardless of their current owning entity.
func (r *PMAttachmentRepository) CloneUploadedByIDToEntity(ctx context.Context, sourceIDs []string, targetEntityType, targetEntityID string) ([]PMAttachmentClone, error) {
	if len(sourceIDs) == 0 {
		return []PMAttachmentClone{}, nil
	}
	var sourceAttachments []model.PMAttachment
	if err := r.db.WithContext(ctx).
		Where("id IN ? AND is_uploaded = true", sourceIDs).
		Find(&sourceAttachments).Error; err != nil {
		return nil, fmt.Errorf("list attachments by id: %w", err)
	}
	byID := make(map[string]model.PMAttachment, len(sourceAttachments))
	for _, attachment := range sourceAttachments {
		byID[attachment.ID] = attachment
	}
	ordered := make([]model.PMAttachment, 0, len(sourceIDs))
	seen := make(map[string]bool, len(sourceIDs))
	for _, id := range sourceIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if attachment, ok := byID[id]; ok {
			ordered = append(ordered, attachment)
		}
	}
	return r.cloneUploadedAttachmentsToEntity(ctx, ordered, targetEntityType, targetEntityID)
}

func (r *PMAttachmentRepository) cloneUploadedAttachmentsToEntity(ctx context.Context, sourceAttachments []model.PMAttachment, targetEntityType, targetEntityID string) ([]PMAttachmentClone, error) {
	if len(sourceAttachments) == 0 {
		return []PMAttachmentClone{}, nil
	}

	pairs := make([]PMAttachmentClone, 0, len(sourceAttachments))
	now := time.Now().UTC()
	for _, source := range sourceAttachments {
		attachment := model.PMAttachment{
			WorkspaceID:  source.WorkspaceID,
			EntityType:   targetEntityType,
			EntityID:     targetEntityID,
			FileName:     source.FileName,
			FileSize:     source.FileSize,
			ContentType:  source.ContentType,
			StorageKey:   source.StorageKey,
			IsUploaded:   source.IsUploaded,
			UploadedByID: source.UploadedByID,
			CreatedAt:    now,
		}
		if err := r.db.WithContext(ctx).Create(&attachment).Error; err != nil {
			return nil, fmt.Errorf("clone attachment: %w", err)
		}
		pairs = append(pairs, PMAttachmentClone{Source: source, Clone: attachment})
	}
	return pairs, nil
}

// CountByStorageKey returns the number of attachment rows referencing a storage object.
func (r *PMAttachmentRepository) CountByStorageKey(ctx context.Context, storageKey string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.PMAttachment{}).
		Where("storage_key = ?", storageKey).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count attachments by storage key: %w", err)
	}
	return count, nil
}

// Delete removes an attachment record.
func (r *PMAttachmentRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMAttachment{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}
	return nil
}

// DeleteEditorUpload removes an attachment only if it is still a pending editor upload.
func (r *PMAttachmentRepository) DeleteEditorUpload(ctx context.Context, id string) (bool, error) {
	result := r.db.WithContext(ctx).
		Where("id = ? AND entity_type = ?", id, "editor_upload").
		Delete(&model.PMAttachment{})
	if result.Error != nil {
		return false, fmt.Errorf("delete attachment: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}
