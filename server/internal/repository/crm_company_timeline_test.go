package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMCompanyTimelineCursorClauseBindsNilPostgresCursor(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=test dbname=test sslmode=disable",
	}), &gorm.Config{
		DisableAutomaticPing: true,
		DryRun:               true,
	})
	if err != nil {
		t.Fatalf("open dry-run postgres database: %v", err)
	}

	result := db.Raw(
		"SELECT 1 FROM candidates WHERE ("+crmCompanyTimelineCursorClause+")",
		map[string]interface{}{"cursor_at": nil, "cursor_id": ""},
	)
	if result.Error != nil {
		t.Fatalf("build timeline cursor query: %v", result.Error)
	}
	if got := result.Statement.SQL.String(); strings.Contains(got, "@cursor_at") {
		t.Fatalf("cursor placeholder was not bound: %q", got)
	}
	if got := result.Statement.SQL.String(); !strings.Contains(got, "CAST($1 AS timestamptz)") {
		t.Fatalf("cursor SQL does not use bind-safe PostgreSQL cast: %q", got)
	}
	if got := len(result.Statement.Vars); got != 4 {
		t.Fatalf("cursor variables = %d, want 4", got)
	}
}

func TestCRMTimelineScopeClauseBindsPostgresNamedParameters(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=test dbname=test sslmode=disable",
	}), &gorm.Config{
		DisableAutomaticPing: true,
		DryRun:               true,
	})
	if err != nil {
		t.Fatalf("open dry-run postgres database: %v", err)
	}

	result := db.Raw(
		"SELECT CAST(@scope_id AS uuid) WHERE @scope_type = 'contact'",
		map[string]interface{}{"scope_id": "711ab62e-f941-4a4c-b188-7e22e129a07c", "scope_type": "contact"},
	)
	if result.Error != nil {
		t.Fatalf("build timeline scope query: %v", result.Error)
	}
	sql := result.Statement.SQL.String()
	if strings.Contains(sql, "@scope_") {
		t.Fatalf("scope placeholder was not bound: %q", sql)
	}
	if !strings.Contains(sql, "CAST($1 AS uuid)") {
		t.Fatalf("scope SQL does not use bind-safe PostgreSQL cast: %q", sql)
	}
}

func TestCRMCompanyTimelinePortableOrderingFilteringAndCursor(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:company-timeline?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Exec(`CREATE TABLE crm_activities (
        id TEXT PRIMARY KEY,
        workspace_id TEXT NOT NULL,
        activity_type TEXT NOT NULL,
        contact_id TEXT,
        company_id TEXT,
        deal_id TEXT,
        owner_member_id TEXT,
        subject TEXT,
        body TEXT,
        occurred_at DATETIME NOT NULL,
        metadata TEXT,
        created_at DATETIME,
        updated_at DATETIME
    )`).Error; err != nil {
		t.Fatalf("create activities table: %v", err)
	}

	workspaceID := "workspace-1"
	companyID := "company-1"
	base := time.Date(2026, time.August, 21, 10, 0, 0, 0, time.UTC)
	subjects := []string{"Newest note", "Call", "Older note"}
	types := []string{model.CRMActivityNote, model.CRMActivityCall, model.CRMActivityNote}
	for index := range subjects {
		subject := subjects[index]
		activity := model.CRMActivity{
			ID:           string(rune('a' + index)),
			WorkspaceID:  workspaceID,
			CompanyID:    &companyID,
			ActivityType: types[index],
			Subject:      &subject,
			OccurredAt:   base.Add(-time.Duration(index) * time.Hour),
		}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
	}

	repo := NewCRMCompanyTimelineRepository(db)
	first, err := repo.List(context.Background(), workspaceID, companyID, model.CRMCompanyTimelineQuery{Filter: model.CRMCompanyTimelineFilterAll, Limit: 2})
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(first) != 2 || first[0].Title != "Newest note" || first[1].Title != "Call" {
		t.Fatalf("unexpected first page: %#v", first)
	}

	cursorAt := first[1].OccurredAt
	second, err := repo.List(context.Background(), workspaceID, companyID, model.CRMCompanyTimelineQuery{
		Filter: model.CRMCompanyTimelineFilterAll, CursorAt: &cursorAt, CursorID: first[1].ID, Limit: 2,
	})
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(second) != 1 || second[0].Title != "Older note" {
		t.Fatalf("unexpected second page: %#v", second)
	}

	notes, err := repo.List(context.Background(), workspaceID, companyID, model.CRMCompanyTimelineQuery{Filter: model.CRMCompanyTimelineFilterNote, Limit: 10})
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("note count = %d, want 2", len(notes))
	}
}

func TestCRMContactTimelinePortableScopesActivitiesDirectlyToContact(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:contact-timeline?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Exec(`CREATE TABLE crm_activities (
        id TEXT PRIMARY KEY,
        workspace_id TEXT NOT NULL,
        activity_type TEXT NOT NULL,
        contact_id TEXT,
        company_id TEXT,
        deal_id TEXT,
        owner_member_id TEXT,
        subject TEXT,
        body TEXT,
        occurred_at DATETIME NOT NULL,
        metadata TEXT,
        created_at DATETIME,
        updated_at DATETIME
    )`).Error; err != nil {
		t.Fatalf("create activities table: %v", err)
	}

	workspaceID := "workspace-1"
	contactID := "contact-1"
	otherContactID := "contact-2"
	companyID := "company-1"
	base := time.Date(2026, time.August, 21, 10, 0, 0, 0, time.UTC)
	directSubject := "Direct note"
	siblingSubject := "Sibling note"
	companySubject := "Company note"
	for _, activity := range []model.CRMActivity{
		{ID: "direct", WorkspaceID: workspaceID, ContactID: &contactID, ActivityType: model.CRMActivityNote, Subject: &directSubject, OccurredAt: base},
		{ID: "sibling", WorkspaceID: workspaceID, ContactID: &otherContactID, ActivityType: model.CRMActivityNote, Subject: &siblingSubject, OccurredAt: base.Add(-time.Hour)},
		{ID: "company", WorkspaceID: workspaceID, CompanyID: &companyID, ActivityType: model.CRMActivityNote, Subject: &companySubject, OccurredAt: base.Add(-2 * time.Hour)},
	} {
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
	}

	repo := NewCRMCompanyTimelineRepository(db)
	items, err := repo.ListContact(context.Background(), workspaceID, contactID, model.CRMTimelineQuery{Filter: model.CRMTimelineFilterAll, Limit: 10})
	if err != nil {
		t.Fatalf("list contact timeline: %v", err)
	}
	if len(items) != 1 || items[0].Title != "Direct note" {
		t.Fatalf("contact timeline = %#v, want only direct note", items)
	}
}
