package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func setupRuleEngineTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:rule_engine_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	stmts := []string{
		`CREATE TABLE automation_rules (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			team_id TEXT,
			workflow_id TEXT,
			trigger_type TEXT NOT NULL,
			trigger_config TEXT NOT NULL DEFAULT '{}',
			action_type TEXT NOT NULL,
			action_config TEXT NOT NULL DEFAULT '{}',
			position INTEGER NOT NULL DEFAULT 0,
			stop_on_match BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			workflow_state_id TEXT NOT NULL,
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
		`CREATE TABLE pm_workflow_states (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workflow_id TEXT NOT NULL,
			name TEXT NOT NULL,
			state_type TEXT NOT NULL DEFAULT 'unstarted',
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	return db
}

func TestEvaluateEvent_CronTrigger_SkipsStoryLoading(t *testing.T) {
	db := setupRuleEngineTestDB(t)
	ruleRepo := repository.NewAutomationRuleRepository(db)
	storyRepo := repository.NewPMTaskRepository(db)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	engine := NewAutomationRuleEngine(ruleRepo, storyRepo, workflowRepo, nil, nil, nil, nil, nil)

	// Create a cron rule
	rule := &model.AutomationRule{
		ID:            "rule-cron-1",
		WorkspaceID:   "ws-1",
		Name:          "Sprint cron",
		Enabled:       true,
		TriggerType:   model.TriggerCron,
		TriggerConfig: json.RawMessage(`{"category":"sprint_hourly"}`),
		ActionType:    model.ActionRunCommand,
		ActionConfig:  json.RawMessage(`{"command_name":"pm.sprint_auto_create"}`),
		Position:      0,
	}
	if err := ruleRepo.Create(context.Background(), rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	// This should NOT panic or error trying to load a story — story ID is empty
	engine.EvaluateEvent(context.Background(), model.AutomationEvent{
		WorkspaceID: "ws-1",
		TriggerType: model.TriggerCron,
	}, nil)
}

func TestMatchesTriggerConfig_StateType(t *testing.T) {
	db := setupRuleEngineTestDB(t)
	ruleRepo := repository.NewAutomationRuleRepository(db)
	storyRepo := repository.NewPMTaskRepository(db)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	engine := NewAutomationRuleEngine(ruleRepo, storyRepo, workflowRepo, nil, nil, nil, nil, nil)

	// Create a workflow state
	if err := db.Exec(`INSERT INTO pm_workflow_states (id, workflow_id, name, state_type) VALUES ('state-1', 'wf-1', 'In Progress', 'started')`).Error; err != nil {
		t.Fatalf("create state: %v", err)
	}

	tests := []struct {
		name      string
		rule      model.AutomationRule
		event     model.AutomationEvent
		wantMatch bool
	}{
		{
			name: "exact state_id match",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerStoryStateEntered,
				TriggerConfig: json.RawMessage(`{"state_id":"state-1"}`),
			},
			event:     model.AutomationEvent{StateID: "state-1"},
			wantMatch: true,
		},
		{
			name: "state_type match",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerStoryStateEntered,
				TriggerConfig: json.RawMessage(`{"state_type":"started"}`),
			},
			event:     model.AutomationEvent{StateID: "state-1"},
			wantMatch: true,
		},
		{
			name: "state_type mismatch",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerStoryStateEntered,
				TriggerConfig: json.RawMessage(`{"state_type":"done"}`),
			},
			event:     model.AutomationEvent{StateID: "state-1"},
			wantMatch: false,
		},
		{
			name: "cron trigger always matches with category",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerCron,
				TriggerConfig: json.RawMessage(`{"category":"sprint_hourly"}`),
			},
			event:     model.AutomationEvent{},
			wantMatch: true,
		},
		{
			name: "cron trigger with empty category does not match",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerCron,
				TriggerConfig: json.RawMessage(`{"category":""}`),
			},
			event:     model.AutomationEvent{},
			wantMatch: false,
		},
		{
			name: "github push matches branch",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerGitHubPush,
				TriggerConfig: json.RawMessage(`{"branch":"main"}`),
			},
			event:     model.AutomationEvent{Branch: "main", RepoFullName: "acme/api"},
			wantMatch: true,
		},
		{
			name: "github pr opened matches repo",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerGitHubPROpened,
				TriggerConfig: json.RawMessage(`{"repo_full_name":"acme/api"}`),
			},
			event:     model.AutomationEvent{BaseBranch: "main", RepoFullName: "acme/api"},
			wantMatch: true,
		},
		{
			name: "github merged matches base branch",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerGitHubPRMerged,
				TriggerConfig: json.RawMessage(`{"base_branch":"main"}`),
			},
			event:     model.AutomationEvent{BaseBranch: "main", RepoFullName: "acme/api"},
			wantMatch: true,
		},
		{
			name: "github merged repo mismatch",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerGitHubPRMerged,
				TriggerConfig: json.RawMessage(`{"repo_full_name":"acme/web"}`),
			},
			event:     model.AutomationEvent{BaseBranch: "main", RepoFullName: "acme/api"},
			wantMatch: false,
		},
		{
			name: "github release matches tag",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerGitHubReleasePub,
				TriggerConfig: json.RawMessage(`{"tag_name":"v1.2.3"}`),
			},
			event:     model.AutomationEvent{TagName: "v1.2.3", RepoFullName: "acme/api"},
			wantMatch: true,
		},
		{
			name: "github check suite conclusion mismatch",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerGitHubCheckSuite,
				TriggerConfig: json.RawMessage(`{"conclusion":"success"}`),
			},
			event:     model.AutomationEvent{Conclusion: "failure", RepoFullName: "acme/api"},
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.matchesTriggerConfig(context.Background(), tt.rule, tt.event)
			if got != tt.wantMatch {
				t.Errorf("matchesTriggerConfig() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestMatchesScope_NilStory(t *testing.T) {
	db := setupRuleEngineTestDB(t)
	ruleRepo := repository.NewAutomationRuleRepository(db)
	storyRepo := repository.NewPMTaskRepository(db)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	engine := NewAutomationRuleEngine(ruleRepo, storyRepo, workflowRepo, nil, nil, nil, nil, nil)

	teamID := "team-1"
	otherTeamID := "team-2"

	tests := []struct {
		name      string
		rule      model.AutomationRule
		event     model.AutomationEvent
		wantMatch bool
	}{
		{
			name:      "nil story, no team filter",
			rule:      model.AutomationRule{},
			event:     model.AutomationEvent{},
			wantMatch: true,
		},
		{
			name:      "nil story, team matches",
			rule:      model.AutomationRule{TeamID: &teamID},
			event:     model.AutomationEvent{TeamID: "team-1"},
			wantMatch: true,
		},
		{
			name:      "nil story, team does not match",
			rule:      model.AutomationRule{TeamID: &otherTeamID},
			event:     model.AutomationEvent{TeamID: "team-1"},
			wantMatch: false,
		},
		{
			name:      "nil story, rule has team but event has no team",
			rule:      model.AutomationRule{TeamID: &teamID},
			event:     model.AutomationEvent{},
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.matchesScope(tt.rule, nil, tt.event)
			if got != tt.wantMatch {
				t.Errorf("matchesScope() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestValidateRuleRequest_NewTypes(t *testing.T) {
	db := setupRuleEngineTestDB(t)
	ruleRepo := repository.NewAutomationRuleRepository(db)
	storyRepo := repository.NewPMTaskRepository(db)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	engine := NewAutomationRuleEngine(ruleRepo, storyRepo, workflowRepo, nil, nil, nil, nil, nil)

	tests := []struct {
		name          string
		triggerType   string
		triggerConfig json.RawMessage
		actionType    string
		actionConfig  json.RawMessage
		wantErr       bool
	}{
		{
			name:          "valid cron + run_command",
			triggerType:   model.TriggerCron,
			triggerConfig: json.RawMessage(`{"category":"sprint_hourly"}`),
			actionType:    model.ActionRunCommand,
			actionConfig:  json.RawMessage(`{"command_name":"pm.sprint_auto_create"}`),
			wantErr:       false,
		},
		{
			name:          "cron missing category",
			triggerType:   model.TriggerCron,
			triggerConfig: json.RawMessage(`{}`),
			actionType:    model.ActionRunCommand,
			actionConfig:  json.RawMessage(`{"command_name":"test"}`),
			wantErr:       true,
		},
		{
			name:          "run_command missing command_name",
			triggerType:   model.TriggerCron,
			triggerConfig: json.RawMessage(`{"category":"test"}`),
			actionType:    model.ActionRunCommand,
			actionConfig:  json.RawMessage(`{}`),
			wantErr:       true,
		},
		{
			name:          "valid start_agent_run",
			triggerType:   model.TriggerStoryStateEntered,
			triggerConfig: json.RawMessage(`{"state_type":"done"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       false,
		},
		{
			name:          "github push start_agent_run requires explicit target",
			triggerType:   model.TriggerGitHubPush,
			triggerConfig: json.RawMessage(`{"branch":"main"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "valid github push start_agent_run with explicit target",
			triggerType:   model.TriggerGitHubPush,
			triggerConfig: json.RawMessage(`{"branch":"main"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1","target_type":"repository","target_id":"repo-1"}`),
			wantErr:       false,
		},
		{
			name:          "github push requires a filter",
			triggerType:   model.TriggerGitHubPush,
			triggerConfig: json.RawMessage(`{}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "github pr opened requires explicit target",
			triggerType:   model.TriggerGitHubPROpened,
			triggerConfig: json.RawMessage(`{"repo_full_name":"acme/api"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "valid github pr opened with explicit target",
			triggerType:   model.TriggerGitHubPROpened,
			triggerConfig: json.RawMessage(`{"repo_full_name":"acme/api"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1","target_type":"repository","target_id":"repo-1"}`),
			wantErr:       false,
		},
		{
			name:          "github pr review requested requires explicit target",
			triggerType:   model.TriggerGitHubPRReviewReq,
			triggerConfig: json.RawMessage(`{"repo_full_name":"acme/api"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "valid github pr review requested with explicit target",
			triggerType:   model.TriggerGitHubPRReviewReq,
			triggerConfig: json.RawMessage(`{"repo_full_name":"acme/api"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1","target_type":"repository","target_id":"repo-1"}`),
			wantErr:       false,
		},
		{
			name:          "github merged start_agent_run requires explicit target",
			triggerType:   model.TriggerGitHubPRMerged,
			triggerConfig: json.RawMessage(`{"base_branch":"main"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "valid github merged start_agent_run with explicit target",
			triggerType:   model.TriggerGitHubPRMerged,
			triggerConfig: json.RawMessage(`{"base_branch":"main"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1","target_type":"repository","target_id":"repo-1"}`),
			wantErr:       false,
		},
		{
			name:          "github merged requires at least one filter",
			triggerType:   model.TriggerGitHubPRMerged,
			triggerConfig: json.RawMessage(`{}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "github release published requires explicit target",
			triggerType:   model.TriggerGitHubReleasePub,
			triggerConfig: json.RawMessage(`{"tag_name":"v1.2.3"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "valid github release published with explicit target",
			triggerType:   model.TriggerGitHubReleasePub,
			triggerConfig: json.RawMessage(`{"tag_name":"v1.2.3"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1","target_type":"repository","target_id":"repo-1"}`),
			wantErr:       false,
		},
		{
			name:          "github release published requires a filter",
			triggerType:   model.TriggerGitHubReleasePub,
			triggerConfig: json.RawMessage(`{}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "github check suite completed requires explicit target",
			triggerType:   model.TriggerGitHubCheckSuite,
			triggerConfig: json.RawMessage(`{"conclusion":"success"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "valid github check suite completed with explicit target",
			triggerType:   model.TriggerGitHubCheckSuite,
			triggerConfig: json.RawMessage(`{"conclusion":"success"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1","target_type":"repository","target_id":"repo-1"}`),
			wantErr:       false,
		},
		{
			name:          "github check suite requires a filter",
			triggerType:   model.TriggerGitHubCheckSuite,
			triggerConfig: json.RawMessage(`{}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "valid cron start_agent_run with explicit target",
			triggerType:   model.TriggerCron,
			triggerConfig: json.RawMessage(`{"category":"sprint_hourly"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1","target_type":"epic","target_id":"epic-1"}`),
			wantErr:       false,
		},
		{
			name:          "start_agent_run missing agent_id",
			triggerType:   model.TriggerStoryStateEntered,
			triggerConfig: json.RawMessage(`{"state_type":"done"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{}`),
			wantErr:       true,
		},
		{
			name:          "start_agent_run with partial explicit target is invalid",
			triggerType:   model.TriggerStoryStateEntered,
			triggerConfig: json.RawMessage(`{"state_type":"done"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1","target_type":"story"}`),
			wantErr:       true,
		},
		{
			name:          "cron start_agent_run missing explicit target is invalid",
			triggerType:   model.TriggerCron,
			triggerConfig: json.RawMessage(`{"category":"sprint_hourly"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "legacy start_flow rejected",
			triggerType:   model.TriggerStoryStateEntered,
			triggerConfig: json.RawMessage(`{"state_type":"done"}`),
			actionType:    model.ActionStartFlow,
			actionConfig:  json.RawMessage(`{"template_id":"tmpl-1"}`),
			wantErr:       true,
		},
		{
			name:          "legacy run_agent rejected",
			triggerType:   model.TriggerStoryStateEntered,
			triggerConfig: json.RawMessage(`{"state_type":"done"}`),
			actionType:    model.ActionRunAgent,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "state_entered with state_type only is valid",
			triggerType:   model.TriggerStoryStateEntered,
			triggerConfig: json.RawMessage(`{"state_type":"started"}`),
			actionType:    model.ActionMoveToState,
			actionConfig:  json.RawMessage(`{"target_state_id":"s-1"}`),
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.validateRuleRequest(tt.triggerType, tt.triggerConfig, tt.actionType, tt.actionConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRuleRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
