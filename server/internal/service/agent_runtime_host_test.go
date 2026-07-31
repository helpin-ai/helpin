package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func TestAgentRuntimeHostExecuteCommandUsesInternalCommandService(t *testing.T) {
	commandService := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	var gotMeta model.InternalCommandContext
	var gotInput json.RawMessage
	commandService.register(InternalCommandDefinition{
		Name:                 "test.echo",
		SupportedTargetTypes: []string{"task"},
		Execute: func(_ context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			gotMeta = meta
			gotInput = append(json.RawMessage(nil), input...)
			return json.RawMessage(`{"ok":true}`), nil
		},
	})
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, commandService, nil)

	resp, err := host.ExecuteCommand(context.Background(), agentruntime.CommandExecutionRequest{
		Meta: agentruntime.CommandExecutionContext{
			AppID:           "helpin",
			RunID:           "run-runtime-1",
			AgentID:         "agent-1",
			ExternalActorID: "user-1",
			WorkspaceID:     "workspace-1",
			Target:          agentruntime.TargetRef{Type: "task", ID: "task-1"},
		},
		CommandName: "test.echo",
		Input:       json.RawMessage(`{"message":"hello"}`),
	})
	if err != nil {
		t.Fatalf("ExecuteCommand returned error: %v", err)
	}
	if resp == nil || string(resp.Output) != `{"ok":true}` || resp.Error != "" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if gotMeta.WorkspaceID != "workspace-1" || gotMeta.RunID != "run-runtime-1" || gotMeta.AgentID != "agent-1" || gotMeta.ActorID != "user-1" || gotMeta.TargetType != "task" || gotMeta.TargetID != "task-1" {
		t.Fatalf("unexpected command meta: %#v", gotMeta)
	}
	if string(gotInput) != `{"message":"hello"}` {
		t.Fatalf("unexpected command input: %s", gotInput)
	}
}

func TestAgentRuntimeHostExecuteCommandReturnsCommandErrorPayload(t *testing.T) {
	commandService := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	host := NewAgentRuntimeHostService("", nil, nil, nil, nil, nil, nil, nil, nil, nil, commandService, nil)

	resp, err := host.ExecuteCommand(context.Background(), agentruntime.CommandExecutionRequest{
		Meta: agentruntime.CommandExecutionContext{
			WorkspaceID: "workspace-1",
			Target:      agentruntime.TargetRef{Type: "task", ID: "task-1"},
		},
		CommandName: "missing.command",
	})
	if err != nil {
		t.Fatalf("ExecuteCommand returned transport error: %v", err)
	}
	if resp == nil || resp.Error == "" {
		t.Fatalf("expected command error payload, got %#v", resp)
	}
}

func TestAgentRuntimeHostExecuteCommandRejectsWrongApp(t *testing.T) {
	commandService := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, commandService, nil)

	_, err := host.ExecuteCommand(context.Background(), agentruntime.CommandExecutionRequest{
		Meta: agentruntime.CommandExecutionContext{
			AppID:       "other-app",
			WorkspaceID: "workspace-1",
			Target:      agentruntime.TargetRef{Type: "task", ID: "task-1"},
		},
		CommandName: "missing.command",
	})
	if err == nil {
		t.Fatal("expected app mismatch error")
	}
	if !errors.Is(err, ErrAgentRuntimeHostForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestAgentRuntimeHostRepositorySpecFallsBackToRuntimeRunMapping(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)
	ensureAgentRuntimeHostRunTable(t, db)
	mustExec(t, db, `INSERT INTO agent_runs (
		id, workspace_id, agent_id, target_type, target_id, runtime_kind, status, external_runtime, external_runtime_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"run-helpin-1", "ws-1", "agent-1", "task", "task-1", "codex", "running", agentRuntimeName, "run-runtime-1")

	host := NewAgentRuntimeHostService(
		"helpin",
		repository.NewAgentRunRepository(db),
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		newGitDeliveryStatusService(db, &fakeGitHubAppClient{}),
	)

	spec, err := host.ResolveRepositorySpec(context.Background(), agentruntime.PrepareWorkspaceRequest{
		AppID:         "helpin",
		RunID:         "run-runtime-1",
		AgentID:       "agent-1",
		RuntimeKind:   "codex",
		Target:        agentruntime.TargetRef{Type: "task", ID: "task-1"},
		WorkspaceMode: agentruntime.WorkspaceModeRepository,
	})
	if err != nil {
		t.Fatalf("ResolveRepositorySpec returned error: %v", err)
	}
	if spec.WorkBranch != "hel-31-fix-merge-status" || spec.Metadata["workspace_id"] != "ws-1" {
		t.Fatalf("unexpected repository spec: %#v", spec)
	}
	if spec.Auth == nil || spec.Auth.Type != "github" || spec.Auth.Token != "github-installation-token" {
		t.Fatalf("repository spec did not include GitHub installation authentication: %#v", spec.Auth)
	}
}

func ensureAgentRuntimeHostRunTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS agent_runs (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		target_type TEXT NOT NULL,
		target_id TEXT NOT NULL,
		runtime_kind TEXT NOT NULL,
		status TEXT NOT NULL,
		external_runtime TEXT,
		external_runtime_id TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create agent_runs table: %v", err)
	}
}
