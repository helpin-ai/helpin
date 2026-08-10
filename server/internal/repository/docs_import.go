package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsImportRepository handles DB operations for docs import jobs.
type DocsImportRepository struct {
	db *gorm.DB
}

// NewDocsImportRepository creates a new DocsImportRepository.
func NewDocsImportRepository(db *gorm.DB) *DocsImportRepository {
	return &DocsImportRepository{db: db}
}

// ListByWorkspace returns all import jobs for a workspace, newest first.
func (r *DocsImportRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.DocsImportJob, error) {
	var jobs []model.DocsImportJob
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(50).
		Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("list docs import jobs: %w", err)
	}
	return jobs, nil
}

// Create inserts a new import job.
func (r *DocsImportRepository) Create(ctx context.Context, job *model.DocsImportJob) error {
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return fmt.Errorf("create docs import job: %w", err)
	}
	return nil
}

// GetByID returns an import job by ID, or nil if not found.
func (r *DocsImportRepository) GetByID(ctx context.Context, id string) (*model.DocsImportJob, error) {
	var job model.DocsImportJob
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs import job: %w", err)
	}
	return &job, nil
}

// UpdateProgress updates the completed, failed, and failures fields of an import job.
func (r *DocsImportRepository) UpdateProgress(ctx context.Context, id string, completed, failed int, failures json.RawMessage) error {
	updates := map[string]interface{}{
		"completed":  completed,
		"failed":     failed,
		"failures":   failures,
		"updated_at": time.Now(),
	}
	if err := r.db.WithContext(ctx).Model(&model.DocsImportJob{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("update docs import job progress: %w", err)
	}
	return nil
}

// UpdateStatus updates the status and optional completed_at of an import job.
func (r *DocsImportRepository) UpdateStatus(ctx context.Context, id, status string, completedAt *time.Time) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if completedAt != nil {
		updates["completed_at"] = completedAt
	}
	if err := r.db.WithContext(ctx).Model(&model.DocsImportJob{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("update docs import job status: %w", err)
	}
	return nil
}

// SetTotal updates the total article count of an import job.
func (r *DocsImportRepository) SetTotal(ctx context.Context, id string, total int) error {
	updates := map[string]interface{}{
		"total":      total,
		"updated_at": time.Now(),
	}
	if err := r.db.WithContext(ctx).Model(&model.DocsImportJob{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("set docs import job total: %w", err)
	}
	return nil
}

// SetRedirectMap updates the redirect_map field of an import job.
func (r *DocsImportRepository) SetRedirectMap(ctx context.Context, id string, redirectMap json.RawMessage) error {
	updates := map[string]interface{}{
		"redirect_map": redirectMap,
		"updated_at":   time.Now(),
	}
	if err := r.db.WithContext(ctx).Model(&model.DocsImportJob{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("set docs import job redirect map: %w", err)
	}
	return nil
}

// SetSummary stores the import summary JSON on the job.
func (r *DocsImportRepository) SetSummary(ctx context.Context, id string, summary json.RawMessage) error {
	updates := map[string]interface{}{
		"summary":    summary,
		"updated_at": time.Now(),
	}
	if err := r.db.WithContext(ctx).Model(&model.DocsImportJob{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("set docs import job summary: %w", err)
	}
	return nil
}

// SetError updates the error field of an import job.
func (r *DocsImportRepository) SetError(ctx context.Context, id, errMsg string) error {
	updates := map[string]interface{}{
		"error":      errMsg,
		"updated_at": time.Now(),
	}
	if err := r.db.WithContext(ctx).Model(&model.DocsImportJob{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("set docs import job error: %w", err)
	}
	return nil
}

// PrepareExecution stores the encrypted request and resets a job for workflow execution.
func (r *DocsImportRepository) PrepareExecution(ctx context.Context, id, encryptedPayload, workflowID string) error {
	updates := map[string]interface{}{
		"payload_encrypted": encryptedPayload,
		"workflow_id":       workflowID,
		"status":            model.DocsImportStatusPending,
		"error":             nil,
		"completed_at":      nil,
		"updated_at":        time.Now(),
	}
	if err := r.db.WithContext(ctx).Model(&model.DocsImportJob{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("prepare docs import execution: %w", err)
	}
	return nil
}

// MarkInterrupted marks an active import as intentionally canceled.
func (r *DocsImportRepository) MarkInterrupted(ctx context.Context, id, message string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":       model.DocsImportStatusInterrupted,
		"error":        message,
		"completed_at": &now,
		"updated_at":   now,
	}
	if err := r.db.WithContext(ctx).Model(&model.DocsImportJob{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("interrupt docs import job: %w", err)
	}
	return nil
}
