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
		`CREATE TABLE crm_signals (id TEXT PRIMARY KEY, workspace_id TEXT, contact_id TEXT, deal_id TEXT, company_id TEXT, signal_type TEXT, source_type TEXT, source_id TEXT, source_thread_id TEXT, summary TEXT, evidence_excerpt TEXT, metadata BLOB, confidence REAL, detected_at DATETIME, detector_kind TEXT, signal_domain TEXT, polarity TEXT, rule_key TEXT, rule_version INTEGER, window_started_at DATETIME, window_ended_at DATETIME, evidence_identity_method TEXT, evidence_identity_trust TEXT, evidence_fingerprint TEXT, dismissed_at DATETIME, dismissed_by_member_id TEXT, dismissal_reason TEXT, reviewed_at DATETIME, acted_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE crm_identity_links (id TEXT PRIMARY KEY, workspace_id TEXT, anonymous_id TEXT, external_user_id TEXT, contact_id TEXT, company_id TEXT, identity_method TEXT, identity_trust TEXT, verified_at DATETIME, verifier_version TEXT, created_at DATETIME)`,
		`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT)`,
		`CREATE TABLE crm_companies (id TEXT PRIMARY KEY, workspace_id TEXT, external_id TEXT)`,
		`CREATE TABLE crm_pipeline_stages (id TEXT PRIMARY KEY, stage_type TEXT)`,
		`CREATE TABLE crm_deals (id TEXT PRIMARY KEY, workspace_id TEXT, stage_id TEXT, updated_at DATETIME)`,
		`CREATE TABLE crm_associations (id TEXT PRIMARY KEY, workspace_id TEXT, from_object_type TEXT, from_object_id TEXT, to_object_type TEXT, to_object_id TEXT)`,
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT, crm_company_id TEXT, crm_contact_id TEXT, subject TEXT, ai_escalated_at DATETIME)`,
		`CREATE TABLE pm_tasks (id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, task_type TEXT, completed BOOLEAN, completed_at DATETIME)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create signal rule schema: %v", err)
		}
	}
	return db, NewCRMSignalRepository(db)
}

func TestRequestedFeatureShippedRuleOnlyUsesFeatureTasks(t *testing.T) {
	db, repo := setupSignalRuleRepositoryTest(t)
	start := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	end := start.Add(48 * time.Hour)
	for _, statement := range []string{
		`INSERT INTO pm_tasks VALUES ('chore-1', 'workspace-1', 'Clean up records', 'chore', true, '2026-08-22 12:00:00')`,
		`INSERT INTO pm_tasks VALUES ('feature-1', 'workspace-1', 'Requested dashboard', 'feature', true, '2026-08-22 13:00:00')`,
		`INSERT INTO crm_associations VALUES ('assoc-chore', 'workspace-1', 'task', 'chore-1', 'company', 'company-1')`,
		`INSERT INTO crm_associations VALUES ('assoc-feature', 'workspace-1', 'task', 'feature-1', 'company', 'company-1')`,
		`INSERT INTO crm_associations VALUES ('assoc-request', 'workspace-1', 'support_conversation', 'conversation-1', 'task', 'feature-1')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed feature rule evidence: %v", err)
		}
	}
	candidates, err := repo.EvaluatePostgresSignalRule(context.Background(), model.CRMSignalRuleConfig{RuleKey: model.CRMSignalRuleRequestedFeatureShipped}, start, end)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates=%#v err=%v", candidates, err)
	}
	if candidates[0].SourceID == nil || *candidates[0].SourceID != "feature-1" {
		t.Fatalf("candidate source = %#v, want feature-1", candidates[0].SourceID)
	}
}

func TestAdditionalCaptureRuleDimensions(t *testing.T) {
	tests := []struct {
		rule, wantSummary, wantPolarity string
	}{
		{model.CRMSignalRuleConfiguredForm, "Configured high-intent form submitted", model.CRMSignalPolarityPositive},
		{model.CRMSignalRuleIdentifiedArticleView, "Identified contact viewed a relevant article", model.CRMSignalPolarityNeutral},
		{model.CRMSignalRuleVersionedInteraction, "Versioned high-intent interaction observed", model.CRMSignalPolarityNeutral},
		{model.CRMSignalRuleSessionDepthSpike, "Deep browsing session", model.CRMSignalPolarityNeutral},
	}
	for _, test := range tests {
		_, domain, polarity, source, summary := behavioralRuleDimensions(test.rule)
		if domain != model.CRMSignalDomainWebBehavior || source != model.CRMSignalSourceWeb || summary != test.wantSummary || polarity != test.wantPolarity {
			t.Fatalf("rule %s dimensions = domain %s polarity %s source %s summary %q", test.rule, domain, polarity, source, summary)
		}
	}
}

func TestDeepSessionPresentationKeepsLegacyEvidenceFingerprint(t *testing.T) {
	observed := time.Date(2026, 8, 27, 10, 30, 0, 0, time.UTC)
	legacy := model.CRMSignalRuleCandidate{
		WorkspaceID: "workspace-1", RuleKey: model.CRMSignalRuleSessionDepthSpike,
		Summary: "Session depth spiked", EvidenceExcerpt: "6 pageviews in one session",
		ObservedAt: observed, AnonymousID: "anonymous-1",
	}
	corrected := legacy
	corrected.Summary = "Deep browsing session"
	if fingerprintRuleCandidate(legacy) != fingerprintRuleCandidate(corrected) {
		t.Fatal("presentation correction must not change the durable evidence fingerprint")
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
	if candidates[0].SignalType != model.CRMSignalRiskSignal || candidates[0].Polarity != model.CRMSignalPolarityNegative {
		t.Fatalf("v1 dimensions=%#v", candidates[0])
	}

	config.Version = 2
	candidates, err = repo.EvaluatePostgresSignalRule(context.Background(), config, start, end)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("v2 candidates=%#v err=%v", candidates, err)
	}
	if candidates[0].SignalType != model.CRMSignalTimelineSignal || candidates[0].Polarity != model.CRMSignalPolarityNeutral {
		t.Fatalf("v2 dimensions=%#v", candidates[0])
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
	signal := &model.CRMSignal{
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
	signal.ID = ""
	laterStart, laterEnd := start.Add(time.Hour), end.Add(time.Hour)
	signal.WindowStartedAt, signal.WindowEndedAt = &laterStart, &laterEnd
	created, err = repo.CreateRuleSignalIfAbsent(ctx, signal)
	if err != nil || created {
		t.Fatalf("overlap-window duplicate created=%v err=%v", created, err)
	}
	signal.ID = ""
	signal.EvidenceFingerprint = "fingerprint-2"
	created, err = repo.CreateRuleSignalIfAbsent(ctx, signal)
	if err != nil || !created {
		t.Fatalf("changed evidence created=%v err=%v", created, err)
	}
}

func TestWorkspaceScopedWatermarksAdvanceIndependently(t *testing.T) {
	_, repo := setupSignalRuleRepositoryTest(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	initial := now.Add(-366 * 24 * time.Hour)
	firstKey := "commercial_state_sync:workspace-1"
	secondKey := "commercial_state_sync:workspace-2"
	for _, key := range []string{firstKey, secondKey} {
		if _, acquired, err := repo.TryAcquireSignalEvaluatorLease(ctx, key, "pod-a", now, time.Minute, initial); err != nil || !acquired {
			t.Fatalf("acquire %s: acquired=%v err=%v", key, acquired, err)
		}
	}
	if err := repo.ReleaseSignalEvaluatorLease(ctx, firstKey, "pod-a", now); err != nil {
		t.Fatalf("release healthy workspace: %v", err)
	}
	if err := repo.AbandonSignalEvaluatorLease(ctx, secondKey, "pod-a"); err != nil {
		t.Fatalf("abandon failed workspace: %v", err)
	}
	later := now.Add(10 * time.Minute)
	firstWatermark, acquired, err := repo.TryAcquireSignalEvaluatorLease(ctx, firstKey, "pod-b", later, time.Minute, initial)
	if err != nil || !acquired || !firstWatermark.Equal(now) {
		t.Fatalf("healthy workspace watermark=%v acquired=%v err=%v", firstWatermark, acquired, err)
	}
	secondWatermark, acquired, err := repo.TryAcquireSignalEvaluatorLease(ctx, secondKey, "pod-b", later, time.Minute, initial)
	if err != nil || !acquired || !secondWatermark.Equal(initial) {
		t.Fatalf("failed workspace watermark=%v acquired=%v err=%v", secondWatermark, acquired, err)
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

func TestResolveBehavioralIdentityByExternalUserID(t *testing.T) {
	db, repo := setupSignalRuleRepositoryTest(t)
	ctx := context.Background()
	contactID := "contact-1"
	if err := db.Table("crm_contacts").Create(map[string]interface{}{
		"id": contactID, "workspace_id": "workspace-1",
	}).Error; err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	if err := db.Table("crm_identity_links").Create(map[string]interface{}{
		"id": "link-external", "workspace_id": "workspace-1", "anonymous_id": "anon-old",
		"external_user_id": "customer-user-42", "contact_id": contactID,
		"identity_method": model.IdentityMethodSignedWidget,
		"identity_trust":  model.IdentityTrustVerified, "created_at": time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("seed external identity link: %v", err)
	}

	resolvedContact, _, method, trust, err := repo.ResolveBehavioralIdentity(
		ctx, "workspace-1", "", "customer-user-42", "",
	)
	if err != nil || resolvedContact == nil || *resolvedContact != contactID {
		t.Fatalf("resolved contact=%v method=%s trust=%s err=%v", resolvedContact, method, trust, err)
	}
	if method != model.IdentityMethodSignedWidget || trust != model.IdentityTrustVerified {
		t.Fatalf("identity provenance method=%s trust=%s", method, trust)
	}
}
