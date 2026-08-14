package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsAPIReferenceRepository persists OpenAPI reference sources and revisions.
type DocsAPIReferenceRepository struct {
	db *gorm.DB
}

// NewDocsAPIReferenceRepository creates a DocsAPIReferenceRepository.
func NewDocsAPIReferenceRepository(db *gorm.DB) *DocsAPIReferenceRepository {
	return &DocsAPIReferenceRepository{db: db}
}

// Create inserts a reference and its initial draft revision atomically.
func (r *DocsAPIReferenceRepository) Create(
	ctx context.Context,
	reference *model.DocsAPIReference,
	revision *model.DocsAPIReferenceRevision,
) (*model.DocsAPIReference, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(reference).Error; err != nil {
			return fmt.Errorf("create docs api reference: %w", err)
		}
		revision.APIReferenceID = reference.ID
		if err := tx.Create(revision).Error; err != nil {
			return fmt.Errorf("create docs api reference revision: %w", err)
		}
		reference.DraftRevisionID = &revision.ID
		if err := tx.Model(reference).Update("draft_revision_id", revision.ID).Error; err != nil {
			return fmt.Errorf("set docs api reference draft: %w", err)
		}
		return nil
	})
	return reference, err
}

// ListBySpace returns active references in display order.
func (r *DocsAPIReferenceRepository) ListBySpace(
	ctx context.Context,
	workspaceID, spaceID string,
) ([]model.DocsAPIReference, error) {
	var references []model.DocsAPIReference
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND space_id = ? AND deleted_at IS NULL", workspaceID, spaceID).
		Order("position ASC, created_at ASC").
		Find(&references).Error
	if err != nil {
		return nil, fmt.Errorf("list docs api references: %w", err)
	}
	return references, nil
}

// ListPublishedBySpace returns references that have a published revision.
func (r *DocsAPIReferenceRepository) ListPublishedBySpace(
	ctx context.Context,
	workspaceID, spaceID string,
) ([]model.DocsAPIReference, error) {
	var references []model.DocsAPIReference
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND space_id = ? AND deleted_at IS NULL AND published_revision_id IS NOT NULL", workspaceID, spaceID).
		Order("position ASC, created_at ASC").
		Find(&references).Error
	if err != nil {
		return nil, fmt.Errorf("list published docs api references: %w", err)
	}
	return references, nil
}

// GetByID returns an active API reference by ID.
func (r *DocsAPIReferenceRepository) GetByID(
	ctx context.Context,
	id string,
) (*model.DocsAPIReference, error) {
	var reference model.DocsAPIReference
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&reference).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get docs api reference: %w", err)
	}
	return &reference, nil
}

// GetPublishedBySlug returns a published API reference by space and slug.
func (r *DocsAPIReferenceRepository) GetPublishedBySlug(
	ctx context.Context,
	workspaceID, spaceID, slug string,
) (*model.DocsAPIReference, error) {
	var reference model.DocsAPIReference
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND space_id = ? AND slug = ? AND deleted_at IS NULL AND published_revision_id IS NOT NULL", workspaceID, spaceID, slug).
		First(&reference).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get published docs api reference: %w", err)
	}
	return &reference, nil
}

// GetRevision returns an immutable revision by ID.
func (r *DocsAPIReferenceRepository) GetRevision(
	ctx context.Context,
	id string,
) (*model.DocsAPIReferenceRevision, error) {
	if id == "" {
		return nil, nil
	}
	var revision model.DocsAPIReferenceRevision
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&revision).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get docs api reference revision: %w", err)
	}
	return &revision, nil
}

// CreateDraft inserts a new draft revision and points the reference to it.
func (r *DocsAPIReferenceRepository) CreateDraft(
	ctx context.Context,
	referenceID string,
	revision *model.DocsAPIReferenceRevision,
	syncedAt time.Time,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		revision.APIReferenceID = referenceID
		if err := tx.Create(revision).Error; err != nil {
			return fmt.Errorf("create docs api reference draft: %w", err)
		}
		updates := map[string]any{
			"draft_revision_id": revision.ID,
			"sync_status":       model.DocsAPIReferenceSyncReady,
			"last_sync_error":   nil,
			"last_synced_at":    syncedAt,
		}
		if err := tx.Model(&model.DocsAPIReference{}).
			Where("id = ? AND deleted_at IS NULL", referenceID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("point docs api reference at draft: %w", err)
		}
		return nil
	})
}

// Update applies metadata changes to a reference.
func (r *DocsAPIReferenceRepository) Update(
	ctx context.Context,
	id string,
	updates map[string]any,
) (*model.DocsAPIReference, error) {
	if len(updates) > 0 {
		if err := r.db.WithContext(ctx).Model(&model.DocsAPIReference{}).
			Where("id = ? AND deleted_at IS NULL", id).
			Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update docs api reference: %w", err)
		}
	}
	return r.GetByID(ctx, id)
}

// Publish promotes the current draft revision to the public revision.
func (r *DocsAPIReferenceRepository) Publish(
	ctx context.Context,
	id, revisionID string,
	publishedAt time.Time,
) (*model.DocsAPIReference, error) {
	updates := map[string]any{
		"published_revision_id": revisionID,
		"published_at":          publishedAt,
	}
	return r.Update(ctx, id, updates)
}

// Unpublish removes the public revision while retaining draft history.
func (r *DocsAPIReferenceRepository) Unpublish(
	ctx context.Context,
	id string,
) (*model.DocsAPIReference, error) {
	return r.Update(ctx, id, map[string]any{
		"published_revision_id": nil,
		"published_at":          nil,
	})
}

// RecordSyncFailure stores a user-safe synchronization error.
func (r *DocsAPIReferenceRepository) RecordSyncFailure(
	ctx context.Context,
	id, message string,
) error {
	return r.db.WithContext(ctx).Model(&model.DocsAPIReference{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"sync_status":     model.DocsAPIReferenceSyncFailed,
			"last_sync_error": message,
		}).Error
}

// NextPosition returns the next display position in a space.
func (r *DocsAPIReferenceRepository) NextPosition(
	ctx context.Context,
	spaceID string,
) (int, error) {
	var maxPosition int
	err := r.db.WithContext(ctx).Model(&model.DocsAPIReference{}).
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Select("COALESCE(MAX(position), -1)").
		Scan(&maxPosition).Error
	if err != nil {
		return 0, fmt.Errorf("get docs api reference position: %w", err)
	}
	return maxPosition + 1, nil
}

// Delete permanently removes an API reference and its immutable revisions.
func (r *DocsAPIReferenceRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("api_reference_id = ?", id).
			Delete(&model.DocsAPIReferenceRevision{}).Error; err != nil {
			return fmt.Errorf("delete docs api reference revisions: %w", err)
		}
		if err := tx.Where("id = ?", id).Delete(&model.DocsAPIReference{}).Error; err != nil {
			return fmt.Errorf("delete docs api reference: %w", err)
		}
		return nil
	})
}

// HardDeleteBySpace removes references and their revision history for a deleted space.
func (r *DocsAPIReferenceRepository) HardDeleteBySpace(ctx context.Context, spaceID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var referenceIDs []string
		if err := tx.Unscoped().Model(&model.DocsAPIReference{}).
			Where("space_id = ?", spaceID).
			Pluck("id", &referenceIDs).Error; err != nil {
			return fmt.Errorf("list docs api references for delete: %w", err)
		}
		if len(referenceIDs) > 0 {
			if err := tx.Where("api_reference_id IN ?", referenceIDs).
				Delete(&model.DocsAPIReferenceRevision{}).Error; err != nil {
				return fmt.Errorf("delete docs api reference revisions: %w", err)
			}
		}
		if err := tx.Unscoped().Where("space_id = ?", spaceID).
			Delete(&model.DocsAPIReference{}).Error; err != nil {
			return fmt.Errorf("delete docs api references: %w", err)
		}
		return nil
	})
}
