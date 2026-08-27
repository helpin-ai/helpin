package repository

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCRMSuggestionRepositoryScopesMutationsToWorkspace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-suggestion-scope?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE crm_suggestions (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, user_id TEXT, suggestion_type TEXT, object_type TEXT, object_id TEXT, title TEXT, description TEXT, context BLOB, signal_ids TEXT DEFAULT '{}', status TEXT, confidence REAL, execution_status TEXT, executed_at DATETIME, execution_error TEXT, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_buyer_signals (id TEXT PRIMARY KEY, workspace_id TEXT, contact_id TEXT, deal_id TEXT, company_id TEXT, signal_type TEXT, source_type TEXT, source_id TEXT, source_thread_id TEXT, summary TEXT, evidence_excerpt TEXT, metadata BLOB, confidence REAL, detected_at DATETIME, detector_kind TEXT, signal_domain TEXT, polarity TEXT, rule_key TEXT, rule_version INTEGER, window_started_at DATETIME, window_ended_at DATETIME, evidence_identity_method TEXT, evidence_identity_trust TEXT, evidence_fingerprint TEXT, dismissed_at DATETIME, dismissed_by_member_id TEXT, dismissal_reason TEXT, reviewed_at DATETIME, acted_at DATETIME, created_at DATETIME)`,
		`INSERT INTO crm_suggestions (id, workspace_id, suggestion_type, title, context, status, confidence, execution_status) VALUES ('suggestion-1', 'ws-1', 'deal_create', 'Mine', '{}', 'pending', .8, 'pending'), ('suggestion-2', 'ws-2', 'deal_create', 'Theirs', '{}', 'pending', .8, 'pending')`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("schema/seed: %v", err)
		}
	}
	repo := NewCRMSuggestionRepository(db)
	ctx := context.Background()

	foreign, err := repo.GetByID(ctx, "ws-1", "suggestion-2")
	if err != nil || foreign != nil {
		t.Fatalf("cross-workspace lookup = %#v, err=%v", foreign, err)
	}
	if err := repo.Delete(ctx, "ws-1", "suggestion-2"); err == nil {
		t.Fatal("expected cross-workspace delete to fail")
	}
	forged := &model.CRMSuggestion{ID: "suggestion-2", WorkspaceID: "ws-2", Title: "Changed", Status: model.CRMSuggestionStatusAccepted}
	if err := repo.Update(ctx, "ws-1", forged); err == nil {
		t.Fatal("expected cross-workspace update to fail")
	}
	owned, err := repo.GetByID(ctx, "ws-2", "suggestion-2")
	if err != nil || owned == nil || owned.Title != "Theirs" || owned.Status != model.CRMSuggestionStatusPending {
		t.Fatalf("foreign suggestion changed: %#v, err=%v", owned, err)
	}
	claimed, err := repo.ClaimPending(ctx, "ws-1", &model.CRMSuggestion{ID: "suggestion-1", Context: model.JSONB{"deal_id": "deal-1"}})
	if err != nil || !claimed {
		t.Fatalf("first claim = %v, err=%v", claimed, err)
	}
	claimed, err = repo.ClaimPending(ctx, "ws-1", &model.CRMSuggestion{ID: "suggestion-1"})
	if err != nil || claimed {
		t.Fatalf("second claim = %v, err=%v; want idempotent rejection", claimed, err)
	}
}
