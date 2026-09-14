package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListAgentsForActorWithoutActorDoesNotLeakTeamScopedAgents(t *testing.T) {
	db := setupAgentScopeTestDB(t)
	seedAgentScopeAgent(t, db, model.Agent{
		ID:                    "workspace-agent",
		WorkspaceID:           "ws-1",
		Name:                  "Workspace Agent",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		AllowedTargets:        json.RawMessage(`["task"]`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		Skills:                model.AgentSkillRefs{},
		ExecutionConfig:       model.JSONBlob(`{}`),
		ApprovalMode:          "always",
		DefaultInvocationMode: "interactive",
	})
	seedAgentScopeAgent(t, db, model.Agent{
		ID:                    "team-agent",
		WorkspaceID:           "ws-1",
		Name:                  "Team Agent",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		AllowedTargets:        json.RawMessage(`["task"]`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		Skills:                model.AgentSkillRefs{},
		ExecutionConfig:       model.JSONBlob(`{}`),
		ApprovalMode:          "always",
		DefaultInvocationMode: "interactive",
		TeamIDs:               []string{"team-a"},
	})

	svc := &AgentService{agentRepo: repository.NewAgentRepository(db)}
	agents, err := svc.ListAgentsForActor(context.Background(), "ws-1", nil)
	if err != nil {
		t.Fatalf("ListAgentsForActor returned error: %v", err)
	}
	if len(agents) != 1 || agents[0].ID != "workspace-agent" {
		t.Fatalf("agents = %#v, want only workspace-agent", agents)
	}
}

func TestListAgentsForActorFiltersTeamScopedAgents(t *testing.T) {
	db := setupAgentScopeTestDB(t)
	seedAgentScopeAgent(t, db, model.Agent{
		ID:                    "team-agent-a",
		WorkspaceID:           "ws-1",
		Name:                  "Team A",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		AllowedTargets:        json.RawMessage(`["task"]`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		Skills:                model.AgentSkillRefs{},
		ExecutionConfig:       model.JSONBlob(`{}`),
		ApprovalMode:          "always",
		DefaultInvocationMode: "interactive",
		TeamIDs:               []string{"team-a"},
	})
	seedAgentScopeAgent(t, db, model.Agent{
		ID:                    "team-agent-b",
		WorkspaceID:           "ws-1",
		Name:                  "Team B",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		AllowedTargets:        json.RawMessage(`["task"]`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		Skills:                model.AgentSkillRefs{},
		ExecutionConfig:       model.JSONBlob(`{}`),
		ApprovalMode:          "always",
		DefaultInvocationMode: "interactive",
		TeamIDs:               []string{"team-b"},
	})

	svc := &AgentService{agentRepo: repository.NewAgentRepository(db)}
	agents, err := svc.ListAgentsForActor(context.Background(), "ws-1", &authorization.Actor{
		Role: "member",
		TeamMemberships: []authorization.TeamRole{
			{TeamID: "team-a", Role: "member"},
		},
	})
	if err != nil {
		t.Fatalf("ListAgentsForActor returned error: %v", err)
	}
	if len(agents) != 1 || agents[0].ID != "team-agent-a" {
		t.Fatalf("agents = %#v, want only team-agent-a", agents)
	}
}

func TestRequireActorCanUseAgentFiltersTeamScopedAgents(t *testing.T) {
	db := setupAgentScopeTestDB(t)
	seedAgentScopeAgent(t, db, model.Agent{
		ID:                    "agent-team-a",
		WorkspaceID:           "ws-1",
		Name:                  "Team A Agent",
		Status:                "idle",
		RuntimeKind:           "codex",
		AllowedTargets:        json.RawMessage(`["support_conversation"]`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		Skills:                model.AgentSkillRefs{{Key: "missing-retired-skill"}},
		ExecutionConfig:       model.JSONBlob(`{}`),
		ApprovalMode:          "always",
		DefaultInvocationMode: "interactive",
		TeamIDs:               []string{"team-a"},
	})

	svc := &AgentService{agentRepo: repository.NewAgentRepository(db)}
	err := svc.RequireActorCanUseAgent(context.Background(), "ws-1", "agent-team-a", &authorization.Actor{
		Role: "member",
		TeamMemberships: []authorization.TeamRole{
			{TeamID: "team-b", Role: "member"},
		},
	})
	if err == nil {
		t.Fatalf("RequireActorCanUseAgent succeeded for actor without team access")
	}

	err = svc.RequireActorCanUseAgent(context.Background(), "ws-1", "agent-team-a", &authorization.Actor{
		Role: "member",
		TeamMemberships: []authorization.TeamRole{
			{TeamID: "team-a", Role: "member"},
		},
	})
	if err != nil {
		t.Fatalf("RequireActorCanUseAgent returned error for actor with team access: %v", err)
	}
}

func setupAgentScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE agents (
 ai_profile_id TEXT,
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			is_system boolean NOT NULL DEFAULT false,
			name text NOT NULL,
			icon_key text NOT NULL DEFAULT '',
			preset_key text,
			preset_version_key text,
			source_preset_key text,
			source_preset_version_key text,
			source_template_id text,
			source_template_key text,
			template_key text,
			template_instance_id text,
			template_version integer,
			active_version_id text,
			role text,
			status text NOT NULL DEFAULT 'idle',
			runtime_kind text NOT NULL DEFAULT 'native_sdk',
			model_tier text NOT NULL DEFAULT '',
			skills text NOT NULL DEFAULT '[]',
			trigger_mode text NOT NULL DEFAULT 'manual',
			provider text,
			model text,
			execution_config text NOT NULL DEFAULT '{}',
			system_prompt text,
			instruction_template_version text NOT NULL DEFAULT '',
			planning_notes text,
			monthly_token_budget integer,
			tokens_used_this_month integer NOT NULL DEFAULT 0,
			active_task_id text,
			team_id text,
			allowed_tools text NOT NULL DEFAULT '[]',
			allowed_commands text NOT NULL DEFAULT '[]',
			allowed_targets text NOT NULL DEFAULT '[]',
			approval_mode text NOT NULL DEFAULT 'always',
			max_concurrent_runs integer NOT NULL DEFAULT 1,
			default_invocation_mode text NOT NULL DEFAULT 'interactive',
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE agent_team_access (
			agent_id text NOT NULL,
			team_id text NOT NULL,
			created_at datetime,
			PRIMARY KEY (agent_id, team_id)
		)`,
		`CREATE TABLE agent_runs (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			agent_id text NOT NULL,
			agent_version_id text,
			conversation_id text,
			target_type text NOT NULL,
			target_id text NOT NULL,
			runtime_kind text NOT NULL DEFAULT 'native_sdk',
			model_tier text NOT NULL DEFAULT '',
			invocation_mode text NOT NULL DEFAULT 'interactive',
			approval_state text NOT NULL DEFAULT 'pending',
			pause_reason text NOT NULL DEFAULT 'none',
			triggered_by_user_id text,
			status text NOT NULL DEFAULT 'queued',
			external_runtime text,
			external_runtime_id text,
			task_queue text,
			runner_pool text,
			input text NOT NULL DEFAULT '{}',
			output_summary text NOT NULL DEFAULT '{}',
			tokens_used integer NOT NULL DEFAULT 0,
			error_message text,
			execution_stage text,
			completed_at datetime,
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE agent_trigger_executions (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			agent_id text NOT NULL,
			actor_id text,
			binding_id text NOT NULL,
			binding_kind text NOT NULL,
			trigger_type text,
			reference_id text,
			reference_type text,
			target_type text,
			target_id text,
			run_id text,
			status text NOT NULL,
			error_message text,
			fired_at datetime,
			started_at datetime,
			completed_at datetime,
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE support_mailboxes (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			name text NOT NULL,
			handle text NOT NULL,
			icon text NOT NULL DEFAULT 'inbox',
			description text,
			routing_prompt text,
			triage_eligible boolean NOT NULL DEFAULT true,
			linked_team_id text,
			visibility_mode text NOT NULL DEFAULT 'members_only',
			assignment_mode text NOT NULL DEFAULT 'manual',
			reply_time_preset text,
			reply_time_custom_minutes integer,
			position integer NOT NULL DEFAULT 0,
			active boolean NOT NULL DEFAULT true,
			created_by_id text NOT NULL DEFAULT '',
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE support_conversations (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			mailbox_id text,
			display_id integer NOT NULL,
			subject text NOT NULL,
			status text NOT NULL DEFAULT 'open',
			flow_state text,
			priority text NOT NULL DEFAULT 'medium',
			channel text NOT NULL DEFAULT 'widget',
			customer_name text,
			customer_email text,
			customer_phone text,
			opened_by_user_id text,
			assigned_user_id text,
			assigned_agent_id text,
			linked_task_id text,
			source text NOT NULL DEFAULT 'internal',
			anonymous_id text,
			crm_contact_id text,
			resolved_at datetime,
			closed_at datetime,
			team_last_seen_at datetime,
			contact_last_seen_at datetime,
			email_unsubscribed boolean NOT NULL DEFAULT false,
			ai_state text,
			ai_resolved_at datetime,
			ai_escalated_at datetime,
			ai_resolution_type text,
			ai_turn_count integer NOT NULL DEFAULT 0,
			customer_requested_human_at datetime,
			ai_active_run_id TEXT,
			human_takeover boolean DEFAULT false,
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE support_widget_sessions (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			conversation_id text,
			anonymous_id text,
			country_code text,
			country_name text,
			created_at datetime
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}
	return db
}

func seedAgentScopeAgent(t *testing.T, db *gorm.DB, agent model.Agent) {
	t.Helper()
	teamIDs := append([]string(nil), agent.TeamIDs...)
	if len(teamIDs) > 0 {
		first := teamIDs[0]
		agent.TeamID = &first
	}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent %s: %v", agent.ID, err)
	}
	for _, teamID := range teamIDs {
		if err := db.Create(&model.AgentTeamAccess{AgentID: agent.ID, TeamID: teamID}).Error; err != nil {
			t.Fatalf("create team access: %v", err)
		}
	}
}
