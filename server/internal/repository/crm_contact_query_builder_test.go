package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
)

func TestCRMContactRepositoryListAppliesQueryBuilderRules(t *testing.T) {
	db := newCRMContactQueryBuilderTestDB(t)
	repo := NewCRMContactRepository(db)
	ctx := context.Background()
	workspaceID := "ws-1"

	createdAt := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	insertCRMContactTestRow(t, db, model.CRMContact{
		ID:             "contact-1",
		WorkspaceID:    workspaceID,
		DisplayID:      "CON-1",
		FirstName:      "Avery",
		LastName:       strPtr("Stone"),
		Email:          strPtr("avery@example.com"),
		JobTitle:       strPtr("Founder"),
		LifecycleStage: model.CRMLifecycleLead,
		LeadStatus:     model.CRMLeadStatusOpen,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	})
	insertCRMContactTestRow(t, db, model.CRMContact{
		ID:             "contact-2",
		WorkspaceID:    workspaceID,
		DisplayID:      "CON-2",
		FirstName:      "Jordan",
		LastName:       strPtr("Reed"),
		Email:          strPtr("jordan@example.com"),
		JobTitle:       strPtr("Engineer"),
		LifecycleStage: model.CRMLifecycleCustomer,
		LeadStatus:     model.CRMLeadStatusNew,
		CreatedAt:      createdAt.Add(48 * time.Hour),
		UpdatedAt:      createdAt.Add(48 * time.Hour),
	})

	contacts, total, err := repo.List(ctx, workspaceID, model.CRMContactListFilters{
		Query: &model.QueryFilterGroup{
			Logic: model.QueryFilterLogicAnd,
			Rules: []model.QueryFilterRule{
				{Field: "job_title", Operator: model.QueryFilterOpContains, Value: strPtr("found")},
				{Field: "created_at", Operator: model.QueryFilterOpOnOrAfter, Value: strPtr("2026-04-10")},
			},
		},
	}, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("list contacts: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(contacts) != 1 || contacts[0].ID != "contact-1" {
		t.Fatalf("contacts = %+v, want contact-1 only", contacts)
	}
}

func TestCRMContactRepositoryListRejectsInvalidFilterOperator(t *testing.T) {
	db := newCRMContactQueryBuilderTestDB(t)
	repo := NewCRMContactRepository(db)

	_, _, err := repo.List(context.Background(), "ws-1", model.CRMContactListFilters{
		Query: &model.QueryFilterGroup{
			Logic: model.QueryFilterLogicAnd,
			Rules: []model.QueryFilterRule{
				{Field: "lifecycle_stage", Operator: model.QueryFilterOpContains, Value: strPtr("lead")},
			},
		},
	}, model.PMPagination{Page: 1, PerPage: 20})
	if err == nil {
		t.Fatal("expected invalid operator error")
	}

	var validationErr *querybuilder.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %T", err)
	}
}

func newCRMContactQueryBuilderTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:crm-contact-query-builder?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	err = db.Exec(`CREATE TABLE IF NOT EXISTS crm_contacts (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		display_id TEXT NOT NULL,
		first_name TEXT NOT NULL,
		last_name TEXT,
		email TEXT,
		phone TEXT,
		job_title TEXT,
		description TEXT,
		labels TEXT NOT NULL DEFAULT '{}',
		primary_location TEXT,
		country_code TEXT,
		country_name TEXT,
		linkedin_url TEXT,
		facebook_url TEXT,
		instagram_url TEXT,
		angellist_url TEXT,
		x_url TEXT,
		lifecycle_stage TEXT NOT NULL,
		lead_status TEXT NOT NULL,
		owner_member_id TEXT,
		avatar_url TEXT,
		source TEXT,
		custom_properties TEXT,
		email_status TEXT NOT NULL DEFAULT 'valid',
		email_status_reason TEXT,
		email_status_updated_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error
	if err != nil {
		t.Fatalf("create crm_contacts table: %v", err)
	}

	if err := db.Exec(`DELETE FROM crm_contacts`).Error; err != nil {
		t.Fatalf("clear crm_contacts table: %v", err)
	}

	return db
}

func insertCRMContactTestRow(t *testing.T, db *gorm.DB, contact model.CRMContact) {
	t.Helper()

	if err := db.Create(&contact).Error; err != nil {
		t.Fatalf("insert contact %s: %v", contact.ID, err)
	}
}

func strPtr(value string) *string {
	return &value
}
