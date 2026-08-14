package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMEnrichmentRepository handles DB operations for CRM enrichments.
type CRMEnrichmentRepository struct {
	db *gorm.DB
}

// NewCRMEnrichmentRepository creates a new CRMEnrichmentRepository.
func NewCRMEnrichmentRepository(db *gorm.DB) *CRMEnrichmentRepository {
	return &CRMEnrichmentRepository{db: db}
}

// DB returns the underlying database for coordinated transactions.
func (r *CRMEnrichmentRepository) DB() *gorm.DB { return r.db }

// WithTx returns a repository backed by tx.
func (r *CRMEnrichmentRepository) WithTx(tx *gorm.DB) *CRMEnrichmentRepository {
	return &CRMEnrichmentRepository{db: tx}
}

// GetByID returns an enrichment result by ID.
func (r *CRMEnrichmentRepository) GetByID(ctx context.Context, id string) (*model.CRMEnrichmentResult, error) {
	var result model.CRMEnrichmentResult
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get enrichment: %w", err)
	}
	return &result, nil
}

// Update persists an enrichment result.
func (r *CRMEnrichmentRepository) Update(ctx context.Context, enrichment *model.CRMEnrichmentResult) error {
	if err := r.db.WithContext(ctx).Save(enrichment).Error; err != nil {
		return fmt.Errorf("update enrichment: %w", err)
	}
	return nil
}

// Create inserts an enrichment result.
func (r *CRMEnrichmentRepository) Create(ctx context.Context, enrichment *model.CRMEnrichmentResult) error {
	if err := r.db.WithContext(ctx).Create(enrichment).Error; err != nil {
		return fmt.Errorf("create enrichment: %w", err)
	}
	return nil
}

// List returns enrichment results with optional filters.
func (r *CRMEnrichmentRepository) List(ctx context.Context, workspaceID string, filters model.CRMEnrichmentListFilters, pagination model.PMPagination) ([]model.CRMEnrichmentResult, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMEnrichmentResult{}).Where("workspace_id = ?", workspaceID)

	if filters.ObjectType != nil && *filters.ObjectType != "" {
		query = query.Where("object_type = ?", *filters.ObjectType)
	}
	if filters.ObjectID != nil && *filters.ObjectID != "" {
		query = query.Where("object_id = ?", *filters.ObjectID)
	}
	if filters.Source != nil && *filters.Source != "" {
		query = query.Where("source = ?", *filters.Source)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count enrichments: %w", err)
	}

	var results []model.CRMEnrichmentResult
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&results).Error; err != nil {
		return nil, 0, fmt.Errorf("list enrichments: %w", err)
	}
	return results, total, nil
}

// Delete removes an enrichment result.
func (r *CRMEnrichmentRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMEnrichmentResult{}).Error; err != nil {
		return fmt.Errorf("delete enrichment: %w", err)
	}
	return nil
}
