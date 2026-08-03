package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestAgentRequiresRepositoryWorkspace(t *testing.T) {
	tests := []struct {
		name  string
		agent *model.Agent
		want  bool
	}{
		{name: "nil agent", agent: nil, want: false},
		{name: "code builder preset", agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetCodeBuilder}, want: true},
		{name: "review agent preset", agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetReviewAgent}, want: true},
		{name: "marketer preset", agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetMarketer}, want: false},
		{name: "support preset", agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetSupportAgent}, want: false},
		{
			// GetRuntimeProfile falls back to code_builder for unknown names;
			// custom agents must never inherit that fallback's RequiresRepo.
			name:  "custom agent without preset",
			agent: &model.Agent{Name: "Custom Runner", RuntimeKind: "native_sdk"},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := agentRequiresRepositoryWorkspace(tt.agent); got != tt.want {
				t.Errorf("agentRequiresRepositoryWorkspace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithRepositoryWorkspaceExecutionConfig(t *testing.T) {
	decode := func(t *testing.T, payload json.RawMessage) map[string]interface{} {
		t.Helper()
		values := map[string]interface{}{}
		if err := json.Unmarshal(payload, &values); err != nil {
			t.Fatalf("decode execution config: %v", err)
		}
		return values
	}
	workspaceMode := func(t *testing.T, values map[string]interface{}) string {
		t.Helper()
		workspaceValue, _ := values["workspace"].(map[string]interface{})
		if workspaceValue == nil {
			t.Fatalf("expected workspace object, got %#v", values)
		}
		mode, _ := workspaceValue["mode"].(string)
		return mode
	}

	t.Run("empty config gets repository mode", func(t *testing.T) {
		values := decode(t, withRepositoryWorkspaceExecutionConfig(nil))
		if workspaceMode(t, values) != "repository" {
			t.Fatalf("expected repository workspace mode, got %#v", values)
		}
	})

	t.Run("existing keys are preserved", func(t *testing.T) {
		values := decode(t, withRepositoryWorkspaceExecutionConfig(json.RawMessage(`{"reasoning_effort":"high","service_tier":"fast"}`)))
		if values["reasoning_effort"] != "high" || values["service_tier"] != "fast" {
			t.Fatalf("expected existing execution config keys preserved, got %#v", values)
		}
		if workspaceMode(t, values) != "repository" {
			t.Fatalf("expected repository workspace mode, got %#v", values)
		}
	})

	t.Run("explicit workspace mode is preserved", func(t *testing.T) {
		values := decode(t, withRepositoryWorkspaceExecutionConfig(json.RawMessage(`{"workspace":{"mode":"host_prepared"}}`)))
		if workspaceMode(t, values) != "host_prepared" {
			t.Fatalf("expected explicit workspace mode preserved, got %#v", values)
		}
	})
}

func TestRuntimeAgentFromHelpinAgentInjectsRepositoryWorkspaceMode(t *testing.T) {
	codeBuilder := &model.Agent{
		ID:              "agent-cb",
		IsSystem:        true,
		Name:            "Forge",
		PresetKey:       model.AgentPresetCodeBuilder,
		RuntimeKind:     "codex",
		ExecutionConfig: []byte(`{"reasoning_effort":"high"}`),
	}
	out := runtimeAgentFromHelpinAgent(codeBuilder, "helpin")
	values := map[string]interface{}{}
	if err := json.Unmarshal(out.ExecutionConfig, &values); err != nil {
		t.Fatalf("decode runtime execution config: %v", err)
	}
	workspaceValue, _ := values["workspace"].(map[string]interface{})
	if workspaceValue == nil || workspaceValue["mode"] != "repository" {
		t.Fatalf("expected workspace.mode=repository for code builder, got %#v", values)
	}
	if workspaceValue["access"] != agentRepositoryAccessReadWrite {
		t.Fatalf("expected code builder read-write workspace access, got %#v", values)
	}
	if values["reasoning_effort"] != "high" {
		t.Fatalf("expected reasoning_effort preserved, got %#v", values)
	}
	if values["preset_key"] != model.AgentPresetCodeBuilder {
		t.Fatalf("expected preset_key propagated, got %#v", values)
	}

	marketer := &model.Agent{
		ID:          "agent-mira",
		IsSystem:    true,
		Name:        "Mira",
		PresetKey:   model.AgentPresetMarketer,
		RuntimeKind: "codex",
	}
	if config := runtimeAgentFromHelpinAgent(marketer, "helpin").ExecutionConfig; len(config) > 0 {
		nonRepoValues := map[string]interface{}{}
		_ = json.Unmarshal(config, &nonRepoValues)
		workspace, _ := nonRepoValues["workspace"].(map[string]interface{})
		if workspace["access"] != agentRepositoryAccessReadOnly {
			t.Fatalf("marketer must get read-only workspace access, got %#v", nonRepoValues)
		}
		if _, ok := workspace["mode"]; ok {
			t.Fatalf("marketer must not get a workspace mode, got %#v", nonRepoValues)
		}
	}
}

func TestRuntimeAgentFromHelpinAgentAlwaysIncludesSupportDeliveryContract(t *testing.T) {
	staleWorkspacePrompt := "You are Echo. Answer customer questions from evidence."
	out := runtimeAgentFromHelpinAgent(&model.Agent{
		ID:           "agent-echo-copy",
		IsSystem:     true,
		PresetKey:    model.AgentPresetSupportAgent,
		RuntimeKind:  "native_sdk",
		SystemPrompt: &staleWorkspacePrompt,
	}, "helpin")
	for _, required := range []string{
		staleWorkspacePrompt,
		"Required live-support delivery contract",
		"send_support_reply",
		"Plain assistant text is never delivered",
	} {
		if !strings.Contains(out.SystemPrompt, required) {
			t.Fatalf("runtime support prompt missing %q:\n%s", required, out.SystemPrompt)
		}
	}
}

func TestRuntimeAgentFromHelpinAgentIncludesImplicitScribeSkills(t *testing.T) {
	qualifiedPrompt := "Publish with `mcp__helpin__publish_task_plan_doc`, then call `mcp__helpin__request_approval`."
	scribe := &model.Agent{
		ID:               "agent-scribe",
		IsSystem:         true,
		Name:             "Scribe",
		PresetKey:        model.AgentPresetTaskPlanner,
		PresetVersionKey: "task_planner_default",
		RuntimeKind:      "codex",
		SystemPrompt:     &qualifiedPrompt,
		// Reproduce a persisted partial selection from before the approval
		// skill became a required Scribe core skill.
		Skills: model.AgentSkillRefs{{Key: "engineering_planner_operating_rules"}},
	}

	out := runtimeAgentFromHelpinAgent(scribe, "helpin")
	got := make(map[string]bool, len(out.Skills))
	for _, ref := range out.Skills {
		got[ref.Key] = true
	}
	// Native-authored skills use runtime-backed logical tools under Codex. The
	// delegated agent must receive the complete Scribe workflow, not only the
	// approval protocol that guards its completion.
	for _, key := range []string{"coding_task_planning", "prd_task_plan_approval", "engineering_planner_operating_rules"} {
		if !got[key] {
			t.Fatalf("expected delegated Scribe skill %q, got %#v", key, out.Skills)
		}
	}
	for _, toolName := range []string{"`publish_task_plan_doc`", "`request_approval`"} {
		if !strings.Contains(out.SystemPrompt, toolName) {
			t.Fatalf("expected delegated Scribe prompt to contain logical tool name %s, got %q", toolName, out.SystemPrompt)
		}
	}
	if strings.Contains(out.SystemPrompt, "mcp__helpin__") {
		t.Fatalf("expected delegated Codex prompt to remove Helpin MCP qualification, got %q", out.SystemPrompt)
	}
}

func TestAgentRepositoryAccessModeMatchesBuiltInAuthority(t *testing.T) {
	tests := []struct {
		preset string
		want   string
	}{
		{preset: model.AgentPresetEpicPlanner, want: agentRepositoryAccessReadOnly},
		{preset: model.AgentPresetTaskPlanner, want: agentRepositoryAccessReadOnly},
		{preset: model.AgentPresetCRMOperator, want: agentRepositoryAccessReadOnly},
		{preset: model.AgentPresetSupportAgent, want: agentRepositoryAccessReadOnly},
		{preset: model.AgentPresetDocumentationAgent, want: agentRepositoryAccessReadOnly},
		{preset: model.AgentPresetMarketer, want: agentRepositoryAccessReadOnly},
		{preset: model.AgentPresetCommandAgent, want: agentRepositoryAccessReadOnly},
		{preset: model.AgentPresetCodeBuilder, want: agentRepositoryAccessReadWrite},
		{preset: model.AgentPresetReviewAgent, want: agentRepositoryAccessReadWrite},
	}
	for _, tt := range tests {
		t.Run(tt.preset, func(t *testing.T) {
			agent := &model.Agent{IsSystem: true, PresetKey: tt.preset}
			if got := agentRepositoryAccessMode(agent); got != tt.want {
				t.Fatalf("agentRepositoryAccessMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAgentRepositoryAccessModeRequiresExplicitWriteToolsForCustomAgent(t *testing.T) {
	readOnly := &model.Agent{AllowedTools: json.RawMessage(`["read_file","run_command","ripgrep"]`)}
	if got := agentRepositoryAccessMode(readOnly); got != agentRepositoryAccessReadOnly {
		t.Fatalf("read-only custom agent access = %q", got)
	}
	writer := &model.Agent{AllowedTools: json.RawMessage(`["read_file","apply_patch"]`)}
	if got := agentRepositoryAccessMode(writer); got != agentRepositoryAccessReadWrite {
		t.Fatalf("writer custom agent access = %q", got)
	}
}

func TestRuntimeAgentFromHelpinAgentRequiresScribePlanDocument(t *testing.T) {
	scribe := &model.Agent{
		ID:          "agent-scribe",
		IsSystem:    true,
		PresetKey:   model.AgentPresetTaskPlanner,
		RuntimeKind: "codex",
	}
	var config map[string]interface{}
	if err := json.Unmarshal(runtimeAgentFromHelpinAgent(scribe, "helpin").ExecutionConfig, &config); err != nil {
		t.Fatalf("decode execution config: %v", err)
	}
	completion, _ := config["completion"].(map[string]interface{})
	required, _ := completion["required_tools"].([]interface{})
	if len(required) != 1 || required[0] != "publish_task_plan_doc" {
		t.Fatalf("unexpected Scribe completion contract %#v", config)
	}
}

func TestRuntimeAgentFromHelpinAgentPropagatesNativeToolBudget(t *testing.T) {
	tests := []struct {
		name  string
		agent *model.Agent
		want  float64
	}{
		{
			name:  "system default",
			agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetMarketer, RuntimeKind: "native_sdk"},
			want:  50,
		},
		{
			name:  "planner",
			agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetEpicPlanner, RuntimeKind: "native_sdk"},
			want:  300,
		},
		{
			name:  "custom agent",
			agent: &model.Agent{IsSystem: false, RuntimeKind: "native_sdk"},
			want:  300,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := runtimeAgentFromHelpinAgent(tt.agent, "helpin")
			var values map[string]interface{}
			if err := json.Unmarshal(out.ExecutionConfig, &values); err != nil {
				t.Fatalf("decode runtime execution config: %v", err)
			}
			if got := values["max_tool_steps"]; got != tt.want {
				t.Fatalf("max_tool_steps = %#v, want %.0f", got, tt.want)
			}
		})
	}
}

// setupCodingDelegationTestDB extends the shared interactive-approval schema
// with the PM, delivery, git, and docs tables the coding launch chain touches.
func setupCodingDelegationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newInteractiveApprovalTestDB(t)
	for _, stmt := range []string{
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL DEFAULT 0,
			workflow_state_id TEXT NOT NULL DEFAULT '',
			workflow_id TEXT NOT NULL DEFAULT '',
			team_id TEXT,
			epic_id TEXT,
			name TEXT NOT NULL DEFAULT '',
			description TEXT,
			task_type TEXT NOT NULL DEFAULT 'feature',
			priority TEXT NOT NULL DEFAULT 'none',
			position INTEGER NOT NULL DEFAULT 0,
			started BOOLEAN NOT NULL DEFAULT 0,
			completed BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			external_id TEXT,
			plan_document_id TEXT,
			slice_type TEXT,
			implementation_brief TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE task_delivery_targets (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			task_id TEXT NOT NULL UNIQUE,
			repository_id TEXT,
			repo_full_name TEXT,
			integration_id TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_state TEXT NOT NULL DEFAULT 'unconfigured',
			target_source TEXT NOT NULL DEFAULT 'manual',
			source_epic_id TEXT,
			active_pr_number INTEGER,
			active_pr_title TEXT,
			active_pr_url TEXT,
			active_pr_status TEXT,
			last_commit_sha TEXT,
			last_run_id TEXT,
			last_synced_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE git_repositories (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			integration_id TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT 'github',
			base_url TEXT,
			external_id TEXT NOT NULL DEFAULT '',
			full_name TEXT NOT NULL,
			default_branch TEXT NOT NULL DEFAULT 'main',
			permissions BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			private BOOLEAN NOT NULL DEFAULT 1,
			archived BOOLEAN NOT NULL DEFAULT 0,
			selected BOOLEAN NOT NULL DEFAULT 1,
			active BOOLEAN NOT NULL DEFAULT 1,
			deleted_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'published',
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_contents (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			document_id TEXT NOT NULL UNIQUE,
			content BLOB,
			content_text TEXT,
			word_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		mustExec(t, db, stmt)
	}
	return db
}

func seedCodingDelegationAgent(t *testing.T, db *gorm.DB, presetKey, invocationMode string, now time.Time) {
	t.Helper()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, provider, execution_config, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-1", true, "Forge", presetKey, "Code Builder", "idle", "codex",
		[]byte("[]"), "manual", "openai", []byte(`{"reasoning_effort":"high"}`), []byte("[]"), []byte("[]"), []byte("[]"), "never",
		1, invocationMode, now, now,
	)
}

func seedCodingDelegationTaskAndDelivery(t *testing.T, db *gorm.DB, now time.Time) {
	t.Helper()
	mustExec(t, db, `INSERT INTO pm_tasks (
		id, workspace_id, display_id, workflow_state_id, name, description, plan_document_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-1", "ws-1", 42, "state-1", "Fix login redirect", "Users bounce back to /login after SSO.", "doc-plan-1", now, now,
	)
	mustExec(t, db, `INSERT INTO git_repositories (
		id, workspace_id, integration_id, full_name, default_branch, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"repo-1", "ws-1", "int-1", "helpin/app", "main", now, now,
	)
	mustExec(t, db, `INSERT INTO task_delivery_targets (
		id, workspace_id, task_id, repository_id, repo_full_name, integration_id, base_branch, working_branch, delivery_state, target_source, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"delivery-1", "ws-1", "task-1", "repo-1", "helpin/app", "int-1", "develop", "HLP-42-fix-login-redirect", "ready", "manual", now, now,
	)
	mustExec(t, db, `INSERT INTO docs_documents (id, workspace_id, title, status) VALUES (?, ?, ?, ?)`,
		"doc-plan-1", "ws-1", "Login redirect fix plan", "published",
	)
	mustExec(t, db, `INSERT INTO docs_contents (id, document_id, content, content_text) VALUES (?, ?, ?, ?)`,
		"content-1", "doc-plan-1", []byte(``), "1. Reproduce redirect loop\n2. Patch session cookie domain",
	)
}

func newCodingDelegationService(t *testing.T, db *gorm.DB, runtimeClient *fakeAgentRuntimeSignalClient) *AgentService {
	t.Helper()
	gitService := &GitService{
		repoRepo:     repository.NewGitRepositoryRepository(db),
		deliveryRepo: repository.NewTaskDeliveryTargetRepository(db),
		taskRepo:     repository.NewPMTaskRepository(db),
	}
	return (&AgentService{
		agentRepo:        repository.NewAgentRepository(db),
		runRepo:          repository.NewAgentRunRepository(db),
		runMessageRepo:   repository.NewAgentRunMessageRepository(db),
		taskRepo:         repository.NewPMTaskRepository(db),
		docsDocumentRepo: repository.NewDocsDocumentRepository(db),
		docsContentRepo:  repository.NewDocsContentRepository(db),
		gitService:       gitService,

		agentRuntimeClient: runtimeClient,
	}).SetAgentRuntimeLaunchEnabled(true)
}

// TestStartTargetRunDelegatesCodeBuilderTaskRunToAgentRuntime proves the
// manual task launch chain reaches createRun's delegation branch with the
// coding-specific launch bridges intact: run branch fields stamped from the
// resolved delivery target, execution-time task context stamped into
// additional_context/instructions, repository workspace mode on the upserted
// runtime agent, and no local Temporal workflow.
func TestStartTargetRunDelegatesCodeBuilderTaskRunToAgentRuntime(t *testing.T) {
	db := setupCodingDelegationTestDB(t)
	now := time.Now().UTC()
	seedCodingDelegationAgent(t, db, model.AgentPresetCodeBuilder, model.InvocationModeAutonomous, now)
	seedCodingDelegationTaskAndDelivery(t, db, now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := newCodingDelegationService(t, db, runtimeClient)

	actorID := "user-1"
	notes := "Prioritize the SSO path."
	run, err := svc.startTargetRunWithOptions(context.Background(), "ws-1", "task", "task-1", model.StartAgentRunRequest{
		AgentID:           "agent-1",
		AdditionalContext: &notes,
	}, &actorID, nil, nil, nil, startTargetRunOptions{})
	if err != nil {
		t.Fatalf("startTargetRunWithOptions returned error: %v", err)
	}

	if len(runtimeClient.upsertAgents) != 1 {
		t.Fatalf("expected one runtime agent upsert, got %d", len(runtimeClient.upsertAgents))
	}
	upserted := runtimeClient.upsertAgents[0]
	execValues := map[string]interface{}{}
	if err := json.Unmarshal(upserted.ExecutionConfig, &execValues); err != nil {
		t.Fatalf("decode upserted execution config: %v", err)
	}
	workspaceValue, _ := execValues["workspace"].(map[string]interface{})
	if workspaceValue == nil || workspaceValue["mode"] != "repository" {
		t.Fatalf("expected repository workspace mode on runtime agent, got %#v", execValues)
	}

	if len(runtimeClient.startRunCalls) != 1 {
		t.Fatalf("expected one runtime start call, got %d", len(runtimeClient.startRunCalls))
	}
	start := runtimeClient.startRunCalls[0]
	if start.HostRunID != run.ID || start.Target.Type != "task" || start.Target.ID != "task-1" {
		t.Fatalf("unexpected runtime start target: %#v", start)
	}
	if start.Metadata["workspace_id"] != "ws-1" || start.Target.Metadata["workspace_id"] != "ws-1" {
		t.Fatalf("task target context requires workspace_id metadata, got metadata=%#v target=%#v", start.Metadata, start.Target.Metadata)
	}
	for key, want := range map[string]interface{}{
		"workspace_mode":     "repository",
		"repository_id":      "repo-1",
		"repo_full_name":     "helpin/app",
		"base_branch":        "develop",
		"work_branch":        "HLP-42-fix-login-redirect",
		"delivery_target_id": "delivery-1",
	} {
		if start.Metadata[key] != want || start.Target.Metadata[key] != want {
			t.Fatalf("runtime start metadata %s = %v/%v, want %v", key, start.Metadata[key], start.Target.Metadata[key], want)
		}
	}
	for _, expected := range []string{
		"Operator notes:\nPrioritize the SSO path.",
		"Task: **Fix login redirect**",
		"Users bounce back to /login after SSO.",
		"Repository branches: base `develop`, working `HLP-42-fix-login-redirect`.",
		"Canonical task planning document: Login redirect fix plan [doc-plan-1]",
		"Patch session cookie domain",
	} {
		if !strings.Contains(start.Instructions, expected) {
			t.Fatalf("expected instructions to contain %q, got:\n%s", expected, start.Instructions)
		}
	}

	reloaded, err := repository.NewAgentRunRepository(db).GetByID(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if reloaded.RepositoryID == nil || *reloaded.RepositoryID != "repo-1" {
		t.Fatalf("expected repository_id stamped from delivery target, got %#v", reloaded.RepositoryID)
	}
	if reloaded.RepoFullName == nil || *reloaded.RepoFullName != "helpin/app" {
		t.Fatalf("expected repo_full_name stamped, got %#v", reloaded.RepoFullName)
	}
	if reloaded.BaseBranch == nil || *reloaded.BaseBranch != "develop" {
		t.Fatalf("expected base_branch stamped, got %#v", reloaded.BaseBranch)
	}
	if reloaded.WorkingBranch == nil || *reloaded.WorkingBranch != "HLP-42-fix-login-redirect" {
		t.Fatalf("expected working_branch stamped, got %#v", reloaded.WorkingBranch)
	}
	if reloaded.DeliveryTargetID == nil || *reloaded.DeliveryTargetID != "delivery-1" {
		t.Fatalf("expected delivery_target_id stamped, got %#v", reloaded.DeliveryTargetID)
	}
	if reloaded.ExternalRuntime == nil || *reloaded.ExternalRuntime != agentRuntimeName || reloaded.ExternalRuntimeID == nil || *reloaded.ExternalRuntimeID != "run_runtime_1" {
		t.Fatalf("expected delegated runtime mapping, got %v/%v", reloaded.ExternalRuntime, reloaded.ExternalRuntimeID)
	}
	if reloaded.WorkflowID != nil || reloaded.WorkflowRunID != nil {
		t.Fatalf("expected no local Temporal workflow, got %v/%v", reloaded.WorkflowID, reloaded.WorkflowRunID)
	}
}

// TestStartTargetRunDelegatesReviewAgentTaskRunCompletesOnFinish covers the
// review-agent flavor of the task chain: the run is still interactive for
// auth/approval semantics, but it must not enter the runtime chat loop after
// its final assistant response.
func TestStartTargetRunDelegatesReviewAgentTaskRunCompletesOnFinish(t *testing.T) {
	db := setupCodingDelegationTestDB(t)
	now := time.Now().UTC()
	seedCodingDelegationAgent(t, db, model.AgentPresetReviewAgent, model.InvocationModeInteractive, now)
	seedCodingDelegationTaskAndDelivery(t, db, now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := newCodingDelegationService(t, db, runtimeClient)

	actorID := "user-1"
	_, err := svc.startTargetRunWithOptions(context.Background(), "ws-1", "task", "task-1", model.StartAgentRunRequest{
		AgentID: "agent-1",
	}, &actorID, nil, nil, nil, startTargetRunOptions{})
	if err != nil {
		t.Fatalf("startTargetRunWithOptions returned error: %v", err)
	}
	if len(runtimeClient.startRunCalls) != 1 {
		t.Fatalf("expected one runtime start call, got %d", len(runtimeClient.startRunCalls))
	}
	start := runtimeClient.startRunCalls[0]
	if start.Mode != model.InvocationModeInteractive || start.TurnPolicy.Mode != agentRuntimeTurnCompleteOnFinish {
		t.Fatalf("expected interactive complete_on_finish policy, got mode=%q turn=%#v", start.Mode, start.TurnPolicy)
	}
	if !strings.Contains(start.Instructions, "Repository branches: base `develop`, working `HLP-42-fix-login-redirect`.") {
		t.Fatalf("review runs need branch context in instructions (review skill compares work branch against base), got:\n%s", start.Instructions)
	}
}

// TestStartTargetRunDelegatesRepositoryTargetRunToAgentRuntime covers the
// repository-target launch case: repo fields stamped directly from the
// repository row and delegation routed without a delivery target.
func TestStartTargetRunDelegatesRepositoryTargetRunToAgentRuntime(t *testing.T) {
	db := setupCodingDelegationTestDB(t)
	now := time.Now().UTC()
	seedCodingDelegationAgent(t, db, model.AgentPresetCodeBuilder, model.InvocationModeAutonomous, now)
	mustExec(t, db, `INSERT INTO git_repositories (
		id, workspace_id, integration_id, full_name, default_branch, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"repo-1", "ws-1", "int-1", "helpin/app", "main", now, now,
	)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := newCodingDelegationService(t, db, runtimeClient)

	actorID := "user-1"
	run, err := svc.startTargetRunWithOptions(context.Background(), "ws-1", "repository", "repo-1", model.StartAgentRunRequest{
		AgentID: "agent-1",
	}, &actorID, nil, nil, nil, startTargetRunOptions{})
	if err != nil {
		t.Fatalf("startTargetRunWithOptions returned error: %v", err)
	}
	if len(runtimeClient.startRunCalls) != 1 {
		t.Fatalf("expected one runtime start call, got %d", len(runtimeClient.startRunCalls))
	}
	start := runtimeClient.startRunCalls[0]
	if start.Target.Type != "repository" || start.Target.ID != "repo-1" {
		t.Fatalf("unexpected runtime start target: %#v", start)
	}

	reloaded, err := repository.NewAgentRunRepository(db).GetByID(context.Background(), "ws-1", run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if reloaded.RepositoryID == nil || *reloaded.RepositoryID != "repo-1" {
		t.Fatalf("expected repository_id stamped, got %#v", reloaded.RepositoryID)
	}
	if reloaded.RepoFullName == nil || *reloaded.RepoFullName != "helpin/app" {
		t.Fatalf("expected repo_full_name stamped, got %#v", reloaded.RepoFullName)
	}
	if reloaded.BaseBranch == nil || *reloaded.BaseBranch != "main" {
		t.Fatalf("expected base_branch defaulted from repo default branch, got %#v", reloaded.BaseBranch)
	}
	if reloaded.ExternalRuntime == nil || *reloaded.ExternalRuntime != agentRuntimeName {
		t.Fatalf("expected delegated runtime mapping, got %#v", reloaded.ExternalRuntime)
	}
	if reloaded.WorkflowID != nil {
		t.Fatalf("expected no local Temporal workflow, got %v", reloaded.WorkflowID)
	}
}

// TestBuildDelegatedTaskLaunchContextBestEffort exercises the launch-context
// builder directly: optional lookups (plan doc, epic) degrade gracefully and
// operator notes survive.
func TestBuildDelegatedTaskLaunchContextBestEffort(t *testing.T) {
	svc := &AgentService{} // no docs/epic repos wired

	notes := "note"
	description := "Plain description"
	task := &model.PMTask{ID: "task-1", Name: "Do the thing", Description: &description}
	delivery := &model.TaskDeliveryTarget{BaseBranch: strPtr("main")}

	got := svc.buildDelegatedTaskLaunchContext(context.Background(), task, delivery, model.StartAgentRunRequest{AdditionalContext: &notes})
	for _, expected := range []string{
		"Operator notes:\nnote",
		"Task: **Do the thing**",
		"Task description:\nPlain description",
		"Repository base branch: `main`.",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected launch context to contain %q, got:\n%s", expected, got)
		}
	}

	t.Run("request branch overrides win", func(t *testing.T) {
		section := delegatedTaskBranchSection(&model.TaskDeliveryTarget{BaseBranch: strPtr("main"), WorkingBranch: strPtr("old-branch")}, strPtr("develop"), strPtr("new-branch"))
		if section != "Repository branches: base `develop`, working `new-branch`." {
			t.Fatalf("unexpected branch section: %q", section)
		}
	})

	t.Run("nil task returns operator notes only", func(t *testing.T) {
		if got := svc.buildDelegatedTaskLaunchContext(context.Background(), nil, nil, model.StartAgentRunRequest{AdditionalContext: &notes}); got != "note" {
			t.Fatalf("expected bare operator notes, got %q", got)
		}
	})

	t.Run("plan document content is truncated", func(t *testing.T) {
		long := strings.Repeat("x", delegatedTaskPlanDocumentContextLimit+50)
		truncated := truncateDelegatedLaunchText(long, delegatedTaskPlanDocumentContextLimit)
		if len(truncated) >= len(long) || !strings.HasSuffix(truncated, "... (truncated)") {
			t.Fatalf("expected truncation suffix, got len=%d", len(truncated))
		}
	})
}
