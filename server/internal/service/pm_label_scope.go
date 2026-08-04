package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func validateLabelScope(ctx context.Context, labelRepo *repository.PMLabelRepository, workspaceID string, labelIDs []string, allowedTeamIDs []string) error {
	return validateLabelScopeWithArchivePolicy(ctx, labelRepo, workspaceID, labelIDs, allowedTeamIDs, false)
}

func validateOperationalLabelScope(ctx context.Context, labelRepo *repository.PMLabelRepository, workspaceID string, labelIDs []string, allowedTeamIDs []string) error {
	return validateLabelScopeWithArchivePolicy(ctx, labelRepo, workspaceID, labelIDs, allowedTeamIDs, true)
}

func validateLabelScopeWithArchivePolicy(ctx context.Context, labelRepo *repository.PMLabelRepository, workspaceID string, labelIDs []string, allowedTeamIDs []string, rejectArchived bool) error {
	if labelRepo == nil || len(labelIDs) == 0 {
		return nil
	}
	labelIDs = dedupeIDs(labelIDs)
	labels, err := labelRepo.ListByIDs(ctx, labelIDs)
	if err != nil {
		return err
	}
	labelsByID := make(map[string]model.PMLabel, len(labels))
	for _, label := range labels {
		labelsByID[label.ID] = label
	}
	allowed := make(map[string]struct{}, len(allowedTeamIDs))
	for _, teamID := range allowedTeamIDs {
		if teamID == "" {
			continue
		}
		allowed[teamID] = struct{}{}
	}
	for _, labelID := range labelIDs {
		label, ok := labelsByID[labelID]
		if !ok || label.WorkspaceID != workspaceID || (rejectArchived && label.Archived) {
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
