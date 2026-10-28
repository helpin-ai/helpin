package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SupportInboundJobRepository struct{ db *gorm.DB }

func NewSupportInboundJobRepository(db *gorm.DB) *SupportInboundJobRepository {
	return &SupportInboundJobRepository{db}
}
func (r *SupportInboundJobRepository) Create(ctx context.Context, job *model.SupportInboundJob) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(job).Error
}

// Claim uses a conditional update so concurrent workers cannot own the same lease.
func (r *SupportInboundJobRepository) Claim(ctx context.Context, kind string, now time.Time) (*model.SupportInboundJob, error) {
	var job model.SupportInboundJob
	err := r.db.WithContext(ctx).Where("kind = ? AND status IN ? AND available_at <= ?", kind, []string{"pending", "processing"}, now).Order("available_at ASC").Limit(1).Find(&job).Error
	if err == nil && job.ID == "" {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	token := uuid.NewString()
	result := r.db.WithContext(ctx).Model(&model.SupportInboundJob{}).Where("id = ? AND status = ? AND lease_token = ? AND available_at <= ?", job.ID, job.Status, job.LeaseToken, now).Updates(map[string]any{"status": "processing", "lease_token": token, "available_at": now.Add(2 * time.Minute), "attempts": gorm.Expr("attempts + 1")})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	job.Status = "processing"
	job.LeaseToken = token
	job.Attempts++
	return &job, nil
}
func (r *SupportInboundJobRepository) Finish(ctx context.Context, job *model.SupportInboundJob, processingErr error, now time.Time) error {
	values := map[string]any{"status": "completed", "payload": "", "last_error": "", "lease_token": ""}
	if processingErr != nil {
		status := "pending"
		if job.Attempts >= 10 || job.RecoveryRequestedAt != nil {
			// A manual retry exposes its outcome immediately; accepted bytes stay
			// available for another attempt instead of waiting through backoff.
			status = "failed"
		}
		delay := time.Minute * time.Duration(1<<min(job.Attempts-1, 8))
		values = map[string]any{"status": status, "last_error": "Processing failed; inspect server logs using the job ID.", "lease_token": "", "available_at": now.Add(delay)}
	}
	return r.db.WithContext(ctx).Model(&model.SupportInboundJob{}).Where("id = ? AND lease_token = ? AND status = ?", job.ID, job.LeaseToken, "processing").Updates(values).Error
}

// SupportInboundWaitingSummary counts unfinished jobs of one kind.
type SupportInboundWaitingSummary struct {
	Waiting int64
	// Retrying counts waiting jobs that already failed at least once.
	Retrying int64
	// OldestSince is when the longest-waiting job began waiting; nil when none wait.
	OldestSince *time.Time
}

// WaitingSummary reports unfinished jobs of a kind. A recovery request starts a
// new wait, so a recovered job counts from the request, not the original receipt.
func (r *SupportInboundJobRepository) WaitingSummary(ctx context.Context, kind string) (SupportInboundWaitingSummary, error) {
	var summary SupportInboundWaitingSummary
	waiting := func() *gorm.DB {
		return r.db.WithContext(ctx).Model(&model.SupportInboundJob{}).Where("kind = ? AND status IN ?", kind, []string{"pending", "processing"})
	}
	if err := waiting().Count(&summary.Waiting).Error; err != nil {
		return summary, fmt.Errorf("count waiting inbound jobs: %w", err)
	}
	if summary.Waiting == 0 {
		return summary, nil
	}
	if err := waiting().Where("attempts > 0").Count(&summary.Retrying).Error; err != nil {
		return summary, fmt.Errorf("count retrying inbound jobs: %w", err)
	}
	var oldest model.SupportInboundJob
	if err := waiting().Select("created_at", "recovery_requested_at").Order("COALESCE(recovery_requested_at, created_at) ASC").Limit(1).Find(&oldest).Error; err != nil {
		return summary, fmt.Errorf("find oldest waiting inbound job: %w", err)
	}
	since := oldest.CreatedAt
	if oldest.RecoveryRequestedAt != nil && oldest.RecoveryRequestedAt.After(since) {
		since = *oldest.RecoveryRequestedAt
	}
	summary.OldestSince = &since
	return summary, nil
}
