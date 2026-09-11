package repository

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func existingBusinessSignalFixture(t *testing.T) (*gorm.DB, *CRMSignalRepository) {
	t.Helper()
	db, repo := setupSignalRuleRepositoryTest(t)
	for _, sql := range []string{
		`CREATE TABLE crm_pipelines (id TEXT PRIMARY KEY,default_commercial_motion TEXT)`,
		`ALTER TABLE crm_deals ADD COLUMN pipeline_id TEXT`, `ALTER TABLE crm_deals ADD COLUMN commercial_motion TEXT`,
		`ALTER TABLE crm_signals ADD COLUMN commercial_motion TEXT`, `ALTER TABLE crm_signals ADD COLUMN superseded_at DATETIME`, `ALTER TABLE crm_signals ADD COLUMN superseded_reason TEXT`,
		`CREATE TABLE crm_signal_motion_states (id TEXT PRIMARY KEY,workspace_id TEXT,entity_type TEXT,entity_id TEXT,resolver_version INTEGER,motions BLOB,input_snapshot BLOB,effective_at DATETIME,created_at DATETIME)`,
		`ALTER TABLE crm_contacts ADD COLUMN lifecycle_stage TEXT`,
		`ALTER TABLE crm_contacts ADD COLUMN lead_status TEXT`,
		`INSERT INTO crm_pipelines VALUES ('pipeline','existing_business')`, `INSERT INTO crm_pipeline_stages VALUES ('stage','open')`,
		`INSERT INTO crm_deals (id,workspace_id,stage_id,pipeline_id) VALUES ('deal','ws-1','stage','pipeline')`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, repo
}

func TestExistingBusinessSignalApplicabilityDoesNotInventIntent(t *testing.T) {
	_, repo := existingBusinessSignalFixture(t)
	id := "deal"
	motions, snapshot, err := repo.resolveSignalMotions(context.Background(), &model.CRMSignal{WorkspaceID: "ws-1", DealID: &id, DetectedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(motions, []string{"adoption", "retention"}) {
		t.Fatalf("motions=%v snapshot=%v", motions, snapshot)
	}
}

func TestExistingBusinessReconciliationPreservesCustomerEvidence(t *testing.T) {
	db, repo := existingBusinessSignalFixture(t)
	for _, motion := range []string{"conversion", "adoption", "retention", "renewal", "expansion"} {
		if err := db.Exec(`INSERT INTO crm_signals (id,workspace_id,deal_id,commercial_motion,detector_kind,rule_key,rule_version,metadata) VALUES (?,'ws-1','deal',?,'rule_derived','customer_event',1,'{}')`, motion, motion).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.ReconcilePipelineMotionSignals(context.Background(), "ws-1", "pipeline"); err != nil {
		t.Fatal(err)
	}
	if err := repo.RefreshEntityMotionSignals(context.Background(), "ws-1", "deal", "deal", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var rows []model.CRMSignal
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if (row.SupersededAt != nil) != (row.CommercialMotion == "conversion") {
			t.Errorf("motion %s superseded=%v", row.CommercialMotion, row.SupersededAt)
		}
	}
	if err := db.Exec("UPDATE crm_pipeline_stages SET stage_type='won'").Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.ReconcileDealMotionSignals(context.Background(), "ws-1", "deal"); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.CRMSignal{}).Where("superseded_at IS NULL").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("closed deal retained %d signals", count)
	}
}

func TestExistingBusinessSourceAndInboxUseEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, evidence, override, want, category string
		stale                                    bool
	}{
		{name: "ambiguous", want: "needs_context", category: ""},
		{name: "stale conversion evidence", evidence: "conversion", stale: true, want: "needs_context", category: ""},
		{name: "renewal evidence", evidence: "renewal", want: "renewal", category: "retention"},
		{name: "expansion evidence", evidence: "expansion", want: "expansion", category: "expansion"},
		{name: "legacy renewal override", override: "renewal", want: "renewal", category: "retention"},
		{name: "legacy expansion override", override: "expansion", want: "expansion", category: "expansion"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := f.Open(t)
			f.InboxTables(t, db)
			repo := NewCRMSituationRepository(db)
			pipeline := uuid.NewString()
			f.Exec(t, db, "INSERT INTO crm_pipelines VALUES (?,?,'existing_business')", pipeline, f.Workspace)
			f.Exec(t, db, "UPDATE crm_deals SET pipeline_id=?,commercial_motion=? WHERE id=?", pipeline, tc.override, f.Deal)
			suggestion := model.CRMSuggestion{ID: f.Suggestion, WorkspaceID: f.Workspace, SuggestionType: "deal_advance", ObjectType: f.Ptr("deal"), ObjectID: f.Ptr(f.Deal), SignalIDs: model.StringArray{}, Context: model.JSONB{}}
			if tc.evidence != "" {
				f.Exec(t, db, "UPDATE crm_signals SET commercial_motion=? WHERE id=?", tc.evidence, f.Signal)
				suggestion.SignalIDs = model.StringArray{f.Signal}
				if tc.stale {
					f.Exec(t, db, "UPDATE crm_signals SET superseded_at=? WHERE id=?", time.Now().UTC(), f.Signal)
				}
			}
			f.Exec(t, db, "UPDATE crm_suggestions SET suggestion_type='deal_advance',object_type='deal',object_id=?,signal_ids=?,context='{}' WHERE id=?", f.Deal, suggestion.SignalIDs, f.Suggestion)
			motion, err := repo.SourceMotion(context.Background(), suggestion)
			if err != nil || motion != tc.want {
				t.Fatalf("source=%s err=%v", motion, err)
			}
			list, err := repo.ListInbox(context.Background(), f.Workspace, f.Sales, inboxFilters())
			if err != nil || len(list.Data) != 1 || list.Data[0].Category != tc.category {
				t.Fatalf("inbox=%+v err=%v", list, err)
			}
		})
	}
}

func TestExistingBusinessCompanyRefreshKeepsSpecificCustomerEvidence(t *testing.T) {
	db, repo := existingBusinessSignalFixture(t)
	for _, sql := range []string{
		`INSERT INTO crm_associations VALUES ('link','ws-1','deal','deal','company','company')`,
		`INSERT INTO crm_signals (id,workspace_id,company_id,commercial_motion,detector_kind,rule_key,rule_version,metadata) VALUES ('renewal','ws-1','company','renewal','rule_derived','renewal_approaching',1,'{}')`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	company := "company"
	motions, _, err := repo.resolveSignalMotions(context.Background(), &model.CRMSignal{WorkspaceID: "ws-1", CompanyID: &company, DetectedAt: time.Now()})
	if err != nil || !slices.Equal(motions, []string{"adoption", "retention"}) {
		t.Fatalf("motions=%v err=%v", motions, err)
	}
	if err := repo.RefreshEntityMotionSignals(context.Background(), "ws-1", "company", company, time.Now()); err != nil {
		t.Fatal(err)
	}
	var row model.CRMSignal
	if err := db.Where("id='renewal'").Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.SupersededAt != nil {
		t.Fatal("generic customer classification invalidated specific renewal evidence")
	}
}

func TestExistingBusinessDealOverridesStaleContactLifecycle(t *testing.T) {
	for _, lifecycle := range []string{"lead", "opportunity"} {
		t.Run(lifecycle, func(t *testing.T) {
			db, repo := existingBusinessSignalFixture(t)
			if err := db.Exec("INSERT INTO crm_contacts VALUES ('contact','ws-1',?,'')", lifecycle).Error; err != nil {
				t.Fatal(err)
			}
			id, contact := "deal", "contact"
			motions, _, err := repo.resolveSignalMotions(context.Background(), &model.CRMSignal{WorkspaceID: "ws-1", DealID: &id, ContactID: &contact, DetectedAt: time.Now()})
			if err != nil || !slices.Equal(motions, []string{"adoption", "retention"}) {
				t.Fatalf("motions=%v err=%v", motions, err)
			}
		})
	}
}

func TestExistingBusinessCompanyKeepsConcurrentNewBusiness(t *testing.T) {
	db, repo := existingBusinessSignalFixture(t)
	for _, sql := range []string{
		`INSERT INTO crm_pipelines VALUES ('new-pipeline','new_business')`,
		`INSERT INTO crm_deals (id,workspace_id,stage_id,pipeline_id) VALUES ('new-deal','ws-1','stage','new-pipeline')`,
		`INSERT INTO crm_associations VALUES ('existing-link','ws-1','deal','deal','company','company'), ('new-link','ws-1','deal','new-deal','company','company')`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	company := "company"
	motions, _, err := repo.resolveSignalMotions(context.Background(), &model.CRMSignal{WorkspaceID: "ws-1", CompanyID: &company, DetectedAt: time.Now()})
	if err != nil || !slices.Equal(motions, []string{"adoption", "conversion", "retention"}) {
		t.Fatalf("motions=%v err=%v", motions, err)
	}
}

func TestExistingBusinessResolverHonorsExplicitDealOverrides(t *testing.T) {
	for _, tc := range []struct {
		name, defaultMotion, override string
		want                          []string
	}{
		{"new override", "existing_business", "new_business", []string{"conversion"}},
		{"existing override", "new_business", "existing_business", []string{"adoption", "retention"}},
		{"expansion override", "existing_business", "expansion", []string{"expansion"}},
		{"renewal override", "existing_business", "renewal", []string{"renewal"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, repo := existingBusinessSignalFixture(t)
			if err := db.Exec("UPDATE crm_pipelines SET default_commercial_motion=?", tc.defaultMotion).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("UPDATE crm_deals SET commercial_motion=?", tc.override).Error; err != nil {
				t.Fatal(err)
			}
			id := "deal"
			motions, _, err := repo.resolveSignalMotions(context.Background(), &model.CRMSignal{WorkspaceID: "ws-1", DealID: &id, DetectedAt: time.Now()})
			if err != nil || !slices.Equal(motions, tc.want) {
				t.Fatalf("motions=%v err=%v", motions, err)
			}
		})
	}
}
