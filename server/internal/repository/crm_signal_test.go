package repository

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMSignalRepositoryDismissalPersistsUntilEvidenceChanges(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-signal-dismissal?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE crm_signals (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, contact_id TEXT, deal_id TEXT, company_id TEXT,
		signal_type TEXT NOT NULL, source_type TEXT NOT NULL DEFAULT 'manual', source_id TEXT,
		source_thread_id TEXT, summary TEXT NOT NULL, evidence_excerpt TEXT,
		metadata BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)), confidence REAL NOT NULL DEFAULT 0,
		detected_at DATETIME NOT NULL, detector_kind TEXT NOT NULL DEFAULT 'llm_extracted',
		signal_domain TEXT NOT NULL DEFAULT 'conversation', polarity TEXT NOT NULL DEFAULT 'neutral',
		rule_key TEXT, rule_version INTEGER, window_started_at DATETIME, window_ended_at DATETIME,
		evidence_identity_method TEXT NOT NULL DEFAULT 'connected_mailbox',
		evidence_identity_trust TEXT NOT NULL DEFAULT 'verified', evidence_fingerprint TEXT NOT NULL DEFAULT '',
		dismissed_at DATETIME, dismissed_by_member_id TEXT, dismissal_reason TEXT, reviewed_at DATETIME, acted_at DATETIME, created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create signal schema: %v", err)
	}
	if err := db.Exec(`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT, first_name TEXT, last_name TEXT)`).Error; err != nil {
		t.Fatalf("create contact schema: %v", err)
	}
	if err := db.Exec(`CREATE TABLE crm_deals (id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, display_id TEXT)`).Error; err != nil {
		t.Fatalf("create deal schema: %v", err)
	}
	repo := NewCRMSignalRepository(db)
	ctx := context.Background()
	contactID, sourceID := "contact-1", "source-1"
	signal := &model.CRMSignal{
		ID: "signal-1", WorkspaceID: "ws-1", ContactID: &contactID,
		SignalType: model.CRMSignalBuyingIntent, SourceType: model.CRMSignalSourceEmail,
		SourceID: &sourceID, Summary: "Asked for pricing", Confidence: 0.9, DetectedAt: time.Now().UTC(),
	}
	if err := repo.CreateSignal(ctx, signal); err != nil {
		t.Fatalf("create signal: %v", err)
	}
	if err := repo.DismissSignal(ctx, "ws-1", signal.ID, "member-1", time.Now().UTC()); err != nil {
		t.Fatalf("dismiss signal: %v", err)
	}
	filters := model.CRMSignalListFilters{ContactID: &contactID}
	rows, total, err := repo.ListSignals(ctx, "ws-1", filters, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("dismissed list = %#v, total=%d, err=%v; want hidden", rows, total, err)
	}
	created, err := repo.CreateSignalIfAbsent(ctx, &model.CRMSignal{
		WorkspaceID: "ws-1", ContactID: &contactID, SignalType: signal.SignalType,
		SourceType: signal.SourceType, SourceID: &sourceID, Summary: signal.Summary,
		Confidence: signal.Confidence, DetectedAt: time.Now().UTC(),
	})
	if err != nil || created {
		t.Fatalf("unchanged evidence created=%v err=%v, want dismissal preserved", created, err)
	}
	created, err = repo.CreateSignalIfAbsent(ctx, &model.CRMSignal{
		WorkspaceID: "ws-1", ContactID: &contactID, SignalType: signal.SignalType,
		SourceType: signal.SourceType, SourceID: &sourceID,
		Summary: "Asked for pricing and procurement terms", Confidence: 0.95, DetectedAt: time.Now().UTC(),
	})
	if err != nil || !created {
		t.Fatalf("changed evidence created=%v err=%v, want refreshed signal", created, err)
	}
	rows, total, err = repo.ListSignals(ctx, "ws-1", filters, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].DismissedAt != nil {
		t.Fatalf("refreshed list = %#v, total=%d, err=%v; want visible", rows, total, err)
	}
	otherSignal := &model.CRMSignal{
		ID: "signal-other-workspace", WorkspaceID: "ws-2", SignalType: model.CRMSignalRiskSignal,
		SourceType: model.CRMSignalSourceManual, Summary: "Other tenant", Confidence: .8, DetectedAt: time.Now().UTC(),
	}
	if err := repo.CreateSignal(ctx, otherSignal); err != nil {
		t.Fatalf("create other workspace signal: %v", err)
	}
	if err := repo.DeleteSignal(ctx, "ws-1", otherSignal.ID); err == nil {
		t.Fatal("expected cross-workspace delete to fail")
	}
	var remaining int64
	if err := db.Model(&model.CRMSignal{}).Where("id = ?", otherSignal.ID).Count(&remaining).Error; err != nil || remaining != 1 {
		t.Fatalf("other workspace signal remaining=%d err=%v", remaining, err)
	}
}

func TestCRMSignalRepositoryListSignalsByCompanyRollsUpCanonicalSignals(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-signal-company?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE crm_signals (id TEXT PRIMARY KEY, workspace_id TEXT, contact_id TEXT, deal_id TEXT, company_id TEXT, signal_type TEXT, source_type TEXT, source_id TEXT, source_thread_id TEXT, summary TEXT, evidence_excerpt TEXT, metadata BLOB, confidence REAL, detected_at DATETIME, detector_kind TEXT, signal_domain TEXT, polarity TEXT, rule_key TEXT, rule_version INTEGER, window_started_at DATETIME, window_ended_at DATETIME, evidence_identity_method TEXT, evidence_identity_trust TEXT, evidence_fingerprint TEXT, dismissed_at DATETIME, dismissed_by_member_id TEXT, dismissal_reason TEXT, reviewed_at DATETIME, acted_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT, first_name TEXT, last_name TEXT)`,
		`CREATE TABLE crm_deals (id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, display_id TEXT, amount REAL, probability INTEGER, owner_member_id TEXT)`,
		`CREATE TABLE crm_companies (id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, domain TEXT, owner_member_id TEXT)`,
		`CREATE TABLE crm_associations (workspace_id TEXT, from_object_type TEXT, from_object_id TEXT, to_object_type TEXT, to_object_id TEXT)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	now := time.Now().UTC()
	for _, statement := range []string{
		`INSERT INTO crm_contacts VALUES ('contact-1','ws-1','Ava','Buyer')`,
		`INSERT INTO crm_deals (id, workspace_id, name, display_id) VALUES ('deal-1','ws-1','Expansion','DEAL-7')`,
		`INSERT INTO crm_companies (id, workspace_id, name, domain) VALUES ('company-1','ws-1','Acme','acme.test')`,
		`INSERT INTO crm_associations VALUES ('ws-1','contact','contact-1','company','company-1')`,
		`INSERT INTO crm_associations VALUES ('ws-1','deal','deal-1','contact','contact-1')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed relation: %v", err)
		}
	}
	contactID, dealID, companyID := "contact-1", "deal-1", "company-1"
	signals := []model.CRMSignal{
		{ID: "direct", WorkspaceID: "ws-1", CompanyID: &companyID, SignalType: model.CRMSignalRiskSignal, SourceType: model.CRMSignalSourceSupport, SignalDomain: model.CRMSignalDomainSupport, Summary: "Escalation", Confidence: .9, DetectedAt: now},
		{ID: "contact", WorkspaceID: "ws-1", ContactID: &contactID, SignalType: model.CRMSignalBuyingIntent, SourceType: model.CRMSignalSourceEmail, SignalDomain: model.CRMSignalDomainConversation, Summary: "Pricing", Confidence: .9, DetectedAt: now.Add(-time.Minute)},
		{ID: "deal", WorkspaceID: "ws-1", DealID: &dealID, SignalType: model.CRMSignalTimelineSignal, SourceType: model.CRMSignalSourceMeeting, SignalDomain: model.CRMSignalDomainConversation, Summary: "Deadline", Confidence: .9, DetectedAt: now.Add(-2 * time.Minute)},
		{ID: "weak", WorkspaceID: "ws-1", ContactID: &contactID, SignalType: model.CRMSignalCompetitorMention, SourceType: model.CRMSignalSourceEmail, Summary: "Maybe", Confidence: .59, DetectedAt: now},
	}
	for i := range signals {
		if err := legacySignalCreateDB(db).Create(&signals[i]).Error; err != nil {
			t.Fatalf("create signal: %v", err)
		}
	}
	rows, total, err := NewCRMSignalRepository(db).ListSignalsByCompany(context.Background(), "ws-1", companyID, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("ListSignalsByCompany: %v", err)
	}
	if total != 3 || len(rows) != 3 {
		t.Fatalf("rows=%d total=%d, want 3", len(rows), total)
	}
	if rows[1].ContactName != "Ava Buyer" || rows[2].DealName != "Expansion" || rows[2].DealDisplayID != "DEAL-7" {
		t.Fatalf("context not hydrated: %#v", rows)
	}
	supportDomain := model.CRMSignalDomainSupport
	filtered, err := NewCRMSignalRepository(db).ListWorkspaceSignalCandidates(context.Background(), "ws-1", model.CRMSignalListFilters{
		Query: &model.QueryFilterGroup{Logic: model.QueryFilterLogicAnd, Rules: []model.QueryFilterRule{{
			Field: "domain", Operator: model.QueryFilterOpIs, Value: &supportDomain,
		}}},
	}, now, 0)
	if err != nil || len(filtered) != 1 || filtered[0].ID != "direct" {
		t.Fatalf("query-builder filtered signals=%#v err=%v", filtered, err)
	}
	_, err = NewCRMSignalRepository(db).ListWorkspaceSignalCandidates(context.Background(), "ws-1", model.CRMSignalListFilters{
		Query: &model.QueryFilterGroup{Rules: []model.QueryFilterRule{{
			Field: "domain", Operator: model.QueryFilterOpContains, Value: &supportDomain,
		}}},
	}, now, 0)
	if err == nil {
		t.Fatal("expected invalid enum query-builder operator to fail")
	}
}

func TestCRMSignalRepositorySavesBoundedHealthSnapshotsAndListsLatestPerDeal(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-health-snapshots?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE crm_deal_health_scores (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT NOT NULL,
		deal_id TEXT NOT NULL, score INTEGER NOT NULL, factors BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
		calculated_at DATETIME NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatalf("create health schema: %v", err)
	}
	repo := NewCRMSignalRepository(db)
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	first := &model.CRMDealHealthScore{WorkspaceID: "ws-1", DealID: "deal-1", Score: 55, Factors: model.JSONB{"signal_count": 0}, CalculatedAt: now, CreatedAt: now}
	if err := repo.SaveCalculatedHealthScore(context.Background(), first); err != nil {
		t.Fatalf("save first score: %v", err)
	}
	updated := &model.CRMDealHealthScore{WorkspaceID: "ws-1", DealID: "deal-1", Score: 72, Factors: model.JSONB{"signal_count": 2}, CalculatedAt: now.Add(time.Hour)}
	if err := repo.SaveCalculatedHealthScore(context.Background(), updated); err != nil {
		t.Fatalf("update daily score: %v", err)
	}
	var count int64
	if err := db.Table("crm_deal_health_scores").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("same-day snapshot count = %d, err=%v; want 1", count, err)
	}
	nextDay := &model.CRMDealHealthScore{WorkspaceID: "ws-1", DealID: "deal-1", Score: 80, Factors: model.JSONB{"signal_count": 3}, CalculatedAt: now.Add(25 * time.Hour)}
	if err := repo.SaveCalculatedHealthScore(context.Background(), nextDay); err != nil {
		t.Fatalf("save next-day score: %v", err)
	}
	rows, total, err := repo.ListHealthScores(context.Background(), "ws-1", model.PMPagination{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("ListHealthScores: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Score != 80 {
		t.Fatalf("latest rows = %#v, total=%d; want one score of 80", rows, total)
	}
}

func TestStoredSignalGroupDirectionDetectsSupersessionFlip(t *testing.T) {
	negative := model.CRMSignal{
		ID: "negative", Polarity: model.CRMSignalPolarityNegative,
		BusinessWeightSnapshot: 20, Confidence: 1,
	}
	positive := model.CRMSignal{
		ID: "positive", Polarity: model.CRMSignalPolarityPositive,
		BusinessWeightSnapshot: 12, Confidence: 1,
	}
	signals := []model.CRMSignal{negative, positive}
	if got := storedSignalGroupDirection(signals, nil); got != -1 {
		t.Fatalf("direction before supersession = %d, want -1", got)
	}
	if got := storedSignalGroupDirection(signals, map[string]bool{"negative": true}); got != 1 {
		t.Fatalf("direction after supersession = %d, want 1", got)
	}
}

func TestSignalMeaningFingerprintIsEntityScoped(t *testing.T) {
	contactA, contactB := "contact-a", "contact-b"
	base := model.CRMSignal{
		WorkspaceID: "workspace", EvidenceFingerprint: "same-evidence",
		CommercialMotion: model.CRMCommercialMotionConversion,
		SignalType:       model.CRMSignalBuyingIntent, Polarity: model.CRMSignalPolarityPositive,
		InterpretationVersion: 1,
	}
	first, second := base, base
	first.ContactID, second.ContactID = &contactA, &contactB
	if signalMeaningFingerprint(first) == signalMeaningFingerprint(second) {
		t.Fatal("identical evidence on different contacts must not collide")
	}
}

func TestPrepareSignalInterpretationsPersistsUnmappedObservationOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-unmapped-observation?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE crm_signals (id TEXT PRIMARY KEY, commercial_motion TEXT)`,
		`CREATE TABLE crm_signal_interpretation_configs (
			id TEXT PRIMARY KEY, workspace_id TEXT, rule_key TEXT, rule_version INTEGER,
			motion TEXT, observation_signal_type TEXT, version INTEGER, signal_type TEXT,
			polarity TEXT, business_weight REAL, half_life_days REAL,
			recommended_action_key TEXT, recommended_action_label TEXT, enabled BOOLEAN,
			created_at DATETIME
		)`,
		`CREATE TABLE crm_signal_observations (
			id TEXT PRIMARY KEY, workspace_id TEXT, contact_id TEXT, deal_id TEXT, company_id TEXT,
			rule_key TEXT, rule_version INTEGER, detector_kind TEXT, signal_domain TEXT,
			source_type TEXT, source_id TEXT, source_thread_id TEXT, summary TEXT,
			evidence_excerpt TEXT, metadata BLOB, confidence REAL, observed_at DATETIME,
			window_started_at DATETIME, window_ended_at DATETIME, identity_method TEXT,
			identity_trust TEXT, evidence_fingerprint TEXT, motions_at_detection BLOB,
			context_snapshot BLOB, replay_calibration_excluded BOOLEAN, created_at DATETIME,
			UNIQUE(workspace_id, rule_key, rule_version, evidence_fingerprint, contact_id, deal_id, company_id)
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	ruleKey, version := "unmapped_rule", 1
	signal := &model.CRMSignal{
		WorkspaceID: "workspace-1", RuleKey: &ruleKey, RuleVersion: &version,
		SignalType: model.CRMSignalBuyingIntent, SourceType: model.CRMSignalSourceCRM,
		Summary: "Unmapped evidence", Confidence: 1, DetectedAt: time.Now().UTC(),
		EvidenceFingerprint: "unmapped-evidence",
	}
	rows, err := NewCRMSignalRepository(db).prepareSignalInterpretations(context.Background(), signal)
	if err != nil {
		t.Fatalf("prepareSignalInterpretations: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("unmapped observation produced signals: %#v", rows)
	}
	var observations int64
	if err := db.Table("crm_signal_observations").Count(&observations).Error; err != nil || observations != 1 {
		t.Fatalf("observation count=%d err=%v", observations, err)
	}
}

func TestPersistSignalMotionStateKeepsOutOfOrderHistory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-motion-state-history?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE crm_signal_motion_states (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL, resolver_version INTEGER NOT NULL, motions BLOB NOT NULL,
		input_snapshot BLOB NOT NULL, effective_at DATETIME NOT NULL, created_at DATETIME,
		UNIQUE(workspace_id, entity_type, entity_id, resolver_version, effective_at)
	)`).Error; err != nil {
		t.Fatalf("create motion state: %v", err)
	}
	companyID := "company-1"
	newer := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	repo := NewCRMSignalRepository(db)
	for _, effectiveAt := range []time.Time{newer, newer.Add(-24 * time.Hour)} {
		signal := &model.CRMSignal{WorkspaceID: "workspace-1", CompanyID: &companyID, DetectedAt: effectiveAt}
		if err := repo.persistSignalMotionState(context.Background(), signal,
			[]string{model.CRMCommercialMotionRetention}, model.JSONB{"at": effectiveAt.Format(time.RFC3339)}); err != nil {
			t.Fatalf("persist motion state: %v", err)
		}
	}
	var states []model.CRMSignalMotionState
	if err := db.Order("effective_at DESC").Find(&states).Error; err != nil {
		t.Fatalf("list motion states: %v", err)
	}
	if len(states) != 2 || !states[0].EffectiveAt.Equal(newer) {
		t.Fatalf("motion history = %#v; want two snapshots with newest current", states)
	}
}

func TestPersistSignalMotionStateSkipsUnchangedSnapshots(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-motion-state-dedupe?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE crm_signal_motion_states (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL, resolver_version INTEGER NOT NULL, motions BLOB NOT NULL,
		input_snapshot BLOB NOT NULL, effective_at DATETIME NOT NULL, created_at DATETIME,
		UNIQUE(workspace_id, entity_type, entity_id, resolver_version, effective_at)
	)`).Error; err != nil {
		t.Fatalf("create motion state: %v", err)
	}
	contactID := "contact-1"
	repo := NewCRMSignalRepository(db)
	for _, effectiveAt := range []time.Time{
		time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC),
	} {
		signal := &model.CRMSignal{WorkspaceID: "workspace-1", ContactID: &contactID, DetectedAt: effectiveAt}
		if err := repo.persistSignalMotionState(context.Background(), signal,
			[]string{model.CRMCommercialMotionRetention}, model.JSONB{"lifecycle_stage": "customer"}); err != nil {
			t.Fatalf("persist motion state: %v", err)
		}
	}
	var count int64
	if err := db.Table("crm_signal_motion_states").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("motion state count=%d err=%v; want one unchanged snapshot", count, err)
	}
}

func TestContactMotionRefreshDoesNotSupersedeDealScopedSignal(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-contact-motion-deal-scope?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT, lifecycle_stage TEXT, lead_status TEXT)`,
		`CREATE TABLE crm_signal_motion_states (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL, resolver_version INTEGER NOT NULL, motions BLOB NOT NULL,
			input_snapshot BLOB NOT NULL, effective_at DATETIME NOT NULL, created_at DATETIME,
			UNIQUE(workspace_id, entity_type, entity_id, resolver_version, effective_at)
		)`,
		`CREATE TABLE crm_signals (
			id TEXT PRIMARY KEY, workspace_id TEXT, contact_id TEXT, deal_id TEXT, company_id TEXT,
			commercial_motion TEXT, polarity TEXT, business_weight_snapshot REAL, confidence REAL,
			dismissed_at DATETIME, superseded_at DATETIME, superseded_reason TEXT,
			direction_changed_by_supersession BOOLEAN DEFAULT 0
		)`,
		`INSERT INTO crm_contacts VALUES ('contact-1', 'workspace-1', 'customer', '')`,
		`INSERT INTO crm_signals VALUES ('contact-only', 'workspace-1', 'contact-1', NULL, NULL, 'conversion', 'positive', 10, 1, NULL, NULL, NULL, 0)`,
		`INSERT INTO crm_signals VALUES ('deal-scoped', 'workspace-1', 'contact-1', 'deal-1', NULL, 'conversion', 'positive', 10, 1, NULL, NULL, NULL, 0)`,
		`ALTER TABLE crm_signals ADD COLUMN detector_kind TEXT`,
		`ALTER TABLE crm_signals ADD COLUMN rule_key TEXT`,
		`ALTER TABLE crm_signals ADD COLUMN rule_version INTEGER`,
		`ALTER TABLE crm_signals ADD COLUMN metadata BLOB`,
		`INSERT INTO crm_signals (id,workspace_id,contact_id,commercial_motion,polarity,detector_kind,rule_key,rule_version,metadata) VALUES ('upgrade','workspace-1','contact-1','expansion','positive','llm_extracted','conversation_signal_extraction',4,'{"commercial_relevance":"relevant"}')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed schema: %v", err)
		}
	}
	if err := NewCRMSignalRepository(db).RefreshEntityMotionSignals(
		context.Background(), "workspace-1", "contact", "contact-1", time.Now().UTC(),
	); err != nil {
		t.Fatalf("refresh contact motion signals: %v", err)
	}
	var rows []struct {
		ID           string
		SupersededAt *time.Time
	}
	if err := db.Table("crm_signals").Order("id").Scan(&rows).Error; err != nil {
		t.Fatalf("load signals: %v", err)
	}
	if len(rows) != 3 || rows[0].ID != "contact-only" || rows[0].SupersededAt == nil {
		t.Fatalf("contact-only signal was not superseded: %#v", rows)
	}
	if rows[1].ID != "deal-scoped" || rows[1].SupersededAt != nil {
		t.Fatalf("deal-scoped signal was superseded by contact refresh: %#v", rows)
	}
	if rows[2].ID != "upgrade" || rows[2].SupersededAt != nil {
		t.Fatalf("customer upgrade superseded by baseline lifecycle: %#v", rows)
	}

}

func TestCommercialStateMotionsAreApplicabilityNotLaneMembership(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	motions := map[string]struct{}{}
	addCommercialStateMotions(motions, model.JSONB{
		"subscription_status":   "active",
		"onboarding_started_at": now.Add(-10 * 24 * time.Hour).Format(time.RFC3339),
		"renewal_at":            now.Add(60 * 24 * time.Hour).Format(time.RFC3339),
	}, now)
	for _, motion := range []string{
		model.CRMCommercialMotionOnboarding, model.CRMCommercialMotionAdoption,
		model.CRMCommercialMotionRetention, model.CRMCommercialMotionRenewal,
	} {
		if _, ok := motions[motion]; !ok {
			t.Fatalf("resolved applicability missing %s: %#v", motion, motions)
		}
	}
}

func TestCompanyRelationshipMotionsIncludeDealsLinkedThroughContacts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-company-indirect-deal-motion?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT, lifecycle_stage TEXT)`,
		`CREATE TABLE crm_associations (workspace_id TEXT, from_object_type TEXT, from_object_id TEXT, to_object_type TEXT, to_object_id TEXT)`,
		`CREATE TABLE crm_pipelines (id TEXT PRIMARY KEY, default_commercial_motion TEXT)`,
		`CREATE TABLE crm_pipeline_stages (id TEXT PRIMARY KEY, stage_type TEXT)`,
		`CREATE TABLE crm_deals (id TEXT PRIMARY KEY, workspace_id TEXT, pipeline_id TEXT, stage_id TEXT, commercial_motion TEXT)`,
		`INSERT INTO crm_contacts VALUES ('contact-1', 'ws-1', 'customer')`,
		`INSERT INTO crm_pipelines VALUES ('pipeline-1', 'expansion')`,
		`INSERT INTO crm_pipeline_stages VALUES ('stage-1', 'open')`,
		`INSERT INTO crm_deals VALUES ('deal-1', 'ws-1', 'pipeline-1', 'stage-1', NULL)`,
		`INSERT INTO crm_associations VALUES ('ws-1', 'contact', 'contact-1', 'company', 'company-1')`,
		`INSERT INTO crm_associations VALUES ('ws-1', 'deal', 'deal-1', 'contact', 'contact-1')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed schema: %v", err)
		}
	}
	motions := map[string]struct{}{}
	snapshot := model.JSONB{}
	if err := NewCRMSignalRepository(db).addCompanyRelationshipMotions(
		context.Background(), "ws-1", "company-1", motions, snapshot,
	); err != nil {
		t.Fatalf("addCompanyRelationshipMotions: %v", err)
	}
	if _, ok := motions[model.CRMCommercialMotionExpansion]; !ok {
		t.Fatalf("indirect expansion deal missing from motions: %#v", motions)
	}
	if got, ok := snapshot["open_deal_motions"].([]string); !ok || len(got) != 1 || got[0] != "expansion" {
		t.Fatalf("open-deal snapshot = %#v", snapshot["open_deal_motions"])
	}
}
