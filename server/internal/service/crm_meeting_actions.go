package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AcceptActionItem creates a canonical PM task and links it to the meeting.
func (s *CRMMeetingService) AcceptActionItem(
	ctx context.Context,
	workspaceID, meetingID, itemID, actorID string,
	req model.AcceptCRMMeetingActionItemRequest,
) (*model.CRMMeetingActionItem, error) {
	item, err := s.repo.GetActionItem(ctx, workspaceID, meetingID, itemID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("meeting action item not found")
	}
	if item.Status == model.CRMMeetingActionDismissed {
		return nil, fmt.Errorf("dismissed meeting action items cannot be accepted")
	}
	if item.Status == model.CRMMeetingActionAccepted {
		if err := s.ensureMeetingTaskAssociation(ctx, workspaceID, meetingID, item.TaskID); err != nil {
			return nil, err
		}
		return item, nil
	}
	teamID := strings.TrimSpace(req.TeamID)
	if teamID == "" {
		return nil, fmt.Errorf("team_id is required")
	}
	ownerIDs := []string{}
	if req.OwnerMemberID != nil && strings.TrimSpace(*req.OwnerMemberID) != "" {
		ownerIDs = append(ownerIDs, strings.TrimSpace(*req.OwnerMemberID))
	} else if item.AssigneeMemberID != nil && strings.TrimSpace(*item.AssigneeMemberID) != "" {
		ownerIDs = append(ownerIDs, strings.TrimSpace(*item.AssigneeMemberID))
	}
	detail, err := s.taskService.Create(ctx, model.CreateTaskRequest{
		WorkspaceID:     workspaceID,
		Name:            strings.TrimSpace(item.Title),
		Description:     item.Details,
		TaskType:        model.PMTaskTypeFeature,
		WorkflowID:      strings.TrimSpace(req.WorkflowID),
		WorkflowStateID: strings.TrimSpace(req.WorkflowStateID),
		TeamID:          &teamID,
		OwnerMemberIDs:  ownerIDs,
		Deadline:        item.DueDate,
	}, actorID)
	if err != nil {
		return nil, err
	}
	item.Status = model.CRMMeetingActionAccepted
	item.TaskID = &detail.Task.ID
	if err := s.repo.UpdateActionItem(ctx, item); err != nil {
		return nil, err
	}
	if err := s.ensureMeetingTaskAssociation(ctx, workspaceID, meetingID, item.TaskID); err != nil {
		return nil, err
	}
	return item, nil
}

// DismissActionItem marks an extracted action as reviewed without creating a task.
func (s *CRMMeetingService) DismissActionItem(
	ctx context.Context,
	workspaceID, meetingID, itemID string,
) (*model.CRMMeetingActionItem, error) {
	item, err := s.repo.GetActionItem(ctx, workspaceID, meetingID, itemID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("meeting action item not found")
	}
	if item.Status == model.CRMMeetingActionAccepted {
		return nil, fmt.Errorf("accepted meeting action items cannot be dismissed")
	}
	if item.Status == model.CRMMeetingActionDismissed {
		return item, nil
	}
	item.Status = model.CRMMeetingActionDismissed
	if err := s.repo.UpdateActionItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *CRMMeetingService) ensureMeetingTaskAssociation(
	ctx context.Context,
	workspaceID, meetingID string,
	taskID *string,
) error {
	if taskID == nil || strings.TrimSpace(*taskID) == "" {
		return nil
	}
	label := "Created from meeting"
	return s.associationRepo.Create(ctx, &model.CRMAssociation{
		WorkspaceID:      workspaceID,
		FromObjectType:   model.CRMObjectMeeting,
		FromObjectID:     meetingID,
		ToObjectType:     model.CRMObjectTask,
		ToObjectID:       strings.TrimSpace(*taskID),
		AssociationLabel: &label,
	})
}
