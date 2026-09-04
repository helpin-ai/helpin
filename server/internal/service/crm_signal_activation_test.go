package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type flakySignalNotificationEmitter struct {
	failures int
	calls    int
}

func (f *flakySignalNotificationEmitter) Emit(
	_ context.Context,
	_ model.NotificationEventInput,
) error {
	f.calls++
	if f.calls <= f.failures {
		return errors.New("temporary notification failure")
	}
	return nil
}

func newSignalActivationTestService(t *testing.T) (*gorm.DB, *CRMSignalService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:signal-activation-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE crm_signal_rule_configs (id TEXT PRIMARY KEY, workspace_id TEXT, rule_key TEXT, version INTEGER, cadence TEXT, enabled BOOLEAN, shadow_mode BOOLEAN, activation_eligible BOOLEAN, thresholds BLOB, business_weight REAL, half_life_days REAL, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_signals (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT NOT NULL, contact_id TEXT, deal_id TEXT, company_id TEXT, signal_type TEXT NOT NULL, source_type TEXT NOT NULL, source_id TEXT, source_thread_id TEXT, summary TEXT NOT NULL, evidence_excerpt TEXT, metadata BLOB, confidence REAL, detected_at DATETIME, detector_kind TEXT, signal_domain TEXT, polarity TEXT, rule_key TEXT, rule_version INTEGER, window_started_at DATETIME, window_ended_at DATETIME, evidence_identity_method TEXT, evidence_identity_trust TEXT, evidence_fingerprint TEXT, observation_id TEXT, commercial_motion TEXT NOT NULL DEFAULT 'conversion', interpretation_version INTEGER NOT NULL DEFAULT 1, business_weight_snapshot REAL NOT NULL DEFAULT 0, half_life_days_snapshot REAL NOT NULL DEFAULT 0, interpretation_snapshot BLOB, meaning_fingerprint TEXT NOT NULL DEFAULT '', recommended_action_key TEXT, recommended_action_label TEXT, replay_calibration_excluded BOOLEAN NOT NULL DEFAULT 0, superseded_at DATETIME, superseded_reason TEXT, direction_changed_by_supersession BOOLEAN NOT NULL DEFAULT 0, dismissed_at DATETIME, dismissed_by_member_id TEXT, dismissal_reason TEXT, reviewed_at DATETIME, acted_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE crm_signal_feedback (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT, signal_id TEXT, member_id TEXT, action TEXT, dismissal_reason TEXT, rule_key TEXT, rule_version INTEGER, signal_domain TEXT, identity_method TEXT, detected_at DATETIME, occurred_at DATETIME, detection_to_event_millis INTEGER, created_at DATETIME)`,
		`CREATE TABLE crm_signal_routing_policies (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT, version INTEGER, enabled BOOLEAN, minimum_priority REAL, required_trust TEXT, route_to_owner BOOLEAN, destination_team_id TEXT, channels BLOB, created_by_member_id TEXT, created_at DATETIME)`,
		`CREATE TABLE crm_signal_deliveries (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT, signal_id TEXT, policy_id TEXT, policy_version INTEGER, channel TEXT, recipient_member_id TEXT, destination_team_id TEXT, status TEXT, delivered_at DATETIME, attempts INTEGER NOT NULL DEFAULT 0, last_attempted_at DATETIME, last_error TEXT, created_at DATETIME)`,
		`CREATE TABLE crm_signal_interpretation_configs (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT, rule_key TEXT NOT NULL, rule_version INTEGER NOT NULL, motion TEXT NOT NULL, observation_signal_type TEXT NOT NULL DEFAULT '*', version INTEGER NOT NULL, signal_type TEXT NOT NULL, polarity TEXT NOT NULL, business_weight REAL NOT NULL, half_life_days REAL NOT NULL, recommended_action_key TEXT, recommended_action_label TEXT, enabled BOOLEAN NOT NULL DEFAULT 1, created_at DATETIME)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("migrate activation schema: %v", err)
		}
	}
	repo := repository.NewCRMSignalRepository(db)
	return db, NewCRMSignalService(repo, nil)
}

func TestSignalPrecisionCountsLatestOutcomeOnce(t *testing.T) {
	db, svc := newSignalActivationTestService(t)
	signal := model.CRMSignal{
		ID: "signal-1", WorkspaceID: "ws-1", SignalType: model.CRMSignalBuyingIntent,
		SourceType: model.CRMSignalSourceEmail, Summary: "Asked for pricing", Confidence: .9,
		DetectedAt: time.Now().UTC().Add(-time.Hour), DetectorKind: model.CRMSignalDetectorRuleDerived,
		SignalDomain: model.CRMSignalDomainConversation, EvidenceIdentityMethod: "connected_mailbox",
	}
	if err := db.Create(&signal).Error; err != nil {
		t.Fatalf("create signal: %v", err)
	}
	if err := svc.RecordSignalFeedback(context.Background(), "ws-1", signal.ID, "member-1", model.CRMSignalFeedbackReviewed, ""); err != nil {
		t.Fatalf("review signal: %v", err)
	}
	if err := svc.RecordSignalFeedback(context.Background(), "ws-1", signal.ID, "member-1", model.CRMSignalFeedbackActed, ""); err != nil {
		t.Fatalf("act on signal: %v", err)
	}
	report, err := svc.SignalPrecisionReport(context.Background(), "ws-1")
	if err != nil || len(report) != 1 {
		t.Fatalf("precision report = %+v, err=%v", report, err)
	}
	if report[0].ReviewedCount != 1 || report[0].ActedCount != 1 || report[0].Precision != 1 {
		t.Fatalf("precision dimensions = %+v", report[0])
	}
}

func TestActivateRuleVersionPromotesGlobalRuleForWorkspace(t *testing.T) {
	db, svc := newSignalActivationTestService(t)
	global := model.CRMSignalRuleConfig{
		ID: "global-v2", RuleKey: model.CRMSignalRuleRepeatedPricingActivity, Version: 2,
		Cadence: model.CRMSignalRuleCadenceDaily, Enabled: true, ShadowMode: true,
		ActivationEligible: true, Thresholds: model.JSONB{"minimum_count": 2}, BusinessWeight: 15, HalfLifeDays: 30,
	}
	if err := db.Create(&global).Error; err != nil {
		t.Fatalf("seed global rule: %v", err)
	}
	seedRuleInterpretation(t, db, global.RuleKey, global.Version)
	if err := svc.ActivateRuleVersion(context.Background(), "ws-1", global.RuleKey, global.Version); err != nil {
		t.Fatalf("activate rule: %v", err)
	}
	var workspaceConfig model.CRMSignalRuleConfig
	if err := db.Where("workspace_id = ? AND rule_key = ? AND version = ?", "ws-1", global.RuleKey, global.Version).First(&workspaceConfig).Error; err != nil {
		t.Fatalf("load workspace config: %v", err)
	}
	if !workspaceConfig.Enabled || workspaceConfig.ShadowMode || !workspaceConfig.ActivationEligible || workspaceConfig.ID == global.ID {
		t.Fatalf("workspace config = %+v", workspaceConfig)
	}
}

func seedRuleInterpretation(t *testing.T, db *gorm.DB, ruleKey string, ruleVersion int) {
	t.Helper()
	interpretation := model.CRMSignalInterpretationConfig{
		RuleKey: ruleKey, RuleVersion: ruleVersion, Motion: model.CRMCommercialMotionConversion,
		ObservationSignalType: "*", Version: 1, SignalType: "inherit", Polarity: "inherit",
		BusinessWeight: 15, HalfLifeDays: 30, Enabled: true,
	}
	if err := db.Create(&interpretation).Error; err != nil {
		t.Fatalf("seed rule interpretation: %v", err)
	}
}

func TestActivateRuleVersionRejectsVersionWithoutInterpretation(t *testing.T) {
	db, svc := newSignalActivationTestService(t)
	global := model.CRMSignalRuleConfig{
		ID: "global-unmapped-v3", RuleKey: model.CRMSignalRuleRepeatedPricingActivity, Version: 3,
		Cadence: model.CRMSignalRuleCadenceDaily, Enabled: true, ShadowMode: true,
		ActivationEligible: true, Thresholds: model.JSONB{}, BusinessWeight: 15, HalfLifeDays: 30,
	}
	if err := db.Create(&global).Error; err != nil {
		t.Fatalf("seed global rule: %v", err)
	}
	// Only the previous version is mapped, so activating v3 would silently stop
	// producing signals for this rule.
	seedRuleInterpretation(t, db, global.RuleKey, 2)

	if err := svc.ActivateRuleVersion(context.Background(), "ws-1", global.RuleKey, global.Version); err == nil {
		t.Fatal("expected activation of an unmapped rule version to fail")
	}
	var promoted int64
	if err := db.Model(&model.CRMSignalRuleConfig{}).
		Where("workspace_id = ? AND rule_key = ?", "ws-1", global.RuleKey).Count(&promoted).Error; err != nil {
		t.Fatalf("count workspace configs: %v", err)
	}
	if promoted != 0 {
		t.Fatalf("unmapped activation created %d workspace rule configs", promoted)
	}
}

func TestActivateRuleVersionRejectsContextOnlyRule(t *testing.T) {
	db, svc := newSignalActivationTestService(t)
	global := model.CRMSignalRuleConfig{
		ID: "global-context", RuleKey: model.CRMSignalRuleExternalEvidence, Version: 1,
		Cadence: model.CRMSignalRuleCadenceDaily, Enabled: true, ShadowMode: true,
		ActivationEligible: false, Thresholds: model.JSONB{}, BusinessWeight: 5, HalfLifeDays: 30,
	}
	if err := db.Create(&global).Error; err != nil {
		t.Fatalf("seed context rule: %v", err)
	}
	if err := svc.ActivateRuleVersion(context.Background(), "ws-1", global.RuleKey, global.Version); err == nil {
		t.Fatal("expected context-only rule activation to fail")
	}
}

func TestSignalFeedbackRequiresReasonAndReportsPrecision(t *testing.T) {
	db, svc := newSignalActivationTestService(t)
	now := time.Now().UTC().Add(-time.Hour)
	signal := model.CRMSignal{
		ID: "signal-1", WorkspaceID: "ws-1", SignalType: model.CRMSignalBuyingIntent,
		SourceType: model.CRMSignalSourceEmail, Summary: "Asked for pricing", Confidence: .9,
		DetectedAt: now, DetectorKind: model.CRMSignalDetectorRuleDerived,
		SignalDomain: model.CRMSignalDomainConversation, EvidenceIdentityMethod: "connected_mailbox",
	}
	if err := db.Create(&signal).Error; err != nil {
		t.Fatalf("create signal: %v", err)
	}
	if err := svc.DismissSignal(context.Background(), "ws-1", signal.ID, "member-1", ""); err == nil {
		t.Fatal("expected dismissal reason validation")
	}
	if err := svc.DismissSignal(context.Background(), "ws-1", signal.ID, "member-1", model.CRMSignalDismissIncorrectEvidence); err != nil {
		t.Fatalf("dismiss signal: %v", err)
	}
	var stored model.CRMSignal
	if err := db.First(&stored, "id = ?", signal.ID).Error; err != nil {
		t.Fatalf("load signal: %v", err)
	}
	if stored.DismissalReason == nil || *stored.DismissalReason != model.CRMSignalDismissIncorrectEvidence || stored.ReviewedAt == nil {
		t.Fatalf("feedback state = %+v", stored)
	}
	report, err := svc.SignalPrecisionReport(context.Background(), "ws-1")
	if err != nil || len(report) != 1 {
		t.Fatalf("precision report = %+v, err=%v", report, err)
	}
	if report[0].IncorrectCount != 1 || report[0].Precision != 0 || report[0].AverageReviewMillis <= 0 {
		t.Fatalf("precision dimensions = %+v", report[0])
	}
}

func TestSignalActivationRequiresLiveRuleAndNoOpenTask(t *testing.T) {
	db, svc := newSignalActivationTestService(t)
	if err := db.Exec(`CREATE TABLE pm_tasks (id TEXT PRIMARY KEY, workspace_id TEXT, completed BOOLEAN, archived BOOLEAN, external_id TEXT, created_at DATETIME)`).Error; err != nil {
		t.Fatalf("create task schema: %v", err)
	}
	ruleKey, version := model.CRMSignalRuleRepeatedPricingActivity, 2
	signal := model.CRMSignal{
		ID: "signal-1", WorkspaceID: "ws-1", RuleKey: &ruleKey, RuleVersion: &version,
		EvidenceIdentityTrust: "verified", EvidenceFingerprint: "fingerprint-1", BusinessPriority: 30,
	}
	policy := &model.CRMSignalRoutingPolicy{MinimumPriority: 12, RequiredTrust: "verified"}
	profile := defaultSignalScoringProfile()
	profile.rules[ruleKey] = model.CRMSignalRuleConfig{RuleKey: ruleKey, Version: version, ActivationEligible: true, ShadowMode: true}
	svc.setSignalActivation(context.Background(), &signal, policy, profile)
	if signal.ActivationEligible || !signalTestContains(signal.ActivationBlockers, "rule_not_activation_eligible") {
		t.Fatalf("shadow rule activation = %+v", signal.ActivationBlockers)
	}

	profile.rules[ruleKey] = model.CRMSignalRuleConfig{RuleKey: ruleKey, Version: version, ActivationEligible: true}
	svc.setSignalActivation(context.Background(), &signal, policy, profile)
	if !signal.ActivationEligible {
		t.Fatalf("eligible signal blocked: %+v", signal.ActivationBlockers)
	}

	signal.Metadata = model.JSONB{"needs_customer_context": true}
	svc.setSignalActivation(context.Background(), &signal, policy, profile)
	if signal.ActivationEligible || !signalTestContains(signal.ActivationBlockers, "needs_customer_context") {
		t.Fatalf("missing relationship allowed activation: %+v", signal.ActivationBlockers)
	}
	signal.Metadata = nil
	if err := db.Exec(`INSERT INTO pm_tasks VALUES ('task-1', 'ws-1', false, false, 'crm-signal:fingerprint-1', ?)`, time.Now().UTC()).Error; err != nil {
		t.Fatalf("seed open task: %v", err)
	}
	svc.setSignalActivation(context.Background(), &signal, policy, profile)
	if signal.ActivationEligible || signal.ExistingOpenTaskID == nil || *signal.ExistingOpenTaskID != "task-1" {
		t.Fatalf("open task gate = eligible %v task %v blockers %+v", signal.ActivationEligible, signal.ExistingOpenTaskID, signal.ActivationBlockers)
	}
}

func TestSignalDeliveryAndPolicyVersionsAreIdempotentAndReversible(t *testing.T) {
	db, svc := newSignalActivationTestService(t)
	ctx := context.Background()
	routeToOwner := true
	first, err := svc.CreateRoutingPolicy(ctx, "ws-1", "member-1", model.CreateCRMSignalRoutingPolicyRequest{MinimumPriority: 12, RequiredTrust: "verified", RouteToOwner: &routeToOwner, Channels: []string{"feed", "feed"}})
	if err != nil {
		t.Fatalf("create first policy: %v", err)
	}
	second, err := svc.CreateRoutingPolicy(ctx, "ws-1", "member-1", model.CreateCRMSignalRoutingPolicyRequest{MinimumPriority: 20, RequiredTrust: "verified", Channels: []string{"notification"}})
	if err != nil {
		t.Fatalf("create second policy: %v", err)
	}
	if first.Version != 1 || second.Version != 2 {
		t.Fatalf("policy versions = %d, %d", first.Version, second.Version)
	}
	if err := svc.ActivateRoutingPolicyVersion(ctx, "ws-1", 1); err != nil {
		t.Fatalf("rollback policy: %v", err)
	}
	active, err := svc.GetRoutingPolicy(ctx, "ws-1")
	if err != nil || active == nil || active.Version != 1 {
		t.Fatalf("active policy = %+v, err=%v", active, err)
	}
	repo := repository.NewCRMSignalRepository(db)
	delivery := &model.CRMSignalDelivery{ID: "delivery-1", WorkspaceID: "ws-1", SignalID: "signal-1", PolicyID: first.ID, PolicyVersion: 1, Channel: "feed", Status: model.CRMSignalDeliverySent}
	created, err := repo.CreateSignalDelivery(ctx, delivery)
	if err != nil || !created {
		t.Fatalf("first delivery created=%v err=%v", created, err)
	}
	delivery.ID = "delivery-2"
	created, err = repo.CreateSignalDelivery(ctx, delivery)
	if err != nil || created {
		t.Fatalf("duplicate delivery created=%v err=%v", created, err)
	}
}

func TestRouteWorkspaceSignalsRetriesFailedNotification(t *testing.T) {
	db, svc := newSignalActivationTestService(t)
	emitter := &flakySignalNotificationEmitter{failures: 1}
	svc.SetActivationDependencies(emitter)
	ctx := context.Background()
	ruleKey, version := model.CRMSignalRuleConversationExtraction, signalEvidenceRuleVersion

	config := model.CRMSignalRuleConfig{
		ID: "conversation-live", WorkspaceID: signalActivationStringPtr("ws-1"), RuleKey: ruleKey,
		Version: version, Cadence: model.CRMSignalRuleCadenceEventDriven, Enabled: true,
		ShadowMode: false, ActivationEligible: true, BusinessWeight: 30, HalfLifeDays: 30,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("seed conversation rule: %v", err)
	}
	if err := db.Model(&model.CRMSignalRuleConfig{}).Where("id = ?", config.ID).
		Update("shadow_mode", false).Error; err != nil {
		t.Fatalf("promote conversation rule: %v", err)
	}
	policy := model.CRMSignalRoutingPolicy{
		ID: "policy-1", WorkspaceID: "ws-1", Version: 1, Enabled: true,
		MinimumPriority: 1, RequiredTrust: model.IdentityTrustVerified,
		Channels: model.JSONBlob(`["notification"]`), CreatedByMemberID: "member-1",
	}
	if err := db.Create(&policy).Error; err != nil {
		t.Fatalf("seed routing policy: %v", err)
	}
	signal := model.CRMSignal{
		ID: "signal-retry", WorkspaceID: "ws-1", SignalType: model.CRMSignalBuyingIntent,
		SourceType: model.CRMSignalSourceEmail, Summary: "Buyer requested pricing",
		Confidence: 1, DetectedAt: time.Now().UTC(), DetectorKind: model.CRMSignalDetectorLLMExtracted,
		SignalDomain: model.CRMSignalDomainConversation, Polarity: model.CRMSignalPolarityPositive,
		RuleKey: &ruleKey, RuleVersion: &version,
		EvidenceIdentityMethod: model.IdentityMethodConnectedMailbox,
		EvidenceIdentityTrust:  model.IdentityTrustVerified, EvidenceFingerprint: "retry-fingerprint",
	}
	if err := db.Create(&signal).Error; err != nil {
		t.Fatalf("seed signal: %v", err)
	}

	if routed, err := svc.RouteWorkspaceSignals(ctx, "ws-1"); err == nil || routed != 0 {
		t.Fatalf("first route routed=%d err=%v, want failed emission", routed, err)
	}
	var failed model.CRMSignalDelivery
	if err := db.First(&failed, "signal_id = ?", signal.ID).Error; err != nil {
		t.Fatalf("load failed delivery: %v", err)
	}
	if failed.Status != model.CRMSignalDeliveryFailed || failed.DeliveredAt != nil || failed.Attempts != 1 {
		t.Fatalf("failed delivery = %+v", failed)
	}

	if routed, err := svc.RouteWorkspaceSignals(ctx, "ws-1"); err != nil || routed != 1 {
		t.Fatalf("retry route routed=%d err=%v", routed, err)
	}
	var sent model.CRMSignalDelivery
	if err := db.First(&sent, "id = ?", failed.ID).Error; err != nil {
		t.Fatalf("load sent delivery: %v", err)
	}
	if sent.Status != model.CRMSignalDeliverySent || sent.DeliveredAt == nil || sent.Attempts != 2 {
		t.Fatalf("sent delivery = %+v", sent)
	}
	var count int64
	if err := db.Model(&model.CRMSignalDelivery{}).Where("signal_id = ?", signal.ID).Count(&count).Error; err != nil {
		t.Fatalf("count deliveries: %v", err)
	}
	if count != 1 || emitter.calls != 2 {
		t.Fatalf("delivery count=%d emitter calls=%d", count, emitter.calls)
	}
}

func signalActivationStringPtr(value string) *string {
	return &value
}

func signalTestContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
