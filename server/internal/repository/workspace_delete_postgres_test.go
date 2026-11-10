package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openWorkspaceDeletePostgres runs each test in a transaction that is rolled
// back, against a migrated disposable database.
func openWorkspaceDeletePostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("WORKSPACE_DELETE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WORKSPACE_DELETE_TEST_DATABASE_URL to a migrated disposable PostgreSQL database")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() {
		tx.Rollback()
		_ = sqlDB.Close()
	})
	return tx
}

type workspaceDeleteFixture struct {
	workspaceID, notificationID, pipelineID, periodID string
}

// seedWorkspaceForDelete creates rows in tables the explicit delete list never
// covered: no foreign key to workspaces, plus child tables without a
// workspace_id that block their parents' deletes.
func seedWorkspaceForDelete(t *testing.T, db *gorm.DB, ownerID, key string) workspaceDeleteFixture {
	t.Helper()
	f := workspaceDeleteFixture{
		workspaceID: uuid.NewString(), notificationID: uuid.NewString(),
		pipelineID: uuid.NewString(), periodID: uuid.NewString(),
	}
	eventID, conversationID := uuid.NewString(), uuid.NewString()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO workspaces (id, name, slug, workspace_key, owner_id) VALUES (?, ?, ?, ?, ?)`,
			[]any{f.workspaceID, "Delete " + key, "delete-" + uuid.NewString()[:8], key, ownerID}},
		{`INSERT INTO support_canned_responses (workspace_id, short_code, content) VALUES (?, 'hello', 'Hi')`,
			[]any{f.workspaceID}},
		{`INSERT INTO crm_pipelines (id, workspace_id, name) VALUES (?, ?, 'Sales')`, []any{f.pipelineID, f.workspaceID}},
		{`INSERT INTO crm_pipeline_stages (pipeline_id, name) VALUES (?, 'New')`, []any{f.pipelineID}},
		{`INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category)
		  VALUES (?, ?, ?, 'task', ?, 'assigned', 'Assigned', 'work')`,
			[]any{f.notificationID, f.workspaceID, ownerID, uuid.NewString()}},
		{`INSERT INTO notification_events (id, notification_id, event_type, title, category) VALUES (?, ?, 'assigned', 'Assigned', 'work')`,
			[]any{eventID, f.notificationID}},
		{`INSERT INTO notification_deliveries (notification_event_id, channel) VALUES (?, 'email')`, []any{eventID}},
		// An anonymized conversation is read-only; deleting its messages first
		// would update its projection and be rejected.
		{`INSERT INTO support_conversations (id, workspace_id, display_id, subject) VALUES (?, ?, 1, 'Erased')`,
			[]any{conversationID, f.workspaceID}},
		{`INSERT INTO support_messages (workspace_id, conversation_id, sender_type, content) VALUES (?, ?, 'customer', 'Hi')`,
			[]any{f.workspaceID, conversationID}},
		{`UPDATE support_conversations SET anonymized_at = now() WHERE id = ?`, []any{conversationID}},
		// AI usage is billing history and outlives the workspace.
		{`INSERT INTO billing_ai_usage_periods (id, workspace_id, period_start, period_end, allowance_microusd, enforcement_mode, pricing_version)
		  VALUES (?, ?, now(), now() + interval '30 days', 1000, 'soft', 'v1')`, []any{f.periodID, f.workspaceID}},
		{`INSERT INTO billing_ai_usage_ledger (workspace_id, period_id, entry_kind, feature_key, category, model_tier, provider,
		  canonical_model, route, service_tier, funding_mode, pricing_version, rate_snapshot, measurement_status, idempotency_key)
		  VALUES (?, ?, 'charge', 'support_ai', 'chat', 'standard', 'anthropic', 'claude', 'direct', 'default', 'included', 'v1', '{}', 'measured', ?)`,
			[]any{f.workspaceID, f.periodID, "ledger:" + uuid.NewString()}},
		{`INSERT INTO billing_ai_usage_settlements (workspace_id, period_id, exact_overage_microusd, rounded_invoice_cents,
		  rounding_adjustment_microusd, idempotency_key, status) VALUES (?, ?, 0, 0, 0, ?, 'settled')`,
			[]any{f.workspaceID, f.periodID, "settlement:" + uuid.NewString()}},
		{`INSERT INTO product_analytics_outbox (workspace_id, semantic_key, event_name, source, occurred_at)
		  VALUES (?, ?, 'workspace_deleted', 'api', now())`, []any{f.workspaceID, "test:" + uuid.NewString()}},
	} {
		if err := db.Exec(stmt.sql, stmt.args...).Error; err != nil {
			t.Fatalf("seed %q: %v", stmt.sql, err)
		}
	}
	return f
}

func workspaceRowCounts(t *testing.T, db *gorm.DB, workspaceID string) map[string]int64 {
	t.Helper()
	var tables []string
	if err := db.Raw(`
		SELECT c.table_name FROM information_schema.columns c
		  JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		 WHERE c.table_schema = current_schema() AND c.column_name = 'workspace_id' AND t.table_type = 'BASE TABLE'`).
		Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	counts := map[string]int64{}
	for _, table := range tables {
		var n int64
		if err := db.Table(table).Where("workspace_id = ?", workspaceID).Count(&n).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n > 0 {
			counts[table] = n
		}
	}
	return counts
}

func TestWorkspaceDeleteRemovesEveryWorkspaceRowPostgres(t *testing.T) {
	db := openWorkspaceDeletePostgres(t)
	ownerID := uuid.NewString()
	if err := db.Exec(`INSERT INTO users (id, email, password_hash, full_name) VALUES (?, ?, 'x', 'Owner')`,
		ownerID, "owner-"+ownerID[:8]+"@example.com").Error; err != nil {
		t.Fatal(err)
	}
	deleted := seedWorkspaceForDelete(t, db, ownerID, "WDA")
	kept := seedWorkspaceForDelete(t, db, ownerID, "WDB")
	keptBefore := workspaceRowCounts(t, db, kept.workspaceID)

	if err := NewWorkspaceRepository(db).Delete(context.Background(), deleted.workspaceID); err != nil {
		t.Fatalf("delete workspace: %v", err)
	}

	left := workspaceRowCounts(t, db, deleted.workspaceID)
	for table := range workspaceRetainedTables {
		delete(left, table)
	}
	if len(left) > 0 {
		t.Errorf("rows left for the deleted workspace: %v", left)
	}
	for _, check := range []struct {
		table, where, id string
	}{
		{"workspaces", "id = ?", deleted.workspaceID},
		{"crm_pipeline_stages", "pipeline_id = ?", deleted.pipelineID},
		{"notification_events", "notification_id = ?", deleted.notificationID},
	} {
		var n int64
		if err := db.Table(check.table).Where(check.where, check.id).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s still has %d rows for the deleted workspace", check.table, n)
		}
	}
	var deliveries int64
	if err := db.Raw(`SELECT count(*) FROM notification_deliveries d
		JOIN notification_events e ON e.id = d.notification_event_id WHERE e.notification_id = ?`,
		kept.notificationID).Scan(&deliveries).Error; err != nil {
		t.Fatal(err)
	}
	if deliveries != 1 {
		t.Errorf("kept workspace notification deliveries = %d, want 1", deliveries)
	}

	// The analytics outbox is retained; its foreign key nulls workspace_id.
	var outbox int64
	if err := db.Raw(`SELECT count(*) FROM product_analytics_outbox WHERE workspace_id IS NULL AND event_name = 'workspace_deleted'`).
		Scan(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	if outbox != 1 {
		t.Errorf("retained analytics outbox rows = %d, want 1", outbox)
	}

	// Billing history keeps the deleted workspace's id as an archived value.
	for _, table := range []string{"billing_ai_usage_periods", "billing_ai_usage_ledger", "billing_ai_usage_settlements"} {
		var n int64
		if err := db.Table(table).Where("workspace_id = ?", deleted.workspaceID).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("retained %s rows for the deleted workspace = %d, want 1", table, n)
		}
	}

	keptAfter := workspaceRowCounts(t, db, kept.workspaceID)
	for table, n := range keptBefore {
		if keptAfter[table] != n {
			t.Errorf("kept workspace %s rows = %d, want %d", table, keptAfter[table], n)
		}
	}
}
