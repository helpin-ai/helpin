package repository

import (
	"context"
	"errors"
	"fmt"

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
		// Uncategorize documents in this collection
		if err := tx.Exec("UPDATE docs_documents SET collection_id = NULL WHERE collection_id = ? AND deleted_at IS NULL", id).Error; err != nil {
			return fmt.Errorf("uncategorize docs in collection: %w", err)
		}
		// Soft-delete the collection
		if err := tx.Exec("UPDATE docs_collections SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL", id).Error; err != nil {
			return fmt.Errorf("delete docs collection: %w", err)
		}
		return nil
	})
}

// Restore un-deletes a collection.
func (r *DocsCollectionRepository) Restore(ctx context.Context, id string) (*model.DocsCollection, error) {
	if err := r.db.WithContext(ctx).Exec("UPDATE docs_collections SET deleted_at = NULL WHERE id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("restore docs collection: %w", err)
	}
	return r.GetByID(ctx, id)
}
