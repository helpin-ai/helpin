package service

import (
	"context"
	"fmt"

	"github.com/d4interactive/teampulse/server/internal/repository"
)

func validateLabelScope(ctx context.Context, labelRepo *repository.PMLabelRepository, workspaceID string, labelIDs []string, allowedTeamIDs []string) error {
	if labelRepo == nil || len(labelIDs) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(allowedTeamIDs))
	for _, teamID := range allowedTeamIDs {
		if teamID == "" {
			continue
		}
		allowed[teamID] = struct{}{}
	}
	for _, labelID := range dedupeIDs(labelIDs) {
		label, err := labelRepo.GetByID(ctx, labelID)
		if err != nil {
			return err
		}
		if label == nil || label.WorkspaceID != workspaceID {
			return fmt.Errorf("label not found")
		}
		if label.TeamID == nil {
			continue
		}
		if _, ok := allowed[*label.TeamID]; !ok {
			return fmt.Errorf("label %q does not belong to the selected team scope", label.Name)
		}
	}
	return nil
}

func allowedTeamIDs(teamIDs ...*string) []string {
	result := make([]string, 0, len(teamIDs))
	for _, teamID := range teamIDs {
		if teamID == nil || *teamID == "" {
			continue
		}
		result = append(result, *teamID)
	}
	return result
}
