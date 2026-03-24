package worker

import (
	"encoding/json"
	"fmt"
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

	payload, _ := json.MarshalIndent(teams, "", "  ")
	return string(payload), nil
}
