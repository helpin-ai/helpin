package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSetupAutomationRuleBindingJoinCastsUUIDToText(t *testing.T) {
	join := setupAutomationRuleBindingJoin("automation_rules", "executions")
	if !strings.Contains(join, "CAST(automation_rules.id AS TEXT) = executions.binding_id") {
		t.Fatalf("binding join = %q, want UUID cast to text", join)
	}
}

func TestValuableAgentRunPredicateDoesNotCompareUUIDToEmptyString(t *testing.T) {
	predicate := valuableAgentRunPredicate()
	if strings.Contains(predicate, "target_id, '')") || strings.Contains(predicate, "target_id <> ''") {
		t.Fatalf("UUID target_id must use a null check, got %q", predicate)
	}
	if !strings.Contains(predicate, "runs.target_id IS NOT NULL") {
		t.Fatalf("valuable run predicate must require a target ID, got %q", predicate)
	}
}

func TestSetupRepositoryPersistsGoalsAndReadsVerifiedEvidence(t *testing.T) {
	dbName := fmt.Sprintf("file:setup_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range setupTestSchema {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create setup test schema: %v", err)
		}
	}

	repo := NewSetupRepository(db)
	ctx := context.Background()
	if err := repo.ReplaceGoals(ctx, "workspace-1", "user-1", []string{model.SetupGoalProductDelivery, model.SetupGoalAutomationMastery}); err != nil {
		t.Fatalf("replace goals: %v", err)
	}
	goals, err := repo.ListGoalKeys(ctx, "workspace-1")
	if err != nil {
		t.Fatalf("list goals: %v", err)
	}
	if len(goals) != 2 || goals[0] != model.SetupGoalProductDelivery || goals[1] != model.SetupGoalAutomationMastery {
		t.Fatalf("goals = %v", goals)
	}
	goalRows, err := repo.ListGoals(ctx, "workspace-1")
	if err != nil {
		t.Fatalf("list goal rows: %v", err)
	}
	firstProductID := goalRows[0].ID
	if err := repo.ReplaceGoals(ctx, "workspace-1", "user-1", []string{model.SetupGoalAutomationMastery, model.SetupGoalProductDelivery}); err != nil {
		t.Fatalf("reorder goals: %v", err)
	}
	goalRows, err = repo.ListGoals(ctx, "workspace-1")
	if err != nil {
		t.Fatalf("list reordered goals: %v", err)
	}
	if goalRows[0].Key != model.SetupGoalAutomationMastery || goalRows[1].ID != firstProductID {
		t.Fatalf("reordered goals lost ordering/history: %+v", goalRows)
	}
	if err := repo.ReplaceGoals(ctx, "workspace-1", "user-1", []string{model.SetupGoalAutomationMastery, model.SetupGoalProductDelivery, model.SetupGoalInternalDocs}); err != nil {
		t.Fatalf("add placeholder goal: %v", err)
	}
	goalRows, err = repo.ListGoals(ctx, "workspace-1")
	if err != nil || goalRows[2].CatalogVersion != 1 {
		t.Fatalf("real journey catalog version = %+v, err = %v", goalRows, err)
	}

	for _, statement := range []string{
		`INSERT INTO workspaces (id, company_product_context) VALUES ('workspace-1', 'A useful product')`,
		`INSERT INTO setup_intents (workspace_id, goal_keys) VALUES ('workspace-1', '["internal_docs"]')`,
		`INSERT INTO workspace_teams (id, workspace_id) VALUES ('team-1', 'workspace-1')`,
		`INSERT INTO workspace_members (id, workspace_id, user_id, status) VALUES ('member-1', 'workspace-1', 'user-1', 'active'), ('member-2', 'workspace-1', 'user-2', 'active')`,
		`INSERT INTO pm_tasks (id, workspace_id, completed, completed_at) VALUES ('task-1', 'workspace-1', 1, '2025-04-05T10:00:00Z'), ('task-2', 'workspace-1', 1, '2025-04-05T12:00:00Z')`,
		`INSERT INTO agents (id, workspace_id, is_system, preset_key, approval_mode, template_key, template_instance_id, source_template_key) VALUES ('agent-1', 'workspace-1', 0, '', 'always', NULL, NULL, ''), ('docs-agent', 'workspace-1', 1, 'documentation_agent', 'never', NULL, NULL, ''), ('template-agent', 'workspace-1', 0, 'documentation_agent', 'always', 'release_notes_writer', 'instance-1', 'release_notes_writer')`,
		`INSERT INTO agent_runs (id, workspace_id, agent_id, status, approval_state, target_type, target_id, triggered_by_user_id, output_summary, completed_at) VALUES ('run-1', 'workspace-1', 'agent-1', 'completed', 'approved', 'task', 'task-1', 'user-1', '{"result":"ok"}', '2025-04-05T10:00:00Z'), ('run-2', 'workspace-1', 'agent-1', 'completed', 'not_required', 'task', 'task-2', 'user-1', '{"result":"ok"}', '2025-04-05T12:00:00Z'), ('template-run', 'workspace-1', 'template-agent', 'completed', 'not_required', 'repository', 'repo-1', 'user-2', '{"result":"ok"}', '2025-04-05T13:00:00Z')`,
		`INSERT INTO automation_rules (id, workspace_id, enabled, template_key) VALUES ('rule-1', 'workspace-1', 1, NULL), ('product-flow', 'workspace-1', 1, 'stale_task_escalation'), ('help-flow', 'workspace-1', 1, 'public_help_freshness_sweep'), ('docs-flow', 'workspace-1', 1, 'docs_freshness_sweep'), ('crm-flow', 'workspace-1', 1, 'buying_signal_to_task'), ('disabled-flow', 'workspace-1', 0, 'release_notes_writer'), ('other-flow', 'workspace-1', 1, 'run_on_a_schedule')`,
		`INSERT INTO agent_trigger_executions (id, workspace_id, status) VALUES ('trigger-1', 'workspace-1', 'completed')`,
		`INSERT INTO support_widget_sessions (id, workspace_id) VALUES ('session-1', 'workspace-1')`,
		`INSERT INTO support_widget_installations (id, workspace_id, active, settings) VALUES ('install-1', 'workspace-1', 1, '{"ai_enabled":true,"ai_agent_id":"agent-1","triage_enabled":true,"widget_help_space_ids":["help-space"]}')`,
		`INSERT INTO support_email_routes (id, workspace_id, active) VALUES ('email-route-1', 'workspace-1', 1)`,
		`INSERT INTO pm_epics (id, workspace_id, owner_member_id, planned_start_date, deadline, archived) VALUES ('epic-1', 'workspace-1', 'member-1', '2025-04-01', '2025-04-30', 0)`,
		`INSERT INTO pm_sprints (id, workspace_id, start_date, end_date, archived) VALUES ('sprint-1', 'workspace-1', '2025-04-01', '2025-04-14', 0)`,
		`UPDATE pm_tasks SET epic_id = 'epic-1', sprint_id = 'sprint-1' WHERE id = 'task-1'`,
		`INSERT INTO pm_task_owners (task_id, user_id) VALUES ('task-1', 'user-1')`,
		`INSERT INTO docs_spaces (id, workspace_id, type, is_system, deleted_at) VALUES ('help-space', 'workspace-1', 'external_capable', 0, NULL), ('internal-space', 'workspace-1', 'internal', 0, NULL)`,
		`INSERT INTO docs_documents (id, workspace_id, space_id, status, published_at, owner_id, next_review_at, deleted_at) VALUES ('doc-1', 'workspace-1', 'help-space', 'published', '2025-04-05T10:00:00Z', NULL, NULL, NULL), ('internal-doc', 'workspace-1', 'internal-space', 'published', '2025-04-05T10:00:00Z', 'user-1', '2025-06-01T00:00:00Z', NULL)`,
		`INSERT INTO docs_contents (document_id, content_text, word_count) VALUES ('doc-1', 'Customer answer', 2), ('internal-doc', 'Internal runbook', 2)`,
		`INSERT INTO docs_helpcenter_articles (id, document_id, public_published_at) VALUES ('help-1', 'doc-1', '2025-04-05T10:00:00Z')`,
		`INSERT INTO docs_helpcenter_configs (workspace_id, is_published) VALUES ('workspace-1', 1)`,
		`INSERT INTO agent_knowledge_sources (id, agent_id, space_id, workspace_id, sync_status, indexed_documents, indexed_chunks) VALUES ('knowledge-1', 'docs-agent', 'internal-space', 'workspace-1', 'ready', 1, 2)`,
		`INSERT INTO agent_runs (id, workspace_id, agent_id, status, approval_state, target_type, target_id, output_summary, completed_at) VALUES ('docs-run', 'workspace-1', 'docs-agent', 'completed', 'not_required', 'document', 'internal-doc', '{"result":"updated"}', '2025-04-06T10:00:00Z')`,
		`INSERT INTO support_content_sources (id, workspace_id, sync_status, indexed_pages, indexed_chunks) VALUES ('brand-1', 'workspace-1', 'ready', 2, 3)`,
		`INSERT INTO support_mailboxes (id, workspace_id, active, triage_eligible, description) VALUES ('inbox-1', 'workspace-1', 1, 1, 'Billing questions')`,
		`INSERT INTO support_triage_rules (id, workspace_id, active, target_mailbox_id) VALUES ('routing-1', 'workspace-1', 1, 'inbox-1')`,
		`INSERT INTO support_coverage_recommendations (id, workspace_id, status) VALUES ('coverage-1', 'workspace-1', 'applied')`,
		`INSERT INTO support_conversations (id, workspace_id, status, channel, source) VALUES ('empty-resolved', 'workspace-1', 'resolved', 'internal', 'internal')`,
		`INSERT INTO support_conversations (id, workspace_id, status, channel, source, linked_task_id) VALUES ('linked-issue', 'workspace-1', 'open', 'email', 'email', 'task-1')`,
		`INSERT INTO crm_contacts (id, workspace_id) VALUES ('contact-1', 'workspace-1')`,
		`INSERT INTO crm_companies (id, workspace_id) VALUES ('company-1', 'workspace-1')`,
		`INSERT INTO crm_pipelines (id, workspace_id) VALUES ('pipeline-1', 'workspace-1')`,
		`INSERT INTO crm_pipeline_stages (id, pipeline_id, stage_type) VALUES ('stage-open', 'pipeline-1', 'open'), ('stage-won', 'pipeline-1', 'won'), ('stage-lost', 'pipeline-1', 'lost')`,
		`INSERT INTO crm_deals (id, workspace_id, owner_member_id, amount, close_date) VALUES ('deal-1', 'workspace-1', 'member-1', 1000, '2025-05-01'), ('unassociated-deal', 'workspace-1', 'member-1', 2000, '2025-06-01')`,
		`INSERT INTO crm_associations (id, workspace_id, from_object_type, from_object_id, to_object_type, to_object_id) VALUES ('contact-deal-1', 'workspace-1', 'contact', 'contact-1', 'deal', 'deal-1')`,
		`INSERT INTO crm_email_accounts (id, workspace_id, is_active, status) VALUES ('crm-email-1', 'workspace-1', 1, 'connected')`,
		`INSERT INTO crm_autonomy_settings (id, workspace_id, enabled, auto_create_deals, auto_progress_deals) VALUES ('autonomy-1', 'workspace-1', 1, 1, 1)`,
		`INSERT INTO crm_suggestions (id, workspace_id, suggestion_type, status, execution_status, executed_at, object_id) VALUES ('suggestion-1', 'workspace-1', 'deal_create', 'accepted', 'succeeded', '2025-04-06T10:00:00Z', 'deal-1')`,
		`INSERT INTO support_email_routes (id, workspace_id, active) VALUES ('other-email', 'workspace-2', 1)`,
		`INSERT INTO support_content_sources (id, workspace_id, sync_status, indexed_pages, indexed_chunks) VALUES ('other-brand', 'workspace-2', 'ready', 5, 5), ('queued-brand', 'workspace-1', 'queued', 0, 0)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed evidence: %v", err)
		}
	}
	pending, err := repo.PendingGoalKeys(ctx, "workspace-1")
	if err != nil || len(pending) != 1 || pending[0] != model.SetupGoalInternalDocs {
		t.Fatalf("pending goals = %v, err = %v", pending, err)
	}
	if err := repo.ClearPendingGoalKeys(ctx, "workspace-1"); err != nil {
		t.Fatalf("clear pending goals: %v", err)
	}

	evidence, err := repo.GetEvidence(ctx, "workspace-1")
	if err != nil {
		t.Fatalf("get evidence: %v", err)
	}
	if !evidence.HasCompanyContext || evidence.TeamCount != 1 || evidence.ActiveMemberCount != 2 {
		t.Fatalf("foundation evidence = %+v", evidence)
	}
	if evidence.InitialWorkCount != 2 || evidence.CompletedTaskCount != 2 || evidence.CompletedTaskDayCount != 1 || evidence.CompletedAgentRunCount != 4 || evidence.TriggeredSuccessRunCount != 1 {
		t.Fatalf("outcome evidence = %+v", evidence)
	}
	if evidence.ValidatedSupportCount != 0 {
		t.Fatalf("empty internal resolution counted as delivery validation: %+v", evidence)
	}
	for _, statement := range []string{
		`INSERT INTO support_conversations (id, workspace_id, status, channel, source, resolved_at) VALUES ('real-resolved', 'workspace-1', 'resolved', 'email', 'email', '2025-04-06T10:00:00Z')`,
		`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, created_at) VALUES ('customer-1', 'workspace-1', 'real-resolved', 'customer', '2025-04-06T09:00:00Z')`,
		`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, sender_user_id, metadata, created_at) VALUES ('reply-1', 'workspace-1', 'real-resolved', 'user', 'user-1', '{"ai_assisted":true}', '2025-04-06T09:30:00Z')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed member AI evidence: %v", err)
		}
	}
	memberAICount, memberAIAt, err := repo.MemberSupportAIReplyEvidence(ctx, "workspace-1", "user-1", time.Unix(0, 0))
	if err != nil || memberAICount != 1 || memberAIAt.IsZero() {
		t.Fatalf("member AI support evidence = %d at %v, err = %v", memberAICount, memberAIAt, err)
	}
	if got := evidence.TaskAchievementTimes["automation.repeat_assisted_value"]; !got.Equal(time.Date(2025, 4, 5, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("same-day distinct-target repeat time = %v, want second qualifying run", got)
	}
	if got := evidence.TaskAchievementTimes["product.first_task_completed"]; !got.Equal(time.Date(2025, 4, 5, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("first task achievement time = %v, want qualifying event time", got)
	}
	memberEvidence, err := repo.MemberValuableAgentRunEvidence(ctx, "workspace-1", "user-1", time.Unix(0, 0))
	if err != nil || memberEvidence.RunCount != 2 || memberEvidence.ProductCount != 2 || memberEvidence.FirstRunAt.IsZero() || memberEvidence.RepeatRunAt.IsZero() {
		t.Fatalf("member evidence = %+v, err = %v", memberEvidence, err)
	}
	if evidence.SupportEmailInboxCount != 1 || evidence.LiveChatInstallationCount != 1 || evidence.PublicHelpDocCount != 1 || evidence.BrandKnowledgeSourceCount != 1 {
		t.Fatalf("support channel and knowledge evidence = %+v", evidence)
	}
	if !evidence.SupportAIAgentActive || evidence.TeamInboxCount != 1 || evidence.AutomaticRoutingCount != 1 {
		t.Fatalf("support AI and routing evidence = %+v", evidence)
	}
	if evidence.LinkedSupportTaskCount != 1 || evidence.CoverageImprovementCount != 1 {
		t.Fatalf("support value evidence = %+v", evidence)
	}
	if evidence.PlannedProjectCount != 1 || evidence.PlannedSprintCount != 1 || evidence.AssignedProjectTaskCount != 1 {
		t.Fatalf("project setup evidence = %+v", evidence)
	}
	if evidence.HelpCenterSpaceCount != 1 || evidence.HelpCenterContentCount != 1 || evidence.HelpCenterSiteCount != 1 || evidence.HelpCenterWidgetCount != 1 {
		t.Fatalf("help center evidence = %+v", evidence)
	}
	if evidence.InternalDocsSpaceCount != 1 || evidence.InternalDocsContentCount != 1 || evidence.InternalDocsPublishedCount != 1 || evidence.InternalDocsOwnershipCount != 1 || evidence.InternalAgentKnowledgeCount != 1 || evidence.InternalDocAgentSuccessCount != 1 {
		t.Fatalf("internal docs evidence = %+v", evidence)
	}
	if evidence.CRMContactCount != 1 || evidence.CRMCompanyCount != 1 || evidence.CRMPipelineCount != 1 || evidence.CRMActionableDealCount != 1 || evidence.CRMConnectedEmailCount != 1 || evidence.CRMAutonomyEnabledCount != 1 || evidence.CRMSignalValueCount != 1 {
		t.Fatalf("CRM evidence = %+v", evidence)
	}
	if evidence.ApprovalGuardCount != 2 {
		t.Fatalf("approval guard evidence = %+v", evidence)
	}
	if evidence.ProductRequiredFlowCount != 1 || evidence.HelpCenterRequiredFlowCount != 1 || evidence.InternalDocsRequiredFlowCount != 1 || evidence.CRMRequiredFlowCount != 1 {
		t.Fatalf("required flow evidence = %+v", evidence)
	}
	if evidence.CustomAgentSuccessCount != 2 {
		t.Fatalf("custom agent success count = %d, want only the two runs from a user-created agent", evidence.CustomAgentSuccessCount)
	}
	if err := db.Exec(`UPDATE support_widget_installations SET settings = '{"ai_enabled":true,"ai_agent_id":"   ","triage_enabled":false}' WHERE workspace_id = 'workspace-1'`).Error; err != nil {
		t.Fatalf("disable support AI and routing evidence: %v", err)
	}
	disabledSupport, err := repo.GetEvidence(ctx, "workspace-1")
	if err != nil {
		t.Fatalf("get disabled support evidence: %v", err)
	}
	if disabledSupport.SupportAIAgentActive || disabledSupport.AutomaticRoutingCount != 0 {
		t.Fatalf("disabled or blank support settings counted as active: %+v", disabledSupport)
	}
	if err := db.Exec(`UPDATE support_widget_installations SET settings = '{"ai_enabled":true,"ai_agent_id":"agent-1","triage_enabled":true}' WHERE workspace_id = 'workspace-1'`).Error; err != nil {
		t.Fatalf("restore support settings evidence: %v", err)
	}
	if err := db.Exec(`UPDATE support_triage_rules SET active = 0 WHERE workspace_id = 'workspace-1'`).Error; err != nil {
		t.Fatalf("disable support routing rule: %v", err)
	}
	if err := db.Exec(`UPDATE support_mailboxes SET description = NULL, routing_prompt = NULL WHERE workspace_id = 'workspace-1'`).Error; err != nil {
		t.Fatalf("clear support AI routing context: %v", err)
	}
	withoutRouting, err := repo.GetEvidence(ctx, "workspace-1")
	if err != nil {
		t.Fatalf("get unconfigured routing evidence: %v", err)
	}
	if withoutRouting.AutomaticRoutingCount != 0 {
		t.Fatalf("unconfigured automatic routing counted: %+v", withoutRouting)
	}
	if err := db.Exec(`UPDATE support_mailboxes SET routing_prompt = 'Route billing issues here' WHERE workspace_id = 'workspace-1'`).Error; err != nil {
		t.Fatalf("configure support AI routing prompt: %v", err)
	}
	withAIRouting, err := repo.GetEvidence(ctx, "workspace-1")
	if err != nil || withAIRouting.AutomaticRoutingCount != 1 {
		t.Fatalf("AI routing evidence = %+v, err = %v", withAIRouting, err)
	}
	windowed, err := repo.GetEvidenceSince(ctx, "workspace-1", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("get windowed evidence: %v", err)
	}
	if !windowed.HasCompanyContext || windowed.InitialWorkCount != 2 || windowed.CompletedTaskCount != 0 || windowed.CompletedAgentRunCount != 0 || windowed.TriggeredSuccessRunCount != 0 {
		t.Fatalf("activation window evidence = %+v", windowed)
	}
	if err := repo.EnsureAchievements(ctx, "workspace-1", model.SetupGoalProductDelivery, []string{"product.first_task_completed"}); err != nil {
		t.Fatalf("ensure achievement: %v", err)
	}
	achievements, err := repo.ListAchievements(ctx, "workspace-1")
	if err != nil || achievements["product.first_task_completed"].IsZero() {
		t.Fatalf("durable achievements = %v, err = %v", achievements, err)
	}
	if err := repo.RecordActionIntent(ctx, "workspace-1", "user-1", model.SetupGoalProductDelivery, "product.initial_work", "pm_create_task"); err != nil {
		t.Fatalf("record action intent: %v", err)
	}
	var intentCount int64
	if err := db.Table("setup_action_intents").Where("workspace_id = ? AND member_id = ?", "workspace-1", "user-1").Count(&intentCount).Error; err != nil || intentCount != 1 {
		t.Fatalf("action intent count = %d, err = %v", intentCount, err)
	}
	firstPreference, err := repo.GetPreference(ctx, "workspace-1", "user-1")
	if err != nil {
		t.Fatalf("create member setup preference: %v", err)
	}
	secondPreference, err := repo.GetPreference(ctx, "workspace-1", "user-1")
	if err != nil {
		t.Fatalf("repeat member setup preference read: %v", err)
	}
	if firstPreference.ID != secondPreference.ID {
		t.Fatalf("preference ID changed across reads: %q then %q", firstPreference.ID, secondPreference.ID)
	}
}

var setupTestSchema = []string{
	`CREATE TABLE setup_goals (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, key TEXT NOT NULL, catalog_version INTEGER NOT NULL DEFAULT 1, source TEXT NOT NULL, status TEXT NOT NULL, position INTEGER NOT NULL, activated_at DATETIME NOT NULL, created_by TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, UNIQUE(workspace_id, key))`,
	`CREATE TABLE setup_intents (workspace_id TEXT PRIMARY KEY, goal_keys TEXT NOT NULL DEFAULT '[]', created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE setup_achievements (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, goal_key TEXT NOT NULL, task_key TEXT NOT NULL, member_id TEXT NOT NULL DEFAULT '', evidence TEXT NOT NULL, achieved_at DATETIME NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, UNIQUE(workspace_id, goal_key, task_key, member_id))`,
	`CREATE TABLE setup_action_intents (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, member_id TEXT NOT NULL, goal_key TEXT NOT NULL, task_key TEXT NOT NULL, action_key TEXT NOT NULL, started_at DATETIME NOT NULL)`,
	`CREATE TABLE member_setup_preferences (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT NOT NULL, sidebar_dismissed BOOLEAN NOT NULL DEFAULT 0, created_at DATETIME, updated_at DATETIME, UNIQUE(workspace_id, user_id))`,
	`CREATE TABLE workspaces (id TEXT PRIMARY KEY, company_product_context TEXT, timezone TEXT NOT NULL DEFAULT 'UTC', created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE workspace_teams (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
	`CREATE TABLE workspace_members (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT, status TEXT NOT NULL)`,
	`CREATE TABLE workspace_invitations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, status TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE pm_tasks (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, epic_id TEXT, sprint_id TEXT, completed BOOLEAN NOT NULL DEFAULT 0, archived BOOLEAN NOT NULL DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, completed_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE pm_epics (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, owner_id TEXT, owner_member_id TEXT, planned_start_date DATETIME, deadline DATETIME, archived BOOLEAN NOT NULL DEFAULT 0)`,
	`CREATE TABLE pm_sprints (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, start_date DATETIME, end_date DATETIME, archived BOOLEAN NOT NULL DEFAULT 0)`,
	`CREATE TABLE pm_task_owners (task_id TEXT NOT NULL, user_id TEXT NOT NULL)`,
	`CREATE TABLE pm_sprint_closeouts (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, closed_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE git_repositories (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, active BOOLEAN NOT NULL DEFAULT 1, selected BOOLEAN NOT NULL DEFAULT 1, deleted_at DATETIME, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE agents (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, is_system BOOLEAN NOT NULL DEFAULT 0, preset_key TEXT NOT NULL DEFAULT '', approval_mode TEXT NOT NULL DEFAULT 'never', template_key TEXT, template_instance_id TEXT, source_template_key TEXT NOT NULL DEFAULT '')`,
	`CREATE TABLE agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, agent_id TEXT NOT NULL, status TEXT NOT NULL, approval_state TEXT NOT NULL DEFAULT 'not_required', target_type TEXT NOT NULL DEFAULT 'task', target_id TEXT NOT NULL DEFAULT '', triggered_by_user_id TEXT, output_summary TEXT NOT NULL DEFAULT '{}', completed_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE agent_run_artifacts (id TEXT PRIMARY KEY, workspace_id TEXT, run_id TEXT NOT NULL, artifact_type TEXT NOT NULL DEFAULT '', inline_content TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE automation_rules (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, enabled BOOLEAN NOT NULL DEFAULT 1, template_key TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE agent_trigger_executions (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, status TEXT NOT NULL, binding_id TEXT, fired_at DATETIME DEFAULT CURRENT_TIMESTAMP, completed_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE support_widget_installations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, active BOOLEAN NOT NULL DEFAULT 1, settings TEXT NOT NULL DEFAULT '{}')`,
	`CREATE TABLE support_widget_sessions (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE support_email_routes (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, active BOOLEAN NOT NULL DEFAULT 1)`,
	`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, status TEXT NOT NULL, subject TEXT NOT NULL DEFAULT '', channel TEXT NOT NULL DEFAULT 'internal', source TEXT NOT NULL DEFAULT 'internal', ai_state TEXT, flow_state TEXT, linked_task_id TEXT, resolved_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE support_messages (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, conversation_id TEXT NOT NULL, sender_type TEXT NOT NULL, message_type TEXT NOT NULL DEFAULT 'reply', sender_user_id TEXT, is_internal BOOLEAN NOT NULL DEFAULT 0, metadata TEXT NOT NULL DEFAULT '{}', created_at DATETIME DEFAULT CURRENT_TIMESTAMP, deleted_at DATETIME)`,
	`CREATE TABLE support_coverage_recommendations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, status TEXT NOT NULL, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE agent_knowledge_sources (id TEXT PRIMARY KEY, agent_id TEXT, space_id TEXT, workspace_id TEXT NOT NULL, sync_status TEXT NOT NULL DEFAULT 'queued', indexed_documents INTEGER NOT NULL DEFAULT 0, indexed_chunks INTEGER NOT NULL DEFAULT 0)`,
	`CREATE TABLE support_content_sources (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, sync_status TEXT NOT NULL DEFAULT 'queued', indexed_pages INTEGER NOT NULL DEFAULT 0, indexed_chunks INTEGER NOT NULL DEFAULT 0)`,
	`CREATE TABLE docs_spaces (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, type TEXT NOT NULL, is_system BOOLEAN NOT NULL DEFAULT 0, deleted_at DATETIME)`,
	`CREATE TABLE docs_documents (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, space_id TEXT, status TEXT, published_at DATETIME, owner_id TEXT, next_review_at DATETIME, deleted_at DATETIME)`,
	`CREATE TABLE docs_contents (document_id TEXT PRIMARY KEY, content_text TEXT, word_count INTEGER NOT NULL DEFAULT 0)`,
	`CREATE TABLE docs_helpcenter_articles (id TEXT PRIMARY KEY, document_id TEXT NOT NULL, public_published_at DATETIME)`,
	`CREATE TABLE docs_helpcenter_configs (workspace_id TEXT PRIMARY KEY, is_published BOOLEAN NOT NULL DEFAULT 0)`,
	`CREATE TABLE support_mailboxes (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, active BOOLEAN NOT NULL DEFAULT 1, triage_eligible BOOLEAN NOT NULL DEFAULT 1, routing_prompt TEXT, description TEXT)`,
	`CREATE TABLE support_triage_rules (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, active BOOLEAN NOT NULL DEFAULT 1, target_mailbox_id TEXT NOT NULL)`,
	`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
	`CREATE TABLE crm_companies (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
	`CREATE TABLE crm_pipelines (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
	`CREATE TABLE crm_pipeline_stages (id TEXT PRIMARY KEY, pipeline_id TEXT NOT NULL, stage_type TEXT NOT NULL)`,
	`CREATE TABLE crm_deals (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, owner_member_id TEXT, amount REAL, close_date DATETIME)`,
	`CREATE TABLE crm_associations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, from_object_type TEXT NOT NULL, from_object_id TEXT NOT NULL, to_object_type TEXT NOT NULL, to_object_id TEXT NOT NULL)`,
	`CREATE TABLE crm_email_accounts (signature TEXT NOT NULL DEFAULT '',
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, is_active BOOLEAN NOT NULL DEFAULT 1, status TEXT NOT NULL)`,
	`CREATE TABLE crm_autonomy_settings (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, enabled BOOLEAN NOT NULL DEFAULT 1, auto_create_deals BOOLEAN NOT NULL DEFAULT 1, auto_progress_deals BOOLEAN NOT NULL DEFAULT 1)`,
	`CREATE TABLE crm_suggestions (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, suggestion_type TEXT NOT NULL, status TEXT NOT NULL, execution_status TEXT, executed_at DATETIME, object_id TEXT)`,
}
