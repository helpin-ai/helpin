//go:build integration

package repository

import (
	"context"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestListSignalOutcomeCalibrationPostgresMatchesMRRDecline(t *testing.T) {
	dsn := os.Getenv("CRM_SIGNAL_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set CRM_SIGNAL_TEST_POSTGRES_DSN to run PostgreSQL signal outcome tests")
	}
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get connection: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close connection: %v", err)
		}
	})
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin fixture: %v", tx.Error)
	}
	t.Cleanup(func() {
		if err := tx.Rollback().Error; err != nil {
			t.Errorf("rollback fixture: %v", err)
		}
	})
	// Temporary tables shadow application tables on this transaction's connection.
	for _, statement := range []string{
		`CREATE TEMP TABLE crm_signals (
			workspace_id text, company_id text, rule_key text, rule_version integer,
			commercial_motion text, evidence_identity_method text, detected_at timestamptz,
			interpretation_snapshot jsonb, replay_calibration_excluded boolean
		) ON COMMIT DROP`,
		`CREATE TEMP TABLE crm_company_commercial_state_history (
			workspace_id text, company_id text, applied_to_current boolean,
			state_updated_at timestamptz, state_snapshot jsonb
		) ON COMMIT DROP`,
	} {
		if err := tx.Exec(statement).Error; err != nil {
			t.Fatalf("create fixture: %v", err)
		}
	}
	for _, fixture := range []struct {
		companyID  string
		detectedAt string
	}{
		{"declined", "now() - interval '100 days'"},
		{"unchanged", "now() - interval '100 days'"},
		{"recent", "now() - interval '5 days'"},
	} {
		if err := tx.Exec(`INSERT INTO crm_signals VALUES (
			'workspace', ?, 'usage_drop', 1, 'retention', 'verified', `+fixture.detectedAt+`,
			'{"commercial_state":{"mrr_minor":5000}}'::jsonb, false)`, fixture.companyID).Error; err != nil {
			t.Fatalf("seed signal %s: %v", fixture.companyID, err)
		}
	}
	if err := tx.Exec(`INSERT INTO crm_company_commercial_state_history VALUES (
		'workspace', 'declined', true, now() - interval '90 days', '{"mrr_minor":3000}'::jsonb)`).Error; err != nil {
		t.Fatalf("seed outcome: %v", err)
	}

	rows, err := NewCRMSignalRepository(tx).ListSignalOutcomeCalibration(context.Background(), "workspace", 90)
	if err != nil {
		t.Fatalf("list outcome calibration: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %#v, want one usage_drop row", rows)
	}
	row := rows[0]
	if row.MaturedSignals != 2 || row.OutcomeMatched != 1 || row.OutcomePrecision != 0.5 || row.HorizonDays != 90 {
		t.Fatalf("row = %#v, want 2 matured, 1 matched, 0.5 precision over 90 days", row)
	}
}
