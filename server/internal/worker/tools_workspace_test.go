package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
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
		AgentID:     "agent-1",
		AllowedTools: map[string]bool{
			"list_workspace_teams": true,
		},
		Services: &ServiceBridge{
			ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
				if name != "workspace.list_teams" {
					t.Fatalf("expected workspace.list_teams command, got %q", name)
				}
				if meta.WorkspaceID != "ws-1" || meta.TargetType != "workspace" || meta.TargetID != "ws-1" || meta.ActorID != "agent-1" {
					t.Fatalf("unexpected command metadata %#v", meta)
				}
				return json.RawMessage(`[{"id":"team-1","name":"Platform","handle":"platform","team_type":"engineering","default_task_type":"feature"},{"id":"team-2","name":"Growth","team_type":"growth","default_task_type":"task"}]`), nil
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

func TestListTeamWorkflowsWithStagesToolReturnsResolvedWorkflow(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		AllowedTools: map[string]bool{
			"list_team_workflows_with_stages": true,
		},
		Services: &ServiceBridge{
			ListTeamWorkflows: func(ctx context.Context, workspaceID string, teamID *string) ([]TeamWorkflowSummary, error) {
				if workspaceID != "ws-1" {
					t.Fatalf("expected workspace_id ws-1, got %q", workspaceID)
				}
				if teamID == nil || *teamID != "team-1" {
					t.Fatalf("expected team_id team-1, got %#v", teamID)
				}
				return []TeamWorkflowSummary{
					{
						TeamID:           "team-1",
						TeamName:         "Platform",
						WorkflowID:       "wf-1",
						WorkflowName:     "Platform Workflow",
						UsesTeamWorkflow: true,
						Stages: []WorkflowStageSummary{
							{ID: "state-1", Name: "Backlog", StateType: "backlog", Position: 0, IsDefault: false},
							{ID: "state-2", Name: "To Do", StateType: "unstarted", Position: 1, IsDefault: true},
						},
					},
				}, nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "list_team_workflows_with_stages", json.RawMessage(`{"team_id":"team-1"}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"workflow_id":"wf-1"`) || !strings.Contains(output, `"state_type":"unstarted"`) {
		t.Fatalf("expected workflow summary JSON output, got %s", output)
	}
}
