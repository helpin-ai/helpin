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

// JevDecisionRepository owns admission and content-free audit records.
type JevDecisionRepository struct{ db *gorm.DB }

// NewJevDecisionRepository constructs the semantic decision store.
func NewJevDecisionRepository(db *gorm.DB) *JevDecisionRepository {
	return &JevDecisionRepository{db: db}
}

// JevDecisionAdmission distinguishes a cached decision, a new call and a cap.
type JevDecisionAdmission struct {
	Attempt      *model.JevDecisionAttempt
	CallProvider bool
	Limited      bool
}

// Reserve serializes cache admission and the per-feature UTC-day call budget.
// A one-minute pending/failure cooldown prevents repeated provider failures from
// creating a retry storm. Abandoned pending calls still consume the daily budget.
func (r *JevDecisionRepository) Reserve(ctx context.Context, input model.JevDecisionAttempt, limit int) (*JevDecisionAdmission, error) {
	if input.WorkspaceID == "" || input.Feature == "" || input.SourceID == "" || input.InputHash == "" || limit < 1 || (input.Mode != "primary" && input.Mode != "shadow") {
		return nil, errors.New("invalid decision admission")
	}
	admission := &JevDecisionAdmission{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var workspace model.Workspace
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", input.WorkspaceID).First(&workspace).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		var previous model.JevDecisionAttempt
		err := tx.Where("workspace_id = ? AND feature = ? AND source_id = ? AND input_hash = ? AND mode = ?", input.WorkspaceID, input.Feature, input.SourceID, input.InputHash, input.Mode).Order("created_at DESC, id DESC").First(&previous).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			if previous.Status == "ready" || now.Sub(previous.CreatedAt) < time.Minute {
				admission.Attempt = &previous
				return nil
			}
			if previous.Status == "pending" {
				if err := tx.Model(&model.JevDecisionAttempt{}).Where("id = ? AND status = ?", previous.ID, "pending").Updates(map[string]any{"status": "failed", "updated_at": now}).Error; err != nil {
					return err
				}
			}
		}
		var count int64
		if err := tx.Model(&model.JevDecisionAttempt{}).Where("workspace_id = ? AND feature = ? AND created_at >= ?", input.WorkspaceID, input.Feature, now.Truncate(24*time.Hour)).Count(&count).Error; err != nil {
			return err
		}
		if count >= int64(limit) {
			admission.Limited = true
			return nil
		}
		input.ID, input.Status, input.Outcome = uuid.NewString(), "pending", model.JSONB{}
		input.CreatedAt, input.UpdatedAt = now, now
		if err := tx.Create(&input).Error; err != nil {
			return err
		}
		admission.Attempt, admission.CallProvider = &input, true
		return nil
	})
	return admission, err
}

// Finish settles only the still-pending attempt; stale completions cannot win.
func (r *JevDecisionRepository) Finish(ctx context.Context, workspaceID, id, status string, outcome model.JSONB) error {
	if (status != "ready" && status != "failed") || outcome == nil {
		return errors.New("invalid decision settlement")
	}
	result := r.db.WithContext(ctx).Model(&model.JevDecisionAttempt{}).Where("workspace_id = ? AND id = ? AND status = ?", workspaceID, id, "pending").Updates(map[string]any{"status": status, "outcome": outcome, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("decision attempt no longer pending")
	}
	return nil
}
