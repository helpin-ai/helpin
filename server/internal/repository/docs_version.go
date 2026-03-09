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
