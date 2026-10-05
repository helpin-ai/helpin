package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrSupportTagLeaseLost rejects expired or replaced workers before any mutation.
var ErrSupportTagLeaseLost = errors.New("support tagging lease is no longer active")

// SupportTagJobRepository leases durable work created by the message insert trigger.
type SupportTagJobRepository struct{ db *gorm.DB }

// NewSupportTagJobRepository binds the automatic tagging queue.
func NewSupportTagJobRepository(db *gorm.DB) *SupportTagJobRepository {
	return &SupportTagJobRepository{db: db}
}

// ClaimNext leases one due job. Expired attempts count against the retry bound.
func (r *SupportTagJobRepository) ClaimNext(ctx context.Context, now time.Time) (*model.SupportTagJob, error) {
	var claimed *model.SupportTagJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job model.SupportTagJob
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("(status = 'pending' AND available_at <= ?) OR (status = 'processing' AND lease_until <= ?)", now, now).
			Order("available_at, message_id").Take(&job).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		claimed = &job
		if job.Attempts >= 5 {
			job.Status = "failed"
			return tx.Model(&job).Updates(map[string]any{"status": "failed", "result_code": "attempts_exhausted", "lease_token": "", "lease_until": nil, "updated_at": now}).Error
		}
		token, until := uuid.NewString(), now.Add(2*time.Minute)
		job.Status, job.LeaseToken, job.LeaseUntil = "processing", token, &until
		job.Attempts++
		return tx.Model(&job).Updates(map[string]any{"status": job.Status, "attempts": job.Attempts, "lease_token": token, "lease_until": until, "updated_at": now}).Error
	})
	return claimed, err
}

// Retry preserves the identity and waits at least Jev's one-minute cooldown.
func (r *SupportTagJobRepository) Retry(ctx context.Context, job model.SupportTagJob, now time.Time) error {
	status := "pending"
	if job.Attempts >= 5 {
		status = "failed"
	}
	result := activeSupportTagClaim(r.db.WithContext(ctx), job, now).Updates(map[string]any{
		"status": status, "result_code": "tagging_unavailable", "lease_token": "", "lease_until": nil,
		"available_at": now.Add(time.Minute * time.Duration(1<<min(max(job.Attempts-1, 0), 4))), "updated_at": now,
	})
	return supportTagClaimResult(result)
}

// Finalize fences a claim under the conversation lock shared by manual tag edits.
// The callback's tag changes, audit messages and job receipt commit together.
// Provider calls must finish before entering this transaction.
func (r *SupportTagJobRepository) Finalize(ctx context.Context, job model.SupportTagJob, apply func(*gorm.DB) (string, error)) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conv model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("workspace_id = ? AND id = ?", job.WorkspaceID, job.ConversationID).Take(&conv).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSupportTagLeaseLost
			}
			return err
		}
		var current model.SupportTagJob
		if err := activeSupportTagClaim(tx, job, time.Now().UTC()).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSupportTagLeaseLost
			}
			return err
		}
		code, err := apply(tx)
		if err != nil {
			return err
		}
		return supportTagClaimResult(activeSupportTagClaim(tx, job, time.Now().UTC()).Updates(map[string]any{
			"status": "done", "result_code": code, "lease_token": "", "lease_until": nil, "updated_at": time.Now().UTC(),
		}))
	})
}

func activeSupportTagClaim(db *gorm.DB, job model.SupportTagJob, now time.Time) *gorm.DB {
	return db.Model(&model.SupportTagJob{}).Where("workspace_id = ? AND conversation_id = ? AND message_id = ? AND status = 'processing' AND lease_token = ? AND lease_until > ?", job.WorkspaceID, job.ConversationID, job.MessageID, job.LeaseToken, now)
}

func supportTagClaimResult(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSupportTagLeaseLost
	}
	return nil
}
