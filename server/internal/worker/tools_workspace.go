package worker

import (
	"encoding/json"
	"fmt"
	"strings"
)

func toolListWorkspaceTeams(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("execution context is required")
	}
	if ctx.Services == nil || ctx.Services.ListWorkspaceTeams == nil {
		return "", fmt.Errorf("workspace team lookup is not available for this agent")
	}

	teams, err := ctx.Services.ListWorkspaceTeams(ctx.Context, ctx.WorkspaceID)
	if err != nil {
		return "", fmt.Errorf("list workspace teams: %w", err)
	}
	if len(teams) == 0 {
		return "No workspace teams found.", nil
	}

	return toCompactJSONString(teams), nil
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
