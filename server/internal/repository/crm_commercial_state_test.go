package repository

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAcceptCompanyCommercialStateIsReplayAndTenantSafe(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-commercial-state-idempotency?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE crm_companies (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, customer_success_owner_member_id TEXT)`,
		`CREATE TABLE workspace_members (id TEXT PRIMARY KEY, workspace_id TEXT, status TEXT)`,
		`CREATE TABLE crm_company_commercial_states (company_id TEXT PRIMARY KEY, workspace_id TEXT, version INTEGER, state BLOB, state_updated_at DATETIME, source_event_id TEXT, accepted_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_company_commercial_state_history (id TEXT PRIMARY KEY, workspace_id TEXT, company_id TEXT, version INTEGER, patch BLOB, state_snapshot BLOB, state_updated_at DATETIME, source_event_id TEXT, applied_to_current BOOLEAN, accepted_at DATETIME, created_at DATETIME, UNIQUE(workspace_id, source_event_id))`,
		`CREATE TABLE crm_company_commercial_state_health (company_id TEXT PRIMARY KEY, workspace_id TEXT, last_accepted_at DATETIME, last_rejected_at DATETIME, last_rejection_reason TEXT, rejected_update_count INTEGER DEFAULT 0, updated_at DATETIME)`,
		`INSERT INTO crm_companies VALUES ('company-1', 'workspace-1', NULL)`,
		`INSERT INTO crm_companies VALUES ('company-2', 'workspace-2', NULL)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create state schema: %v", err)
		}
	}
	repo := NewCRMSignalRepository(db)
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	accept := func(workspaceID, companyID string) (bool, error) {
		return repo.AcceptCompanyCommercialState(
			context.Background(), workspaceID, companyID, "shared-event-id",
			model.JSONB{"plan_key": "growth"}, model.JSONB{"plan_key": "growth"},
			time.Time{}, now, now,
		)
	}
	created, err := accept("workspace-1", "company-1")
	if err != nil || !created {
		t.Fatalf("first accept created=%v err=%v", created, err)
	}
	created, err = accept("workspace-1", "company-1")
	if err != nil || created {
		t.Fatalf("replay accept created=%v err=%v", created, err)
	}
	created, err = accept("workspace-2", "company-2")
	if err != nil || !created {
		t.Fatalf("cross-tenant same event ID created=%v err=%v", created, err)
	}
}

func TestSignalOperationalHistoryRetention(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-signal-history-retention?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE crm_signal_motion_states (id TEXT PRIMARY KEY, effective_at DATETIME)`,
		`CREATE TABLE crm_usage_weekday_baselines (id TEXT PRIMARY KEY, window_ended_at DATETIME)`,
		`INSERT INTO crm_signal_motion_states VALUES ('old-motion', '2025-01-01'), ('new-motion', '2026-08-01')`,
		`INSERT INTO crm_usage_weekday_baselines VALUES ('old-baseline', '2026-01-01'), ('new-baseline', '2026-08-01')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed retention schema: %v", err)
		}
	}
	repo := NewCRMSignalRepository(db)
	cutoff := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if deleted, err := repo.PruneSignalMotionStates(context.Background(), cutoff); err != nil || deleted != 1 {
		t.Fatalf("prune motion states deleted=%d err=%v", deleted, err)
	}
	if deleted, err := repo.PruneUsageWeekdayBaselines(context.Background(), cutoff); err != nil || deleted != 1 {
		t.Fatalf("prune baselines deleted=%d err=%v", deleted, err)
	}
	for _, table := range []string{"crm_signal_motion_states", "crm_usage_weekday_baselines"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s count=%d err=%v", table, count, err)
		}
	}
}
