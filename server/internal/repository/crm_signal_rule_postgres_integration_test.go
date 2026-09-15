//go:build integration

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestActivationStalledPostgresFiltersCommercialState(t *testing.T) {
	dsn := os.Getenv("CRM_SIGNAL_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set CRM_SIGNAL_TEST_POSTGRES_DSN to run PostgreSQL signal rule tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
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
		`CREATE TEMP TABLE crm_company_commercial_states (workspace_id text, company_id text, state jsonb) ON COMMIT DROP`,
		`CREATE TEMP TABLE crm_signal_condition_states (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id text, company_id text,
			rule_key text, rule_version integer, motion text, scope_key text, armed boolean,
			triggered_at timestamptz, rearmed_at timestamptz, last_value double precision, updated_at timestamptz
		) ON COMMIT DROP`,
	} {
		if err := tx.Exec(statement).Error; err != nil {
			t.Fatalf("create fixture: %v", err)
		}
	}
	end := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	for _, fixture := range []struct {
		companyID string
		state     string
	}{
		{"stalled", `{"onboarding_started_at":"2026-09-06T00:00:00Z"}`},
		{"recent", `{"onboarding_started_at":"2026-09-06T00:00:01Z"}`},
		{"activated", `{"onboarding_started_at":"2026-09-01T00:00:00Z","first_value_at":"2026-09-02T00:00:00Z"}`},
		{"first-value-key-present", `{"onboarding_started_at":"2026-09-01T00:00:00Z","first_value_at":null}`},
		{"missing-onboarding", `{}`},
	} {
		if err := tx.Exec(`INSERT INTO crm_company_commercial_states VALUES (?, ?, ?::jsonb)`, "workspace", fixture.companyID, fixture.state).Error; err != nil {
			t.Fatalf("seed %s: %v", fixture.companyID, err)
		}
	}
	repo := NewCRMSignalRepository(tx)
	config := model.CRMSignalRuleConfig{RuleKey: model.CRMSignalRuleActivationStalled, Version: 1}
	candidates, err := repo.EvaluatePostgresSignalRule(context.Background(), config, end.Add(-24*time.Hour), end)
	if err != nil {
		t.Fatalf("evaluate activation stalled: %v", err)
	}
	if len(candidates) != 1 || candidates[0].CompanyID == nil || *candidates[0].CompanyID != "stalled" {
		t.Fatalf("candidates = %#v, want only stalled company at the seven-day cutoff", candidates)
	}
}
