package repository

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ClaimJob leases one committed routing change for bounded reconciliation.
func (r *SupportMergeRepository) ClaimJob(ctx context.Context) (*model.SupportMergeJob, error) {
	var job model.SupportMergeJob
	now := time.Now().UTC()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("completed_at IS NULL AND available_at <= ? AND (lease_until IS NULL OR lease_until < ?)", now, now).Order("available_at,id").First(&job).Error; err != nil {
			return err
		}
		until := now.Add(time.Minute)
		job.LeaseUntil = &until
		return tx.Model(&job).Update("lease_until", until).Error
	})
	return &job, err
}

// FinishJob completes a successful reconciliation or schedules a retry.
func (r *SupportMergeRepository) FinishJob(ctx context.Context, job *model.SupportMergeJob, success bool) error {
	fields := map[string]any{"lease_until": nil, "available_at": time.Now().UTC().Add(time.Minute)}
	if success {
		fields["completed_at"] = time.Now().UTC()
	}
	return r.db.WithContext(ctx).Model(&model.SupportMergeJob{}).Where("id = ? AND lease_until = ?", job.ID, job.LeaseUntil).Updates(fields).Error
}
