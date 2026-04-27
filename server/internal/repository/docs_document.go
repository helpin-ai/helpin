package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsDocumentRepository handles DB operations for documents.
type DocsDocumentRepository struct {
	db         *gorm.DB
	useSortKey bool
}

// NewDocsDocumentRepository creates a new DocsDocumentRepository.
func NewDocsDocumentRepository(db *gorm.DB, useSortKey bool) *DocsDocumentRepository {
	return &DocsDocumentRepository{db: db, useSortKey: useSortKey}
}

// docOrderBy returns the canonical ORDER BY clause for documents
// within a bucket. When the sort_key flag is on, uses sort_key ASC;
// otherwise falls back to the legacy position-based order.
func (r *DocsDocumentRepository) docOrderBy() string {
	if r.useSortKey {
		return "sort_key ASC, id ASC"
	}
	return "position ASC, created_at ASC, id ASC"
}

// LastSortKeyInBucket returns the highest sort_key among docs in the
// given bucket (space + collection), or "" if the bucket is empty.
func (r *DocsDocumentRepository) LastSortKeyInBucket(ctx context.Context, spaceID string, collectionID *string) (string, error) {
	var key string
	q := r.db.WithContext(ctx).
		Model(&model.DocsDocument{}).
		Select("COALESCE(MAX(sort_key), '')").
		Where("space_id = ? AND deleted_at IS NULL", spaceID)
	if collectionID != nil {
		q = q.Where("collection_id = ?", *collectionID)
	} else {
		q = q.Where("collection_id IS NULL")
	}
	if err := q.Row().Scan(&key); err != nil {
		return "", fmt.Errorf("last sort key in doc bucket: %w", err)
	}
	// Ignore the sentinel '~' — it's an un-backfilled row.
	if key == "~" {
		return "", nil
	}
	return key, nil
}

// UpdateSortKey sets the sort_key on a single document.
func (r *DocsDocumentRepository) UpdateSortKey(ctx context.Context, id, key string) error {
	return r.db.WithContext(ctx).
		Model(&model.DocsDocument{}).
		Where("id = ?", id).
		Update("sort_key", key).Error
}

// DB exposes the underlying *gorm.DB for cross-table queries in the
// service layer. Prefer dedicated repo methods where possible.
func (r *DocsDocumentRepository) DB() *gorm.DB { return r.db }

// UpdateFields applies partial field updates to a document by ID.
func (r *DocsDocumentRepository) UpdateFields(ctx context.Context, id string, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.DocsDocument{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// Create inserts a new document.
func (r *DocsDocumentRepository) Create(ctx context.Context, doc *model.DocsDocument) (*model.DocsDocument, error) {
	if err := r.db.WithContext(ctx).Create(doc).Error; err != nil {
		return nil, fmt.Errorf("create docs document: %w", err)
	}
	return doc, nil
}

// GetByID returns a document by ID (excluding soft-deleted).
func (r *DocsDocumentRepository) GetByID(ctx context.Context, id string) (*model.DocsDocument, error) {
	var doc model.DocsDocument
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&doc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs document: %w", err)
	}
	return &doc, nil
}

// GetByIDIncludeDeleted returns a document by ID, including soft-deleted.
func (r *DocsDocumentRepository) GetByIDIncludeDeleted(ctx context.Context, id string) (*model.DocsDocument, error) {
	var doc model.DocsDocument
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&doc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs document (include deleted): %w", err)
	}
	return &doc, nil
}

// ListByIDs returns non-deleted documents by ID for a workspace.
func (r *DocsDocumentRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.DocsDocument, error) {
	if len(ids) == 0 {
		return []model.DocsDocument{}, nil
	}

	var docs []model.DocsDocument
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id IN ? AND deleted_at IS NULL", workspaceID, ids).
		Find(&docs).Error; err != nil {
		return nil, fmt.Errorf("list docs documents by ids: %w", err)
	}
	return docs, nil
}

// List returns documents for a workspace with optional filters.
func (r *DocsDocumentRepository) List(ctx context.Context, workspaceID string, spaceID, collectionID, status, teamID *string, draftViewerID string, includeArchived bool) ([]model.DocsDocument, error) {
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND deleted_at IS NULL", workspaceID)
	if spaceID != nil && *spaceID != "" {
		query = query.Where("space_id = ?", *spaceID)
	}
	if collectionID != nil && *collectionID != "" {
		query = query.Where("collection_id = ?", *collectionID)
	}
	if status != nil && *status != "" {
		query = query.Where("status = ?", *status)
	} else if !includeArchived {
		// By default, exclude archived documents unless explicitly requested.
		query = query.Where("status != ?", model.DocStatusArchived)
	}
	if teamID != nil && *teamID != "" {
		query = query.Where("team_id = ?", *teamID)
	}

	// If draftViewerID is set, hide other users' drafts (admins/owners pass empty to see all).
	if draftViewerID != "" {
		query = query.Where("status != ? OR created_by = ?", model.DocStatusDraft, draftViewerID)
	}

	var docs []model.DocsDocument
	// Space-scoped: use canonical bucket order. Otherwise: recency order.
	if spaceID != nil && *spaceID != "" {
		if r.useSortKey {
			query = query.Order("collection_id ASC NULLS FIRST, sort_key ASC, id ASC")
		} else {
			query = query.Order("collection_id ASC NULLS FIRST, position ASC, created_at ASC")
		}
	} else {
		query = query.Order("is_pinned DESC, updated_at DESC")
	}
	if err := query.Find(&docs).Error; err != nil {
		return nil, fmt.Errorf("list docs documents: %w", err)
	}
	return docs, nil
}

// Update applies partial updates to a document.
func (r *DocsDocumentRepository) Update(ctx context.Context, id string, updates map[string]interface{}) (*model.DocsDocument, error) {
	if err := r.db.WithContext(ctx).Model(&model.DocsDocument{}).Where("id = ? AND deleted_at IS NULL", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update docs document: %w", err)
	}
	return r.GetByID(ctx, id)
}

// UpdateStatus sets the document status.
func (r *DocsDocumentRepository) UpdateStatus(ctx context.Context, id, status string) error {
	updates := map[string]interface{}{"status": status}
	if status == model.DocStatusPublished {
		updates["published_at"] = gorm.Expr("NOW()")
	}
	if err := r.db.WithContext(ctx).Model(&model.DocsDocument{}).Where("id = ? AND deleted_at IS NULL", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("update docs document status: %w", err)
	}
	return nil
}

// Move changes a document's space and/or collection.
func (r *DocsDocumentRepository) Move(ctx context.Context, id, spaceID string, collectionID *string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var doc model.DocsDocument
		if err := tx.Where("id = ? AND deleted_at IS NULL", id).First(&doc).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("get docs document for move: %w", err)
		}

		sameBucket := doc.SpaceID == spaceID &&
			((doc.CollectionID == nil && collectionID == nil) ||
				(doc.CollectionID != nil && collectionID != nil && *doc.CollectionID == *collectionID))
		if sameBucket {
			return nil
		}

		if err := normalizeDocumentBucketTx(tx, spaceID, collectionID); err != nil {
			return err
		}

		var targetCount int64
		targetQuery := tx.Model(&model.DocsDocument{}).
			Where("space_id = ? AND deleted_at IS NULL", spaceID)
		if collectionID != nil {
			targetQuery = targetQuery.Where("collection_id = ?", *collectionID)
		} else {
			targetQuery = targetQuery.Where("collection_id IS NULL")
		}
		if err := targetQuery.Count(&targetCount).Error; err != nil {
			return fmt.Errorf("count target docs bucket: %w", err)
		}

		updates := map[string]interface{}{
			"space_id":      spaceID,
			"collection_id": collectionID,
			"position":      int(targetCount),
		}
		if err := tx.Model(&model.DocsDocument{}).
			Where("id = ? AND deleted_at IS NULL", id).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("move docs document: %w", err)
		}

		return normalizeDocumentBucketTx(tx, doc.SpaceID, doc.CollectionID)
	})
}

// Delete soft-deletes a document.
func (r *DocsDocumentRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Exec("UPDATE docs_documents SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL", id).Error; err != nil {
		return fmt.Errorf("delete docs document: %w", err)
	}
	return nil
}

// HardDelete permanently removes a document row.
func (r *DocsDocumentRepository) HardDelete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.DocsDocument{}).Error; err != nil {
		return fmt.Errorf("hard delete docs document: %w", err)
	}
	return nil
}

// HardDeleteByIDs permanently removes document rows.
func (r *DocsDocumentRepository) HardDeleteByIDs(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&model.DocsDocument{}).Error; err != nil {
		return fmt.Errorf("hard delete docs documents: %w", err)
	}
	return nil
}

// Restore un-deletes a document.
func (r *DocsDocumentRepository) Restore(ctx context.Context, id string) (*model.DocsDocument, error) {
	if err := r.db.WithContext(ctx).Exec("UPDATE docs_documents SET deleted_at = NULL WHERE id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("restore docs document: %w", err)
	}
	return r.GetByID(ctx, id)
}

// GetByShareToken returns a published, non-deleted document by its share token.
func (r *DocsDocumentRepository) GetByShareToken(ctx context.Context, token string) (*model.DocsDocument, error) {
	var doc model.DocsDocument
	if err := r.db.WithContext(ctx).
		Where("share_token = ? AND is_publicly_shared = true AND deleted_at IS NULL", token).
		First(&doc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs document by share token: %w", err)
	}
	return &doc, nil
}

// NextPosition returns the next position for a document in the given bucket.
func (r *DocsDocumentRepository) NextPosition(ctx context.Context, spaceID string, collectionID *string) (int, error) {
	if err := r.NormalizeBucket(ctx, spaceID, collectionID); err != nil {
		return 0, err
	}
	var maxPos *int
	query := r.db.WithContext(ctx).
		Model(&model.DocsDocument{}).
		Where("space_id = ? AND deleted_at IS NULL", spaceID)
	if collectionID != nil {
		query = query.Where("collection_id = ?", *collectionID)
	} else {
		query = query.Where("collection_id IS NULL")
	}
	err := query.Select("COALESCE(MAX(position), -1)").Scan(&maxPos).Error
	if err != nil {
		return 0, fmt.Errorf("next document position: %w", err)
	}
	if maxPos == nil {
		return 0, nil
	}
	return *maxPos + 1, nil
}

// Reorder sets contiguous positions for the given document IDs within a bucket.
func (r *DocsDocumentRepository) Reorder(ctx context.Context, spaceID string, collectionID *string, orderedIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range orderedIDs {
			query := tx.Model(&model.DocsDocument{}).
				Where("id = ? AND space_id = ? AND deleted_at IS NULL", id, spaceID)
			if collectionID != nil {
				query = query.Where("collection_id = ?", *collectionID)
			} else {
				query = query.Where("collection_id IS NULL")
			}
			if err := query.UpdateColumn("position", i).Error; err != nil {
				return fmt.Errorf("reorder document %s: %w", id, err)
			}
		}
		return nil
	})
}

// MoveToCollection moves a document to a new collection (or uncategorized) and appends to the end.
func (r *DocsDocumentRepository) MoveToCollection(ctx context.Context, id, spaceID string, collectionID *string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get the next position in the target bucket.
		var maxPos int
		targetQuery := tx.Model(&model.DocsDocument{}).
			Where("space_id = ? AND deleted_at IS NULL", spaceID)
		if collectionID != nil {
			targetQuery = targetQuery.Where("collection_id = ?", *collectionID)
		} else {
			targetQuery = targetQuery.Where("collection_id IS NULL")
		}
		targetQuery.Select("COALESCE(MAX(position), -1)").Scan(&maxPos)

		// Move the document.
		updates := map[string]interface{}{
			"collection_id": collectionID,
			"position":      maxPos + 1,
		}
		if err := tx.Model(&model.DocsDocument{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return fmt.Errorf("move document %s: %w", id, err)
		}

		// Normalize the source bucket (close the gap).
		// This is done by re-numbering all docs in the old bucket, but since we don't
		// know the old bucket here, normalization should be called by the service layer.
		return nil
	})
}

// NormalizeBucket re-numbers positions in a bucket to be contiguous starting from 0.
func (r *DocsDocumentRepository) NormalizeBucket(ctx context.Context, spaceID string, collectionID *string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return normalizeDocumentBucketTx(tx, spaceID, collectionID)
	})
}

func normalizeDocumentBucketTx(tx *gorm.DB, spaceID string, collectionID *string) error {
	query := tx.Where("space_id = ? AND deleted_at IS NULL", spaceID)
	if collectionID != nil {
		query = query.Where("collection_id = ?", *collectionID)
	} else {
		query = query.Where("collection_id IS NULL")
	}

	var docs []model.DocsDocument
	if err := query.Order("position ASC, created_at ASC, id ASC").Find(&docs).Error; err != nil {
		return fmt.Errorf("list docs bucket for normalization: %w", err)
	}

	for i, doc := range docs {
		if doc.Position == i {
			continue
		}
		if err := tx.Model(&model.DocsDocument{}).
			Where("id = ? AND deleted_at IS NULL", doc.ID).
			UpdateColumn("position", i).Error; err != nil {
			return fmt.Errorf("normalize doc %s: %w", doc.ID, err)
		}
	}

	return nil
}
