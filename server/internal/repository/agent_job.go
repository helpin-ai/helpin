package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AgentJobRepository handles DB operations for the agent job queue.
type AgentJobRepository struct {
	db *gorm.DB
}

// NewAgentJobRepository creates a new AgentJobRepository.
func NewAgentJobRepository(db *gorm.DB) *AgentJobRepository {
	return &AgentJobRepository{db: db}
}

// Create creates a new job.
func (r *AgentJobRepository) Create(ctx context.Context, job *model.AgentJob) error {
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return fmt.Errorf("create agent job: %w", err)
	}
	return nil
}

// ClaimJob atomically claims the next available job using FOR UPDATE SKIP LOCKED.
// Returns nil if no job is available.
func (r *AgentJobRepository) ClaimJob(ctx context.Context, workerID string) (*model.AgentJob, error) {
	var job model.AgentJob
	now := time.Now()

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND available_at <= ? AND attempt < max_attempts", "pending", now).
			Order("available_at ASC").
			First(&job).Error; err != nil {
			return err
		}

		job.Status = "claimed"
		job.LockedBy = &workerID
		job.LockedAt = &now
		job.Attempt++

		return tx.Save(&job).Error
	})

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("claim agent job: %w", err)
	}

	return &job, nil
}

// CompleteJob marks a job as completed.
func (r *AgentJobRepository) CompleteJob(ctx context.Context, jobID string) error {
	if err := r.db.WithContext(ctx).Model(&model.AgentJob{}).Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":    "completed",
			"locked_by": nil,
		}).Error; err != nil {
		return fmt.Errorf("complete agent job: %w", err)
	}
	return nil
}

// FailJob marks a job as failed with exponential backoff for retry.
func (r *AgentJobRepository) FailJob(ctx context.Context, jobID string, errMsg string) error {
	var job model.AgentJob
	if err := r.db.WithContext(ctx).Where("id = ?", jobID).First(&job).Error; err != nil {
		return fmt.Errorf("fail agent job: %w", err)
	}

	if job.Attempt >= job.MaxAttempts {
		job.Status = "failed"
	} else {
		job.Status = "pending"
		backoff := time.Duration(1<<uint(job.Attempt)) * time.Minute
		retryAt := time.Now().Add(backoff)
		job.AvailableAt = retryAt
	}
	job.LastError = &errMsg
	job.LockedBy = nil
	job.LockedAt = nil

	if err := r.db.WithContext(ctx).Save(&job).Error; err != nil {
		return fmt.Errorf("fail agent job: %w", err)
	}
	return nil
}

// GetByRunID returns the job for a given run.
func (r *AgentJobRepository) GetByRunID(ctx context.Context, runID string) (*model.AgentJob, error) {
	var job model.AgentJob
	if err := r.db.WithContext(ctx).Where("run_id = ?", runID).First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent job by run: %w", err)
	}
	return &job, nil
}

// CompleteByRunID marks the job for a run as completed if it exists.
func (r *AgentJobRepository) CompleteByRunID(ctx context.Context, runID string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.AgentJob{}).
		Where("run_id = ? AND status IN ?", runID, []string{"pending", "claimed"}).
		Updates(map[string]interface{}{
			"status":    "completed",
			"locked_by": nil,
			"locked_at": nil,
		}).Error; err != nil {
		return fmt.Errorf("complete agent job by run: %w", err)
	}
	return nil
}
