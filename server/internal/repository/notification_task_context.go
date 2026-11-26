package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// TaskNotificationContext captures the task relationships at notification time.
// User IDs, not display names, determine why a recipient receives an update.
type TaskNotificationContext struct {
	Title       string
	RequesterID string
	StateName   string
	Owners      []TaskNotificationOwner `gorm:"-"`
}

type TaskNotificationOwner struct {
	UserID string
	Name   string
}

func (r *NotificationRepository) TaskNotificationContext(ctx context.Context, workspaceID, taskID string) (*TaskNotificationContext, error) {
	var result TaskNotificationContext
	err := r.db.WithContext(ctx).Table("pm_tasks t").
		Select("t.name AS title, COALESCE(requester.user_id, t.requester_id, '') AS requester_id, COALESCE(state.name, '') AS state_name").
		Joins("LEFT JOIN workspace_members requester ON requester.id = t.requester_member_id AND requester.workspace_id = t.workspace_id").
		Joins("LEFT JOIN pm_workflow_states state ON state.id = t.workflow_state_id").
		Where("t.id = ? AND t.workspace_id = ?", taskID, workspaceID).Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	err = r.db.WithContext(ctx).Table("pm_task_owners owner").
		Select("owner.user_id, COALESCE(u.full_name, '') AS name").
		Joins("LEFT JOIN users u ON u.id = owner.user_id").
		Where("owner.task_id = ?", taskID).Order("owner.user_id").Scan(&result.Owners).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// A task can be linked from either side of a CRM association.
func (r *NotificationRepository) IsCRMFollowUp(ctx context.Context, workspaceID, taskID string) (bool, error) {
	var linked bool
	err := r.db.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM crm_associations WHERE workspace_id=? AND
        ((from_object_type='task' AND from_object_id=? AND to_object_type IN ('deal','contact','company'))
         OR (to_object_type='task' AND to_object_id=? AND from_object_type IN ('deal','contact','company'))))`, workspaceID, taskID, taskID).Scan(&linked).Error
	return linked, err
}
