package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupSignalRuleRepositoryTest(t *testing.T) (*gorm.DB, *CRMSignalRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:signal_rules_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE crm_signal_rule_configs (id TEXT PRIMARY KEY, workspace_id TEXT, rule_key TEXT, version INTEGER, cadence TEXT, enabled BOOLEAN, shadow_mode BOOLEAN, activation_eligible BOOLEAN, thresholds BLOB, business_weight REAL, half_life_days REAL, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_signal_evaluator_watermarks (cadence TEXT PRIMARY KEY, watermark DATETIME, lease_owner TEXT, lease_until DATETIME, last_started_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_buyer_signals (id TEXT PRIMARY KEY, workspace_id TEXT, contact_id TEXT, deal_id TEXT, company_id TEXT, signal_type TEXT, source_type TEXT, source_id TEXT, source_thread_id TEXT, summary TEXT, evidence_excerpt TEXT, metadata BLOB, confidence REAL, detected_at DATETIME, detector_kind TEXT, signal_domain TEXT, polarity TEXT, rule_key TEXT, rule_version INTEGER, window_started_at DATETIME, window_ended_at DATETIME, evidence_identity_method TEXT, evidence_identity_trust TEXT, evidence_fingerprint TEXT, dismissed_at DATETIME, dismissed_by_member_id TEXT, dismissal_reason TEXT, reviewed_at DATETIME, acted_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE crm_identity_links (id TEXT PRIMARY KEY, workspace_id TEXT, anonymous_id TEXT, contact_id TEXT, company_id TEXT, identity_method TEXT, identity_trust TEXT, verified_at DATETIME, verifier_version TEXT, created_at DATETIME)`,
		`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT)`,
		`CREATE TABLE crm_companies (id TEXT PRIMARY KEY, workspace_id TEXT, external_id TEXT)`,
		`CREATE TABLE crm_pipeline_stages (id TEXT PRIMARY KEY, stage_type TEXT)`,
		`CREATE TABLE crm_deals (id TEXT PRIMARY KEY, workspace_id TEXT, stage_id TEXT, updated_at DATETIME)`,
		`CREATE TABLE crm_associations (id TEXT PRIMARY KEY, workspace_id TEXT, from_object_type TEXT, from_object_id TEXT, to_object_type TEXT, to_object_id TEXT)`,
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT, crm_company_id TEXT, crm_contact_id TEXT, subject TEXT, ai_escalated_at DATETIME)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create signal rule schema: %v", err)
		}
	}
	return db, NewCRMSignalRepository(db)
}

func TestAdditionalCaptureRuleDimensions(t *testing.T) {
	tests := []struct {
		rule, wantSummary string
	}{
		{model.CRMSignalRuleConfiguredForm, "Configured high-intent form submitted"},
		{model.CRMSignalRuleIdentifiedArticleView, "Identified contact viewed a relevant article"},
		{model.CRMSignalRuleVersionedInteraction, "Versioned high-intent interaction observed"},
	}
	for _, test := range tests {
		_, domain, _, source, summary := behavioralRuleDimensions(test.rule)
		if domain != model.CRMSignalDomainWebBehavior || source != model.CRMSignalSourceWeb || summary != test.wantSummary {
			t.Fatalf("rule %s dimensions = domain %s source %s summary %q", test.rule, domain, source, summary)
		}
	}
}

func TestEvaluateSupportEscalationRule(t *testing.T) {
	db, repo := setupSignalRuleRepositoryTest(t)
	start := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	end := start.Add(48 * time.Hour)
	if err := db.Table("support_conversations").Create(map[string]interface{}{
		"id": "conversation-1", "workspace_id": "workspace-1", "crm_company_id": "company-1",
		"crm_contact_id": "contact-1", "subject": "Production outage", "ai_escalated_at": start.Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed escalation: %v", err)
	}
	config := model.CRMSignalRuleConfig{RuleKey: model.CRMSignalRuleSupportAIEscalation, Version: 1, Cadence: model.CRMSignalRuleCadenceDaily}
	candidates, err := repo.EvaluatePostgresSignalRule(context.Background(), config, start, end)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates=%#v err=%v", candidates, err)
	}
	if candidates[0].EvidenceIdentityMethod != model.IdentityMethodVerifiedSupport || candidates[0].EvidenceIdentityTrust != model.IdentityTrustVerified {
		t.Fatalf("candidate provenance=%#v", candidates[0])
	}
}

func TestSignalRuleConfigLeaseAndIdempotency(t *testing.T) {
	db, repo := setupSignalRuleRepositoryTest(t)
	ctx := context.Background()
	configs := []model.CRMSignalRuleConfig{
		{ID: "config-v1", RuleKey: model.CRMSignalRuleDealGoneDark, Version: 1, Cadence: model.CRMSignalRuleCadenceDaily, Enabled: true},
		{ID: "config-v2", RuleKey: model.CRMSignalRuleDealGoneDark, Version: 2, Cadence: model.CRMSignalRuleCadenceDaily, Enabled: true},
	}
	if err := db.Create(&configs).Error; err != nil {
		t.Fatalf("seed configs: %v", err)
	}
	active, err := repo.ListActiveSignalRuleConfigs(ctx, model.CRMSignalRuleCadenceDaily)
	if err != nil || len(active) != 1 || active[0].Version != 2 {
		t.Fatalf("active configs = %#v, err=%v", active, err)
	}

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	watermark, acquired, err := repo.TryAcquireSignalEvaluatorLease(ctx, model.CRMSignalRuleCadenceDaily, "pod-a", now, time.Minute, now.Add(-48*time.Hour))
	if err != nil || !acquired || !watermark.Equal(now.Add(-48*time.Hour)) {
		t.Fatalf("first lease watermark=%v acquired=%v err=%v", watermark, acquired, err)
	}
	if _, acquired, err := repo.TryAcquireSignalEvaluatorLease(ctx, model.CRMSignalRuleCadenceDaily, "pod-b", now, time.Minute, now); err != nil || acquired {
		t.Fatalf("competing lease acquired=%v err=%v", acquired, err)
	}
	if err := repo.ReleaseSignalEvaluatorLease(ctx, model.CRMSignalRuleCadenceDaily, "pod-a", now); err != nil {
		t.Fatalf("release lease: %v", err)
	}

	start, end := now.Add(-24*time.Hour), now
	rule, version := model.CRMSignalRuleDealGoneDark, 2
	dealID := "deal-1"
	signal := &model.CRMBuyerSignal{
		WorkspaceID: "workspace-1", DealID: &dealID, SignalType: model.CRMSignalRiskSignal,
		SourceType: model.CRMSignalSourceCRM, Summary: "Deal went dark", Confidence: 1, DetectedAt: start,
		DetectorKind: model.CRMSignalDetectorRuleDerived, RuleKey: &rule, RuleVersion: &version,
		WindowStartedAt: &start, WindowEndedAt: &end, EvidenceFingerprint: "fingerprint-1",
	}
	created, err := repo.CreateRuleSignalIfAbsent(ctx, signal)
	if err != nil || !created {
		t.Fatalf("create rule signal created=%v err=%v", created, err)
	}
	signal.ID = ""
	created, err = repo.CreateRuleSignalIfAbsent(ctx, signal)
	if err != nil || created {
		t.Fatalf("duplicate rule signal created=%v err=%v", created, err)
	}
}

func TestResolveBehavioralIdentityAndOpenDeal(t *testing.T) {
	db, repo := setupSignalRuleRepositoryTest(t)
	ctx := context.Background()
	contactID, companyID, dealID := "contact-1", "company-1", "deal-1"
	seeds := []struct {
		table string
		value map[string]interface{}
	}{
		{"crm_pipeline_stages", map[string]interface{}{"id": "stage-open", "stage_type": model.CRMStageTypeOpen}},
		{"crm_contacts", map[string]interface{}{"id": contactID, "workspace_id": "workspace-1"}},
		{"crm_companies", map[string]interface{}{"id": companyID, "workspace_id": "workspace-1", "external_id": "external-company"}},
		{"crm_deals", map[string]interface{}{"id": dealID, "workspace_id": "workspace-1", "stage_id": "stage-open", "updated_at": time.Now().UTC()}},
		{"crm_identity_links", map[string]interface{}{"id": "link-1", "workspace_id": "workspace-1", "anonymous_id": "anon-1", "contact_id": contactID, "company_id": companyID, "identity_method": model.IdentityMethodSignedWidget, "identity_trust": model.IdentityTrustVerified, "created_at": time.Now().UTC()}},
		{"crm_associations", map[string]interface{}{"id": "assoc-1", "workspace_id": "workspace-1", "from_object_type": model.CRMObjectDeal, "from_object_id": dealID, "to_object_type": model.CRMObjectCompany, "to_object_id": companyID}},
	}
	for _, seed := range seeds {
		if err := db.Table(seed.table).Create(seed.value).Error; err != nil {
			t.Fatalf("seed %s: %v", seed.table, err)
		}
	}

	resolvedContact, resolvedCompany, method, trust, err := repo.ResolveBehavioralIdentity(ctx, "workspace-1", "anon-1", "", "")
	if err != nil || resolvedContact == nil || *resolvedContact != contactID || resolvedCompany == nil || *resolvedCompany != companyID {
		t.Fatalf("resolved contact=%v company=%v method=%s trust=%s err=%v", resolvedContact, resolvedCompany, method, trust, err)
	}
	resolvedDeal, err := repo.ResolveOpenDealForIdentity(ctx, "workspace-1", resolvedContact, resolvedCompany)
	if err != nil || resolvedDeal == nil || *resolvedDeal != dealID {
		t.Fatalf("resolved deal=%v err=%v", resolvedDeal, err)
	}
}
