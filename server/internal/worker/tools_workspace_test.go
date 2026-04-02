package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestListWorkspaceTeamsToolIsRegistered(t *testing.T) {
	registry := NewToolRegistry(nil)

	for _, def := range registry.Definitions() {
		if def.Name == "list_workspace_teams" {
			return
		}
	}

	t.Fatal("expected list_workspace_teams tool definition")
}

func TestListWorkspaceTeamsToolReturnsWorkspaceTeams(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		AllowedTools: map[string]bool{
			"list_workspace_teams": true,
		},
		Services: &ServiceBridge{
			ListWorkspaceTeams: func(ctx context.Context, workspaceID string) ([]WorkspaceTeamSummary, error) {
				if workspaceID != "ws-1" {
					t.Fatalf("expected workspace_id ws-1, got %q", workspaceID)
				}
				handle := "platform"
				return []WorkspaceTeamSummary{
					{ID: "team-1", Name: "Platform", Handle: &handle, TeamType: "engineering", DefaultTaskType: "feature"},
					{ID: "team-2", Name: "Growth", TeamType: "growth", DefaultTaskType: "task"},
				}, nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "list_workspace_teams", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"id":"team-1"`) || !strings.Contains(output, `"name":"Growth"`) {
		t.Fatalf("expected team list JSON output, got %s", output)
	}
}
