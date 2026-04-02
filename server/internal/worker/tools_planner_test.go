package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestApproveEpicSpecToolReturnsApprovedSummary(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		TargetType:  "epic",
		TargetID:    "epic-1",
		AgentID:     "agent-1",
		AllowedTools: map[string]bool{
			"approve_epic_spec": true,
		},
		Services: &ServiceBridge{
			ApproveEpicSpec: func(ctx context.Context, workspaceID, epicID, actorID string, versionID *string) (*model.ApprovedSpecSummary, error) {
				if workspaceID != "ws-1" || epicID != "epic-1" || actorID != "agent-1" {
					t.Fatalf("unexpected tool context: workspace=%q epic=%q actor=%q", workspaceID, epicID, actorID)
				}
				if versionID == nil || *versionID != "ver-1" {
					t.Fatalf("expected version_id ver-1, got %#v", versionID)
				}
				return &model.ApprovedSpecSummary{
					Stage:          model.PlanningStageDraftSpec,
					SpecDocumentID: "doc-1",
					SpecVersionID:  "ver-1",
				}, nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "approve_epic_spec", json.RawMessage(`{"version_id":"ver-1"}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"spec_document_id":"doc-1"`) || !strings.Contains(output, `"spec_version_id":"ver-1"`) {
		t.Fatalf("expected approved spec summary JSON, got %s", output)
	}
}

func TestEnsureTaskPlanDocToolReturnsDocumentMetadata(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		TargetType:  "story",
		TargetID:    "story-1",
		AgentID:     "agent-1",
		AllowedTools: map[string]bool{
			"ensure_story_plan_doc": true,
		},
		Services: &ServiceBridge{
			EnsureTaskPlanDoc: func(ctx context.Context, workspaceID, storyID, actorID string) (*model.DocsDocument, error) {
				if workspaceID != "ws-1" || storyID != "story-1" || actorID != "agent-1" {
					t.Fatalf("unexpected tool context: workspace=%q story=%q actor=%q", workspaceID, storyID, actorID)
				}
				return &model.DocsDocument{
					ID:     "doc-story-1",
					Title:  "Story Plan",
					Status: model.DocStatusDraft,
				}, nil
			},
			GetDocumentContent: func(ctx context.Context, documentID string) (string, error) {
				if documentID != "doc-story-1" {
					t.Fatalf("unexpected document id %q", documentID)
				}
				return "# Outcome\n\nExisting draft", nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "ensure_story_plan_doc", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, marker := range []string{`"document_id":"doc-story-1"`, `"has_draft_content":true`} {
		if !strings.Contains(output, marker) {
			t.Fatalf("expected output to contain %q, got %s", marker, output)
		}
	}
}

func TestCreateStoryBatchToolAcceptsProposedStoriesAlias(t *testing.T) {
	registry := NewToolRegistry(nil)
	versionID := "ver-1"
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		TargetType:  "epic",
		TargetID:    "epic-1",
		AgentID:     "agent-1",
		Epic: &model.PMEpic{
			ID:                    "epic-1",
			ApprovedSpecVersionID: &versionID,
		},
		AllowedTools: map[string]bool{
			"create_story_batch": true,
		},
		Services: &ServiceBridge{
			CreateTaskBatch: func(ctx context.Context, workspaceID, epicID, actorID string, stories []model.ProposedTask) (CreateTaskBatchResult, error) {
				if workspaceID != "ws-1" || epicID != "epic-1" || actorID != "agent-1" {
					t.Fatalf("unexpected tool context: workspace=%q epic=%q actor=%q", workspaceID, epicID, actorID)
				}
				if len(stories) != 1 || stories[0].Name != "Story A" {
					t.Fatalf("unexpected stories payload %#v", stories)
				}
				return CreateTaskBatchResult{
					Tasks: []CreateTaskBatchTaskResult{{Ref: "story_1", TaskID: "story-db-1", Name: "Story A"}},
				}, nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "create_story_batch", json.RawMessage(`{
		"proposed_stories": [
			{"ref":"story_1","name":"Story A","description":"Do A","story_type":"feature","acceptance_criteria":["works"]}
		]
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"story_id":"story-db-1"`) {
		t.Fatalf("expected created story batch result, got %s", output)
	}
}

func TestCreateStoryBatchToolReturnsRepairOrientedParseError(t *testing.T) {
	registry := NewToolRegistry(nil)
	versionID := "ver-1"
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		TargetType:  "epic",
		TargetID:    "epic-1",
		AgentID:     "agent-1",
		Epic: &model.PMEpic{
			ID:                    "epic-1",
			ApprovedSpecVersionID: &versionID,
		},
		AllowedTools: map[string]bool{
			"create_story_batch": true,
		},
		Services: &ServiceBridge{
			CreateTaskBatch: func(ctx context.Context, workspaceID, epicID, actorID string, stories []model.ProposedTask) (CreateTaskBatchResult, error) {
				return CreateTaskBatchResult{}, nil
			},
		},
	}

	_, err := registry.ExecuteAllowed(ctx, "create_story_batch", json.RawMessage(`{"stories":`))
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "create_story_batch input must be valid JSON") {
		t.Fatalf("expected repair-oriented parse error, got %v", err)
	}
}

func TestCreateStoryBatchToolReturnsRepairOrientedStoryValidationError(t *testing.T) {
	registry := NewToolRegistry(nil)
	versionID := "ver-1"
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		TargetType:  "epic",
		TargetID:    "epic-1",
		AgentID:     "agent-1",
		Epic: &model.PMEpic{
			ID:                    "epic-1",
			ApprovedSpecVersionID: &versionID,
		},
		AllowedTools: map[string]bool{
			"create_story_batch": true,
		},
		Services: &ServiceBridge{
			CreateTaskBatch: func(ctx context.Context, workspaceID, epicID, actorID string, stories []model.ProposedTask) (CreateTaskBatchResult, error) {
				return CreateTaskBatchResult{}, nil
			},
		},
	}

	_, err := registry.ExecuteAllowed(ctx, "create_story_batch", json.RawMessage(`{
		"stories": [
			{"description":"Do A","story_type":"feature","acceptance_criteria":["works"]}
		]
	}`))
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), `create_story_batch stories are invalid: story 1 is missing name; use field "name"`) {
		t.Fatalf("expected repair-oriented validation error, got %v", err)
	}
}

func TestCreateStoryBatchToolUsesInternalCommandExecutorWhenAvailable(t *testing.T) {
	registry := NewToolRegistry(nil)
	versionID := "ver-1"
	called := false
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		TargetType:  "epic",
		TargetID:    "epic-1",
		AgentID:     "agent-1",
		RunID:       "run-1",
		Epic: &model.PMEpic{
			ID:                    "epic-1",
			ApprovedSpecVersionID: &versionID,
		},
		AllowedTools: map[string]bool{
			"create_story_batch": true,
		},
		Services: &ServiceBridge{
			ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
				called = true
				if name != "pm.create_story_batch" {
					t.Fatalf("unexpected command name %q", name)
				}
				if meta.WorkspaceID != "ws-1" || meta.TargetType != "epic" || meta.TargetID != "epic-1" || meta.AgentID != "agent-1" || meta.RunID != "run-1" {
					t.Fatalf("unexpected command meta %#v", meta)
				}
				return json.RawMessage(`{"stories":[{"ref":"story_1","story_id":"story-db-1","name":"Story A"}]}`), nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "create_story_batch", json.RawMessage(`{
		"stories": [
			{"ref":"story_1","name":"Story A","description":"Do A","story_type":"feature","acceptance_criteria":["works"]}
		]
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !called {
		t.Fatal("expected internal command executor to be used")
	}
	if output != `{"stories":[{"ref":"story_1","story_id":"story-db-1","name":"Story A"}]}` {
		t.Fatalf("unexpected output %q", output)
	}
}
