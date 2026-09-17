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

// ErrPMTaskRevisionChanged prevents reviewed suggestions overwriting newer edits.
var ErrPMTaskRevisionChanged = errors.New("task changed since review")

// RequireRevision locks and checks a task inside an existing mutation transaction.
func (r *PMTaskRepository) RequireRevision(ctx context.Context, id string, expected time.Time) error {
	if !r.inMutationTransaction {
		return errors.New("revision check requires task mutation transaction")
	}
	var task model.PMTask
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "updated_at", "archived").Where("id = ?", id).First(&task).Error; err != nil {
		return err
	}
	if task.Archived || !task.UpdatedAt.Equal(expected) {
		return ErrPMTaskRevisionChanged
	}
	return nil
}

// MarkReviewed merges one human decision without losing concurrent dismissals.
func (r *PMTriageRepository) MarkReviewed(ctx context.Context, workspaceID, actorID, id, key, status string) error {
	if status != "accepted" && status != "dismissed" {
		return errors.New("invalid triage review status")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var assessment model.PMTriageAssessment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ? AND actor_id = ? AND status = ? AND mode = ?", id, workspaceID, actorID, "ready", "primary").First(&assessment).Error; err != nil {
			return err
		}
		if assessment.Reviewed == nil {
			assessment.Reviewed = model.JSONB{}
		}
		if previous, ok := assessment.Reviewed[key]; ok && previous != status {
			return errors.New("suggestion already reviewed")
		}
		assessment.Reviewed[key] = status
		return tx.Model(&model.PMTriageAssessment{}).Where("id = ?", id).Updates(map[string]any{"reviewed": assessment.Reviewed, "updated_at": time.Now().UTC()}).Error
	})
}

// CreateWithTaskRevisions atomically fences a reviewed relationship against edits
// and team moves of either task. Task locks use a stable order to avoid deadlocks.
func (r *PMTaskLinkRepository) CreateWithTaskRevisions(ctx context.Context, link *model.PMTaskLink, expected map[string]time.Time, scope PMTriageScope) error {
	if len(expected) != 2 || link.WorkspaceID != scope.WorkspaceID {
		return ErrPMTaskRevisionChanged
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tasks []model.PMTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id IN ?", scope.WorkspaceID, []string{link.SourceTaskID, link.TargetTaskID}).Order("id ASC").Find(&tasks).Error; err != nil {
			return err
		}
		if len(tasks) != 2 {
			return ErrPMTaskRevisionChanged
		}
		for _, task := range tasks {
			revision, ok := expected[task.ID]
			if !ok || task.Archived || !task.UpdatedAt.Equal(revision) {
				return ErrPMTaskRevisionChanged
			}
			if !scope.AllTeams {
				allowed := false
				for _, teamID := range scope.TeamIDs {
					if task.TeamID != nil && *task.TeamID == teamID {
						allowed = true
					}
				}
				if !allowed {
					return ErrPMTaskRevisionChanged
				}
			}
		}
		if link.LinkType == model.PMTaskLinkTypeRelatesTo || link.LinkType == model.PMTaskLinkTypeDuplicates {
			var existing model.PMTaskLink
			err := tx.Where("workspace_id = ? AND link_type = ? AND ((source_task_id = ? AND target_task_id = ?) OR (source_task_id = ? AND target_task_id = ?))", link.WorkspaceID, link.LinkType, link.SourceTaskID, link.TargetTaskID, link.TargetTaskID, link.SourceTaskID).First(&existing).Error
			if err == nil {
				*link = existing
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if link.ID == "" {
			link.ID = uuid.NewString()
		}
		return (&PMTaskLinkRepository{db: tx}).Create(ctx, link)
	})
}
