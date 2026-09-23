package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// sampleDataItemsTestSchema mirrors migration 202609220041 for SQLite tests.
const sampleDataItemsTestSchema = `CREATE TABLE sample_data_items (
	id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
	workspace_id TEXT NOT NULL,
	entity_type TEXT NOT NULL,
	entity_id TEXT NOT NULL UNIQUE,
	created_by TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`

func setupSampleDataTestDB(t *testing.T, statements ...string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:sample_data_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
	return db
}

func TestSetupEvidenceIgnoresSampleData(t *testing.T) {
	db := setupSampleDataTestDB(t, setupTestSchema...)
	ctx := context.Background()
	for _, statement := range []string{
		`INSERT INTO workspaces (id) VALUES ('ws')`,
		`INSERT INTO workspace_teams (id, workspace_id) VALUES ('team', 'ws')`,
		`INSERT INTO pm_epics (id, workspace_id, owner_member_id, planned_start_date, deadline) VALUES ('epic', 'ws', 'member', '2026-01-01', '2026-02-01')`,
		`INSERT INTO pm_tasks (id, workspace_id, epic_id, completed, completed_at) VALUES ('task', 'ws', 'epic', 1, '2026-01-05T10:00:00Z')`,
		`INSERT INTO pm_task_owners (task_id, user_id) VALUES ('task', 'user')`,
		`INSERT INTO support_conversations (id, workspace_id, status, channel, source, linked_task_id) VALUES ('conversation', 'ws', 'resolved', 'widget', 'widget', 'task')`,
		`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type) VALUES ('message', 'ws', 'conversation', 'customer')`,
		`INSERT INTO docs_spaces (id, workspace_id, type) VALUES ('space', 'ws', 'external_capable')`,
		`INSERT INTO docs_documents (id, workspace_id, space_id, status) VALUES ('doc', 'ws', 'space', 'draft')`,
		`INSERT INTO docs_contents (document_id, content_text, word_count) VALUES ('doc', 'Tracking your order', 3)`,
		`INSERT INTO crm_contacts (id, workspace_id) VALUES ('contact', 'ws')`,
		`INSERT INTO crm_companies (id, workspace_id) VALUES ('company', 'ws')`,
		`INSERT INTO crm_pipelines (id, workspace_id) VALUES ('pipeline', 'ws')`,
		`INSERT INTO crm_pipeline_stages (id, pipeline_id, stage_type) VALUES ('open', 'pipeline', 'open'), ('won', 'pipeline', 'won'), ('lost', 'pipeline', 'lost')`,
		`INSERT INTO crm_deals (id, workspace_id, owner_member_id, amount, close_date) VALUES ('deal', 'ws', 'member', 100, '2026-03-01')`,
		`INSERT INTO crm_associations (id, workspace_id, from_object_type, from_object_id, to_object_type, to_object_id) VALUES ('assoc', 'ws', 'contact', 'contact', 'deal', 'deal')`,
		// A sample Flow someone turned on is still sample content.
		`INSERT INTO automation_rules (id, workspace_id, enabled) VALUES ('flow', 'ws', 1)`,
		`INSERT INTO sample_data_items (workspace_id, entity_type, entity_id) VALUES
			('ws', 'workspace_team', 'team'), ('ws', 'pm_epic', 'epic'), ('ws', 'pm_task', 'task'),
			('ws', 'support_conversation', 'conversation'), ('ws', 'docs_space', 'space'), ('ws', 'docs_document', 'doc'),
			('ws', 'crm_contact', 'contact'), ('ws', 'crm_company', 'company'), ('ws', 'crm_pipeline', 'pipeline'), ('ws', 'crm_deal', 'deal'),
			('ws', 'automation_rule', 'flow')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	evidence, err := NewSetupRepository(db).GetEvidence(ctx, "ws")
	if err != nil {
		t.Fatalf("get evidence: %v", err)
	}
	counts := map[string]int64{
		"team":              evidence.TeamCount,
		"initial work":      evidence.InitialWorkCount,
		"completed tasks":   evidence.CompletedTaskCount,
		"completed days":    evidence.CompletedTaskDayCount,
		"planned projects":  evidence.PlannedProjectCount,
		"assigned tasks":    evidence.AssignedProjectTaskCount,
		"validated support": evidence.ValidatedSupportCount,
		"resolved":          evidence.ResolvedConversationCount,
		"linked support":    evidence.LinkedSupportTaskCount,
		"help spaces":       evidence.HelpCenterSpaceCount,
		"help content":      evidence.HelpCenterContentCount,
		"crm contacts":      evidence.CRMContactCount,
		"crm companies":     evidence.CRMCompanyCount,
		"crm pipelines":     evidence.CRMPipelineCount,
		"crm deals":         evidence.CRMActionableDealCount,
		"enabled flows":     evidence.EnabledAutomationCount,
	}
	for name, count := range counts {
		if count != 0 {
			t.Errorf("%s evidence = %d, want sample data ignored", name, count)
		}
	}
	for _, key := range []string{"product.initial_work", "product.first_task_completed", "support.pm_task_linked"} {
		if _, ok := evidence.TaskAchievementTimes[key]; ok {
			t.Errorf("achievement %q recorded from sample data", key)
		}
	}

	if err := db.Exec(`INSERT INTO pm_tasks (id, workspace_id) VALUES ('real-task', 'ws')`).Error; err != nil {
		t.Fatal(err)
	}
	evidence, err = NewSetupRepository(db).GetEvidence(ctx, "ws")
	if err != nil {
		t.Fatal(err)
	}
	if evidence.InitialWorkCount != 1 {
		t.Fatalf("initial work = %d, want the real task counted", evidence.InitialWorkCount)
	}
}

func sampleRemovalSchema() []string {
	return []string{
		sampleDataItemsTestSchema,
		`CREATE TABLE pm_tasks (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, epic_id TEXT, team_id TEXT)`,
		`CREATE TABLE pm_task_owners (task_id TEXT NOT NULL, user_id TEXT NOT NULL)`,
		`CREATE TABLE pm_activity_log (id TEXT PRIMARY KEY, workspace_id TEXT, entity_type TEXT, entity_id TEXT)`,
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, linked_task_id TEXT, crm_contact_id TEXT)`,
		`CREATE TABLE docs_spaces (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL)`,
		`CREATE TABLE docs_documents (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, space_id TEXT)`,
		`CREATE TABLE docs_contents (document_id TEXT PRIMARY KEY)`,
	}
}

func TestSampleDataRepositoryDeleteEntityRemovesOwnedRows(t *testing.T) {
	db := setupSampleDataTestDB(t, append(sampleRemovalSchema(),
		`INSERT INTO pm_tasks (id, workspace_id) VALUES ('task', 'ws')`,
		`INSERT INTO pm_task_owners (task_id, user_id) VALUES ('task', 'user')`,
		`INSERT INTO pm_activity_log (id, workspace_id, entity_type, entity_id) VALUES ('activity', 'ws', 'task', 'task')`,
		`INSERT INTO support_conversations (id, workspace_id, linked_task_id) VALUES ('real-conversation', 'ws', 'task')`,
	)...)
	repo := NewSampleDataRepository(db)
	if err := repo.DeleteEntity(context.Background(), "ws", "pm_task", "task"); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	for table, where := range map[string]string{"pm_tasks": "id = 'task'", "pm_task_owners": "task_id = 'task'", "pm_activity_log": "entity_id = 'task'"} {
		var count int64
		if err := db.Table(table).Where(where).Count(&count).Error; err != nil || count != 0 {
			t.Errorf("%s rows = %d, err = %v", table, count, err)
		}
	}
	var linked *string
	if err := db.Table("support_conversations").Select("linked_task_id").Where("id = 'real-conversation'").Scan(&linked).Error; err != nil || linked != nil {
		t.Fatalf("real conversation link = %v, err = %v; want detached, not deleted", linked, err)
	}
}

func TestSampleDataRepositoryKeepsSpaceHoldingRealDocuments(t *testing.T) {
	db := setupSampleDataTestDB(t, append(sampleRemovalSchema(),
		`INSERT INTO docs_spaces (id, workspace_id) VALUES ('space', 'ws')`,
		`INSERT INTO docs_documents (id, workspace_id, space_id) VALUES ('user-doc', 'ws', 'space')`,
	)...)
	err := NewSampleDataRepository(db).DeleteEntity(context.Background(), "ws", "docs_space", "space")
	if !errors.Is(err, ErrSampleEntityRetained) {
		t.Fatalf("err = %v, want ErrSampleEntityRetained", err)
	}
	var count int64
	db.Table("docs_spaces").Count(&count)
	if count != 1 {
		t.Fatalf("space deleted although it holds a real document")
	}
}

func TestSampleDataRepositoryCreatesDisabledFlowAndRemovesIt(t *testing.T) {
	db := setupSampleDataTestDB(t, append(sampleRemovalSchema(),
		`CREATE TABLE automation_rules (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, name TEXT NOT NULL, description TEXT, enabled BOOLEAN NOT NULL DEFAULT 1, workflow_id TEXT, trigger_type TEXT NOT NULL, trigger_config TEXT NOT NULL DEFAULT '{}', action_type TEXT NOT NULL, action_config TEXT NOT NULL DEFAULT '{}', position INTEGER NOT NULL DEFAULT 0, stop_on_match BOOLEAN NOT NULL DEFAULT 0, created_by TEXT, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE agent_trigger_executions (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, binding_id TEXT)`,
	)...)
	ctx := context.Background()
	repo := NewSampleDataRepository(db)
	for _, id := range []string{"flow", "used-flow"} {
		rule := &model.AutomationRule{ID: id, WorkspaceID: "ws", Name: "Sample", Enabled: true, TriggerType: model.TriggerGitHubPRMerged,
			TriggerConfig: []byte(`{"base_branch":"main"}`), ActionType: model.ActionMoveToState, ActionConfig: []byte(`{"target_state_id":"done"}`)}
		if err := repo.CreateDisabledAutomationRule(ctx, rule); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	var enabled int64
	db.Table("automation_rules").Where("enabled = ?", true).Count(&enabled)
	if enabled != 0 {
		t.Fatalf("enabled sample Flows = %d, want every sample Flow created turned off", enabled)
	}
	if err := repo.CreateDisabledAutomationRule(ctx, &model.AutomationRule{WorkspaceID: "ws"}); err == nil {
		t.Fatal("expected an error for a Flow without an identity")
	}

	if err := repo.DeleteEntity(ctx, "ws", model.SampleEntityAutomationRule, "flow"); err != nil {
		t.Fatalf("delete unused Flow: %v", err)
	}
	// A Flow someone turned on and ran keeps its run history.
	if err := db.Exec(`INSERT INTO agent_trigger_executions (id, workspace_id, binding_id) VALUES ('execution', 'ws', 'used-flow')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteEntity(ctx, "ws", model.SampleEntityAutomationRule, "used-flow"); !errors.Is(err, ErrSampleEntityRetained) {
		t.Fatalf("delete used Flow err = %v, want ErrSampleEntityRetained", err)
	}
	var ids []string
	db.Table("automation_rules").Pluck("id", &ids)
	if len(ids) != 1 || ids[0] != "used-flow" {
		t.Fatalf("remaining Flows = %v, want only the Flow with run history", ids)
	}
}

func TestSampleDataRepositoryDeleteEntityRejectsUnknownType(t *testing.T) {
	db := setupSampleDataTestDB(t, sampleRemovalSchema()...)
	if err := NewSampleDataRepository(db).DeleteEntity(context.Background(), "ws", "users", "x"); err == nil {
		t.Fatal("expected an error for an unknown entity type")
	}
}

func TestEmailFallbackCandidatesExcludeSampleConversations(t *testing.T) {
	db := setupSampleDataTestDB(t,
		sampleDataItemsTestSchema,
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'open', customer_email TEXT, email_unsubscribed BOOLEAN NOT NULL DEFAULT 0, contact_last_seen_at DATETIME)`,
		`CREATE TABLE support_messages (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, conversation_id TEXT NOT NULL, sender_type TEXT NOT NULL, message_type TEXT NOT NULL DEFAULT 'reply', system_event_type TEXT, sender_user_id TEXT, sender_agent_id TEXT, sender_display_name TEXT, sender_avatar_url TEXT, content TEXT NOT NULL DEFAULT '', is_internal BOOLEAN NOT NULL DEFAULT 0, metadata TEXT DEFAULT '{}', via_channel TEXT, email_notified_at DATETIME, email_read_at DATETIME, cancellable_until DATETIME, deleted_at DATETIME, created_at DATETIME, updated_at DATETIME)`,
		`INSERT INTO support_conversations (id, workspace_id, customer_email) VALUES ('real', 'ws', 'real@example.com'), ('sample', 'ws', 'sample@example.com')`,
		`INSERT INTO sample_data_items (workspace_id, entity_type, entity_id) VALUES ('ws', 'support_conversation', 'sample')`,
	)
	sentAt := time.Now().Add(-5 * time.Minute)
	if err := db.Exec(`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, created_at) VALUES ('m-real', 'ws', 'real', 'user', ?), ('m-sample', 'ws', 'sample', 'user', ?)`, sentAt, sentAt).Error; err != nil {
		t.Fatal(err)
	}
	messages, err := NewSupportMessageRepository(db).ListEmailFallbackReconciliationCandidates(context.Background(), time.Now().Add(-time.Hour), time.Now(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].ID != "m-real" {
		t.Fatalf("candidates = %+v, want only the real conversation's reply", messages)
	}
}
