//go:build integration

package repository

import (
	"context"
	"fmt"
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

func TestSupportSignalPostgresQueries(t *testing.T) {
	dsn := os.Getenv("CRM_SIGNAL_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set CRM_SIGNAL_TEST_POSTGRES_DSN")
	}
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	for _, q := range []string{
		`CREATE TEMP TABLE support_conversations (id text,workspace_id text,crm_company_id text,created_at timestamptz,status text) ON COMMIT DROP`,
		`CREATE TEMP TABLE support_messages (conversation_id text,created_at timestamptz,message_type text,metadata text) ON COMMIT DROP`,
	} {
		if err := tx.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	end := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 8; i++ {
		at := end.AddDate(0, 0, -10)
		if i < 3 {
			at = end.AddDate(0, 0, -1)
		}
		if err := tx.Exec(`INSERT INTO support_conversations VALUES (?,?,?,?,'open')`, fmt.Sprint(i), "workspace", "company", at).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []struct {
		days   int
		rating string
	}{{1, "1"}, {2, "2"}, {40, "4"}, {41, "5"}, {1, "invalid"}} {
		if err := tx.Exec(`INSERT INTO support_messages VALUES ('0',?,'csat_survey',?)`, end.AddDate(0, 0, -f.days), `{"rating":"`+f.rating+`"}`).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewCRMSignalRepository(tx)
	t.Run("fractional volume multiplier", func(t *testing.T) {
		rows, err := repo.EvaluatePostgresSignalRule(context.Background(), model.CRMSignalRuleConfig{RuleKey: model.CRMSignalRuleSupportVolumeSpike}, end.Add(-time.Hour), end)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("got %d candidates", len(rows))
		}
	})
	t.Run("rating regex does not consume a parameter", func(t *testing.T) {
		rows, err := repo.EvaluatePostgresSignalRule(context.Background(), model.CRMSignalRuleConfig{RuleKey: model.CRMSignalRuleSupportCSATDeterioration}, end.Add(-time.Hour), end)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("got %d candidates", len(rows))
		}
	})
}
