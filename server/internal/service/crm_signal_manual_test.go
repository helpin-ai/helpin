package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateManualSignalOwnsTrustAndValidatesWorkspaceReferences(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-manual-signal?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE crm_buyer_signals (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT NOT NULL, contact_id TEXT, deal_id TEXT, company_id TEXT, signal_type TEXT NOT NULL, source_type TEXT NOT NULL, source_id TEXT, source_thread_id TEXT, summary TEXT NOT NULL, evidence_excerpt TEXT, metadata BLOB, confidence REAL, detected_at DATETIME, detector_kind TEXT, signal_domain TEXT, polarity TEXT, rule_key TEXT, rule_version INTEGER, window_started_at DATETIME, window_ended_at DATETIME, evidence_identity_method TEXT, evidence_identity_trust TEXT, evidence_fingerprint TEXT, dismissed_at DATETIME, dismissed_by_member_id TEXT, dismissal_reason TEXT, reviewed_at DATETIME, acted_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT)`,
		`CREATE TABLE crm_deals (id TEXT PRIMARY KEY, workspace_id TEXT, commercial_motion TEXT)`,
		`CREATE TABLE crm_companies (id TEXT PRIMARY KEY, workspace_id TEXT, customer_success_owner_member_id TEXT)`,
		`INSERT INTO crm_contacts VALUES ('contact-1', 'ws-1'), ('contact-2', 'ws-2')`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("schema/seed: %v", err)
		}
	}
	svc := NewCRMSignalService(repository.NewCRMSignalRepository(db), nil)
	ctx := context.Background()
	foreignContact := "contact-2"
	if _, err := svc.CreateSignal(ctx, model.CreateCRMBuyerSignalRequest{WorkspaceID: "ws-1", ContactID: &foreignContact, SignalType: model.CRMSignalBuyingIntent, Summary: "Spoofed"}); err == nil {
		t.Fatal("expected cross-workspace contact to be rejected")
	}
	contactID := "contact-1"
	signal, err := svc.CreateSignal(ctx, model.CreateCRMBuyerSignalRequest{
		WorkspaceID: "ws-1", ContactID: &contactID, SignalType: model.CRMSignalBuyingIntent, Summary: "User observed buying intent",
	})
	if err != nil {
		t.Fatalf("create manual signal: %v", err)
	}
	if signal.SourceType != model.CRMSignalSourceManual || signal.DetectorKind != model.CRMSignalDetectorManual || signal.RuleKey != nil || signal.RuleVersion != nil || signal.EvidenceIdentityMethod != model.IdentityMethodManualEntry || signal.EvidenceIdentityTrust != model.IdentityTrustUntrusted {
		t.Fatalf("manual trust boundary was not enforced: %#v", signal)
	}
}
