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
		`CREATE TABLE agent_team_access (
			agent_id TEXT NOT NULL,
			team_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (agent_id, team_id)
		)`,
		`CREATE TABLE crm_email_accounts (
			signature TEXT NOT NULL DEFAULT '',
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
 ai_profile_id TEXT,
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			icon_key TEXT NOT NULL DEFAULT '',
			preset_key TEXT,
			preset_version_key TEXT,
			source_preset_key TEXT,
			source_preset_version_key TEXT,
			source_template_id TEXT,
			source_template_key TEXT NOT NULL DEFAULT '',
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
			active_version_id TEXT,
			role TEXT,
			status TEXT NOT NULL,
			runtime_kind TEXT,
			model_tier TEXT NOT NULL DEFAULT '',
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
			model_tier TEXT NOT NULL DEFAULT '',
			parent_run_id TEXT,
			dock_chat_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL,
			triggered_by_user_id TEXT,
			status TEXT NOT NULL,
			workflow_id TEXT,
			workflow_run_id TEXT,
			external_runtime TEXT,
			external_runtime_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			agent_version_id TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input TEXT,
			output_summary TEXT,
			cached_input_tokens INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
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
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
			position INTEGER NOT NULL DEFAULT 0,
			stop_on_match BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_trigger_executions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			actor_id TEXT,
			binding_id TEXT NOT NULL,
			binding_kind TEXT NOT NULL,
			trigger_type TEXT,
			reference_id TEXT,
			reference_type TEXT,
			target_type TEXT,
			target_id TEXT,
			run_id TEXT,
			status TEXT NOT NULL,
			error_message TEXT,
			fired_at DATETIME NOT NULL,
			started_at DATETIME,
			completed_at DATETIME,
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
	if err := db.Create(&model.Agent{
		ID:          "agent-1",
		WorkspaceID: workspaceID,
		Name:        "Scheduled Agent",
		Status:      "idle",
		RuntimeKind: "opencode",
		TriggerMode: "manual",
	}).Error; err != nil {
		t.Fatalf("create scheduled agent: %v", err)
	}
	if err := db.Create(&model.AutomationRule{
		ID:            "rule-cron-1",
		WorkspaceID:   workspaceID,
		Name:          "Hourly Repo Sweep",
		Enabled:       true,
		TriggerType:   model.TriggerCron,
		TriggerConfig: json.RawMessage(`{"schedule":"0 * * * *"}`),
		ActionType:    model.ActionStartAgentRun,
		ActionConfig:  json.RawMessage(`{"agent_id":"agent-1","target_type":"repository","target_id":"repo-1"}`),
	}).Error; err != nil {
		t.Fatalf("create cron automation rule: %v", err)
	}
	completedAt := now.Add(5 * time.Minute)
	if err := db.Create(&model.AgentTriggerExecution{
		ID:            "exec-rule-cron-1",
		WorkspaceID:   workspaceID,
		AgentID:       "agent-1",
		BindingID:     "automation_rule.cron",
		BindingKind:   "automation_rule",
		TriggerType:   testStringPtr(model.TriggerCron),
		ReferenceID:   testStringPtr("rule-cron-1"),
		ReferenceType: testStringPtr("automation_rule"),
		RunID:         testStringPtr("run-rule-cron-1"),
		Status:        model.AgentTriggerExecutionStatusCompleted,
		FiredAt:       now,
		CompletedAt:   &completedAt,
	}).Error; err != nil {
		t.Fatalf("create automation trigger execution: %v", err)
	}
	previousCompletedAt := now.Add(-25 * time.Hour)
	if err := db.Create(&model.AgentTriggerExecution{
		ID:            "exec-rule-cron-previous",
		WorkspaceID:   workspaceID,
		AgentID:       "agent-1",
		BindingID:     "automation_rule.cron",
		BindingKind:   "automation_rule",
		TriggerType:   testStringPtr(model.TriggerCron),
		ReferenceID:   testStringPtr("rule-cron-1"),
		ReferenceType: testStringPtr("automation_rule"),
		Status:        model.AgentTriggerExecutionStatusFailed,
		FiredAt:       now.Add(-25 * time.Hour),
		CompletedAt:   &previousCompletedAt,
	}).Error; err != nil {
		t.Fatalf("create previous automation trigger execution: %v", err)
	}

	svc := NewAutomationInventoryService(
		repository.NewSettingsRepository(db),
		repository.NewPMAutomationRepository(db),
		repository.NewCRMEmailRepository(db),
		repository.NewAutomationHealthRepository(db),
		repository.NewAutomationRuleRepository(db),
		repository.NewAgentTriggerExecutionRepository(db),
		repository.NewAgentRunRepository(db),
		repository.NewAgentRepository(db),
		nil,
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
		t.Fatalf("expected 1 CRM signal ingestion item, got %d", got)
	}
	if got := len(itemsByCatalog["crm.contact_summary_refresh"]); got != 1 {
		t.Fatalf("expected 1 contact summary item, got %d", got)
	}
	if got := len(itemsByCatalog["crm.deal_summary_refresh"]); got != 1 {
		t.Fatalf("expected 1 deal summary item, got %d", got)
	}
	if got := len(itemsByCatalog["crm.company_summary_refresh"]); got != 1 {
		t.Fatalf("expected 1 company summary item, got %d", got)
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

	signalIngestion := itemsByCatalog["crm.buyer_signal_ingestion"][0]
	if !signalIngestion.Enabled {
		t.Fatal("expected CRM signal ingestion to be enabled with an active mailbox")
	}
	if signalIngestion.Health.Status != model.AutomationHealthHealthy {
		t.Fatalf("expected CRM signal ingestion health to be healthy, got %s", signalIngestion.Health.Status)
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

	if got := len(itemsByCatalog["automation_rule"]); got != 1 {
		t.Fatalf("expected 1 automation_rule item, got %d", got)
	}
	ruleItem := itemsByCatalog["automation_rule"][0]
	if ruleItem.Health.LastSuccessAt == nil || !ruleItem.Health.LastSuccessAt.Equal(completedAt) {
		t.Fatalf("expected automation rule last_success_at %v, got %v", completedAt, ruleItem.Health.LastSuccessAt)
	}
	if got := ruleItem.Health.Metrics["total_runs"]; got != float64(2) && got != int64(2) && got != 2 {
		t.Fatalf("expected automation rule total_runs 2, got %#v", got)
	}
	if got := ruleItem.Health.Metrics["error_runs"]; got != float64(1) && got != int64(1) && got != 1 {
		t.Fatalf("expected automation rule error_runs 1, got %#v", got)
	}
	if got, ok := ruleItem.Health.Metrics["next_run_at"].(string); !ok || got == "" {
		t.Fatalf("expected automation rule next_run_at, got %#v", ruleItem.Health.Metrics["next_run_at"])
	}
	if got := ruleItem.Health.Metrics["last_execution_id"]; got != "exec-rule-cron-1" {
		t.Fatalf("expected automation rule last_execution_id exec-rule-cron-1, got %#v", got)
	}
	if got := ruleItem.Health.Metrics["last_run_id"]; got != "run-rule-cron-1" {
		t.Fatalf("expected automation rule last_run_id run-rule-cron-1, got %#v", got)
	}

	triggerCatalog, err := svc.triggerCatalogItems(ctx, workspaceID)
	if err != nil {
		t.Fatalf("get trigger catalog: %v", err)
	}

	triggerCounts := make(map[string]int, len(triggerCatalog))
	for _, entry := range triggerCatalog {
		triggerCounts[entry.ID] = entry.BindingCount
	}

	if got := triggerCounts["automation_rule.cron"]; got != 1 {
		t.Fatalf("expected automation_rule.cron count 1, got %d", got)
	}
}

func TestAutomationActivityIncludesRunsWithoutTriggerExecutions(t *testing.T) {
	t.Parallel()

	dbName := fmt.Sprintf("file:automation-activity-runs-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE agents (
 ai_profile_id TEXT,
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			icon_key TEXT NOT NULL DEFAULT '',
			preset_key TEXT,
			source_template_key TEXT NOT NULL DEFAULT '',
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
			active_version_id TEXT,
			status TEXT NOT NULL,
			runtime_kind TEXT,
			model_tier TEXT NOT NULL DEFAULT '',
			skills BLOB NOT NULL DEFAULT x'5b5d',
			trigger_mode TEXT,
			execution_config BLOB NOT NULL DEFAULT x'7b7d',
			instruction_template_version TEXT NOT NULL DEFAULT '',
			allowed_tools BLOB NOT NULL DEFAULT x'5b5d',
			allowed_commands BLOB NOT NULL DEFAULT x'5b5d',
			allowed_targets BLOB NOT NULL DEFAULT x'5b5d',
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
			model_tier TEXT NOT NULL DEFAULT '',
			invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			parent_run_id TEXT,
			dock_chat_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL DEFAULT 'not_required',
			pause_reason TEXT NOT NULL DEFAULT 'none',
			triggered_by_user_id TEXT,
			status TEXT NOT NULL,
			workflow_id TEXT,
			workflow_run_id TEXT,
			external_runtime TEXT,
			external_runtime_id TEXT,
			task_queue TEXT,
			runner_pool TEXT,
			agent_version_id TEXT,
			repository_id TEXT,
			repo_full_name TEXT,
			base_branch TEXT,
			working_branch TEXT,
			delivery_target_id TEXT,
			execution_stage TEXT,
			last_heartbeat_at DATETIME,
			input BLOB,
			output_summary BLOB NOT NULL DEFAULT x'7b7d',
			cached_input_tokens INTEGER NOT NULL DEFAULT 0,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			tokens_used INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_trigger_executions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			actor_id TEXT,
			binding_id TEXT NOT NULL,
			binding_kind TEXT NOT NULL,
			trigger_type TEXT,
			reference_id TEXT,
			reference_type TEXT,
			target_type TEXT,
			target_id TEXT,
			run_id TEXT,
			status TEXT NOT NULL,
			error_message TEXT,
			fired_at DATETIME NOT NULL,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}

	ctx := context.Background()
	workspaceID := "ws-activity"
	now := time.Now().UTC()
	input, _ := json.Marshal(model.AgentRunInputPayload{
		Trigger: &model.AgentRunTriggerContext{
			Source:      model.AgentRunTriggerSourceCommandBar,
			TriggerType: model.AgentRunTriggerTypeCommandBar,
			FiredAt:     &now,
		},
		Target: &model.AgentRunTargetContext{TargetType: "workspace", TargetID: workspaceID},
	})
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, name, status, runtime_kind, created_at, updated_at
	) VALUES (?, ?, ?, 'idle', 'native_sdk', ?, ?)`, "agent-command", workspaceID, "Command Agent", now, now).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := db.Exec(`INSERT INTO agent_runs (
		id, workspace_id, agent_id, target_type, target_id, runtime_kind, status, input, created_at, updated_at
	) VALUES (?, ?, ?, 'workspace', ?, 'native_sdk', 'queued', ?, ?, ?)`,
		"run-command", workspaceID, "agent-command", workspaceID, []byte(input), now, now,
	).Error; err != nil {
		t.Fatalf("create command run: %v", err)
	}

	svc := NewAutomationInventoryService(
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewAgentTriggerExecutionRepository(db),
		repository.NewAgentRunRepository(db),
		repository.NewAgentRepository(db),
		nil,
		nil,
		nil,
	)
	result, err := svc.ListTriggerExecutions(ctx, workspaceID, model.TriggerExecutionListFilters{}, model.PMPagination{Page: 1, PerPage: 25})
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	if result.Total != 1 || len(result.Data) != 1 {
		t.Fatalf("expected one synthetic run activity row, got total=%d data=%#v", result.Total, result.Data)
	}
	item := result.Data[0]
	if item.ExecutionID != "run:run-command" || item.RunID == nil || *item.RunID != "run-command" {
		t.Fatalf("expected synthetic run id, got %#v", item)
	}
	if item.BindingKind != model.AgentRunTriggerSourceCommandBar || item.BindingID != "command_bar.run" {
		t.Fatalf("expected command-bar activity source, got %s/%s", item.BindingKind, item.BindingID)
	}

	result, err = svc.ListTriggerExecutions(ctx, workspaceID, model.TriggerExecutionListFilters{BindingKind: testStringPtr(model.AgentRunTriggerSourceCommandBar)}, model.PMPagination{Page: 1, PerPage: 25})
	if err != nil {
		t.Fatalf("list command-bar activity by source: %v", err)
	}
	if result.Total != 1 || len(result.Data) != 1 || result.Data[0].BindingID != "command_bar.run" {
		t.Fatalf("expected command-bar source filter to find synthetic row, got total=%d data=%#v", result.Total, result.Data)
	}

	result, err = svc.ListTriggerExecutions(ctx, workspaceID, model.TriggerExecutionListFilters{RunID: testStringPtr("run-command")}, model.PMPagination{Page: 1, PerPage: 25})
	if err != nil {
		t.Fatalf("list activity by run_id: %v", err)
	}
	if result.Total != 1 || len(result.Data) != 1 {
		t.Fatalf("expected run_id filter to find synthetic row, got total=%d data=%#v", result.Total, result.Data)
	}

	manualInput, _ := json.Marshal(model.AgentRunInputPayload{
		Trigger: &model.AgentRunTriggerContext{
			Source:      model.AgentRunTriggerSourceManual,
			TriggerType: model.AgentRunTriggerTypeManual,
			FiredAt:     &now,
		},
		Target: &model.AgentRunTargetContext{TargetType: "task", TargetID: "task-1"},
	})
	if err := db.Exec(`INSERT INTO agent_runs (
		id, workspace_id, agent_id, target_type, target_id, runtime_kind, status, input, created_at, updated_at
	) VALUES (?, ?, ?, 'task', ?, 'native_sdk', 'queued', ?, ?, ?)`,
		"run-manual-task", workspaceID, "agent-command", "task-1", []byte(manualInput), now.Add(time.Minute), now.Add(time.Minute),
	).Error; err != nil {
		t.Fatalf("create manual task run: %v", err)
	}

	result, err = svc.ListTriggerExecutions(ctx, workspaceID, model.TriggerExecutionListFilters{BindingID: testStringPtr("manual.task_run")}, model.PMPagination{Page: 1, PerPage: 25})
	if err != nil {
		t.Fatalf("list manual task activity by binding: %v", err)
	}
	if result.Total != 1 || len(result.Data) != 1 {
		t.Fatalf("expected manual task binding filter to find synthetic row, got total=%d data=%#v", result.Total, result.Data)
	}
	item = result.Data[0]
	if item.BindingKind != model.AgentRunTriggerSourceManual || item.BindingID != "manual.task_run" || item.TargetType == nil || *item.TargetType != "task" {
		t.Fatalf("expected manual task activity source, got %#v", item)
	}
}

func TestAutomationActivityResolvesTargetDisplayInfo(t *testing.T) {
	dbName := fmt.Sprintf("file:automation-activity-targets-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	stmts := []string{
		`CREATE TABLE agents (
 ai_profile_id TEXT,
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			icon_key TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			runtime_kind TEXT,
			model_tier TEXT NOT NULL DEFAULT '',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_trigger_executions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			actor_id TEXT,
			binding_id TEXT NOT NULL,
			binding_kind TEXT NOT NULL,
			trigger_type TEXT,
			reference_id TEXT,
			reference_type TEXT,
			target_type TEXT,
			target_id TEXT,
			run_id TEXT,
			status TEXT NOT NULL,
			error_message TEXT,
			fired_at DATETIME NOT NULL,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			task_type TEXT NOT NULL DEFAULT 'feature',
			workflow_id TEXT NOT NULL DEFAULT 'workflow-1',
			workflow_state_id TEXT NOT NULL DEFAULT 'state-1',
			priority TEXT NOT NULL DEFAULT 'none',
			severity TEXT NOT NULL DEFAULT 'none',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			health TEXT NOT NULL DEFAULT 'none',
			planning_state TEXT NOT NULL DEFAULT 'not_started',
			spec_clarifications TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			visibility TEXT NOT NULL,
			sort_key TEXT NOT NULL,
			created_by TEXT NOT NULL,
			deleted_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE support_conversations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			subject TEXT,
			status TEXT NOT NULL,
			priority TEXT NOT NULL,
			channel TEXT NOT NULL,
			source TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_contacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id TEXT NOT NULL,
			first_name TEXT NOT NULL,
			last_name TEXT,
			email TEXT,
			lifecycle_stage TEXT NOT NULL,
			lead_status TEXT NOT NULL,
			custom_properties TEXT,
			email_status TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_deals (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id TEXT NOT NULL,
			name TEXT NOT NULL,
			pipeline_id TEXT NOT NULL,
			stage_id TEXT NOT NULL,
			currency TEXT NOT NULL,
			commercial_motion TEXT,
			custom_properties TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			display_name TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT,
			password_hash TEXT,
			full_name TEXT,
			email_verified_at DATETIME,
			google_subject TEXT,
			default_workspace_id TEXT,
			totp_secret_encrypted TEXT,
			totp_verified BOOLEAN NOT NULL DEFAULT 0,
			recovery_codes_encrypted TEXT,
			is_platform_admin BOOLEAN NOT NULL DEFAULT 0,
			avatar_url TEXT,
			avatar_style TEXT,
			avatar_seed TEXT,
			avatar_background_mode TEXT,
			avatar_background_color TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspace_members (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			user_id TEXT,
			email TEXT NOT NULL,
			display_name TEXT NOT NULL,
			role TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE git_repositories (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			full_name TEXT NOT NULL,
			deleted_at DATETIME,
			active BOOLEAN NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE support_coverage_gaps (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			dedupe_key TEXT NOT NULL,
			gap_kind TEXT NOT NULL DEFAULT 'content',
			gap_category TEXT NOT NULL DEFAULT 'unknown',
			v1_gap_type TEXT NOT NULL DEFAULT 'needs_review',
			title TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'open',
			confidence REAL NOT NULL DEFAULT 0,
			evidence_count INTEGER NOT NULL DEFAULT 0,
			failure_mode TEXT NOT NULL DEFAULT '',
			source_signal TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '{}',
			first_seen_at DATETIME,
			last_seen_at DATETIME,
			embedding TEXT,
			embedding_provider TEXT NOT NULL DEFAULT '',
			embedding_model TEXT NOT NULL DEFAULT '',
			embedding_version TEXT NOT NULL DEFAULT '',
			embedding_dimensions INTEGER NOT NULL DEFAULT 0,
			embedding_text_hash TEXT NOT NULL DEFAULT '',
			embedding_updated_at DATETIME,
			nearest_content_score REAL NOT NULL DEFAULT 0,
			nearest_content_document_id TEXT,
			nearest_content_title TEXT NOT NULL DEFAULT '',
			nearest_content_checked_at DATETIME,
			impact_score REAL NOT NULL DEFAULT 0,
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
	workspaceID := "ws-activity-targets"
	now := time.Now().UTC()
	if err := db.Exec(`INSERT INTO workspaces (id, name, slug, display_name, created_at, updated_at) VALUES (?, 'TeamPulse', 'teampulse', 'TeamPulse Workspace', ?, ?)`, workspaceID, now, now).Error; err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := db.Exec(`INSERT INTO agents (id, workspace_id, name, status, runtime_kind, created_at, updated_at) VALUES ('agent-1', ?, 'Agent One', 'idle', 'native_sdk', ?, ?)`, workspaceID, now, now).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := db.Exec(`INSERT INTO pm_tasks (id, workspace_id, display_id, name, created_at, updated_at) VALUES ('task-1', ?, 42, 'Fix activity labels', ?, ?)`, workspaceID, now, now).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := db.Exec(`INSERT INTO pm_epics (id, workspace_id, name, created_at, updated_at) VALUES ('epic-1', ?, 'Launch reliability work', ?, ?)`, workspaceID, now, now).Error; err != nil {
		t.Fatalf("create epic: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, space_id, title, status, visibility, sort_key, created_by, created_at, updated_at) VALUES ('doc-1', ?, 'space-1', 'Runbook', 'draft', 'workspace_wide', '~', 'user-1', ?, ?)`, workspaceID, now, now).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id, display_id, subject, status, priority, channel, source, created_at, updated_at) VALUES ('conversation-1', ?, 7, 'Billing question', 'open', 'medium', 'email', 'email', ?, ?)`, workspaceID, now, now).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := db.Exec(`INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, last_name, email, lifecycle_stage, lead_status, custom_properties, email_status, created_at, updated_at) VALUES ('contact-1', ?, 'CON-1', 'Ada', 'Lovelace', 'ada@example.com', 'lead', 'new', '{}', 'valid', ?, ?)`, workspaceID, now, now).Error; err != nil {
		t.Fatalf("create contact: %v", err)
	}
	if err := db.Exec(`INSERT INTO crm_deals (id, workspace_id, display_id, name, pipeline_id, stage_id, currency, custom_properties, created_at, updated_at) VALUES ('deal-1', ?, 'DEAL-1', 'Enterprise rollout', 'pipeline-1', 'stage-1', 'USD', '{}', ?, ?)`, workspaceID, now, now).Error; err != nil {
		t.Fatalf("create deal: %v", err)
	}
	if err := db.Exec(`INSERT INTO git_repositories (id, workspace_id, full_name, active) VALUES ('repo-1', ?, 'acme/api', 1)`, workspaceID).Error; err != nil {
		t.Fatalf("create repository: %v", err)
	}
	if err := db.Exec(`INSERT INTO support_coverage_gaps (id, workspace_id, dedupe_key, title, first_seen_at, last_seen_at, created_at, updated_at) VALUES ('gap-1', ?, 'gap-key', 'Refund policy gap', ?, ?, ?, ?)`, workspaceID, now, now, now, now).Error; err != nil {
		t.Fatalf("create coverage gap: %v", err)
	}

	targets := []struct {
		id         string
		targetType string
		targetID   string
		wantTitle  string
		wantKey    string
	}{
		{"exec-task", "task", "task-1", "Fix activity labels", "42"},
		{"exec-epic", "epic", "epic-1", "Launch reliability work", ""},
		{"exec-doc", "document", "doc-1", "Runbook", ""},
		{"exec-support", "support_conversation", "conversation-1", "Billing question", ""},
		{"exec-contact", "crm_contact", "contact-1", "Ada Lovelace", ""},
		{"exec-deal", "crm_deal", "deal-1", "Enterprise rollout", ""},
		{"exec-repo", "repository", "repo-1", "acme/api", ""},
		{"exec-workspace", "workspace", workspaceID, "TeamPulse", ""},
		{"exec-gap", "support_coverage_gap", "gap-1", "Refund policy gap", ""},
	}
	for idx, target := range targets {
		firedAt := now.Add(time.Duration(idx) * time.Minute)
		if err := db.Create(&model.AgentTriggerExecution{
			ID:          target.id,
			WorkspaceID: workspaceID,
			AgentID:     "agent-1",
			BindingID:   "manual." + target.targetType,
			BindingKind: model.AgentRunTriggerSourceManual,
			TargetType:  &target.targetType,
			TargetID:    &target.targetID,
			Status:      model.AgentTriggerExecutionStatusCompleted,
			FiredAt:     firedAt,
		}).Error; err != nil {
			t.Fatalf("create execution %s: %v", target.id, err)
		}
	}

	svc := NewAutomationInventoryService(
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewAgentTriggerExecutionRepository(db),
		nil,
		repository.NewAgentRepository(db),
		repository.NewWorkspaceRepository(db),
		repository.NewPMTaskRepository(db),
		nil,
	).SetTargetResolvers(
		repository.NewPMEpicRepository(db),
		repository.NewDocsDocumentRepository(db),
		repository.NewSupportConversationRepository(db),
		repository.NewCRMContactRepository(db),
		repository.NewCRMDealRepository(db),
		repository.NewGitRepositoryRepository(db),
		repository.NewSupportCoverageRepository(db),
	)

	result, err := svc.ListTriggerExecutions(ctx, workspaceID, model.TriggerExecutionListFilters{}, model.PMPagination{Page: 1, PerPage: 25})
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}

	byID := make(map[string]model.AutomationTriggerExecutionListItem, len(result.Data))
	for _, item := range result.Data {
		byID[item.ExecutionID] = item
	}
	for _, target := range targets {
		item, ok := byID[target.id]
		if !ok {
			t.Fatalf("missing activity row %s", target.id)
		}
		if item.TargetTitle == nil || *item.TargetTitle != target.wantTitle {
			t.Fatalf("%s target title = %#v, want %q", target.id, item.TargetTitle, target.wantTitle)
		}
		if target.wantKey == "" {
			if item.TargetKey != nil {
				t.Fatalf("%s target key = %#v, want nil", target.id, *item.TargetKey)
			}
			continue
		}
		if item.TargetKey == nil || *item.TargetKey != target.wantKey {
			t.Fatalf("%s target key = %#v, want %q", target.id, item.TargetKey, target.wantKey)
		}
	}
}

func intPtr(value int) *int {
	return &value
}
