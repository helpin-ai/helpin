package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

type automationRuleRunEngineStub struct {
	stoppedRuleIDs []string
	stopErr        error
}

func (s *automationRuleRunEngineStub) StartRuleSchedule(
	context.Context,
	string,
	string,
	string,
) error {
	return nil
}

func (s *automationRuleRunEngineStub) StopRuleSchedule(_ context.Context, ruleID string) error {
	s.stoppedRuleIDs = append(s.stoppedRuleIDs, ruleID)
	return s.stopErr
}

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
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
			position INTEGER NOT NULL DEFAULT 0,
			stop_on_match BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL DEFAULT 0,
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
		`CREATE TABLE workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			slug TEXT NOT NULL DEFAULT '',
			workspace_key TEXT NOT NULL DEFAULT '',
			owner_id TEXT NOT NULL DEFAULT '',
			organization_id TEXT,
			description TEXT,
			company_product_context TEXT,
			website_url TEXT,
			logo_url TEXT,
			timezone TEXT NOT NULL DEFAULT 'UTC',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_team_repo_defaults (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL,
			repository_id TEXT NOT NULL,
			base_branch TEXT NOT NULL DEFAULT 'main',
			branch_template TEXT NOT NULL DEFAULT '{task_key}-{slug}',
			auto_sync_states BOOLEAN NOT NULL DEFAULT 1,
			review_state_id TEXT,
			done_state_id TEXT,
			closed_state_id TEXT,
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
			permissions TEXT NOT NULL DEFAULT '{}',
			private BOOLEAN NOT NULL DEFAULT 1,
			archived BOOLEAN NOT NULL DEFAULT 0,
			selected BOOLEAN NOT NULL DEFAULT 1,
			active BOOLEAN NOT NULL DEFAULT 1,
			deleted_at DATETIME,
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

func TestCreateRuleForActorAttributesCreator(t *testing.T) {
	db := setupRuleEngineTestDB(t)
	ruleRepo := repository.NewAutomationRuleRepository(db)
	engine := NewAutomationRuleEngine(ruleRepo, nil, nil, nil, nil, nil, nil, nil)

	rule, err := engine.CreateRuleForActor(context.Background(), "ws-1", "user-1", model.CreateAutomationRuleRequest{
		Name:          "Move approved task",
		TriggerType:   model.TriggerAgentRunApproved,
		TriggerConfig: json.RawMessage(`{"state_id":"state-1"}`),
		ActionType:    model.ActionMoveToState,
		ActionConfig:  json.RawMessage(`{"target_state_id":"state-2"}`),
	})
	if err != nil {
		t.Fatalf("CreateRuleForActor returned error: %v", err)
	}
	if rule.CreatedBy == nil || *rule.CreatedBy != "user-1" {
		t.Fatalf("CreatedBy = %#v, want user-1", rule.CreatedBy)
	}
}

func TestExecuteScheduledRuleDisablesCronRuleWhenAgentIsMissing(t *testing.T) {
	db := setupRuleEngineTestDB(t)
	if err := db.Exec(`CREATE TABLE agents (
 ai_profile_id TEXT,
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		icon_key TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'idle',
		runtime_kind TEXT NOT NULL DEFAULT 'native_sdk',
		model_tier TEXT NOT NULL DEFAULT '',
		source_template_key TEXT NOT NULL DEFAULT '',
		template_key TEXT,
		template_instance_id TEXT,
		template_version INTEGER,
		active_version_id TEXT,
		trigger_mode TEXT NOT NULL DEFAULT 'manual',
		approval_mode TEXT NOT NULL DEFAULT 'class_default',
		is_system BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create agents table: %v", err)
	}

	ruleRepo := repository.NewAutomationRuleRepository(db)
	engine := NewAutomationRuleEngine(ruleRepo, nil, nil, nil, nil, nil, nil, nil)
	engine.SetAgentService(&AgentService{agentRepo: repository.NewAgentRepository(db)})

	rule := &model.AutomationRule{
		ID:            "rule-missing-agent",
		WorkspaceID:   "ws-1",
		Name:          "Missing agent cron",
		Enabled:       true,
		TriggerType:   model.TriggerCron,
		TriggerConfig: json.RawMessage(`{"preset":"hourly"}`),
		ActionType:    model.ActionStartAgentRun,
		ActionConfig:  json.RawMessage(`{"agent_id":"missing-agent","target_type":"workspace","target_id":"ws-1"}`),
	}
	if err := ruleRepo.Create(context.Background(), rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	if err := engine.ExecuteScheduledRule(context.Background(), "ws-1", rule.ID); err != nil {
		t.Fatalf("ExecuteScheduledRule returned error: %v", err)
	}

	updated, err := ruleRepo.GetByID(context.Background(), "ws-1", rule.ID)
	if err != nil {
		t.Fatalf("get rule: %v", err)
	}
	if updated == nil || updated.Enabled {
		t.Fatalf("expected missing-agent cron rule to be disabled, got %#v", updated)
	}
}

func TestExecuteScheduledRuleStopsOrphanSchedule(t *testing.T) {
	db := setupRuleEngineTestDB(t)
	ruleRepo := repository.NewAutomationRuleRepository(db)
	runEngine := &automationRuleRunEngineStub{}
	engine := NewAutomationRuleEngine(ruleRepo, nil, nil, nil, nil, nil, nil, nil).
		SetRunEngine(runEngine)

	err := engine.ExecuteScheduledRule(context.Background(), "ws-1", "missing-rule")
	if !errors.Is(err, temporalapp.ErrScheduledRuleNotFound) {
		t.Fatalf("ExecuteScheduledRule error = %v, want missing-rule sentinel", err)
	}
	if len(runEngine.stoppedRuleIDs) != 1 || runEngine.stoppedRuleIDs[0] != "missing-rule" {
		t.Fatalf("stopped rule IDs = %#v, want missing-rule", runEngine.stoppedRuleIDs)
	}
}

func TestExecuteScheduledRuleRetriesWhenOrphanTerminationFails(t *testing.T) {
	db := setupRuleEngineTestDB(t)
	ruleRepo := repository.NewAutomationRuleRepository(db)
	stopErr := errors.New("Temporal unavailable")
	runEngine := &automationRuleRunEngineStub{stopErr: stopErr}
	engine := NewAutomationRuleEngine(ruleRepo, nil, nil, nil, nil, nil, nil, nil).
		SetRunEngine(runEngine)

	err := engine.ExecuteScheduledRule(context.Background(), "ws-1", "missing-rule")
	if !errors.Is(err, stopErr) {
		t.Fatalf("ExecuteScheduledRule error = %v, want termination error", err)
	}
	if errors.Is(err, temporalapp.ErrScheduledRuleNotFound) {
		t.Fatal("termination failure must remain transient so cleanup is retried")
	}
}

func TestGitHubRunEventContextIncludesPullRequest(t *testing.T) {
	event := model.AutomationEvent{
		TriggerType:       model.TriggerGitHubPRMerged,
		RepoFullName:      "acme/api",
		RepositoryID:      "repo-1",
		Branch:            "feature/review",
		BaseBranch:        "main",
		PullRequestNumber: 42,
	}

	got := githubRunEventContext(event)
	if got.EventType != "pull_request_merged" || got.RepoFullName != "acme/api" || got.RepositoryID != "repo-1" {
		t.Fatalf("github context = %#v", got)
	}
	if got.PullRequest == nil {
		t.Fatal("expected pull request context")
	}
	if got.PullRequest.Number != 42 || got.PullRequest.BaseBranch != "main" || got.PullRequest.HeadBranch != "feature/review" {
		t.Fatalf("pull request context = %#v", got.PullRequest)
	}
}

func TestResolveRunBranchOverrides_UsesEffectiveTaskDeliveryBranches(t *testing.T) {
	db := setupRuleEngineTestDB(t)
	ctx := context.Background()

	if err := db.Create(&model.Workspace{
		ID:           "ws-1",
		Name:         "Workspace",
		Slug:         "workspace",
		WorkspaceKey: "HLP",
		OwnerID:      "owner-1",
	}).Error; err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := db.Create(&model.GitRepository{
		ID:            "repo-1",
		WorkspaceID:   "ws-1",
		IntegrationID: "integration-1",
		Provider:      "github",
		ExternalID:    "101",
		FullName:      "acme/api",
		DefaultBranch: "main",
		Permissions:   json.RawMessage(`{}`),
		Private:       true,
		Selected:      true,
	}).Error; err != nil {
		t.Fatalf("create repository: %v", err)
	}
	teamID := "team-1"
	if err := db.Create(&model.PMTeamRepoDefault{
		ID:             "team-default-1",
		TeamID:         teamID,
		RepositoryID:   "repo-1",
		BaseBranch:     "develop",
		BranchTemplate: "{task_key}-{slug}",
		AutoSyncStates: true,
	}).Error; err != nil {
		t.Fatalf("create team repo default: %v", err)
	}
	task := &model.PMTask{
		ID:              "task-1",
		WorkspaceID:     "ws-1",
		DisplayID:       42,
		Name:            "Ship review agent",
		TaskType:        "feature",
		WorkflowID:      "wf-1",
		WorkflowStateID: "state-1",
		TeamID:          &teamID,
		Priority:        "none",
		Severity:        "none",
	}
	if err := db.Exec(
		`INSERT INTO pm_tasks (id, workspace_id, display_id, name, task_type, workflow_id, workflow_state_id, team_id, priority, position, started, completed, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID,
		task.WorkspaceID,
		task.DisplayID,
		task.Name,
		task.TaskType,
		task.WorkflowID,
		task.WorkflowStateID,
		teamID,
		task.Priority,
		0,
		false,
		false,
		time.Now().UTC(),
		time.Now().UTC(),
	).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	gitSvc := NewGitService(
		nil,
		repository.NewGitRepositoryRepository(db),
		nil,
		repository.NewTaskDeliveryTargetRepository(db),
		repository.NewSettingsRepository(db),
		repository.NewWorkspaceRepository(db),
		repository.NewOrganizationRepository(db),
		repository.NewPMTaskRepository(db),
		nil,
		nil,
		nil,
		"",
		"",
		"",
	)
	engine := NewAutomationRuleEngine(
		repository.NewAutomationRuleRepository(db),
		repository.NewPMTaskRepository(db),
		repository.NewPMWorkflowRepository(db),
		repository.NewTaskDeliveryTargetRepository(db),
		gitSvc,
		nil,
		nil,
		nil,
	)

	baseBranch, workingBranch, err := engine.resolveRunBranchOverrides(ctx, model.AutomationEvent{
		WorkspaceID: "ws-1",
		TaskID:      task.ID,
		TargetType:  "task",
		TargetID:    task.ID,
	}, task, model.ActionConfigRunAgent{
		BaseBranch:    "{base_branch}",
		WorkingBranch: "{task_branch}",
	})
	if err != nil {
		t.Fatalf("resolveRunBranchOverrides returned error: %v", err)
	}
	if baseBranch != "develop" {
		t.Fatalf("baseBranch = %q, want develop", baseBranch)
	}
	if workingBranch != "hlp-42-ship-review-agent" {
		t.Fatalf("workingBranch = %q, want hlp-42-ship-review-agent", workingBranch)
	}
}

func TestExecuteMergeBranchUpdatesDeliveryStatusAfterSuccessfulMerge(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)
	app := &fakeGitHubAppClient{}
	gitSvc := newGitDeliveryStatusService(db, app)
	engine := NewAutomationRuleEngine(
		nil,
		nil,
		nil,
		repository.NewTaskDeliveryTargetRepository(db),
		gitSvc,
		nil,
		NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
	)

	err := engine.executeMergeBranch(context.Background(),
		&model.AutomationRule{ID: "rule-1", Name: "Merge reviewed branch"},
		model.AutomationEvent{WorkspaceID: "ws-1", TaskID: "task-1"},
		nil,
		model.ActionConfigMergeBranch{TargetBranch: "{base_branch}"},
	)
	if err != nil {
		t.Fatalf("executeMergeBranch returned error: %v", err)
	}
	if len(app.mergeCalls) != 1 {
		t.Fatalf("merge calls = %d (%s), want 1", len(app.mergeCalls), formatMergeCalls(app.mergeCalls))
	}
	if app.mergeCalls[0].Base != "main" || app.mergeCalls[0].Head != "hel-31-fix-merge-status" {
		t.Fatalf("unexpected merge call: %#v", app.mergeCalls[0])
	}
	assertMergedDeliveryStatus(t, db)
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
				TriggerType:   model.TriggerTaskStateEntered,
				TriggerConfig: json.RawMessage(`{"state_id":"state-1"}`),
			},
			event:     model.AutomationEvent{StateID: "state-1"},
			wantMatch: true,
		},
		{
			name: "state_type match",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerTaskStateEntered,
				TriggerConfig: json.RawMessage(`{"state_type":"started"}`),
			},
			event:     model.AutomationEvent{StateID: "state-1"},
			wantMatch: true,
		},
		{
			name: "state_type mismatch",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerTaskStateEntered,
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
			name: "doc published matches document target",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerDocPublished,
				TriggerConfig: json.RawMessage(`{}`),
			},
			event:     model.AutomationEvent{TargetType: "document", TargetID: "doc-1"},
			wantMatch: true,
		},
		{
			name: "doc published requires target",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerDocPublished,
				TriggerConfig: json.RawMessage(`{}`),
			},
			event:     model.AutomationEvent{},
			wantMatch: false,
		},
		{
			name: "ai section regenerated matches section target",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerAISectionRegenerated,
				TriggerConfig: json.RawMessage(`{}`),
			},
			event:     model.AutomationEvent{TargetType: "ai_section", TargetID: "section-1"},
			wantMatch: true,
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
			name: "github closed matches base branch",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerGitHubPRClosed,
				TriggerConfig: json.RawMessage(`{"base_branch":"main"}`),
			},
			event:     model.AutomationEvent{BaseBranch: "main", RepoFullName: "acme/api"},
			wantMatch: true,
		},
		{
			name: "github closed repo mismatch",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerGitHubPRClosed,
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
			name: "github release matches tag pattern and kind",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerGitHubReleasePub,
				TriggerConfig: json.RawMessage(`{"tag_pattern":"v1.*","release_kinds":["minor"]}`),
			},
			event:     model.AutomationEvent{TagName: "v1.4.0", RepoFullName: "acme/api", ReleaseKind: "minor"},
			wantMatch: true,
		},
		{
			name: "github release blocks prerelease by default",
			rule: model.AutomationRule{
				TriggerType:   model.TriggerGitHubReleasePub,
				TriggerConfig: json.RawMessage(`{"repo_full_name":"acme/api"}`),
			},
			event:     model.AutomationEvent{TagName: "v1.4.0-rc1", RepoFullName: "acme/api", IsPrerelease: true, ReleaseKind: "prerelease"},
			wantMatch: false,
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
			triggerConfig: json.RawMessage(`{"preset":"hourly"}`),
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
			triggerType:   model.TriggerTaskStateEntered,
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
			name:          "github pr closed requires explicit target",
			triggerType:   model.TriggerGitHubPRClosed,
			triggerConfig: json.RawMessage(`{"base_branch":"main"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "valid github pr closed with explicit target",
			triggerType:   model.TriggerGitHubPRClosed,
			triggerConfig: json.RawMessage(`{"base_branch":"main"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1","target_type":"repository","target_id":"repo-1"}`),
			wantErr:       false,
		},
		{
			name:          "github pr closed requires at least one filter",
			triggerType:   model.TriggerGitHubPRClosed,
			triggerConfig: json.RawMessage(`{}`),
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
			name:          "github release published can rely on event target",
			triggerType:   model.TriggerGitHubReleasePub,
			triggerConfig: json.RawMessage(`{"tag_name":"v1.2.3"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
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
			name:          "github release published supports release kind filter only",
			triggerType:   model.TriggerGitHubReleasePub,
			triggerConfig: json.RawMessage(`{"release_kinds":["minor"]}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       false,
		},
		{
			name:          "release docs output requires space id",
			triggerType:   model.TriggerGitHubReleasePub,
			triggerConfig: json.RawMessage(`{"repo_full_name":"acme/api"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1","output":{"type":"docs_document"}}`),
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
			triggerConfig: json.RawMessage(`{"preset":"hourly"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1","target_type":"epic","target_id":"epic-1"}`),
			wantErr:       false,
		},
		{
			name:          "start_agent_run missing agent_id",
			triggerType:   model.TriggerTaskStateEntered,
			triggerConfig: json.RawMessage(`{"state_type":"done"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{}`),
			wantErr:       true,
		},
		{
			name:          "start_agent_run with partial explicit target is invalid",
			triggerType:   model.TriggerTaskStateEntered,
			triggerConfig: json.RawMessage(`{"state_type":"done"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1","target_type":"task"}`),
			wantErr:       true,
		},
		{
			name:          "cron start_agent_run without explicit target is valid",
			triggerType:   model.TriggerCron,
			triggerConfig: json.RawMessage(`{"schedule":"0 * * * *"}`),
			actionType:    model.ActionStartAgentRun,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       false,
		},
		{
			name:          "legacy start_flow rejected",
			triggerType:   model.TriggerTaskStateEntered,
			triggerConfig: json.RawMessage(`{"state_type":"done"}`),
			actionType:    model.ActionStartFlow,
			actionConfig:  json.RawMessage(`{"template_id":"tmpl-1"}`),
			wantErr:       true,
		},
		{
			name:          "legacy run_agent rejected",
			triggerType:   model.TriggerTaskStateEntered,
			triggerConfig: json.RawMessage(`{"state_type":"done"}`),
			actionType:    model.ActionRunAgent,
			actionConfig:  json.RawMessage(`{"agent_id":"agent-1"}`),
			wantErr:       true,
		},
		{
			name:          "state_entered with state_type only is valid",
			triggerType:   model.TriggerTaskStateEntered,
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

func TestResolveCronTriggerConfig(t *testing.T) {
	tests := []struct {
		name         string
		cfg          model.TriggerConfigCron
		wantSchedule string
		wantPreset   string
		wantErr      bool
	}{
		{
			name:         "explicit schedule wins",
			cfg:          model.TriggerConfigCron{Schedule: "15 * * * *"},
			wantSchedule: "15 * * * *",
		},
		{
			name:         "preset maps to cron",
			cfg:          model.TriggerConfigCron{Preset: "hourly"},
			wantSchedule: "0 * * * *",
			wantPreset:   "hourly",
		},
		{
			name:         "legacy category maps to preset",
			cfg:          model.TriggerConfigCron{Category: "workspace_daily"},
			wantSchedule: "0 0 * * *",
			wantPreset:   "daily",
		},
		{
			name:         "raw cron in legacy category is accepted",
			cfg:          model.TriggerConfigCron{Category: "0 6 * * 1"},
			wantSchedule: "0 6 * * 1",
		},
		{
			name:    "six field cron is rejected",
			cfg:     model.TriggerConfigCron{Schedule: "0 0 6 * * 1"},
			wantErr: true,
		},
		{
			name:    "empty config is rejected",
			cfg:     model.TriggerConfigCron{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSchedule, gotPreset, err := resolveCronTriggerConfig(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveCronTriggerConfig() error = %v", err)
			}
			if gotSchedule != tt.wantSchedule {
				t.Fatalf("schedule = %q, want %q", gotSchedule, tt.wantSchedule)
			}
			if gotPreset != tt.wantPreset {
				t.Fatalf("preset = %q, want %q", gotPreset, tt.wantPreset)
			}
		})
	}
}
