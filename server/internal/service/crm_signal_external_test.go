package service

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestExternalSignalEvidenceIsNormalizedAndIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:external-signal-evidence?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE crm_signal_rule_configs (id TEXT PRIMARY KEY, workspace_id TEXT, rule_key TEXT, version INTEGER, cadence TEXT, enabled BOOLEAN, shadow_mode BOOLEAN, activation_eligible BOOLEAN, thresholds BLOB, business_weight REAL, half_life_days REAL, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_buyer_signals (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT, contact_id TEXT, deal_id TEXT, company_id TEXT, signal_type TEXT, source_type TEXT, source_id TEXT, source_thread_id TEXT, summary TEXT, evidence_excerpt TEXT, metadata BLOB, confidence REAL, detected_at DATETIME, detector_kind TEXT, signal_domain TEXT, polarity TEXT, rule_key TEXT, rule_version INTEGER, window_started_at DATETIME, window_ended_at DATETIME, evidence_identity_method TEXT, evidence_identity_trust TEXT, evidence_fingerprint TEXT, dismissed_at DATETIME, dismissed_by_member_id TEXT, dismissal_reason TEXT, reviewed_at DATETIME, acted_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE crm_signal_external_evidence (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT, provider TEXT, provider_evidence_id TEXT, evidence_type TEXT, rule_key TEXT, rule_version INTEGER, signal_type TEXT, signal_domain TEXT, polarity TEXT, summary TEXT, evidence_excerpt TEXT, source_url TEXT, contact_id TEXT, deal_id TEXT, company_id TEXT, identity_method TEXT, identity_trust TEXT, provenance BLOB, observed_at DATETIME, signal_id TEXT, created_at DATETIME, UNIQUE(workspace_id, provider, provider_evidence_id, rule_key, rule_version))`,
		`CREATE TABLE crm_companies (id TEXT PRIMARY KEY, workspace_id TEXT, customer_success_owner_member_id TEXT)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO crm_signal_rule_configs (id, rule_key, version, cadence, enabled, shadow_mode, activation_eligible, thresholds, business_weight, half_life_days) VALUES ('external-v1', ?, 1, 'daily', true, true, false, '{}', 5, 30)`, model.CRMSignalRuleExternalEvidence).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	if err := db.Exec(`INSERT INTO crm_companies (id, workspace_id) VALUES ('company-1', 'ws-1'), ('company-2', 'ws-2')`).Error; err != nil {
		t.Fatalf("seed companies: %v", err)
	}
	svc := NewCRMSignalService(repository.NewCRMSignalRepository(db), nil)
	companyID := "company-1"
	req := model.IngestCRMSignalExternalEvidenceRequest{
		WorkspaceID: "ws-1", Provider: "Clearbit", ProviderEvidenceID: "funding-42",
		EvidenceType: model.CRMExternalEvidenceFunding, RuleKey: model.CRMSignalRuleExternalEvidence, RuleVersion: 1,
		SignalType: model.CRMSignalBuyingIntent, SignalDomain: model.CRMSignalDomainMarket, Polarity: model.CRMSignalPolarityPositive,
		Summary: "Acme raised a Series B", CompanyID: &companyID, IdentityMethod: "provider_company_match",
		IdentityTrust: "probabilistic", Provenance: map[string]interface{}{"provider_confidence": 0.92}, ObservedAt: time.Now().UTC().Add(-time.Hour),
	}
	evidence, created, err := svc.IngestExternalEvidence(context.Background(), req)
	if err != nil || !created {
		t.Fatalf("ingest evidence created=%v evidence=%+v err=%v", created, evidence, err)
	}
	if evidence.Provider != "clearbit" || evidence.SignalID == nil {
		t.Fatalf("normalized evidence = %+v", evidence)
	}
	evidence, created, err = svc.IngestExternalEvidence(context.Background(), req)
	if err != nil || created || evidence.SignalID == nil {
		t.Fatalf("duplicate evidence created=%v evidence=%+v err=%v", created, evidence, err)
	}
	var signals []model.CRMBuyerSignal
	if err := db.Find(&signals).Error; err != nil || len(signals) != 1 {
		t.Fatalf("signals=%+v err=%v", signals, err)
	}
	if signals[0].SourceType != model.CRMSignalSourceExternal || signals[0].SignalDomain != model.CRMSignalDomainMarket || signals[0].EvidenceIdentityMethod != "external_provider:clearbit" || signals[0].EvidenceIdentityTrust != "probabilistic" {
		t.Fatalf("normalized signal = %+v", signals[0])
	}
	otherCompanyID := "company-2"
	req.ProviderEvidenceID = "funding-43"
	req.CompanyID = &otherCompanyID
	if _, _, err := svc.IngestExternalEvidence(context.Background(), req); err == nil {
		t.Fatal("expected cross-workspace company reference to be rejected")
	}
}

func TestExternalSignalEvidenceRejectsUnknownDimensions(t *testing.T) {
	svc := NewCRMSignalService(nil, nil)
	_, _, err := svc.IngestExternalEvidence(context.Background(), model.IngestCRMSignalExternalEvidenceRequest{
		WorkspaceID: "ws-1", Provider: "vendor", ProviderEvidenceID: "1", EvidenceType: "rumor",
		RuleKey: model.CRMSignalRuleExternalEvidence, RuleVersion: 1, SignalType: model.CRMSignalBuyingIntent,
		SignalDomain: model.CRMSignalDomainMarket, Polarity: model.CRMSignalPolarityPositive, Summary: "Rumor",
		IdentityMethod: "provider", IdentityTrust: "untrusted", ObservedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected invalid evidence dimensions")
	}
}

func TestExternalSignalEvidenceRejectsDisabledRuleVersion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:disabled-external-signal-rule?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE crm_signal_rule_configs (id TEXT PRIMARY KEY, workspace_id TEXT, rule_key TEXT, version INTEGER, enabled BOOLEAN)`).Error; err != nil {
		t.Fatalf("schema: %v", err)
	}
	if err := db.Exec(`INSERT INTO crm_signal_rule_configs (id, rule_key, version, enabled) VALUES ('disabled-v1', ?, 1, false)`, model.CRMSignalRuleExternalEvidence).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	svc := NewCRMSignalService(repository.NewCRMSignalRepository(db), nil)
	companyID := "company-1"
	_, _, err = svc.IngestExternalEvidence(context.Background(), model.IngestCRMSignalExternalEvidenceRequest{
		WorkspaceID: "ws-1", Provider: "vendor", ProviderEvidenceID: "1",
		EvidenceType: model.CRMExternalEvidenceFunding, RuleKey: model.CRMSignalRuleExternalEvidence, RuleVersion: 1,
		SignalType: model.CRMSignalBuyingIntent, SignalDomain: model.CRMSignalDomainMarket,
		Polarity: model.CRMSignalPolarityPositive, Summary: "Funding event", CompanyID: &companyID,
		IdentityMethod: "provider", IdentityTrust: "probabilistic", ObservedAt: time.Now().UTC(),
	})
	if err == nil || err.Error() != "signal rule version is disabled" {
		t.Fatalf("expected disabled rule error, got %v", err)
	}
}
