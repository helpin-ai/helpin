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

func TestAutomationInventoryService_AssemblesBuiltIns(t *testing.T) {
	t.Parallel()

	dbName := fmt.Sprintf("file:automation-inventory-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	stmts := []string{
		`CREATE TABLE workspace_teams (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			handle TEXT,
			description TEXT,
			manager_id TEXT,
			team_type TEXT NOT NULL DEFAULT 'engineering',
			default_task_type TEXT NOT NULL DEFAULT 'feature',
			docs_publisher_enabled BOOLEAN NOT NULL DEFAULT 0,
			sprints_enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous'
		)`,
		`CREATE TABLE pm_automations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			automation_type TEXT NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT 0,
			team_id TEXT,
			config_state_id TEXT,
			config_int INTEGER,
			config_int2 INTEGER,
			config_int3 INTEGER,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_email_accounts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			member_id TEXT NOT NULL,
			provider TEXT NOT NULL,
			email_address TEXT NOT NULL,
			normalized_email_address TEXT,
			access_token_encrypted TEXT,
			refresh_token_encrypted TEXT,
			sync_state TEXT,
			last_history_id TEXT,
			last_synced_at DATETIME,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'connected',
			disconnected_at DATETIME,
			oauth_state TEXT,
			token_expires_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			preset_key TEXT,
			preset_version_key TEXT,
			source_preset_key TEXT,
			source_preset_version_key TEXT,
			role TEXT,
			status TEXT NOT NULL,
			runtime_kind TEXT,
			skills TEXT,
			trigger_mode TEXT,
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT x'7b7d',
			system_prompt TEXT,
			instruction_template_version TEXT NOT NULL DEFAULT '',
			planning_notes TEXT,
			tools TEXT,
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER,
			active_task_id TEXT,
			team_id TEXT,
			allowed_tools TEXT,
			allowed_commands TEXT,
			allowed_targets TEXT,
			schedule TEXT,
			approval_mode TEXT NOT NULL DEFAULT 'preset_default',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			task_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL,
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			parent_run_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL,
			triggered_by_user_id TEXT,
			status TEXT NOT NULL,
			workflow_id TEXT,
			workflow_run_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input TEXT,
			output_summary TEXT,
			tokens_used INTEGER,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE automation_rules (
			id TEXT PRIMARY KEY,
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
		`CREATE TABLE automation_health_snapshots (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			catalog_id TEXT NOT NULL,
			scope_type TEXT NOT NULL,
			scope_id TEXT NOT NULL,
			status TEXT NOT NULL,
			last_seen_at DATETIME,
			last_success_at DATETIME,
			last_error_at DATETIME,
			last_error_message TEXT,
			metrics TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}

	ctx := context.Background()
	workspaceID := "ws-automation"
	teamID := "team-1"

	if err := db.Create(&model.WorkspaceTeam{ID: teamID, WorkspaceID: workspaceID, Name: "Core Product"}).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := db.Create(&model.CRMEmailAccount{
		ID:           "acct-1",
		WorkspaceID:  workspaceID,
		MemberID:     "member-1",
		Provider:     model.CRMEmailProviderGmail,
		EmailAddress: "owner@example.com",
		IsActive:     true,
		Status:       model.CRMEmailAccountStatusConnected,
	}).Error; err != nil {
		t.Fatalf("create email account: %v", err)
	}
	if err := db.Create(&model.PMAutomation{
		WorkspaceID:    workspaceID,
		AutomationType: model.PMAutomationTypeEpicAutoStart,
		Enabled:        true,
	}).Error; err != nil {
		t.Fatalf("create workspace pm automation: %v", err)
	}
	if err := db.Create(&model.PMAutomation{
		WorkspaceID:    workspaceID,
		AutomationType: model.PMAutomationTypeSprintAutoCreate,
		Enabled:        true,
		TeamID:         &teamID,
		ConfigInt:      intPtr(2),
		ConfigInt2:     intPtr(2),
	}).Error; err != nil {
		t.Fatalf("create team pm automation: %v", err)
	}

	now := time.Now().UTC()
	if err := db.Create(&model.AutomationHealthSnapshot{
		WorkspaceID:   workspaceID,
		CatalogID:     "crm.buyer_signal_ingestion",
		ScopeType:     model.AutomationScopeWorkspace,
		ScopeID:       workspaceID,
		Status:        model.AutomationHealthHealthy,
		LastSeenAt:    &now,
		LastSuccessAt: &now,
	}).Error; err != nil {
		t.Fatalf("create automation health snapshot: %v", err)
	}
	schedule := "0 * * * *"
	if err := db.Create(&model.Agent{
		ID:          "agent-1",
		WorkspaceID: workspaceID,
		Name:        "Scheduled Agent",
		Status:      "idle",
		RuntimeKind: "opencode",
		TriggerMode: "manual",
		Schedule:    &schedule,
	}).Error; err != nil {
		t.Fatalf("create scheduled agent: %v", err)
	}
	if err := db.Create(&model.AutomationRule{
		ID:            "rule-cron-1",
		WorkspaceID:   workspaceID,
		Name:          "Hourly Repo Sweep",
		Enabled:       true,
		TriggerType:   model.TriggerCron,
		TriggerConfig: json.RawMessage(`{}`),
		ActionType:    model.ActionStartAgentRun,
		ActionConfig:  json.RawMessage(`{"agent_id":"agent-1","target_type":"repository","target_id":"repo-1"}`),
	}).Error; err != nil {
		t.Fatalf("create cron automation rule: %v", err)
	}

	svc := NewAutomationInventoryService(
		repository.NewSettingsRepository(db),
		repository.NewPMAutomationRepository(db),
		repository.NewCRMEmailRepository(db),
		repository.NewAutomationHealthRepository(db),
		repository.NewAutomationRuleRepository(db),
		repository.NewAgentTriggerExecutionRepository(db),
		repository.NewAgentRepository(db),
		nil,
		nil,
	)

	result, err := svc.GetWorkspaceInventory(ctx, workspaceID)
	if err != nil {
		t.Fatalf("get inventory: %v", err)
	}

	if len(result.Groups) != 2 {
		t.Fatalf("expected 2 inventory groups, got %d", len(result.Groups))
	}

	itemsByCatalog := make(map[string][]model.AutomationInventoryItem)
	for _, item := range result.Items {
		itemsByCatalog[item.CatalogID] = append(itemsByCatalog[item.CatalogID], item)
	}

	if got := len(itemsByCatalog["crm.buyer_signal_ingestion"]); got != 1 {
		t.Fatalf("expected 1 buyer signal ingestion item, got %d", got)
	}
	if got := len(itemsByCatalog["crm.contact_summary_refresh"]); got != 1 {
		t.Fatalf("expected 1 contact summary item, got %d", got)
	}
	if got := len(itemsByCatalog["crm.deal_summary_refresh"]); got != 1 {
		t.Fatalf("expected 1 deal summary item, got %d", got)
	}
	if got := len(itemsByCatalog["pm.epic_auto_start"]); got != 1 {
		t.Fatalf("expected 1 epic auto-start item, got %d", got)
	}
	if got := len(itemsByCatalog["pm.epic_auto_complete"]); got != 1 {
		t.Fatalf("expected 1 epic auto-complete item, got %d", got)
	}
	if got := len(itemsByCatalog["pm.sprint_auto_create"]); got != 1 {
		t.Fatalf("expected 1 sprint auto-create item, got %d", got)
	}
	if got := len(itemsByCatalog["pm.sprint_move_unfinished"]); got != 1 {
		t.Fatalf("expected 1 sprint move-unfinished item, got %d", got)
	}

	buyerSignal := itemsByCatalog["crm.buyer_signal_ingestion"][0]
	if !buyerSignal.Enabled {
		t.Fatal("expected buyer signal ingestion to be enabled with an active mailbox")
	}
	if buyerSignal.Health.Status != model.AutomationHealthHealthy {
		t.Fatalf("expected buyer signal ingestion health to be healthy, got %s", buyerSignal.Health.Status)
	}

	epicAutoComplete := itemsByCatalog["pm.epic_auto_complete"][0]
	if epicAutoComplete.Enabled {
		t.Fatal("expected epic auto-complete to be inactive without a config row")
	}
	if epicAutoComplete.Health.Status != model.AutomationHealthInactive {
		t.Fatalf("expected epic auto-complete health to be inactive, got %s", epicAutoComplete.Health.Status)
	}

	sprintAutoCreate := itemsByCatalog["pm.sprint_auto_create"][0]
	if sprintAutoCreate.ScopeType != model.AutomationScopeTeam || sprintAutoCreate.ScopeID != teamID {
		t.Fatalf("expected sprint auto-create to be team-scoped for %s, got %s/%s", teamID, sprintAutoCreate.ScopeType, sprintAutoCreate.ScopeID)
	}

	triggerCatalog, err := svc.triggerCatalogItems(ctx, workspaceID)
	if err != nil {
		t.Fatalf("get trigger catalog: %v", err)
	}

	triggerCounts := make(map[string]int, len(triggerCatalog))
	for _, entry := range triggerCatalog {
		triggerCounts[entry.ID] = entry.BindingCount
	}

	if got := triggerCounts["agent.schedule"]; got != 1 {
		t.Fatalf("expected agent.schedule count 1, got %d", got)
	}
	if got := triggerCounts["automation_rule.cron"]; got != 1 {
		t.Fatalf("expected automation_rule.cron count 1, got %d", got)
	}
}

func intPtr(value int) *int {
	return &value
}
