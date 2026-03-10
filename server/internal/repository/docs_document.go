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
	db *gorm.DB
}

// NewDocsDocumentRepository creates a new DocsDocumentRepository.
func NewDocsDocumentRepository(db *gorm.DB) *DocsDocumentRepository {
	return &DocsDocumentRepository{db: db}
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
func (r *DocsDocumentRepository) List(ctx context.Context, workspaceID string, spaceID, collectionID, docType, status, teamID *string, draftViewerID string) ([]model.DocsDocument, error) {
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND deleted_at IS NULL", workspaceID)
	if spaceID != nil && *spaceID != "" {
		query = query.Where("space_id = ?", *spaceID)
	}
	if collectionID != nil && *collectionID != "" {
		query = query.Where("collection_id = ?", *collectionID)
	}
	if docType != nil && *docType != "" {
		query = query.Where("doc_type = ?", *docType)
	}
	if status != nil && *status != "" {
		query = query.Where("status = ?", *status)
	} else {
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
	if err := query.Order("is_pinned DESC, updated_at DESC").Find(&docs).Error; err != nil {
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
	updates := map[string]interface{}{
		"space_id":      spaceID,
		"collection_id": collectionID,
	}
	if err := r.db.WithContext(ctx).Model(&model.DocsDocument{}).Where("id = ? AND deleted_at IS NULL", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("move docs document: %w", err)
	}
	return nil
}

// Delete soft-deletes a document.
func (r *DocsDocumentRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Exec("UPDATE docs_documents SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL", id).Error; err != nil {
		return fmt.Errorf("delete docs document: %w", err)
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
