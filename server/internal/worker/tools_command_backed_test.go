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
		StoryID:     "story-1",
		AgentID:     "agent-1",
		RunID:       "run-1",
		AllowedTools: map[string]bool{
			"update_story_state": true,
		},
		Services: &ServiceBridge{
			ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
				called = true
				if name != "pm.update_story_state" {
					t.Fatalf("unexpected command name %q", name)
				}
				if meta.WorkspaceID != "ws-1" || meta.TargetType != "story" || meta.TargetID != "story-1" || meta.AgentID != "agent-1" || meta.RunID != "run-1" {
					t.Fatalf("unexpected command meta %#v", meta)
				}
				return json.RawMessage(`{"story_id":"story-1","state_id":"state-2"}`), nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "update_story_state", json.RawMessage(`{"state_id":"state-2"}`))
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
