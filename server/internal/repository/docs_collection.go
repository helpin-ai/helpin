package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsCollectionRepository handles DB operations for docs collections.
type DocsCollectionRepository struct {
	db *gorm.DB
}

// NewDocsCollectionRepository creates a new DocsCollectionRepository.
func NewDocsCollectionRepository(db *gorm.DB) *DocsCollectionRepository {
	return &DocsCollectionRepository{db: db}
}

// Create inserts a new collection.
func (r *DocsCollectionRepository) Create(ctx context.Context, coll *model.DocsCollection) (*model.DocsCollection, error) {
	if err := r.db.WithContext(ctx).Create(coll).Error; err != nil {
		return nil, fmt.Errorf("create docs collection: %w", err)
	}
	return coll, nil
}

// GetByID returns a collection by ID.
func (r *DocsCollectionRepository) GetByID(ctx context.Context, id string) (*model.DocsCollection, error) {
	var coll model.DocsCollection
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&coll).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs collection: %w", err)
	}
	return &coll, nil
}

// ListBySpace returns all collections in a space, ordered by position.
func (r *DocsCollectionRepository) ListBySpace(ctx context.Context, spaceID string) ([]model.DocsCollection, error) {
	var colls []model.DocsCollection
	if err := r.db.WithContext(ctx).
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Order("position ASC, created_at ASC").
		Find(&colls).Error; err != nil {
		return nil, fmt.Errorf("list docs collections: %w", err)
	}
	return colls, nil
}

// Update applies partial updates to a collection.
func (r *DocsCollectionRepository) Update(ctx context.Context, id string, updates map[string]interface{}) (*model.DocsCollection, error) {
	if err := r.db.WithContext(ctx).Model(&model.DocsCollection{}).Where("id = ? AND deleted_at IS NULL", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update docs collection: %w", err)
	}
	return r.GetByID(ctx, id)
}

// Delete soft-deletes a collection and uncategorizes its documents.
func (r *DocsCollectionRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var coll model.DocsCollection
		if err := tx.Where("id = ? AND deleted_at IS NULL", id).First(&coll).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("get docs collection for delete: %w", err)
		}

		if err := normalizeDocumentBucketTx(tx, coll.SpaceID, nil); err != nil {
			return err
		}

		var docs []model.DocsDocument
		if err := tx.
			Where("collection_id = ? AND deleted_at IS NULL", id).
			Order("position ASC, created_at ASC, id ASC").
			Find(&docs).Error; err != nil {
			return fmt.Errorf("list docs in collection for delete: %w", err)
		}

		var existingUncategorizedCount int64
		if err := tx.Model(&model.DocsDocument{}).
			Where("space_id = ? AND collection_id IS NULL AND deleted_at IS NULL", coll.SpaceID).
			Count(&existingUncategorizedCount).Error; err != nil {
			return fmt.Errorf("count uncategorized docs: %w", err)
		}

		for i, doc := range docs {
			updates := map[string]interface{}{
				"collection_id": nil,
				"position":      int(existingUncategorizedCount) + i,
			}
			if err := tx.Model(&model.DocsDocument{}).
				Where("id = ? AND deleted_at IS NULL", doc.ID).
				Updates(updates).Error; err != nil {
				return fmt.Errorf("move doc %s to uncategorized: %w", doc.ID, err)
			}
		}

		if err := tx.Model(&model.DocsCollection{}).
			Where("id = ? AND deleted_at IS NULL", id).
			Update("deleted_at", time.Now().UTC()).Error; err != nil {
			return fmt.Errorf("delete docs collection: %w", err)
		}

		if err := normalizeCollectionPositionsTx(tx, coll.SpaceID); err != nil {
			return err
		}

		return normalizeDocumentBucketTx(tx, coll.SpaceID, nil)
	})
}

// Restore un-deletes a collection.
func (r *DocsCollectionRepository) Restore(ctx context.Context, id string) (*model.DocsCollection, error) {
	if err := r.db.WithContext(ctx).Exec("UPDATE docs_collections SET deleted_at = NULL WHERE id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("restore docs collection: %w", err)
	}
	return r.GetByID(ctx, id)
}

// NextPosition returns the next position for a collection in the given space.
func (r *DocsCollectionRepository) NextPosition(ctx context.Context, spaceID string) (int, error) {
	if err := r.NormalizeSpace(ctx, spaceID); err != nil {
		return 0, err
	}
	var maxPos *int
	err := r.db.WithContext(ctx).
		Model(&model.DocsCollection{}).
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Select("COALESCE(MAX(position), -1)").
		Scan(&maxPos).Error
	if err != nil {
		return 0, fmt.Errorf("next collection position: %w", err)
	}
	if maxPos == nil {
		return 0, nil
	}
	return *maxPos + 1, nil
}

// Reorder sets contiguous positions for the given collection IDs within a space.
func (r *DocsCollectionRepository) Reorder(ctx context.Context, spaceID string, orderedIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range orderedIDs {
			if err := tx.Model(&model.DocsCollection{}).
				Where("id = ? AND space_id = ? AND deleted_at IS NULL", id, spaceID).
				UpdateColumn("position", i).Error; err != nil {
				return fmt.Errorf("reorder collection %s: %w", id, err)
			}
		}
		return nil
	})
}

// NormalizeSpace re-numbers collection positions in a space to be contiguous starting from 0.
func (r *DocsCollectionRepository) NormalizeSpace(ctx context.Context, spaceID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return normalizeCollectionPositionsTx(tx, spaceID)
	})
}

func normalizeCollectionPositionsTx(tx *gorm.DB, spaceID string) error {
	var colls []model.DocsCollection
	if err := tx.
		Where("space_id = ? AND deleted_at IS NULL", spaceID).
		Order("position ASC, created_at ASC, id ASC").
		Find(&colls).Error; err != nil {
		return fmt.Errorf("list collections for normalization: %w", err)
	}

	for i, coll := range colls {
		if coll.Position == i {
			continue
		}
		if err := tx.Model(&model.DocsCollection{}).
			Where("id = ? AND deleted_at IS NULL", coll.ID).
			UpdateColumn("position", i).Error; err != nil {
			return fmt.Errorf("normalize collection %s: %w", coll.ID, err)
		}
	}

	return nil
}
