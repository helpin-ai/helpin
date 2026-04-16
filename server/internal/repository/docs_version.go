package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsVersionRepository handles DB operations for document versions.
type DocsVersionRepository struct {
	db *gorm.DB
}

// NewDocsVersionRepository creates a new DocsVersionRepository.
func NewDocsVersionRepository(db *gorm.DB) *DocsVersionRepository {
	return &DocsVersionRepository{db: db}
}

// Create inserts a new version snapshot.
func (r *DocsVersionRepository) Create(ctx context.Context, documentID, createdBy string, content json.RawMessage, contentText string, snapshotLabel *string, versionType string, wordCount int) (*model.DocsVersion, error) {
	v := &model.DocsVersion{
		DocumentID:    documentID,
		Content:       content,
		ContentText:   contentText,
		SnapshotLabel: snapshotLabel,
		VersionType:   versionType,
		WordCount:     wordCount,
		CreatedBy:     createdBy,
	}
	if err := r.db.WithContext(ctx).Create(v).Error; err != nil {
		return nil, fmt.Errorf("create docs version: %w", err)
	}
	return v, nil
}

// GetByID returns a version by ID.
func (r *DocsVersionRepository) GetByID(ctx context.Context, id string) (*model.DocsVersion, error) {
	var v model.DocsVersion
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&v).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs version: %w", err)
	}
	return &v, nil
}

// ListByDocument returns all versions for a document, newest first.
func (r *DocsVersionRepository) ListByDocument(ctx context.Context, documentID string) ([]model.DocsVersion, error) {
	var versions []model.DocsVersion
	if err := r.db.WithContext(ctx).
		Where("document_id = ?", documentID).
		Order("created_at DESC").
		Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("list docs versions: %w", err)
	}
	return versions, nil
}

// ListByDocumentIDs returns all versions for the provided documents.
func (r *DocsVersionRepository) ListByDocumentIDs(ctx context.Context, documentIDs []string) ([]model.DocsVersion, error) {
	if len(documentIDs) == 0 {
		return []model.DocsVersion{}, nil
	}
	var versions []model.DocsVersion
	if err := r.db.WithContext(ctx).
		Where("document_id IN ?", documentIDs).
		Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("list docs versions by documents: %w", err)
	}
	return versions, nil
}

// ListByWorkspaceExcludingDocuments returns versions for documents that remain in a workspace.
func (r *DocsVersionRepository) ListByWorkspaceExcludingDocuments(ctx context.Context, workspaceID string, excludeDocumentIDs []string) ([]model.DocsVersion, error) {
	var versions []model.DocsVersion
	query := r.db.WithContext(ctx).
		Model(&model.DocsVersion{}).
		Select("docs_versions.*").
		Joins("JOIN docs_documents dd ON dd.id = docs_versions.document_id").
		Where("dd.workspace_id = ? AND dd.deleted_at IS NULL", workspaceID)
	if len(excludeDocumentIDs) > 0 {
		query = query.Where("docs_versions.document_id NOT IN ?", excludeDocumentIDs)
	}
	if err := query.Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("list surviving docs versions: %w", err)
	}
	return versions, nil
}

// DeleteByDocumentIDs hard-deletes versions for the provided documents.
func (r *DocsVersionRepository) DeleteByDocumentIDs(ctx context.Context, documentIDs []string) error {
	if len(documentIDs) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Where("document_id IN ?", documentIDs).Delete(&model.DocsVersion{}).Error; err != nil {
		return fmt.Errorf("delete docs versions by documents: %w", err)
	}
	return nil
}

// GetLatestByDocument returns the most recent version for a document.
func (r *DocsVersionRepository) GetLatestByDocument(ctx context.Context, documentID string) (*model.DocsVersion, error) {
	var v model.DocsVersion
	if err := r.db.WithContext(ctx).
		Where("document_id = ?", documentID).
		Order("created_at DESC").
		First(&v).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest docs version: %w", err)
	}
	return &v, nil
}

// UpdateLabel updates the snapshot_label of a version.
func (r *DocsVersionRepository) UpdateLabel(ctx context.Context, id string, label *string) (*model.DocsVersion, error) {
	if err := r.db.WithContext(ctx).
		Model(&model.DocsVersion{}).
		Where("id = ?", id).
		Update("snapshot_label", label).Error; err != nil {
		return nil, fmt.Errorf("update docs version label: %w", err)
	}
	return r.GetByID(ctx, id)
}

// wordCount counts words in a plain text string.
func WordCount(text string) int {
	fields := strings.Fields(text)
	return len(fields)
}
