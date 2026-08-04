package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/authorization"
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

func TestAgentRuntimeHostExecuteCommandPropagatesResolvedActor(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "user-actor-1", "actor@example.com", "Runtime Actor", "hash")
	seedWorkspace(t, db, "workspace-actor-1", "Runtime Workspace", "runtime-workspace", "user-actor-1")
	seedWorkspaceMember(t, db, "member-actor-1", "workspace-actor-1", "user-actor-1", "actor@example.com", "Runtime Actor", "manager")
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, "team-actor-1", "workspace-actor-1", "Runtime Team")
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, "team-member-actor-1", "team-actor-1", "member-actor-1", "owner")

	commandService := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	var gotActor *authorization.Actor
	commandService.register(InternalCommandDefinition{
		Name:                 "test.capture_actor",
		SupportedTargetTypes: []string{"task"},
		Execute: func(ctx context.Context, _ model.InternalCommandContext, _ json.RawMessage) (json.RawMessage, error) {
			gotActor = authorization.GetActor(ctx)
			return json.RawMessage(`{"ok":true}`), nil
		},
	})
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, commandService, nil).
		SetAuthorizationService(authorization.NewAuthzService(db, authorization.NewGORMMemberRepository(db), nil))

	resp, err := host.ExecuteCommand(context.Background(), agentruntime.CommandExecutionRequest{
		Meta: agentruntime.CommandExecutionContext{
			AppID:           "helpin",
			ExternalActorID: "user-actor-1",
			WorkspaceID:     "workspace-actor-1",
			Target:          agentruntime.TargetRef{Type: "task", ID: "task-actor-1"},
		},
		CommandName: "test.capture_actor",
	})
	if err != nil {
		t.Fatalf("ExecuteCommand returned error: %v", err)
	}
	if resp == nil || resp.Error != "" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if gotActor == nil {
		t.Fatal("expected resolved actor in command execution context")
	}
	if gotActor.Role != "manager" {
		t.Fatalf("expected manager workspace role, got %q", gotActor.Role)
	}
	if len(gotActor.TeamMemberships) != 1 || gotActor.TeamMemberships[0].TeamID != "team-actor-1" || gotActor.TeamMemberships[0].Role != "owner" {
		t.Fatalf("expected owner team role to survive actor propagation, got %#v", gotActor.TeamMemberships)
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

func TestAgentRuntimeHostResolveSupportConversationBypassesMailboxMembership(t *testing.T) {
	db := newTestDB(t)
	mustExec(t, db, `INSERT INTO workspaces (
		id, name, slug, owner_id, description, website_url, timezone, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"ws-1", "Usermaven", "usermaven", "user-1", "Product and website analytics for SaaS teams.", "https://usermaven.com", "UTC")
	mustExec(t, db, `INSERT INTO support_mailboxes (
		id, workspace_id, name, handle, visibility_mode, created_by_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"mailbox-1", "ws-1", "Support", "support", "members_only", "user-1")
	mustExec(t, db, `INSERT INTO support_conversations (
		id, workspace_id, mailbox_id, display_id, subject, status, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"conversation-1", "ws-1", "mailbox-1", 1, "Widget installation help", "open")

	host := NewAgentRuntimeHostService(
		"helpin",
		nil, repository.NewWorkspaceRepository(db), nil, nil,
		repository.NewSupportConversationRepository(db),
		nil, nil, nil, nil, nil, nil,
	)

	resolved, err := host.ResolveTargetContext(context.Background(), agentruntime.TargetContextRequest{
		AppID: "helpin",
		Target: agentruntime.TargetRef{
			Type: "support_conversation",
			ID:   "conversation-1",
		},
		Metadata: map[string]interface{}{"workspace_id": "ws-1"},
	})
	if err != nil {
		t.Fatalf("ResolveTargetContext returned error: %v", err)
	}
	if resolved == nil || resolved.Target.Type != "support_conversation" || resolved.Target.ID != "conversation-1" {
		t.Fatalf("unexpected support target context: %#v", resolved)
	}
	for _, expected := range []string{
		"Support conversation: Widget installation help for Usermaven",
		"'your plans' to Usermaven",
		"Product website: https://usermaven.com",
		"Workspace summary: Product and website analytics for SaaS teams.",
	} {
		if !strings.Contains(resolved.Summary, expected) {
			t.Fatalf("support target summary does not contain %q: %q", expected, resolved.Summary)
		}
	}
	workspace, ok := resolved.Data["workspace"].(map[string]interface{})
	if !ok || workspace["name"] != "Usermaven" || workspace["description"] != "Product and website analytics for SaaS teams." || workspace["website_url"] != "https://usermaven.com" {
		t.Fatalf("support target lacks workspace product context: %#v", resolved.Data)
	}
	productContext, ok := resolved.Data["product_context"].(map[string]interface{})
	if !ok || productContext["name"] != "Usermaven" || productContext["resolve_generic_product_references"] != true {
		t.Fatalf("support target lacks product resolution policy: %#v", resolved.Data)
	}
	if resolved.Summary == "Support conversation: Widget installation help" {
		t.Fatalf("unexpected support target summary: %q", resolved.Summary)
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
