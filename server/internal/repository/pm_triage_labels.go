package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ApplyTriageLabels adds approved existing labels while holding the source task
// lock. Manual removals and task edits win over in-flight provider decisions.
func (r *PMTaskRepository) ApplyTriageLabels(ctx context.Context, scope PMTriageScope, taskID, actorID, assessmentID string, expectedUpdatedAt time.Time, labelIDs []string) ([]string, error) {
	added := []string{}
	err := r.withTransaction(ctx, func(tx *gorm.DB) error {
		var task model.PMTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", taskID, scope.WorkspaceID).First(&task).Error; err != nil {
			return err
		}
		if task.Archived || !task.UpdatedAt.Equal(expectedUpdatedAt) {
			return nil
		}
		if !scope.AllTeams {
			allowed := false
			for _, id := range scope.TeamIDs {
				if task.TeamID != nil && *task.TeamID == id {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil
			}
		}
		var assessment model.PMTriageAssessment
		if err := tx.Where("id = ? AND workspace_id = ? AND actor_id = ? AND source_kind = ? AND source_id = ? AND status = ? AND mode = ?", assessmentID, scope.WorkspaceID, actorID, "task", taskID, "ready", "primary").First(&assessment).Error; err != nil {
			return err
		}
		for _, id := range labelIDs {
			var label model.PMLabel
			query := tx.Where("id = ? AND workspace_id = ? AND archived = ?", id, scope.WorkspaceID, false)
			if task.TeamID == nil {
				query = query.Where("team_id IS NULL")
			} else {
				query = query.Where("(team_id IS NULL OR team_id = ?)", *task.TeamID)
			}
			if err := query.First(&label).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}
			var suppressed int64
			if err := tx.Model(&model.PMTriageLabelSuppression{}).Where("workspace_id = ? AND task_id = ? AND label_id = ?", scope.WorkspaceID, taskID, id).Count(&suppressed).Error; err != nil {
				return err
			}
			if suppressed > 0 {
				continue
			}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PMTaskLabel{TaskID: taskID, LabelID: id})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				continue
			}
			metadata, err := json.Marshal(map[string]string{"provider": "typesafe", "assessment_id": assessmentID, "source": "pm_triage", "label_name": label.Name})
			if err != nil {
				return err
			}
			field := "label"
			activity := model.PMActivityLog{ID: uuid.NewString(), WorkspaceID: scope.WorkspaceID, EntityType: "task", EntityID: taskID, ActorID: &actorID, Action: "label_added", FieldName: &field, NewValue: &id, Metadata: metadata}
			if err := tx.Create(&activity).Error; err != nil {
				return err
			}
			added = append(added, id)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("apply triage labels: %w", err)
	}
	return added, nil
}

// RemoveLabelWithTriageSuppression atomically records human intent and removes
// the label. All automatic applications lock the same task row.
func (r *PMTaskRepository) RemoveLabelWithTriageSuppression(ctx context.Context, workspaceID, taskID, labelID string) error {
	return r.withTransaction(ctx, func(tx *gorm.DB) error {
		var task model.PMTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ? AND workspace_id = ?", taskID, workspaceID).First(&task).Error; err != nil {
			return err
		}
		// Only persist existing same-workspace labels so arbitrary IDs cannot create
		// foreign references or change the idempotent remove endpoint's behavior.
		var label model.PMLabel
		err := tx.Where("id = ? AND workspace_id = ?", labelID, workspaceID).First(&label).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PMTriageLabelSuppression{WorkspaceID: workspaceID, TaskID: taskID, LabelID: labelID}).Error; err != nil {
				return err
			}
		}
		return tx.Where("task_id = ? AND label_id = ?", taskID, labelID).Delete(&model.PMTaskLabel{}).Error
	})
}

// ReplaceLabelsWithTriageSuppression preserves removals made through task forms,
// not only the dedicated remove-label endpoint. Call within the task mutation transaction.
func (r *PMTaskRepository) ReplaceLabelsWithTriageSuppression(ctx context.Context, workspaceID, taskID string, labelIDs []string) error {
	return r.withTransaction(ctx, func(tx *gorm.DB) error {
		var task model.PMTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ? AND workspace_id = ?", taskID, workspaceID).First(&task).Error; err != nil {
			return err
		}
		var existing []model.PMTaskLabel
		if err := tx.Where("task_id = ?", taskID).Find(&existing).Error; err != nil {
			return err
		}
		keep := map[string]bool{}
		for _, id := range labelIDs {
			keep[id] = true
		}
		for _, label := range existing {
			if keep[label.LabelID] {
				continue
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PMTriageLabelSuppression{WorkspaceID: workspaceID, TaskID: taskID, LabelID: label.LabelID}).Error; err != nil {
				return err
			}
		}
		tasks := &PMTaskRepository{db: tx, inMutationTransaction: true}
		return tasks.ReplaceLabels(ctx, taskID, labelIDs)
	})
}
