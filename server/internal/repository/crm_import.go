package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMImportRepository handles DB operations for CRM import jobs.
type CRMImportRepository struct {
	db *gorm.DB
}

// NewCRMImportRepository creates a new CRMImportRepository.
func NewCRMImportRepository(db *gorm.DB) *CRMImportRepository {
	return &CRMImportRepository{db: db}
}

// Create inserts an import job.
func (r *CRMImportRepository) Create(ctx context.Context, job *model.CRMImportJob) error {
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return fmt.Errorf("create import job: %w", err)
	}
	return nil
}

// GetByID returns an import job by ID.
func (r *CRMImportRepository) GetByID(ctx context.Context, id string) (*model.CRMImportJob, error) {
	var job model.CRMImportJob
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get import job: %w", err)
	}
	return &job, nil
}

// List returns import jobs for a workspace.
func (r *CRMImportRepository) List(ctx context.Context, workspaceID string, pagination model.PMPagination) ([]model.CRMImportJob, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMImportJob{}).Where("workspace_id = ?", workspaceID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count import jobs: %w", err)
	}

	var jobs []model.CRMImportJob
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&jobs).Error; err != nil {
		return nil, 0, fmt.Errorf("list import jobs: %w", err)
	}
	return jobs, total, nil
}

// UpdateStatus sets the status of an import job.
func (r *CRMImportRepository) UpdateStatus(ctx context.Context, id, status string) error {
	if err := r.db.WithContext(ctx).Model(&model.CRMImportJob{}).Where("id = ?", id).Update("status", status).Error; err != nil {
		return fmt.Errorf("update import status: %w", err)
	}
	return nil
}

// UpdateProgress updates the progress counters of an import job.
func (r *CRMImportRepository) UpdateProgress(ctx context.Context, job *model.CRMImportJob) error {
	if err := r.db.WithContext(ctx).Model(&model.CRMImportJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
		"status":         job.Status,
		"processed_rows": job.ProcessedRows,
		"created_rows":   job.CreatedRows,
		"updated_rows":   job.UpdatedRows,
		"error_count":    job.ErrorCount,
		"error_log":      job.ErrorLog,
	}).Error; err != nil {
		return fmt.Errorf("update import progress: %w", err)
	}
	return nil
}

// GetDB exposes the database for batch operations during import processing.
func (r *CRMImportRepository) GetDB() *gorm.DB {
	return r.db
}
