package repository

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/model"
	"testing"
	"time"
)

func TestCommercialSignalFilteringPrecedesPagination(t *testing.T) {
	db, repo := setupSignalRuleRepositoryTest(t)
	now := time.Now().UTC()
	cases := []struct {
		id, source, detector, rule string
		version                    int
		meta                       model.JSONB
	}{
		{"qualified", "support", "llm_extracted", model.CRMSignalRuleConversationExtraction, 4, model.JSONB{"commercial_relevance": "relevant"}},
		{"manual", "manual", "manual", "", 0, model.JSONB{}},
		{"payment", "product", "rule_derived", model.CRMSignalRulePaymentFailed, 1, model.JSONB{}},
		{"old-conversation", "support", "llm_extracted", model.CRMSignalRuleConversationExtraction, 3, model.JSONB{}},
		{"escalation", "support", "rule_derived", model.CRMSignalRuleSupportAIEscalation, 1, model.JSONB{}},
		{"support-volume", "support", "rule_derived", model.CRMSignalRuleSupportVolumeSpike, 1, model.JSONB{}},
		{"missing-assessment", "support", "llm_extracted", model.CRMSignalRuleConversationExtraction, 4, model.JSONB{}},
	}
	for i, tc := range cases {
		if err := db.Table("crm_signals").Create(map[string]interface{}{
			"id": tc.id, "workspace_id": "ws-1", "source_type": tc.source, "detector_kind": tc.detector, "rule_key": tc.rule, "rule_version": tc.version, "metadata": tc.meta, "signal_type": "risk_signal", "summary": tc.id, "confidence": .95, "polarity": "negative", "detected_at": now.Add(time.Duration(i) * time.Minute),
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO crm_signals (id,workspace_id,source_type,detector_kind,confidence,detected_at) VALUES ('other','ws-2','manual','manual',1,?)`, now).Error; err != nil {
		t.Fatal(err)
	}
	filters := model.CRMSignalListFilters{CommercialOnly: true}
	rows, total, err := repo.ListSignals(context.Background(), "ws-1", filters, model.PMPagination{Page: 1, PerPage: 1})
	if err != nil || total != 3 || len(rows) != 1 || rows[0].ID != "payment" {
		t.Fatalf("commercial page=%+v total=%d err=%v", rows, total, err)
	}
	candidates, err := repo.ListWorkspaceSignalCandidates(context.Background(), "ws-1", filters, now, 1)
	if err != nil || len(candidates) != 1 || candidates[0].ID != "payment" {
		t.Fatalf("commercial feed candidates=%+v err=%v", candidates, err)
	}
	_, total, err = repo.ListSignals(context.Background(), "ws-1", model.CRMSignalListFilters{}, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil || total != int64(len(cases)) {
		t.Fatalf("raw context total=%d err=%v", total, err)
	}
}

func TestCommercialContextLoadsOnlyWorkspaceCompanyFacts(t *testing.T) {
	db, repo := setupSignalRuleRepositoryTest(t)
	for _, sql := range []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, name TEXT,company_product_context TEXT,description TEXT,website_url TEXT)`,
		`INSERT INTO workspaces VALUES ('ws-1','Seller','We sell software subscriptions.',NULL,NULL)`,
		`ALTER TABLE crm_contacts ADD COLUMN lifecycle_stage TEXT`,
		`INSERT INTO crm_contacts VALUES ('contact-1','ws-1','subscriber')`,
		`INSERT INTO crm_companies VALUES ('company-1','ws-1',NULL), ('foreign','ws-2',NULL)`,
		`CREATE TABLE crm_company_commercial_states (company_id TEXT PRIMARY KEY,workspace_id TEXT,state BLOB)`,
		`INSERT INTO crm_company_commercial_states VALUES ('company-1','ws-1','{"subscription_status":"past_due"}'), ('foreign','ws-2','{"subscription_status":"active"}')`,
		`INSERT INTO crm_associations VALUES ('link','ws-1','contact','contact-1','company','company-1'), ('bad-link','ws-2','contact','contact-1','company','foreign')`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	contact := "contact-1"
	c, err := repo.SignalCommercialContext(context.Background(), model.SignalSourcePayload{WorkspaceID: "ws-1", ContactID: &contact})
	if err != nil || c.CompanyID == nil || *c.CompanyID != "company-1" || c.SubscriptionStatus != "past_due" {
		t.Fatalf("context=%+v err=%v", c, err)
	}
	foreign := "foreign"
	if _, err := repo.SignalCommercialContext(context.Background(), model.SignalSourcePayload{WorkspaceID: "ws-1", CompanyID: &foreign}); err == nil {
		t.Fatal("accepted foreign company")
	}
}

func TestCommercialThreadSuppressionKeepsDifferentEvents(t *testing.T) {
	db, repo := setupSignalRuleRepositoryTest(t)
	now := time.Now().UTC()
	if err := db.Exec(`INSERT INTO crm_signals (id,workspace_id,source_thread_id,source_id,signal_type,detected_at,rule_key,rule_version,metadata) VALUES ('old','ws-1','thread','message-1','timeline_signal',?,'conversation_signal_extraction',3,'{}'), ('new','ws-1','thread','message-2','timeline_signal',?,'conversation_signal_extraction',4,'{"commercial_event":"purchase_deadline"}')`, now, now).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		event string
		want  bool
	}{{"purchase_deadline", true}, {"cancellation_deadline", false}} {
		got, err := repo.HasRecentSignalForThread(context.Background(), "ws-1", "thread", "timeline_signal", "message-3", now.Add(-time.Hour), tc.event)
		if err != nil || got != tc.want {
			t.Fatalf("event=%s suppressed=%v err=%v", tc.event, got, err)
		}
	}
}

func TestCommercialDealClosureStillSupersedesEventSignals(t *testing.T) {
	db, repo := setupSignalRuleRepositoryTest(t)
	for _, statement := range []string{
		`CREATE TABLE crm_pipelines (id TEXT PRIMARY KEY, default_commercial_motion TEXT)`,
		`ALTER TABLE crm_deals ADD COLUMN pipeline_id TEXT`,
		`ALTER TABLE crm_deals ADD COLUMN commercial_motion TEXT`,
		`ALTER TABLE crm_signals ADD COLUMN commercial_motion TEXT`,
		`ALTER TABLE crm_signals ADD COLUMN superseded_at DATETIME`,
		`ALTER TABLE crm_signals ADD COLUMN superseded_reason TEXT`,
		`INSERT INTO crm_pipelines VALUES ('pipeline','new_business')`,
		`INSERT INTO crm_pipeline_stages VALUES ('stage','open')`,
		`INSERT INTO crm_deals (id,workspace_id,stage_id,pipeline_id) VALUES ('deal','ws-1','stage','pipeline')`,
		`INSERT INTO crm_signals (id,workspace_id,deal_id,commercial_motion,detector_kind,rule_key,rule_version,metadata) VALUES ('risk','ws-1','deal','retention','llm_extracted','conversation_signal_extraction',4,'{"commercial_relevance":"relevant"}')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.ReconcileDealMotionSignals(context.Background(), "ws-1", "deal"); err != nil {
		t.Fatal(err)
	}
	var saved model.CRMSignal
	if err := db.First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.SupersededAt != nil {
		t.Fatal("open new-business deal erased explicit retention evidence")
	}
	if err := db.Exec(`UPDATE crm_pipeline_stages SET stage_type='won'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.ReconcileDealMotionSignals(context.Background(), "ws-1", "deal"); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.SupersededAt == nil {
		t.Fatal("closed deal retained active evidence")
	}
}
