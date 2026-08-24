package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestRuleSweepWindowUsesRequiredOverlap(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 7, 0, 0, time.UTC)
	end, initial, overlap := ruleSweepWindow(model.CRMSignalRuleCadenceMicroBatch, now)
	if !end.Equal(time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)) || !initial.Equal(end.Add(-time.Hour)) || overlap != 15*time.Minute {
		t.Fatalf("micro-batch window end=%v initial=%v overlap=%v", end, initial, overlap)
	}
	end, _, overlap = ruleSweepWindow(model.CRMSignalRuleCadenceDaily, now)
	if !end.Equal(time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)) || overlap != 48*time.Hour {
		t.Fatalf("daily window end=%v overlap=%v", end, overlap)
	}
}

func TestPersistBehavioralCandidateKeepsUntrustedEvidenceContextOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:signal_evaluator_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE crm_buyer_signals (id TEXT PRIMARY KEY, workspace_id TEXT, contact_id TEXT, deal_id TEXT, company_id TEXT, signal_type TEXT, source_type TEXT, source_id TEXT, source_thread_id TEXT, summary TEXT, evidence_excerpt TEXT, metadata BLOB, confidence REAL, detected_at DATETIME, detector_kind TEXT, signal_domain TEXT, polarity TEXT, rule_key TEXT, rule_version INTEGER, window_started_at DATETIME, window_ended_at DATETIME, evidence_identity_method TEXT, evidence_identity_trust TEXT, evidence_fingerprint TEXT, dismissed_at DATETIME, dismissed_by_member_id TEXT, created_at DATETIME)`,
		`CREATE TABLE crm_identity_links (id TEXT PRIMARY KEY, workspace_id TEXT, anonymous_id TEXT, contact_id TEXT, company_id TEXT, identity_method TEXT, identity_trust TEXT, verified_at DATETIME, verifier_version TEXT, created_at DATETIME)`,
		`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT)`,
		`CREATE TABLE crm_companies (id TEXT PRIMARY KEY, workspace_id TEXT, external_id TEXT)`,
		`CREATE TABLE crm_pipeline_stages (id TEXT PRIMARY KEY, stage_type TEXT)`,
		`CREATE TABLE crm_deals (id TEXT PRIMARY KEY, workspace_id TEXT, stage_id TEXT, updated_at DATETIME)`,
		`CREATE TABLE crm_associations (id TEXT PRIMARY KEY, workspace_id TEXT, from_object_type TEXT, from_object_id TEXT, to_object_type TEXT, to_object_id TEXT)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	companyID := "company-1"
	if err := db.Table("crm_companies").Create(map[string]interface{}{"id": companyID, "workspace_id": "workspace-1"}).Error; err != nil {
		t.Fatalf("seed company: %v", err)
	}
	if err := db.Table("crm_identity_links").Create(map[string]interface{}{"id": "link-1", "workspace_id": "workspace-1", "anonymous_id": "anon-1",
		"company_id": companyID, "identity_method": model.IdentityMethodBrowserClaim, "identity_trust": model.IdentityTrustUntrusted, "created_at": time.Now().UTC()}).Error; err != nil {
		t.Fatalf("seed identity link: %v", err)
	}

	repo := repository.NewCRMSignalRepository(db)
	evaluator := NewCRMSignalRuleEvaluator(repo, nil, nil, "test-owner")
	config := model.CRMSignalRuleConfig{RuleKey: model.CRMSignalRuleRepeatedPricingActivity, Version: 1,
		Cadence: model.CRMSignalRuleCadenceMicroBatch, ActivationEligible: true, ShadowMode: false}
	observed := time.Date(2026, 8, 24, 11, 55, 0, 0, time.UTC)
	candidate := model.CRMSignalRuleCandidate{WorkspaceID: "workspace-1", AnonymousID: "anon-1",
		RuleKey: config.RuleKey, SignalType: model.CRMSignalBuyingIntent, SignalDomain: model.CRMSignalDomainWebBehavior,
		Polarity: model.CRMSignalPolarityPositive, SourceType: model.CRMSignalSourceWeb,
		Summary: "Repeated pricing activity", EvidenceExcerpt: "/pricing", EvidenceFingerprint: "pricing-1",
		EvidenceIdentityMethod: model.IdentityMethodBrowserClaim, EvidenceIdentityTrust: model.IdentityTrustUntrusted,
		ObservedAt: observed, Metadata: model.JSONB{}}
	created, err := evaluator.persistCandidate(context.Background(), config, &candidate, observed.Add(-15*time.Minute), observed.Add(5*time.Minute))
	if err != nil || !created {
		t.Fatalf("persist candidate created=%v err=%v", created, err)
	}
	var signal model.CRMBuyerSignal
	if err := db.First(&signal).Error; err != nil {
		t.Fatalf("read signal: %v", err)
	}
	if signal.EvidenceIdentityTrust != model.IdentityTrustUntrusted || signal.Metadata["activation_eligible"] != false {
		t.Fatalf("stored trust=%s metadata=%#v", signal.EvidenceIdentityTrust, signal.Metadata)
	}
}
