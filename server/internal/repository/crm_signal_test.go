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
	if err := db.Exec(`CREATE TABLE crm_buyer_signals (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, contact_id TEXT, deal_id TEXT, company_id TEXT,
		signal_type TEXT NOT NULL, source_type TEXT NOT NULL DEFAULT 'manual', source_id TEXT,
		source_thread_id TEXT, summary TEXT NOT NULL, evidence_excerpt TEXT,
		metadata BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)), confidence REAL NOT NULL DEFAULT 0,
		detected_at DATETIME NOT NULL, evidence_fingerprint TEXT NOT NULL DEFAULT '',
		dismissed_at DATETIME, dismissed_by_member_id TEXT, created_at DATETIME
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
	signal := &model.CRMBuyerSignal{
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
	filters := model.CRMBuyerSignalListFilters{ContactID: &contactID}
	rows, total, err := repo.ListSignals(ctx, "ws-1", filters, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("dismissed list = %#v, total=%d, err=%v; want hidden", rows, total, err)
	}
	created, err := repo.CreateSignalIfAbsent(ctx, &model.CRMBuyerSignal{
		WorkspaceID: "ws-1", ContactID: &contactID, SignalType: signal.SignalType,
		SourceType: signal.SourceType, SourceID: &sourceID, Summary: signal.Summary,
		Confidence: signal.Confidence, DetectedAt: time.Now().UTC(),
	})
	if err != nil || created {
		t.Fatalf("unchanged evidence created=%v err=%v, want dismissal preserved", created, err)
	}
	created, err = repo.CreateSignalIfAbsent(ctx, &model.CRMBuyerSignal{
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
}

func TestCRMSignalRepositoryListSignalsByCompanyRollsUpCanonicalSignals(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crm-signal-company?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE crm_buyer_signals (id TEXT PRIMARY KEY, workspace_id TEXT, contact_id TEXT, deal_id TEXT, company_id TEXT, signal_type TEXT, source_type TEXT, source_id TEXT, source_thread_id TEXT, summary TEXT, evidence_excerpt TEXT, metadata BLOB, confidence REAL, detected_at DATETIME, evidence_fingerprint TEXT, dismissed_at DATETIME, dismissed_by_member_id TEXT, created_at DATETIME)`,
		`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT, first_name TEXT, last_name TEXT)`,
		`CREATE TABLE crm_deals (id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, display_id TEXT)`,
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
		`INSERT INTO crm_deals VALUES ('deal-1','ws-1','Expansion','DEAL-7')`,
		`INSERT INTO crm_associations VALUES ('ws-1','contact','contact-1','company','company-1')`,
		`INSERT INTO crm_associations VALUES ('ws-1','deal','deal-1','contact','contact-1')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed relation: %v", err)
		}
	}
	contactID, dealID, companyID := "contact-1", "deal-1", "company-1"
	signals := []model.CRMBuyerSignal{
		{ID: "direct", WorkspaceID: "ws-1", CompanyID: &companyID, SignalType: model.CRMSignalRiskSignal, SourceType: model.CRMSignalSourceSupport, Summary: "Escalation", Confidence: .9, DetectedAt: now},
		{ID: "contact", WorkspaceID: "ws-1", ContactID: &contactID, SignalType: model.CRMSignalBuyingIntent, SourceType: model.CRMSignalSourceEmail, Summary: "Pricing", Confidence: .9, DetectedAt: now.Add(-time.Minute)},
		{ID: "deal", WorkspaceID: "ws-1", DealID: &dealID, SignalType: model.CRMSignalTimelineSignal, SourceType: model.CRMSignalSourceMeeting, Summary: "Deadline", Confidence: .9, DetectedAt: now.Add(-2 * time.Minute)},
		{ID: "weak", WorkspaceID: "ws-1", ContactID: &contactID, SignalType: model.CRMSignalCompetitorMention, SourceType: model.CRMSignalSourceEmail, Summary: "Maybe", Confidence: .59, DetectedAt: now},
	}
	for i := range signals {
		if err := db.Create(&signals[i]).Error; err != nil {
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
