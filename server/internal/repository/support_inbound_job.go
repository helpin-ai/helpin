package repository

import (
	"context"
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
		if job.Attempts >= 10 {
			status = "failed"
		}
		delay := time.Minute * time.Duration(1<<min(job.Attempts-1, 8))
		values = map[string]any{"status": status, "last_error": "Processing failed; inspect server logs using the job ID.", "lease_token": "", "available_at": now.Add(delay)}
	}
	return r.db.WithContext(ctx).Model(&model.SupportInboundJob{}).Where("id = ? AND lease_token = ? AND status = ?", job.ID, job.LeaseToken, "processing").Updates(values).Error
}
