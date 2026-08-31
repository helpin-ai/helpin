package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func setupAgentRunActivityTest(t *testing.T) (*AgentService, *gormTestDB) {
	t.Helper()
	db := newTestDB(t)
	createAgentRunActivityTables(t, db)
	svc := &AgentService{
		agentRepo:          repository.NewAgentRepository(db),
		runRepo:            repository.NewAgentRunRepository(db),
		runMessageRepo:     repository.NewAgentRunMessageRepository(db),
		taskRepo:           repository.NewPMTaskRepository(db),
		epicRepo:           repository.NewPMEpicRepository(db),
		conversationRepo:   repository.NewSupportConversationRepository(db),
		docsDocumentRepo:   repository.NewDocsDocumentRepository(db),
		crmContactRepo:     repository.NewCRMContactRepository(db),
		crmDealRepo:        repository.NewCRMDealRepository(db),
		activitySvc:        NewPMActivityService(repository.NewPMActivityRepository(db)),
		agentRuntimeClient: &fakeAgentRuntimeSignalClient{},
	}
	return svc, &gormTestDB{DB: db}
}

type gormTestDB struct {
	*gorm.DB
}

func createAgentRunActivityTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	tables := []string{
		`CREATE TABLE agents (
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
			source_template_key TEXT,
			template_key TEXT,
			template_instance_id TEXT,
			template_version INTEGER,
			active_version_id TEXT,
			role TEXT,
			status TEXT NOT NULL DEFAULT 'idle',
			runtime_kind TEXT NOT NULL DEFAULT 'native_sdk',
			model_tier TEXT NOT NULL DEFAULT '',
			skills BLOB NOT NULL DEFAULT '[]',
			trigger_mode TEXT NOT NULL DEFAULT 'manual',
			provider TEXT,
			model TEXT,
			execution_config BLOB NOT NULL DEFAULT '{}',
			system_prompt TEXT,
			instruction_template_version TEXT NOT NULL DEFAULT '',
			planning_notes TEXT,
			monthly_token_budget INTEGER,
			tokens_used_this_month INTEGER NOT NULL DEFAULT 0,
			active_task_id TEXT,
			team_id TEXT,
			allowed_tools BLOB NOT NULL DEFAULT '[]',
			allowed_commands BLOB NOT NULL DEFAULT '[]',
			allowed_targets BLOB NOT NULL DEFAULT '[]',
			approval_mode TEXT NOT NULL DEFAULT 'preset_default',
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'interactive',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			task_id TEXT,
			conversation_id TEXT,
			target_type TEXT NOT NULL DEFAULT 'task',
			target_id TEXT NOT NULL,
			runtime_kind TEXT NOT NULL DEFAULT 'native_sdk',
			model_tier TEXT NOT NULL DEFAULT '',
			invocation_mode TEXT NOT NULL DEFAULT 'interactive',
			parent_run_id TEXT,
			dock_chat_id TEXT,
			handoff_state TEXT,
			approval_state TEXT NOT NULL DEFAULT 'not_required',
			pause_reason TEXT NOT NULL DEFAULT 'none',
			triggered_by_user_id TEXT,
			status TEXT NOT NULL DEFAULT 'queued',
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
			input BLOB NOT NULL DEFAULT '{}',
			output_summary BLOB NOT NULL DEFAULT '{}',
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
		`CREATE TABLE agent_run_messages (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			actor_user_id TEXT,
			runtime_message_id TEXT,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			message_type TEXT NOT NULL DEFAULT 'message',
			content_blocks BLOB,
			turn_segments BLOB,
			tool_invocations BLOB,
			token_usage BLOB,
			sequence_no INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS pm_tasks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			task_type TEXT NOT NULL DEFAULT 'feature',
			workflow_id TEXT NOT NULL,
			workflow_state_id TEXT NOT NULL,
			priority TEXT NOT NULL DEFAULT 'none',
			severity TEXT NOT NULL DEFAULT 'none',
			position INTEGER NOT NULL DEFAULT 0,
			started BOOLEAN NOT NULL DEFAULT 0,
			completed BOOLEAN NOT NULL DEFAULT 0,
			blocked BOOLEAN NOT NULL DEFAULT 0,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			visibility TEXT NOT NULL DEFAULT 'workspace_wide',
			owner_id TEXT,
			team_id TEXT,
			template_key TEXT,
			excerpt TEXT,
			icon TEXT,
			tags BLOB,
			position INTEGER NOT NULL DEFAULT 0,
			sort_key TEXT NOT NULL DEFAULT '~',
			is_pinned BOOLEAN NOT NULL DEFAULT 0,
			is_publicly_shared BOOLEAN NOT NULL DEFAULT 0,
			share_token TEXT,
			is_locked BOOLEAN NOT NULL DEFAULT 0,
			locked_by TEXT,
			last_reviewed_at DATETIME,
			next_review_at DATETIME,
			published_at DATETIME,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS pm_epics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			health TEXT NOT NULL DEFAULT 'no_health',
			archived BOOLEAN NOT NULL DEFAULT 0,
			planning_state TEXT NOT NULL DEFAULT 'not_started',
			spec_clarifications BLOB NOT NULL DEFAULT '[]',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS support_conversations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			subject TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'open',
			priority TEXT NOT NULL DEFAULT 'medium',
			channel TEXT NOT NULL DEFAULT 'widget',
			source TEXT NOT NULL DEFAULT 'internal',
			email_unsubscribed BOOLEAN NOT NULL DEFAULT 0,
			ai_turn_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS crm_contacts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id TEXT NOT NULL,
			first_name TEXT NOT NULL,
			last_name TEXT,
			email TEXT,
			lifecycle_stage TEXT NOT NULL DEFAULT 'subscriber',
			lead_status TEXT NOT NULL DEFAULT 'new',
			custom_properties BLOB NOT NULL DEFAULT '{}',
			email_status TEXT NOT NULL DEFAULT 'valid',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS crm_deals (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id TEXT NOT NULL,
			name TEXT NOT NULL,
			pipeline_id TEXT NOT NULL,
			stage_id TEXT NOT NULL,
			amount REAL,
			currency TEXT NOT NULL DEFAULT 'USD',
			close_date DATETIME,
			owner_member_id TEXT,
			commercial_motion TEXT,
			probability INTEGER,
			custom_properties BLOB NOT NULL DEFAULT '{}',
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, table := range tables {
		if err := db.Exec(table).Error; err != nil {
			t.Fatalf("create agent run activity test table: %v", err)
		}
	}
}

func TestEnrichRunTargetsResolvesTaskTargetInfo(t *testing.T) {
	svc, testDB := setupAgentRunActivityTest(t)
	ctx := context.Background()
	workspaceID := "ws-agent-activity"
	taskID := "task-1"
	now := time.Now()
	if err := testDB.Create(&model.PMTask{
		ID:              taskID,
		WorkspaceID:     workspaceID,
		DisplayID:       239,
		Name:            "Fix shared run link",
		TaskType:        model.PMTaskTypeFeature,
		WorkflowID:      "workflow-1",
		WorkflowStateID: "state-1",
		Priority:        model.PMTaskPriorityNone,
		Severity:        model.PMTaskSeverityNone,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	runs := []model.AgentRun{{
		ID:          "run-task",
		WorkspaceID: workspaceID,
		TargetType:  "task",
		TargetID:    taskID,
	}}

	svc.enrichRunTargets(ctx, workspaceID, runs)

	if runs[0].TargetInfo == nil {
		t.Fatal("expected task target info")
	}
	if runs[0].TargetInfo.Title != "Fix shared run link" {
		t.Fatalf("target title = %q", runs[0].TargetInfo.Title)
	}
}

func TestEnrichRunTargetsResolvesDocumentTargetInfo(t *testing.T) {
	svc, testDB := setupAgentRunActivityTest(t)
	ctx := context.Background()
	workspaceID := "ws-agent-activity"
	documentID := "doc-1"
	now := time.Now()
	if err := testDB.Create(&model.DocsDocument{
		ID:          documentID,
		WorkspaceID: workspaceID,
		SpaceID:     "space-1",
		Title:       "API setup guide",
		Status:      model.DocStatusDraft,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		SortKey:     "~",
		CreatedBy:   "user-1",
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}
	runs := []model.AgentRun{{
		ID:          "run-document",
		WorkspaceID: workspaceID,
		TargetType:  "document",
		TargetID:    documentID,
	}}

	svc.enrichRunTargets(ctx, workspaceID, runs)

	if runs[0].TargetInfo == nil {
		t.Fatal("expected document target info")
	}
	if runs[0].TargetInfo.Title != "API setup guide" {
		t.Fatalf("target title = %q", runs[0].TargetInfo.Title)
	}
}

func TestEnrichRunTargetsResolvesSprintTargetInfo(t *testing.T) {
	svc, testDB := setupAgentRunActivityTest(t)
	ctx := context.Background()
	workspaceID := "ws-agent-activity"
	sprintID := "sprint-1"
	now := time.Now()
	if err := testDB.Create(&model.PMSprint{
		ID:          sprintID,
		WorkspaceID: workspaceID,
		Name:        "Platform reliability sprint",
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("create sprint: %v", err)
	}
	svc.SetPMSprintService(NewPMSprintService(
		repository.NewPMSprintRepository(testDB.DB),
		repository.NewPMTaskRepository(testDB.DB),
		repository.NewPMLabelRepository(testDB.DB),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	))
	runs := []model.AgentRun{{
		ID:          "run-sprint",
		WorkspaceID: workspaceID,
		TargetType:  "sprint",
		TargetID:    sprintID,
	}}

	svc.enrichRunTargets(ctx, workspaceID, runs)

	if runs[0].TargetInfo == nil {
		t.Fatal("expected sprint target info")
	}
	if runs[0].TargetInfo.Title != "Platform reliability sprint" {
		t.Fatalf("target title = %q", runs[0].TargetInfo.Title)
	}
}

func TestEnrichRunTargetsResolvesLinkedTargetTitles(t *testing.T) {
	svc, testDB := setupAgentRunActivityTest(t)
	ctx := context.Background()
	workspaceID := "ws-agent-activity"
	now := time.Now()
	if err := testDB.Create(&model.PMEpic{
		ID:                 "epic-1",
		WorkspaceID:        workspaceID,
		Name:               "Billing automation cleanup",
		Health:             model.PMEpicHealthNone,
		PlanningState:      "not_started",
		SpecClarifications: json.RawMessage(`[]`),
		CreatedAt:          now,
		UpdatedAt:          now,
	}).Error; err != nil {
		t.Fatalf("create epic: %v", err)
	}
	if err := testDB.Create(&model.SupportConversation{
		ID:          "conversation-1",
		WorkspaceID: workspaceID,
		DisplayID:   42,
		Subject:     "Cannot connect custom domain",
		Status:      model.SupportConversationStatusOpen,
		Priority:    "medium",
		Channel:     "email",
		Source:      "email",
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("create support conversation: %v", err)
	}
	lastName := "Case"
	email := "annie@example.com"
	if err := testDB.Create(&model.CRMContact{
		ID:               "contact-1",
		WorkspaceID:      workspaceID,
		DisplayID:        "CON-1",
		FirstName:        "Annie",
		LastName:         &lastName,
		Email:            &email,
		LifecycleStage:   model.CRMLifecycleLead,
		LeadStatus:       model.CRMLeadStatusNew,
		CustomProperties: model.JSONB{},
		EmailStatus:      model.CRMContactEmailStatusValid,
		CreatedAt:        now,
		UpdatedAt:        now,
	}).Error; err != nil {
		t.Fatalf("create crm contact: %v", err)
	}
	if err := testDB.Create(&model.CRMDeal{
		ID:               "deal-1",
		WorkspaceID:      workspaceID,
		DisplayID:        "DEAL-1",
		Name:             "Enterprise renewal",
		PipelineID:       "pipeline-1",
		StageID:          "stage-1",
		Currency:         "USD",
		CustomProperties: model.JSONB{},
		CreatedAt:        now,
		UpdatedAt:        now,
	}).Error; err != nil {
		t.Fatalf("create crm deal: %v", err)
	}

	runs := []model.AgentRun{
		{ID: "run-epic", WorkspaceID: workspaceID, TargetType: "epic", TargetID: "epic-1"},
		{ID: "run-support", WorkspaceID: workspaceID, TargetType: "support_conversation", TargetID: "conversation-1"},
		{ID: "run-contact", WorkspaceID: workspaceID, TargetType: "crm_contact", TargetID: "contact-1"},
		{ID: "run-deal", WorkspaceID: workspaceID, TargetType: "crm_deal", TargetID: "deal-1"},
	}

	svc.enrichRunTargets(ctx, workspaceID, runs)

	expectedTitles := map[string]string{
		"run-epic":    "Billing automation cleanup",
		"run-support": "Cannot connect custom domain",
		"run-contact": "Annie Case",
		"run-deal":    "Enterprise renewal",
	}
	for idx := range runs {
		run := runs[idx]
		if run.TargetInfo == nil {
			t.Fatalf("%s target info is nil", run.ID)
		}
		if run.TargetInfo.Title != expectedTitles[run.ID] {
			t.Fatalf("%s target title = %q, want %q", run.ID, run.TargetInfo.Title, expectedTitles[run.ID])
		}
	}
}

func seedAgentRunActivityAgent(t *testing.T, db *gorm.DB, workspaceID, agentID string) {
	t.Helper()
	if err := db.Create(&model.Agent{
		ID:                    agentID,
		WorkspaceID:           workspaceID,
		Name:                  "Atlas",
		RuntimeKind:           "native_sdk",
		Status:                "idle",
		AllowedTargets:        json.RawMessage(`["task","epic"]`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		DefaultInvocationMode: model.InvocationModeInteractive,
	}).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
}

func seedAgentRunActivityRun(t *testing.T, db *gorm.DB, run *model.AgentRun) {
	t.Helper()
	if run.RuntimeKind == "" {
		run.RuntimeKind = "native_sdk"
	}
	if run.InvocationMode == "" {
		run.InvocationMode = model.InvocationModeInteractive
	}
	if run.ApprovalState == "" {
		run.ApprovalState = "not_required"
	}
	if run.PauseReason == "" {
		run.PauseReason = model.AgentRunPauseReasonNone
	}
	if len(run.Input) == 0 {
		run.Input = json.RawMessage(`{}`)
	}
	if len(run.OutputSummary) == 0 {
		run.OutputSummary = json.RawMessage(`{}`)
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
}

func latestAgentRunActivity(t *testing.T, db *gorm.DB, entityType, entityID string) model.PMActivityLog {
	t.Helper()
	var activity model.PMActivityLog
	if err := db.Where("entity_type = ? AND entity_id = ? AND field_name = ?", entityType, entityID, "agent_run").
		Order("created_at DESC").
		First(&activity).Error; err != nil {
		t.Fatalf("find agent run activity: %v", err)
	}
	return activity
}

func TestCancelRunLogsCancellingActorForTaskActivity(t *testing.T) {
	svc, testDB := setupAgentRunActivityTest(t)
	ctx := context.Background()
	workspaceID := "ws-agent-activity"
	agentID := "agent-atlas"
	taskID := "task-1"
	runID := "run-cancel"
	actorID := "user-cancel"

	seedAgentRunActivityAgent(t, testDB.DB, workspaceID, agentID)
	seedAgentRunActivityRun(t, testDB.DB, &model.AgentRun{
		ID:          runID,
		WorkspaceID: workspaceID,
		AgentID:     agentID,
		TaskID:      &taskID,
		TargetType:  "task",
		TargetID:    taskID,
		Status:      model.AgentRunStatusRunning,
	})

	if _, err := svc.CancelRun(ctx, workspaceID, runID, actorID); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}

	activity := latestAgentRunActivity(t, testDB.DB, "task", taskID)
	if activity.ActorID == nil || *activity.ActorID != actorID {
		t.Fatalf("expected actor %q, got %#v", actorID, activity.ActorID)
	}
	if activity.NewValue == nil || *activity.NewValue != "cancelled" {
		t.Fatalf("expected cancelled activity, got %#v", activity.NewValue)
	}
}

func TestApproveRunLogsApprovingActorForEpicActivity(t *testing.T) {
	svc, testDB := setupAgentRunActivityTest(t)
	ctx := context.Background()
	workspaceID := "ws-agent-activity"
	agentID := "agent-atlas"
	epicID := "epic-1"
	runID := "run-approve"
	actorID := "user-approve"

	seedAgentRunActivityAgent(t, testDB.DB, workspaceID, agentID)
	heartbeat := time.Now()
	seedAgentRunActivityRun(t, testDB.DB, &model.AgentRun{
		ID:                runID,
		WorkspaceID:       workspaceID,
		AgentID:           agentID,
		TargetType:        "epic",
		TargetID:          epicID,
		Status:            model.AgentRunStatusPaused,
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		ApprovalState:     "pending",
		LastHeartbeatAt:   &heartbeat,
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_activity_approve"),
	})

	if _, err := svc.ApproveRun(ctx, workspaceID, runID, actorID, model.ApproveAgentRunRequest{}); err != nil {
		t.Fatalf("ApproveRun: %v", err)
	}

	activity := latestAgentRunActivity(t, testDB.DB, "epic", epicID)
	if activity.ActorID == nil || *activity.ActorID != actorID {
		t.Fatalf("expected actor %q, got %#v", actorID, activity.ActorID)
	}
	if activity.NewValue == nil || *activity.NewValue != "approved" {
		t.Fatalf("expected approved activity, got %#v", activity.NewValue)
	}
}

func TestRequestRunChangesLogsActorForTaskActivity(t *testing.T) {
	svc, testDB := setupAgentRunActivityTest(t)
	ctx := context.Background()
	workspaceID := "ws-agent-activity"
	agentID := "agent-atlas"
	taskID := "task-1"
	runID := "run-changes"
	actorID := "user-changes"

	seedAgentRunActivityAgent(t, testDB.DB, workspaceID, agentID)
	seedAgentRunActivityRun(t, testDB.DB, &model.AgentRun{
		ID:                runID,
		WorkspaceID:       workspaceID,
		AgentID:           agentID,
		TaskID:            &taskID,
		TargetType:        "task",
		TargetID:          taskID,
		Status:            model.AgentRunStatusPaused,
		PauseReason:       model.AgentRunPauseReasonHumanApproval,
		ApprovalState:     "pending",
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_activity_changes"),
	})

	if _, err := svc.RequestRunChanges(ctx, workspaceID, runID, actorID, model.SendAgentRunRequestChangesRequest{Content: "Please simplify the plan."}); err != nil {
		t.Fatalf("RequestRunChanges: %v", err)
	}

	activity := latestAgentRunActivity(t, testDB.DB, "task", taskID)
	if activity.NewValue == nil || *activity.NewValue != "changes_requested" {
		t.Fatalf("expected changes_requested activity, got %#v", activity.NewValue)
	}
}

func TestSendRunMessageLogsNoteSnippetForTaskActivity(t *testing.T) {
	svc, testDB := setupAgentRunActivityTest(t)
	ctx := context.Background()
	workspaceID := "ws-agent-activity"
	agentID := "agent-atlas"
	taskID := "task-1"
	runID := "run-note"
	actorID := "user-note"
	content := "Please keep the implementation focused on the smallest possible change and avoid broad refactors."

	seedAgentRunActivityAgent(t, testDB.DB, workspaceID, agentID)
	seedAgentRunActivityRun(t, testDB.DB, &model.AgentRun{
		ID:                runID,
		WorkspaceID:       workspaceID,
		AgentID:           agentID,
		TaskID:            &taskID,
		TargetType:        "task",
		TargetID:          taskID,
		Status:            model.AgentRunStatusPaused,
		PauseReason:       model.AgentRunPauseReasonHumanInput,
		ExternalRuntime:   strPtr("agent-runtime"),
		ExternalRuntimeID: strPtr("run_rt_activity_note"),
	})

	if _, err := svc.SendRunMessage(ctx, workspaceID, runID, actorID, model.SendAgentRunMessageRequest{Content: content}); err != nil {
		t.Fatalf("SendRunMessage: %v", err)
	}

	activity := latestAgentRunActivity(t, testDB.DB, "task", taskID)
	if activity.NewValue == nil || *activity.NewValue != "note_added" {
		t.Fatalf("expected note_added activity, got %#v", activity.NewValue)
	}
	var metadata map[string]string
	if err := json.Unmarshal(activity.Metadata, &metadata); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if !strings.Contains(metadata["note_snippet"], "Please keep the implementation focused") {
		t.Fatalf("expected note snippet in metadata, got %#v", metadata)
	}
	if len(metadata["note_snippet"]) > 80 {
		t.Fatalf("expected short note snippet, got %d chars", len(metadata["note_snippet"]))
	}
}
