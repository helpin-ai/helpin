package worker

import (
	"encoding/json"
	"fmt"
	"strings"
)

func toolListWorkspaceTeams(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if output, ok, err := executeInternalCommand(ctx, "workspace", ctx.WorkspaceID, "workspace.list_teams", input); ok {
		if err != nil {
			return "", fmt.Errorf("list workspace teams: %w", err)
		}
		return string(output), nil
	}
	return "", fmt.Errorf("list_workspace_teams requires internal commands")
}

func toolListTeamWorkflowsWithStages(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("execution context is required")
	}
	if ctx.Services == nil || ctx.Services.ListTeamWorkflows == nil {
		return "", fmt.Errorf("team workflow lookup is not available for this agent")
	}

	var params struct {
		TeamID *string `json:"team_id"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return "", fmt.Errorf("parse input: %w", err)
		}
	}
	if params.TeamID != nil {
		trimmed := strings.TrimSpace(*params.TeamID)
		params.TeamID = &trimmed
		if trimmed == "" {
			params.TeamID = nil
		}
	}

	workflows, err := ctx.Services.ListTeamWorkflows(ctx.Context, ctx.WorkspaceID, params.TeamID)
	if err != nil {
		return "", fmt.Errorf("list team workflows: %w", err)
	}
	if len(workflows) == 0 {
		return "No team workflows found.", nil
	}
	return toCompactJSONString(workflows), nil
}
