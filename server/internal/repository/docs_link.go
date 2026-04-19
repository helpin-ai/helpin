package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsLinkRepository handles DB operations for document links.
type DocsLinkRepository struct {
	db *gorm.DB
}

// NewDocsLinkRepository creates a new DocsLinkRepository.
func NewDocsLinkRepository(db *gorm.DB) *DocsLinkRepository {
	return &DocsLinkRepository{db: db}
}

// Create inserts a new link.
func (r *DocsLinkRepository) Create(ctx context.Context, link *model.DocsLink) (*model.DocsLink, error) {
	if err := r.db.WithContext(ctx).Create(link).Error; err != nil {
		return nil, fmt.Errorf("create docs link: %w", err)
	}
	return link, nil
}

// GetByID returns a link by ID.
func (r *DocsLinkRepository) GetByID(ctx context.Context, id string) (*model.DocsLink, error) {
	var link model.DocsLink
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs link: %w", err)
	}
	return &link, nil
}

// ListByDocument returns all links for a document.
func (r *DocsLinkRepository) ListByDocument(ctx context.Context, documentID string) ([]model.DocsLink, error) {
	var links []model.DocsLink
	if err := r.db.WithContext(ctx).
		Where("document_id = ?", documentID).
		Order("created_at DESC").
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list docs links by document: %w", err)
	}
	return links, nil
}

// ListByObject returns all links for a PM/Support object (reverse lookup).
func (r *DocsLinkRepository) ListByObject(ctx context.Context, workspaceID, objectType, objectID string) ([]model.DocsLink, error) {
	var links []model.DocsLink
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND linked_object_type = ? AND linked_object_id = ?", workspaceID, objectType, objectID).
		Order("created_at DESC").
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list docs links by object: %w", err)
	}
	return links, nil
}

// Delete removes a link by ID.
func (r *DocsLinkRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.DocsLink{}).Error; err != nil {
		return fmt.Errorf("delete docs link: %w", err)
	}
	return nil
}

// DeleteByDocumentIDs hard-deletes links for the provided documents.
func (r *DocsLinkRepository) DeleteByDocumentIDs(ctx context.Context, documentIDs []string) error {
	if len(documentIDs) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Where("document_id IN ?", documentIDs).Delete(&model.DocsLink{}).Error; err != nil {
		return fmt.Errorf("delete docs links by documents: %w", err)
	}
	return nil
}
