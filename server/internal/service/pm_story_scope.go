package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

func validateEpicScope(ctx context.Context, epicRepo *repository.PMEpicRepository, workspaceID string, epicID, storyTeamID *string) error {
	if epicRepo == nil || epicID == nil || *epicID == "" {
		return nil
	}

	epic, err := epicRepo.GetByID(ctx, *epicID)
	if err != nil {
		return err
	}
	if epic == nil || epic.Epic.WorkspaceID != workspaceID {
		return fmt.Errorf("epic not found")
	}
	if epic.Epic.TeamID == nil || *epic.Epic.TeamID == "" {
		return nil
	}
	if storyTeamID == nil || *storyTeamID == "" || *epic.Epic.TeamID != *storyTeamID {
		return fmt.Errorf("epic %q does not belong to the selected team", epic.Epic.Name)
	}
	return nil
}

func validateSprintScope(ctx context.Context, sprintRepo *repository.PMSprintRepository, workspaceID string, sprintID, storyTeamID *string) error {
	if sprintRepo == nil || sprintID == nil || *sprintID == "" {
		return nil
	}

	sprint, err := sprintRepo.GetByID(ctx, *sprintID)
	if err != nil {
		return err
	}
	if sprint == nil || sprint.Sprint.WorkspaceID != workspaceID {
		return fmt.Errorf("sprint not found")
	}
	if sprint.Sprint.TeamID == nil || *sprint.Sprint.TeamID == "" {
		if storyTeamID == nil || *storyTeamID == "" {
			return nil
		}
		return fmt.Errorf("sprint %q does not belong to the selected team", sprint.Sprint.Name)
	}
	if storyTeamID == nil || *storyTeamID == "" || *sprint.Sprint.TeamID != *storyTeamID {
		return fmt.Errorf("sprint %q does not belong to the selected team", sprint.Sprint.Name)
	}
	return nil
}
