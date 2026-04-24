package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestUpdateStoryStateToolUsesInternalCommandExecutorWhenAvailable(t *testing.T) {
	registry := NewToolRegistry(nil)
	called := false
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		TaskID:      "story-1",
		AgentID:     "agent-1",
		RunID:       "run-1",
		AllowedTools: map[string]bool{
			"update_task_state": true,
		},
		Services: &ServiceBridge{
			ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
				called = true
				if name != "pm.update_task_state" {
					t.Fatalf("unexpected command name %q", name)
				}
				if meta.WorkspaceID != "ws-1" || meta.TargetType != "task" || meta.TargetID != "story-1" || meta.AgentID != "agent-1" || meta.RunID != "run-1" {
					t.Fatalf("unexpected command meta %#v", meta)
				}
				return json.RawMessage(`{"story_id":"story-1","state_id":"state-2"}`), nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "update_task_state", json.RawMessage(`{"state_id":"state-2"}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !called {
		t.Fatal("expected internal command executor to be used")
	}
	if output != `{"story_id":"story-1","state_id":"state-2"}` {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestCreateTaskToolUsesInternalCommandExecutorWhenAvailable(t *testing.T) {
	registry := NewToolRegistry(nil)
	called := false
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		TargetType:  "epic",
		TargetID:    "epic-1",
		AgentID:     "agent-1",
		RunID:       "run-1",
		AllowedTools: map[string]bool{
			"create_task": true,
		},
		Services: &ServiceBridge{
			ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
				called = true
				if name != "pm.create_task" {
					t.Fatalf("unexpected command name %q", name)
				}
				if meta.WorkspaceID != "ws-1" || meta.TargetType != "epic" || meta.TargetID != "epic-1" || meta.AgentID != "agent-1" || meta.RunID != "run-1" {
					t.Fatalf("unexpected command meta %#v", meta)
				}
				var payload struct {
					Name   string  `json:"name"`
					TeamID string  `json:"team_id"`
					EpicID *string `json:"epic_id"`
				}
				if err := json.Unmarshal(input, &payload); err != nil {
					t.Fatalf("unmarshal command input: %v", err)
				}
				if payload.Name != "Ship workflow tool" || payload.TeamID != "team-1" || payload.EpicID == nil || *payload.EpicID != "epic-1" {
					t.Fatalf("unexpected command input %s", string(input))
				}
				return json.RawMessage(`{"task_id":"task-1","name":"Ship workflow tool","workflow_id":"wf-1","state_id":"state-2"}`), nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "create_task", json.RawMessage(`{"name":"Ship workflow tool","team_id":"team-1"}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !called {
		t.Fatal("expected internal command executor to be used")
	}
	if output != `{"task_id":"task-1","name":"Ship workflow tool","workflow_id":"wf-1","state_id":"state-2"}` {
		t.Fatalf("unexpected output %q", output)
	}
}
