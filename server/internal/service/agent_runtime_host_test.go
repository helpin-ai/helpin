package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

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

func TestAgentRuntimeHostExecuteCommandPropagatesSeparateAuditActor(t *testing.T) {
	commandService := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	var gotMeta model.InternalCommandContext
	commandService.register(InternalCommandDefinition{
		Name:                 "test.audit_actor",
		SupportedTargetTypes: []string{"workspace"},
		Execute: func(_ context.Context, meta model.InternalCommandContext, _ json.RawMessage) (json.RawMessage, error) {
			gotMeta = meta
			return json.RawMessage(`{"ok":true}`), nil
		},
	})
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, commandService, nil)

	resp, err := host.ExecuteCommand(context.Background(), agentruntime.CommandExecutionRequest{
		Meta: agentruntime.CommandExecutionContext{
			AppID:            "helpin",
			WorkspaceID:      "workspace-audit",
			AgentID:          "agent-audit",
			TargetType:       "workspace",
			TargetID:         "workspace-audit",
			RunInputMetadata: map[string]interface{}{"audit_actor_id": "user-audit"},
		},
		CommandName: "test.audit_actor",
		Input:       json.RawMessage(`{}`),
	})
	if err != nil || resp.Error != "" {
		t.Fatalf("ExecuteCommand error=%v response=%#v", err, resp)
	}
	if gotMeta.ActorID != "" || gotMeta.AuditActorID != "user-audit" || gotMeta.AgentID != "agent-audit" {
		t.Fatalf("command metadata = %#v", gotMeta)
	}
}

func TestAgentRuntimeHostExecuteCommandResolvesAgentTeamScope(t *testing.T) {
	tests := []struct {
		name        string
		agentID     string
		teamIDs     []string
		wantTeamIDs []string
		actorID     string
	}{
		{name: "team scoped human run", agentID: "agent-runtime-team", teamIDs: []string{"team-b", "team-a"}, wantTeamIDs: []string{"team-a", "team-b"}, actorID: "human-runtime"},
		{name: "workspace scoped", agentID: "agent-runtime-workspace", wantTeamIDs: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupAgentScopeTestDB(t)
			seedAgentScopeAgent(t, db, model.Agent{
				ID: tt.agentID, WorkspaceID: "workspace-runtime-scope", Name: tt.name, Status: "idle", RuntimeKind: "native_sdk",
				Skills: model.AgentSkillRefs{}, ExecutionConfig: model.JSONBlob(`{}`), AllowedTools: json.RawMessage(`[]`),
				AllowedCommands: json.RawMessage(`[]`), AllowedTargets: json.RawMessage(`[]`), ApprovalMode: "always",
				DefaultInvocationMode: "interactive", TeamIDs: tt.teamIDs,
			})

			commandService := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
			var gotMeta model.InternalCommandContext
			commandService.register(InternalCommandDefinition{
				Name:                 "test.agent_scope",
				SupportedTargetTypes: []string{"workspace"},
				Execute: func(_ context.Context, meta model.InternalCommandContext, _ json.RawMessage) (json.RawMessage, error) {
					gotMeta = meta
					return json.RawMessage(`{"ok":true}`), nil
				},
			})
			host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, commandService, nil).
				SetAgentRepository(repository.NewAgentRepository(db))

			resp, err := host.ExecuteCommand(context.Background(), agentruntime.CommandExecutionRequest{
				Meta: agentruntime.CommandExecutionContext{
					AppID: "helpin", WorkspaceID: "workspace-runtime-scope", AgentID: tt.agentID, ExternalActorID: tt.actorID,
					Target: agentruntime.TargetRef{Type: "workspace", ID: "workspace-runtime-scope"},
				},
				CommandName: "test.agent_scope",
			})
			if err != nil || resp == nil || resp.Error != "" {
				t.Fatalf("ExecuteCommand error=%v response=%#v", err, resp)
			}
			if !gotMeta.AgentScopeResolved {
				t.Fatalf("agent scope unresolved: %#v", gotMeta)
			}
			if !slices.Equal(gotMeta.AgentTeamIDs, tt.wantTeamIDs) {
				t.Fatalf("agent team IDs = %#v, want %#v", gotMeta.AgentTeamIDs, tt.wantTeamIDs)
			}
			if gotMeta.ActorID != tt.actorID {
				t.Fatalf("actor ID = %q, want %q", gotMeta.ActorID, tt.actorID)
			}
			if gotMeta.ActorRole != "" || len(gotMeta.ActorTeamIDs) != 0 {
				t.Fatalf("agent scope fabricated human authorization: %#v", gotMeta)
			}
		})
	}
}

func TestAgentRuntimeHostExecuteCommandRejectsUnresolvedAgentScope(t *testing.T) {
	db := setupAgentScopeTestDB(t)
	called := false
	commandService := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	commandService.register(InternalCommandDefinition{
		Name:                 "test.requires_agent_scope",
		SupportedTargetTypes: []string{"workspace"},
		Execute: func(_ context.Context, _ model.InternalCommandContext, _ json.RawMessage) (json.RawMessage, error) {
			called = true
			return json.RawMessage(`{"ok":true}`), nil
		},
	})
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, commandService, nil).
		SetAgentRepository(repository.NewAgentRepository(db))
	resp, err := host.ExecuteCommand(context.Background(), agentruntime.CommandExecutionRequest{
		Meta: agentruntime.CommandExecutionContext{
			AppID: "helpin", WorkspaceID: "workspace-runtime-scope",
			Target: agentruntime.TargetRef{Type: "workspace", ID: "workspace-runtime-scope"},
		},
		CommandName: "test.requires_agent_scope",
	})
	if err != nil {
		t.Fatalf("ExecuteCommand transport error: %v", err)
	}
	if resp == nil || !strings.Contains(resp.Error, "agent_id is required") {
		t.Fatalf("response = %#v, want missing agent scope error", resp)
	}
	if called {
		t.Fatal("command executed without resolved agent scope")
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

func TestAgentRuntimeHostResolveSupportCoverageGapTarget(t *testing.T) {
	_, coverage, db := setupCoverageTestEnv(t)
	if err := db.Exec(`CREATE TABLE workspaces (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, slug TEXT NOT NULL, workspace_key TEXT,
		owner_id TEXT NOT NULL, organization_id TEXT, description TEXT,
		company_product_context TEXT, website_url TEXT, logo_url TEXT,
		timezone TEXT NOT NULL DEFAULT 'UTC', created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create workspace target table: %v", err)
	}
	now := time.Now().UTC()
	workspace := &model.Workspace{ID: "ws-gap", Name: "Acme", Slug: "acme", OwnerID: "user-1"}
	if err := db.Create(workspace).Error; err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	gap := &model.SupportCoverageGap{ID: "gap-1", WorkspaceID: workspace.ID, Title: "Password reset docs are missing"}
	if err := db.Exec(`INSERT INTO support_coverage_gaps (
		id, workspace_id, dedupe_key, title, status, evidence_count, confidence,
		metadata, first_seen_at, last_seen_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		gap.ID, gap.WorkspaceID, "reset-password", gap.Title, model.SupportCoverageGapStatusOpen,
		2, 0.91, []byte(`{}`), now, now, now, now,
	).Error; err != nil {
		t.Fatalf("seed coverage gap: %v", err)
	}
	if err := db.Exec(`INSERT INTO support_gap_evidence (
		id, gap_id, workspace_id, evidence_type, excerpt, metadata, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"evidence-1", gap.ID, workspace.ID, model.SupportEventDocsIssueFeedback,
		"How can I reset my password?", []byte(`{}`), now,
	).Error; err != nil {
		t.Fatalf("seed gap evidence: %v", err)
	}
	host := NewAgentRuntimeHostService(
		"helpin", nil, repository.NewWorkspaceRepository(db), nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
	).SetSupportCoverageService(coverage)

	resolved, err := host.ResolveTargetContext(context.Background(), agentruntime.TargetContextRequest{
		AppID: "helpin", Target: agentruntime.TargetRef{Type: "support_coverage_gap", ID: gap.ID},
		Metadata: map[string]interface{}{"workspace_id": workspace.ID},
	})
	if err != nil {
		t.Fatalf("ResolveTargetContext returned error: %v", err)
	}
	if resolved.Target.Display == nil || resolved.Target.Display.Title != gap.Title {
		t.Fatalf("unexpected gap target display: %#v", resolved.Target.Display)
	}
	if !strings.Contains(resolved.Summary, "How can I reset my password?") || !strings.Contains(resolved.Summary, "evidence_count=2") {
		t.Fatalf("gap target summary lacks evidence: %q", resolved.Summary)
	}
	gapData, ok := resolved.Data["support_coverage_gap"].(map[string]interface{})
	if !ok || gapData["id"] != gap.ID || gapData["title"] != gap.Title {
		t.Fatalf("unexpected typed gap context: %#v", resolved.Data)
	}
}

func TestAgentRuntimeHostResolveSprintTarget(t *testing.T) {
	sprintService, db, workspaceID := newSprintTestEnvWithDB(t)
	seedPMSprintCommandTeam(t, db, workspaceID, "team-runtime-sprint")
	teamID := "team-runtime-sprint"
	color := "#336699"
	if err := db.Create(&model.PMLabel{ID: "label-runtime-sprint", WorkspaceID: workspaceID, Name: "Runtime", Color: &color}).Error; err != nil {
		t.Fatalf("seed label: %v", err)
	}
	description := "Runtime sprint context"
	sprint, err := sprintService.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: workspaceID,
		Name:        "Runtime Sprint",
		Description: &description,
		StartDate:   commandMustDate(t, "2026-09-01"),
		EndDate:     commandMustDate(t, "2026-09-15"),
		TeamID:      &teamID,
		LabelIDs:    []string{"label-runtime-sprint"},
	}, "actor-1")
	if err != nil {
		t.Fatalf("create sprint: %v", err)
	}

	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetPMSprintService(sprintService)
	resolved, err := host.ResolveTargetContext(context.Background(), agentruntime.TargetContextRequest{
		AppID:    "helpin",
		Target:   agentruntime.TargetRef{Type: "sprint", ID: sprint.Sprint.ID},
		Metadata: map[string]interface{}{"workspace_id": workspaceID},
	})
	if err != nil {
		t.Fatalf("ResolveTargetContext: %v", err)
	}
	if resolved.Summary != "Sprint: Runtime Sprint" || resolved.Target.Display == nil || resolved.Target.Display.Title != "Runtime Sprint" {
		t.Fatalf("unexpected sprint display: %#v", resolved)
	}
	if resolved.Data["start_date"] != "2026-09-01" || resolved.Data["end_date"] != "2026-09-15" || resolved.Data["team_id"] != teamID {
		t.Fatalf("unexpected sprint context dates/team: %#v", resolved.Data)
	}
	labels, ok := resolved.Data["labels"].([]map[string]interface{})
	if !ok || len(labels) != 1 || labels[0]["name"] != "Runtime" {
		t.Fatalf("unexpected sprint labels: %#v", resolved.Data["labels"])
	}

	_, err = host.ResolveTargetContext(context.Background(), agentruntime.TargetContextRequest{
		AppID:    "helpin",
		Target:   agentruntime.TargetRef{Type: "sprint", ID: sprint.Sprint.ID},
		Metadata: map[string]interface{}{"workspace_id": "workspace-other"},
	})
	if !errors.Is(err, ErrAgentRuntimeHostForbidden) {
		t.Fatalf("cross-workspace sprint error = %v, want forbidden", err)
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

func TestAgentRuntimeHostRepositorySpecRestoresDynamicCheckoutTargetAfterResume(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)
	ensureAgentRuntimeHostRunTable(t, db)
	mustExec(t, db, `INSERT INTO agent_runs (
		id, workspace_id, agent_id, target_type, target_id, runtime_kind, status, external_runtime, external_runtime_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"run-helpin-dock", "ws-1", "agent-ask", "workspace", "ws-1", "native_sdk", "running", agentRuntimeName, "run-runtime-dock")

	host := NewAgentRuntimeHostService(
		"helpin",
		repository.NewAgentRunRepository(db),
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		newGitDeliveryStatusService(db, &fakeGitHubAppClient{}),
	)

	spec, err := host.ResolveRepositorySpec(context.Background(), agentruntime.PrepareWorkspaceRequest{
		AppID:         "helpin",
		RunID:         "run-runtime-dock",
		AgentID:       "agent-ask",
		RuntimeKind:   "native_sdk",
		Target:        agentruntime.TargetRef{Type: "workspace", ID: "ws-1"},
		WorkspaceMode: agentruntime.WorkspaceModeRepository,
		Metadata: map[string]interface{}{
			"workspace_id":   "ws-1",
			"repository_id":  "repo-1",
			"repo_full_name": "acme/api",
			"base_branch":    "release",
			"work_branch":    "agent/dock-read",
		},
	})
	if err != nil {
		t.Fatalf("ResolveRepositorySpec returned error after dynamic checkout resume: %v", err)
	}
	if spec.Metadata["repository_id"] != "repo-1" || spec.Metadata["repo_full_name"] != "acme/api" {
		t.Fatalf("dynamic checkout repository identity was not restored: %#v", spec.Metadata)
	}
	if spec.BaseBranch != "release" || spec.WorkBranch != "agent/dock-read" {
		t.Fatalf("dynamic checkout branches were not preserved: %#v", spec)
	}
}

func TestAgentRuntimeHostRepositorySpecRestoresCoverageCheckoutAcrossRuntimeKinds(t *testing.T) {
	for _, runtimeKind := range []string{"codex", "native_sdk"} {
		t.Run(runtimeKind, func(t *testing.T) {
			db := newTestDB(t)
			seedGitDeliveryStatusFixture(t, db)
			ensureAgentRuntimeHostRunTable(t, db)
			runtimeRunID := "run-runtime-gap-" + runtimeKind
			mustExec(t, db, `INSERT INTO agent_runs (
				id, workspace_id, agent_id, target_type, target_id, runtime_kind, status, external_runtime, external_runtime_id, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
				"run-helpin-gap-"+runtimeKind, "ws-1", "agent-quill", "support_coverage_gap", "gap-1", runtimeKind, "running", agentRuntimeName, runtimeRunID)

			host := NewAgentRuntimeHostService(
				"helpin",
				repository.NewAgentRunRepository(db),
				nil, nil, nil, nil, nil, nil, nil, nil, nil,
				newGitDeliveryStatusService(db, &fakeGitHubAppClient{}),
			)

			spec, err := host.ResolveRepositorySpec(context.Background(), agentruntime.PrepareWorkspaceRequest{
				AppID:         "helpin",
				RunID:         runtimeRunID,
				AgentID:       "agent-quill",
				RuntimeKind:   runtimeKind,
				Target:        agentruntime.TargetRef{Type: "support_coverage_gap", ID: "gap-1"},
				WorkspaceMode: agentruntime.WorkspaceModeRepository,
				Metadata: map[string]interface{}{
					"workspace_id":   "ws-1",
					"repository_id":  "repo-1",
					"repo_full_name": "acme/api",
					"base_branch":    "release",
					"work_branch":    "agent/quill-review",
				},
			})
			if err != nil {
				t.Fatalf("ResolveRepositorySpec returned error after coverage-gap resume: %v", err)
			}
			if spec.Metadata["repository_id"] != "repo-1" || spec.Metadata["repo_full_name"] != "acme/api" {
				t.Fatalf("coverage checkout repository identity was not restored: %#v", spec.Metadata)
			}
			if spec.BaseBranch != "release" || spec.WorkBranch != "agent/quill-review" {
				t.Fatalf("coverage checkout branches were not preserved: %#v", spec)
			}
		})
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
