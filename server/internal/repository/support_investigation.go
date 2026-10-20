package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportInvestigationRepository leases and settles durable post-handover work.
type SupportInvestigationRepository struct{ db *gorm.DB }

// NewSupportInvestigationRepository creates the durable queue repository.
func NewSupportInvestigationRepository(db *gorm.DB) *SupportInvestigationRepository {
	return &SupportInvestigationRepository{db: db}
}

// Claim uses a conditional update even after selection: multiple sweep workers
// cannot own the same job. The lease recovers abandoned work after a restart.
func (r *SupportInvestigationRepository) Claim(ctx context.Context, now time.Time) (*model.SupportInvestigation, error) {
	var job model.SupportInvestigation
	err := r.db.WithContext(ctx).Where("status = ? AND due_at <= ? AND (lease_until IS NULL OR lease_until < ?)", "pending", now, now).Order("due_at ASC").First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	until := now.Add(2 * time.Minute)
	result := r.db.WithContext(ctx).Model(&model.SupportInvestigation{}).Where("id = ? AND status = ? AND (lease_until IS NULL OR lease_until < ?)", job.ID, "pending", now).Updates(map[string]any{"lease_until": until, "started_at": gorm.Expr("COALESCE(started_at, ?)", now)})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, nil
	}
	job.LeaseUntil = &until
	if job.StartedAt == nil {
		job.StartedAt = &now
	}
	return &job, nil
}

// Release makes an unfinished job available to the next sweep.
func (r *SupportInvestigationRepository) Release(ctx context.Context, job *model.SupportInvestigation, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model.SupportInvestigation{}).Where("id = ? AND lease_until = ? AND status = ?", job.ID, job.LeaseUntil, "pending").Updates(map[string]any{"lease_until": nil, "due_at": now.Add(30 * time.Second)}).Error
}

// Finish commits the private note and job outcome together. Retrying a crash
// cannot create a second note. No public message or conversation control writes.
func (r *SupportInvestigationRepository) Finish(ctx context.Context, job *model.SupportInvestigation, note *model.SupportMessage) (bool, error) {
	changed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Match routing/ownership lock order so merges and erasure cannot race note
		// insertion. Completed work never resurrects or follows a merged thread.
		if err := LockSupportRouting(tx, job.WorkspaceID); err != nil {
			return err
		}
		var conv model.SupportConversation
		convErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", job.WorkspaceID, job.ConversationID).First(&conv).Error
		if convErr != nil && !errors.Is(convErr, gorm.ErrRecordNotFound) {
			return convErr
		}
		if convErr != nil || conv.AnonymizedAt != nil || conv.MergedIntoID != nil {
			note = nil
		}

		result := tx.Model(&model.SupportInvestigation{}).Where("id = ? AND workspace_id = ? AND status = ? AND lease_until = ?", job.ID, job.WorkspaceID, "pending", job.LeaseUntil).Updates(map[string]any{"status": "completed", "lease_until": nil, "brief": "", "question": ""})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		if note != nil {
			if !note.IsInternal || note.WorkspaceID != job.WorkspaceID || note.ConversationID != job.ConversationID {
				return errors.New("invalid investigation note scope")
			}
			if err := NewSupportMessageRepository(tx).Create(ctx, note); err != nil {
				return err
			}
		}
		changed = note != nil
		return nil
	})
	return changed && err == nil, err
}
func queueSupportInvestigation(tx *gorm.DB, job *model.SupportInvestigation, workspaceID, conversationID string) error {
	if job == nil {
		return nil
	}
	if job.WorkspaceID != workspaceID || job.ConversationID != conversationID {
		return errors.New("invalid investigation scope")
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(job).Error
}
