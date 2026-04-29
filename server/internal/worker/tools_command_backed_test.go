package worker

import (
	"context"
	"encoding/json"
	"strings"
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

func TestEnrichCRMContactToolUsesInternalCommandExecutorWhenAvailable(t *testing.T) {
	registry := NewToolRegistry(nil)
	called := false
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		TargetType:  "crm_contact",
		TargetID:    "contact-1",
		AgentID:     "agent-1",
		RunID:       "run-1",
		AllowedTools: map[string]bool{
			"enrich_crm_contact": true,
		},
		Services: &ServiceBridge{
			ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
				called = true
				if name != "crm.enrich_contact" {
					t.Fatalf("unexpected command name %q", name)
				}
				if meta.WorkspaceID != "ws-1" || meta.TargetType != "crm_contact" || meta.TargetID != "contact-1" || meta.AgentID != "agent-1" || meta.RunID != "run-1" {
					t.Fatalf("unexpected command meta %#v", meta)
				}
				return json.RawMessage(`{"status":"applied","object_type":"contact","object_id":"contact-1","applied":[],"skipped":[]}`), nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "enrich_crm_contact", json.RawMessage(`{"contact_id":"contact-1","fields":[{"field":"job_title","value":"VP Sales","source_url":"https://example.com","evidence":"Source lists title.","confidence":0.9}]}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !called {
		t.Fatal("expected internal command executor to be used")
	}
	if !strings.Contains(output, `"object_id":"contact-1"`) {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestEnrichCRMCompanyToolUsesInternalCommandExecutorWhenAvailable(t *testing.T) {
	registry := NewToolRegistry(nil)
	called := false
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		TargetType:  "crm_contact",
		TargetID:    "contact-1",
		AgentID:     "agent-1",
		RunID:       "run-1",
		AllowedTools: map[string]bool{
			"enrich_crm_company": true,
		},
		Services: &ServiceBridge{
			ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
				called = true
				if name != "crm.enrich_company" {
					t.Fatalf("unexpected command name %q", name)
				}
				if meta.WorkspaceID != "ws-1" || meta.TargetType != "crm_company" || meta.TargetID != "company-1" || meta.AgentID != "agent-1" || meta.RunID != "run-1" {
					t.Fatalf("unexpected command meta %#v", meta)
				}
				return json.RawMessage(`{"status":"applied","object_type":"company","object_id":"company-1","applied":[],"skipped":[]}`), nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "enrich_crm_company", json.RawMessage(`{"company_id":"company-1","fields":[{"field":"industry","value":"Product Analytics","source_url":"https://example.com","evidence":"Source lists industry.","confidence":0.9}]}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !called {
		t.Fatal("expected internal command executor to be used")
	}
	if !strings.Contains(output, `"object_id":"company-1"`) {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestEnsureTaskLabelToolUsesInternalCommandExecutorWhenAvailable(t *testing.T) {
	registry := NewToolRegistry(nil)
	called := false
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		RunID:       "run-1",
		AllowedTools: map[string]bool{
			"ensure_task_label": true,
		},
		Services: &ServiceBridge{
			ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
				called = true
				if name != "pm.ensure_label" {
					t.Fatalf("unexpected command name %q", name)
				}
				if meta.WorkspaceID != "ws-1" || meta.TargetType != "workspace" || meta.TargetID != "ws-1" {
					t.Fatalf("unexpected command meta %#v", meta)
				}
				var payload struct {
					Name string `json:"name"`
				}
				if err := json.Unmarshal(input, &payload); err != nil {
					t.Fatalf("unmarshal command input: %v", err)
				}
				if payload.Name != "security" {
					t.Fatalf("unexpected command input %s", string(input))
				}
				return json.RawMessage(`{"label_id":"label-1","name":"security","created":true}`), nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "ensure_task_label", json.RawMessage(`{"name":"security"}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !called {
		t.Fatal("expected internal command executor to be used")
	}
	if output != `{"label_id":"label-1","name":"security","created":true}` {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestListTasksToolUsesInternalCommandExecutorWhenAvailable(t *testing.T) {
	registry := NewToolRegistry(nil)
	called := false
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		RunID:       "run-1",
		AllowedTools: map[string]bool{
			"list_tasks": true,
		},
		Services: &ServiceBridge{
			ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
				called = true
				if name != "pm.list_tasks" {
					t.Fatalf("unexpected command name %q", name)
				}
				var payload struct {
					LabelID         string `json:"label_id"`
					OpenOnly        bool   `json:"open_only"`
					IncludeComments bool   `json:"include_comments"`
				}
				if err := json.Unmarshal(input, &payload); err != nil {
					t.Fatalf("unmarshal command input: %v", err)
				}
				if payload.LabelID != "label-1" || !payload.OpenOnly || !payload.IncludeComments {
					t.Fatalf("unexpected command input %s", string(input))
				}
				return json.RawMessage(`{"tasks":[{"task_id":"task-1","name":"Fix secret"}],"total":1,"limit":100}`), nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "list_tasks", json.RawMessage(`{"label_id":"label-1","open_only":true,"include_comments":true,"limit":100}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !called {
		t.Fatal("expected internal command executor to be used")
	}
	if output != `{"tasks":[{"task_id":"task-1","name":"Fix secret"}],"total":1,"limit":100}` {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestAddTaskCommentToolUsesExplicitTaskIDWithInternalCommandExecutor(t *testing.T) {
	registry := NewToolRegistry(nil)
	called := false
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
		RunID:       "run-1",
		AllowedTools: map[string]bool{
			"add_task_comment": true,
		},
		Services: &ServiceBridge{
			ExecuteInternalCommand: func(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
				called = true
				if name != "pm.add_task_comment" {
					t.Fatalf("unexpected command name %q", name)
				}
				if meta.TargetType != "task" || meta.TargetID != "task-1" {
					t.Fatalf("unexpected command meta %#v", meta)
				}
				var payload struct {
					TaskID  string `json:"task_id"`
					Content string `json:"content"`
				}
				if err := json.Unmarshal(input, &payload); err != nil {
					t.Fatalf("unmarshal command input: %v", err)
				}
				if payload.TaskID != "task-1" || payload.Content != "new evidence" {
					t.Fatalf("unexpected command input %s", string(input))
				}
				return json.RawMessage(`{"task_id":"task-1","comment_id":"comment-1"}`), nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "add_task_comment", json.RawMessage(`{"task_id":"task-1","content":" new evidence "}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !called {
		t.Fatal("expected internal command executor to be used")
	}
	if output != `{"task_id":"task-1","comment_id":"comment-1"}` {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestExecuteAllowedUsesCanonicalScannerToolAlias(t *testing.T) {
	registry := NewToolRegistry(nil)
	previous := securityScannerCommandRunner
	t.Cleanup(func() { securityScannerCommandRunner = previous })
	securityScannerCommandRunner = func(ctx *ExecutionContext, program string, args []string, env []string) (string, string, error) {
		if program != "semgrep" {
			t.Fatalf("expected semgrep command, got %q", program)
		}
		return `{"results":[]}`, "", nil
	}

	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkDir:     t.TempDir(),
		WorkspaceID: "ws-1",
		Agent:       &model.Agent{Name: "Sentinel"},
		AllowedTools: map[string]bool{
			ToolScanSemgrep: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "run_semgrep", json.RawMessage(`{"scan_paths":["."]}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"scanner":"semgrep"`) {
		t.Fatalf("expected semgrep output, got %s", output)
	}
}
