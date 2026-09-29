package service

import (
	"context"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// taskWorkflow is the shared policy for task creation and team/workflow changes.
// A team workflow takes precedence over the workspace-wide fallback.
func (s *PMTaskService) taskWorkflow(ctx context.Context, workspaceID, teamID, workflowID string) (*model.WorkflowWithStates, error) {
	teamWorkflow, err := s.workflowRepo.GetByTeamID(ctx, workspaceID, teamID)
	if err != nil {
		return nil, err
	}
	if workflowID != "" {
		workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
		if err != nil {
			return nil, err
		}
		if workflow == nil || workflow.Workflow.WorkspaceID != workspaceID {
			return nil, errCommandNotFound("workflow")
		}
		workflowTeam := stringValue(workflow.Workflow.TeamID)
		if (workflowTeam != "" && workflowTeam != teamID) || (teamWorkflow != nil && workflowTeam == "") {
			return nil, errCommandInput("workflow_id does not belong to team_id; use the team's workflow")
		}
		return workflow, nil
	}
	if teamWorkflow != nil {
		return teamWorkflow, nil
	}
	workflow, err := s.workflowRepo.GetDefaultWorkflow(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if workflow == nil {
		return s.workflowRepo.SeedDefaultWorkflow(ctx, workspaceID)
	}
	return workflow, nil
}

func (s *PMTaskService) resolveTaskWorkflow(ctx context.Context, workspaceID, teamID, workflowID, stateID string) (string, string, error) {
	workflow, err := s.taskWorkflow(ctx, workspaceID, strings.TrimSpace(teamID), strings.TrimSpace(workflowID))
	if err != nil {
		return "", "", err
	}
	stateID = strings.TrimSpace(stateID)
	if stateID == "" {
		stateID = stringValue(workflow.Workflow.DefaultStateID)
		if stateID == "" {
			for _, state := range workflow.States {
				if state.IsDefault {
					stateID = state.ID
					break
				}
			}
		}
		if stateID == "" && len(workflow.States) > 0 {
			stateID = workflow.States[0].ID
		}
	}
	for _, state := range workflow.States {
		if state.ID == stateID {
			return workflow.Workflow.ID, state.ID, nil
		}
	}
	return "", "", errCommandInput("workflow_state_id must belong to workflow_id")
}

func (s *PMTaskService) resolveTaskWorkflowUpdate(ctx context.Context, current *model.PMTask, req model.UpdateTaskRequest) (string, string, error) {
	teamID := stringValue(current.TeamID)
	workflowID, stateID := current.WorkflowID, current.WorkflowStateID
	if req.TeamID != nil {
		nextTeamID := strings.TrimSpace(*req.TeamID)
		if nextTeamID == "" {
			return "", "", errCommandInput("team_id is required")
		}
		if nextTeamID != teamID {
			if err := requireTeamMembershipForCreate(ctx, &nextTeamID); err != nil {
				return "", "", err
			}
			workflowID = ""
		}
		teamID = nextTeamID
	}
	if req.WorkflowID != nil {
		workflowID = strings.TrimSpace(*req.WorkflowID)
	}
	workflow, err := s.taskWorkflow(ctx, current.WorkspaceID, teamID, workflowID)
	if err != nil {
		return "", "", err
	}
	if req.WorkflowStateID != nil {
		stateID = strings.TrimSpace(*req.WorkflowStateID)
	} else if workflow.Workflow.ID != current.WorkflowID {
		previous, err := s.workflowRepo.GetStateByID(ctx, current.WorkflowStateID)
		if err != nil {
			return "", "", err
		}
		stateID = ""
		if previous != nil {
			for _, state := range workflow.States {
				if state.StateType != previous.StateType {
					continue
				}
				if stateID == "" {
					stateID = state.ID
				}
				if strings.EqualFold(strings.TrimSpace(state.Name), strings.TrimSpace(previous.Name)) {
					stateID = state.ID
					break
				}
			}
		}
		if stateID == "" {
			return "", "", errCommandInput("select a status in the destination workflow; no equivalent status exists")
		}
	}
	for _, state := range workflow.States {
		if state.ID == stateID {
			return workflow.Workflow.ID, state.ID, nil
		}
	}
	return "", "", errCommandInput("workflow_state_id must belong to workflow_id")
}
