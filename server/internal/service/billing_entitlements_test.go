package service

import (
	"fmt"
	"gorm.io/gorm"
	"testing"
	"time"
)

func createEntitlementBillingTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, stmt := range []string{
		`CREATE TABLE workspace_billing (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL UNIQUE,
			plan TEXT NOT NULL DEFAULT 'growth',
			status TEXT NOT NULL DEFAULT 'trialing',
			stripe_customer_id TEXT,
			stripe_subscription_id TEXT,
			stripe_price_id TEXT,
			billing_interval TEXT NOT NULL DEFAULT 'monthly',
			included_credits INTEGER NOT NULL DEFAULT 25000,
			credits_used INTEGER NOT NULL DEFAULT 0,
			on_demand_enabled BOOLEAN NOT NULL DEFAULT 0,
			on_demand_blocks_invoiced INTEGER NOT NULL DEFAULT 0,
			current_period_start DATETIME NOT NULL,
			current_period_end DATETIME NOT NULL,
			trial_ends_at DATETIME,
			pending_plan TEXT,
			pending_billing_interval TEXT,
			pending_change_at DATETIME,
			cancel_at_period_end BOOLEAN NOT NULL DEFAULT 0,
			canceled_at DATETIME,
			billing_notice_type TEXT,
			billing_notice_message TEXT,
			billing_notice_at DATETIME,
			payment_failed_at DATETIME,
			trial_will_end_at DATETIME,
			last_stripe_event_id TEXT,
			payment_method_id TEXT,
			billing_owner_user_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE billing_credit_ledger (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			feature_key TEXT NOT NULL DEFAULT '',
			credits INTEGER NOT NULL,
			idempotency_key TEXT NOT NULL UNIQUE,
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create entitlement billing table: %v", err)
		}
	}
}

func createCRMImportEntitlementTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE crm_import_jobs (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		source TEXT NOT NULL DEFAULT 'csv',
		status TEXT NOT NULL DEFAULT 'pending',
		object_type TEXT NOT NULL,
		file_url TEXT,
		column_mapping TEXT NOT NULL DEFAULT '{}',
		total_rows INTEGER NOT NULL DEFAULT 0,
		processed_rows INTEGER NOT NULL DEFAULT 0,
		created_rows INTEGER NOT NULL DEFAULT 0,
		updated_rows INTEGER NOT NULL DEFAULT 0,
		error_count INTEGER NOT NULL DEFAULT 0,
		error_log TEXT NOT NULL DEFAULT '[]',
		created_by TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create crm_import_jobs table: %v", err)
	}
}

func seedCRMContactsForEntitlement(t *testing.T, db *gorm.DB, workspaceID string, count int) {
	t.Helper()

	now := time.Now()
	for i := 0; i < count; i++ {
		if err := db.Exec(`INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, lifecycle_stage, lead_status, custom_properties, created_at, updated_at) VALUES (?, ?, ?, ?, 'subscriber', 'new', '{}', ?, ?)`,
			fmt.Sprintf("contact-%d", i), workspaceID, fmt.Sprintf("CON-%d", i+1), fmt.Sprintf("Contact %d", i), now, now).Error; err != nil {
			t.Fatalf("seed contact %d: %v", i, err)
		}
	}
}
