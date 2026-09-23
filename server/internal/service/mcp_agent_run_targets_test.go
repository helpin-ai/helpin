package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type mcpAgentRunTargetEnv struct {
	t         *testing.T
	db        *gorm.DB
	service   *MCPService
	principal *model.MCPPrincipal
	actor     *authorization.Actor
}

// setupMCPAgentRunTargetTest wires the public MCP start_agent_run path to a
// real AgentService over SQLite, with the production system agents Forge
// (code builder) and Sub-agent (command agent), one task with no repository
// and one task with a ready delivery target, plus one epic.
func setupMCPAgentRunTargetTest(t *testing.T) *mcpAgentRunTargetEnv {
	t.Helper()
	db := setupCodingDelegationTestDB(t)
	now := time.Now().UTC()
	for _, statement := range []string{
		`ALTER TABLE pm_tasks ADD COLUMN estimate INTEGER`,
		`ALTER TABLE pm_tasks ADD COLUMN archived BOOLEAN NOT NULL DEFAULT 0`,
		`CREATE TABLE pm_workflow_states (id TEXT PRIMARY KEY, state_type TEXT NOT NULL DEFAULT 'unstarted')`,
		`CREATE TABLE pm_objectives (id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE pm_epic_objectives (epic_id TEXT NOT NULL, objective_id TEXT NOT NULL)`,
		`CREATE TABLE pm_labels (id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT NOT NULL DEFAULT '', color TEXT)`,
		`CREATE TABLE pm_epic_labels (epic_id TEXT NOT NULL, label_id TEXT NOT NULL)`,
		`CREATE TABLE pm_epics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			name TEXT NOT NULL,
			description TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			health TEXT NOT NULL DEFAULT 'no_health',
			archived BOOLEAN NOT NULL DEFAULT 0,
			planning_state TEXT NOT NULL DEFAULT 'not_started',
			spec_clarifications BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE mcp_audit_events (id text primary key, workspace_id text not null, user_id text, event_type text not null, tool_name text, outcome text not null, created_at datetime not null)`,
		`CREATE TABLE mcp_connections (id text primary key, workspace_id text not null default '', user_id text not null)`,
		`CREATE TABLE mcp_service_principals (id text primary key, workspace_id text not null default '', actor_user_id text not null)`,
		`CREATE TABLE mcp_agent_run_attributions (run_id text primary key, workspace_id text not null, connection_id text, service_principal_id text, client_name text not null default '', created_at datetime)`,
		`INSERT INTO mcp_connections (id, workspace_id, user_id) VALUES ('connection-1', 'ws-1', 'user-1')`,
	} {
		mustExec(t, db, statement)
	}

	// task-1 has a ready delivery target (repo-1); task-2 has none.
	seedCodingDelegationTaskAndDelivery(t, db, now)
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, workflow_state_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"task-2", "ws-1", 43, "state-1", "Summarize flaky tests", now, now)
	mustExec(t, db, `INSERT INTO pm_epics (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"epic-1", "ws-1", "Checkout revamp", now, now)

	seedMCPAgentRunTargetAgent(t, db, "agent-forge", "Forge", model.AgentPresetCodeBuilder, nil, now)
	seedMCPAgentRunTargetAgent(t, db, "agent-sub", "Sub-agent", model.AgentPresetCommandAgent, commandAgentPresetTools(), now)

	agents := newCodingDelegationService(t, db, &fakeAgentRuntimeSignalClient{})
	agents.epicRepo = repository.NewPMEpicRepository(db)
	agents.gitService.deliveryRepo = repository.NewTaskDeliveryTargetRepository(db)

	return &mcpAgentRunTargetEnv{
		t:  t,
		db: db,
		service: &MCPService{
			repo:   repository.NewMCPRepository(db),
			agents: agents,
			config: MCPServiceConfig{AgentRunEnabled: true, PMWriteEnabled: true},
		},
		principal: &model.MCPPrincipal{
			WorkspaceID: "ws-1", UserID: "user-1", ConnectionID: "connection-1", ClientName: "Claude",
			Toolsets: []string{MCPToolsetAgents, MCPToolsetPM},
			Scopes:   []string{MCPScopeAgentsRead, MCPScopeAgentsRun, MCPScopePMRead, MCPScopePMWrite},
		},
		actor: &authorization.Actor{WorkspaceID: "ws-1", UserID: "user-1", Role: "admin"},
	}
}

func seedMCPAgentRunTargetAgent(t *testing.T, db *gorm.DB, id, name, presetKey string, tools []string, now time.Time) {
	t.Helper()
	allowedTools := []byte("[]")
	if len(tools) > 0 {
		allowedTools = mustJSONStringSlice(tools)
	}
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, provider, execution_config, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, "ws-1", true, name, presetKey, name, "idle", "native_sdk",
		[]byte("[]"), "manual", "openai", []byte(`{}`), allowedTools, []byte("[]"), []byte("[]"), "never",
		1, model.InvocationModeAutonomous, now, now,
	)
}

func (e *mcpAgentRunTargetEnv) start(arguments string) (*MCPToolResult, error) {
	e.t.Helper()
	return e.service.executeSpecialMCPTool(context.Background(), e.principal, e.actor, "start_agent_run", json.RawMessage(arguments))
}

func mcpToolErrorCode(err error) string {
	var toolErr *MCPToolError
	if errors.As(err, &toolErr) {
		return toolErr.Code
	}
	return ""
}

func TestMCPStartAgentRunTaskAndEpicTargets(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
		wantCode  string
	}{
		{name: "Forge on a task with a repository", arguments: `{"agent_id":"agent-forge","target_type":"task","target_id":"task-1"}`},
		{name: "Sub-agent on a task with a repository", arguments: `{"agent_id":"agent-sub","target_type":"task","target_id":"task-1"}`},
		{name: "Forge on a task after choosing a repository", arguments: `{"agent_id":"agent-forge","target_type":"task","target_id":"task-2","repository_id":"repo-1"}`},
		{name: "Sub-agent on an epic", arguments: `{"agent_id":"agent-sub","target_type":"epic","target_id":"epic-1"}`},
		{name: "Forge on a task without a repository", arguments: `{"agent_id":"agent-forge","target_type":"task","target_id":"task-2"}`, wantCode: MCPErrorCodeRepositoryRequired},
		{name: "Sub-agent on a task without a repository", arguments: `{"agent_id":"agent-sub","target_type":"task","target_id":"task-2"}`, wantCode: MCPErrorCodeRepositoryRequired},
		{name: "unknown repository", arguments: `{"agent_id":"agent-forge","target_type":"task","target_id":"task-2","repository_id":"repo-missing"}`, wantCode: MCPErrorCodeRepositoryRequired},
		{name: "Forge on an epic", arguments: `{"agent_id":"agent-forge","target_type":"epic","target_id":"epic-1"}`, wantCode: MCPErrorCodeAgentTargetNotAllowed},
		{name: "missing task", arguments: `{"agent_id":"agent-sub","target_type":"task","target_id":"task-missing"}`, wantCode: MCPErrorCodeTargetNotFound},
		{name: "missing epic", arguments: `{"agent_id":"agent-sub","target_type":"epic","target_id":"epic-missing"}`, wantCode: MCPErrorCodeTargetNotFound},
		{name: "repository for a non-task target", arguments: `{"agent_id":"agent-sub","target_type":"epic","target_id":"epic-1","repository_id":"repo-1"}`, wantCode: MCPErrorCodeRepositoryNotApplicable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupMCPAgentRunTargetTest(t)
			result, err := env.start(tt.arguments)
			if tt.wantCode != "" {
				if got := mcpToolErrorCode(err); got != tt.wantCode {
					t.Fatalf("start_agent_run error = %v (code %q), want code %q", err, got, tt.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("start_agent_run returned error: %v", err)
			}
			data, ok := result.Data.(map[string]any)
			if !ok || data["run_id"] == "" || data["run_id"] == nil {
				t.Fatalf("start_agent_run data = %#v, want a run_id", result.Data)
			}
		})
	}
}

func TestMCPStartAgentRunRepositoryIDSetsTaskDeliveryTarget(t *testing.T) {
	env := setupMCPAgentRunTargetTest(t)
	if _, err := env.start(`{"agent_id":"agent-forge","target_type":"task","target_id":"task-2","repository_id":"repo-1"}`); err != nil {
		t.Fatalf("start_agent_run returned error: %v", err)
	}
	target, err := repository.NewTaskDeliveryTargetRepository(env.db).GetByTask(context.Background(), "ws-1", "task-2")
	if err != nil || target == nil {
		t.Fatalf("GetByTask() = %v, %v", target, err)
	}
	if derefString(target.RepositoryID) != "repo-1" || derefString(target.BaseBranch) != "main" {
		t.Fatalf("delivery target = repo %q base %q, want repo-1 on main", derefString(target.RepositoryID), derefString(target.BaseBranch))
	}
}

func TestMCPStartAgentRunRepositoryIDRequiresPMWriteGrant(t *testing.T) {
	env := setupMCPAgentRunTargetTest(t)
	env.principal.Scopes = []string{MCPScopeAgentsRead, MCPScopeAgentsRun}
	_, err := env.start(`{"agent_id":"agent-forge","target_type":"task","target_id":"task-2","repository_id":"repo-1"}`)
	if !errors.Is(err, ErrMCPForbidden) {
		t.Fatalf("start_agent_run error = %v, want ErrMCPForbidden", err)
	}
}

func TestMCPStartAgentRunActiveRunOnTargetIsTyped(t *testing.T) {
	env := setupMCPAgentRunTargetTest(t)
	if _, err := env.start(`{"agent_id":"agent-sub","target_type":"epic","target_id":"epic-1"}`); err != nil {
		t.Fatalf("first start_agent_run returned error: %v", err)
	}
	mustExec(t, env.db, `UPDATE agents SET allowed_targets = ? WHERE id = ?`, mustJSONStringSlice([]string{"task", "epic"}), "agent-forge")
	_, err := env.start(`{"agent_id":"agent-forge","target_type":"epic","target_id":"epic-1"}`)
	if got := mcpToolErrorCode(err); got != MCPErrorCodeTargetBusy {
		t.Fatalf("start_agent_run error = %v (code %q), want %q", err, got, MCPErrorCodeTargetBusy)
	}
}

func TestMCPStartAgentRunLimitsAreTyped(t *testing.T) {
	env := setupMCPAgentRunTargetTest(t)
	for i := 0; i < 10; i++ {
		mustExec(t, env.db, `INSERT INTO mcp_audit_events (id, workspace_id, user_id, event_type, tool_name, outcome, created_at) VALUES (?, 'ws-1', 'user-1', 'tool.call', 'start_agent_run', 'success', ?)`,
			"audit-"+string(rune('a'+i)), time.Now())
	}
	_, err := env.start(`{"agent_id":"agent-sub","target_type":"epic","target_id":"epic-1"}`)
	if got := mcpToolErrorCode(err); got != MCPErrorCodeAgentRunLimit {
		t.Fatalf("start_agent_run error = %v (code %q), want %q", err, got, MCPErrorCodeAgentRunLimit)
	}
	if !errors.Is(err, ErrMCPRateLimited) {
		t.Fatalf("start_agent_run error = %v, want it to wrap ErrMCPRateLimited", err)
	}
}

func TestMCPStartAgentRunRejectedAgentLeavesTaskRepositoryUnchanged(t *testing.T) {
	env := setupMCPAgentRunTargetTest(t)
	mustExec(t, env.db, `UPDATE agents SET allowed_targets = ? WHERE id = ?`, mustJSONStringSlice([]string{"epic"}), "agent-sub")
	_, err := env.start(`{"agent_id":"agent-sub","target_type":"task","target_id":"task-2","repository_id":"repo-1"}`)
	if got := mcpToolErrorCode(err); got != MCPErrorCodeAgentTargetNotAllowed {
		t.Fatalf("start_agent_run error = %v (code %q), want %q", err, got, MCPErrorCodeAgentTargetNotAllowed)
	}
	target, err := repository.NewTaskDeliveryTargetRepository(env.db).GetByTask(context.Background(), "ws-1", "task-2")
	if err != nil {
		t.Fatalf("GetByTask() error = %v", err)
	}
	if target != nil && target.RepositoryID != nil {
		t.Fatalf("delivery repository = %q, want unchanged", *target.RepositoryID)
	}
}

func TestMCPStartAgentRunBaseBranchRequiresRepositoryID(t *testing.T) {
	env := setupMCPAgentRunTargetTest(t)
	_, err := env.start(`{"agent_id":"agent-forge","target_type":"task","target_id":"task-2","base_branch":"develop"}`)
	if !errors.Is(err, ErrMCPInvalidArguments) {
		t.Fatalf("start_agent_run error = %v, want ErrMCPInvalidArguments", err)
	}
}
