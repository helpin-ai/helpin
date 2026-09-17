package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMTriageRepository persists provider admission, decisions and human review.
type PMTriageRepository struct{ db *gorm.DB }

// NewPMTriageRepository constructs the PM decision audit store.
func NewPMTriageRepository(db *gorm.DB) *PMTriageRepository { return &PMTriageRepository{db: db} }

// PMTriageAdmission distinguishes cached decisions, in-flight attempts and caps.
type PMTriageAdmission struct {
	Assessment   *model.PMTriageAssessment
	CallProvider bool
	Limited      bool
}

// Reserve serializes admission per workspace. Failed and abandoned attempts
// count against the UTC daily cap. Identical failures have a one-minute cooldown.
func (r *PMTriageRepository) Reserve(ctx context.Context, input model.PMTriageAssessment, limit int) (*PMTriageAdmission, error) {
	if input.WorkspaceID == "" || input.ActorID == "" || input.SourceID == "" || strings.TrimSpace(input.SourceHash) == "" || strings.TrimSpace(input.ContextHash) == "" || limit < 1 {
		return nil, errors.New("invalid triage admission")
	}
	if (input.SourceKind != "task" && input.SourceKind != "support_conversation") || (input.Mode != "primary" && input.Mode != "shadow") {
		return nil, errors.New("invalid triage source or mode")
	}
	admission := &PMTriageAdmission{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ws model.Workspace
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", input.WorkspaceID).First(&ws).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		var previous model.PMTriageAssessment
		err := tx.Where("workspace_id = ? AND actor_id = ? AND source_kind = ? AND source_id = ? AND context_hash = ? AND source_hash = ? AND mode = ?", input.WorkspaceID, input.ActorID, input.SourceKind, input.SourceID, input.ContextHash, input.SourceHash, input.Mode).Order("created_at DESC, id DESC").First(&previous).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			if previous.Status == "ready" || now.Sub(previous.CreatedAt) < time.Minute {
				admission.Assessment = &previous
				return nil
			}
			if previous.Status == "pending" {
				if err := tx.Model(&model.PMTriageAssessment{}).Where("id = ? AND status = ?", previous.ID, "pending").Updates(map[string]any{"status": "failed", "updated_at": now}).Error; err != nil {
					return err
				}
			}
		}
		var count int64
		if err := tx.Model(&model.PMTriageAssessment{}).Where("workspace_id = ? AND created_at >= ?", input.WorkspaceID, now.Truncate(24*time.Hour)).Count(&count).Error; err != nil {
			return err
		}
		if count >= int64(limit) {
			admission.Limited = true
			return nil
		}
		input.ID = uuid.NewString()
		input.Status = "pending"
		input.Outcome = model.JSONB{}
		input.Reviewed = model.JSONB{}
		input.CreatedAt = now
		input.UpdatedAt = now
		if err := tx.Create(&input).Error; err != nil {
			return err
		}
		admission.Assessment = &input
		admission.CallProvider = true
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reserve PM triage: %w", err)
	}
	return admission, nil
}

// Finish persists a single pending attempt. Stale provider completions cannot
// overwrite a failed attempt or a previously stored decision.
func (r *PMTriageRepository) Finish(ctx context.Context, workspaceID, actorID, id, status string, outcome model.JSONB) error {
	if status != "ready" && status != "failed" {
		return errors.New("invalid triage completion status")
	}
	if outcome == nil {
		outcome = model.JSONB{}
	}
	result := r.db.WithContext(ctx).Model(&model.PMTriageAssessment{}).
		Where("id = ? AND workspace_id = ? AND actor_id = ? AND status = ?", id, workspaceID, actorID, "pending").
		Updates(map[string]any{"status": status, "outcome": outcome, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return fmt.Errorf("finish PM triage: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return errors.New("triage attempt no longer pending")
	}
	return nil
}

// Get returns only an assessment belonging to this actor and workspace. Source
// and candidate permissions must still be rechecked before exposing the outcome.
func (r *PMTriageRepository) Get(ctx context.Context, workspaceID, actorID, id string) (*model.PMTriageAssessment, error) {
	var assessment model.PMTriageAssessment
	err := r.db.WithContext(ctx).Where("id = ? AND workspace_id = ? AND actor_id = ?", id, workspaceID, actorID).First(&assessment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get PM triage: %w", err)
	}
	return &assessment, nil
}
