package repository

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMDealRepositoryListFiltersByContactAssociation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	statements := []string{
		`CREATE TABLE crm_pipelines (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, name TEXT NOT NULL, is_default BOOLEAN, position INTEGER, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_pipeline_stages (id TEXT PRIMARY KEY, pipeline_id TEXT NOT NULL, name TEXT NOT NULL, stage_type TEXT, position INTEGER, probability INTEGER, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_deals (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, display_id TEXT NOT NULL, name TEXT NOT NULL, pipeline_id TEXT NOT NULL, stage_id TEXT NOT NULL, amount REAL, currency TEXT, close_date DATETIME, owner_member_id TEXT, probability INTEGER, custom_properties TEXT, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE crm_associations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, from_object_type TEXT NOT NULL, from_object_id TEXT NOT NULL, to_object_type TEXT NOT NULL, to_object_id TEXT NOT NULL, association_label TEXT, created_at DATETIME)`,
		`INSERT INTO crm_pipelines (id, workspace_id, name) VALUES ('pipeline-1', 'workspace-1', 'Sales')`,
		`INSERT INTO crm_pipeline_stages (id, pipeline_id, name, stage_type) VALUES ('stage-1', 'pipeline-1', 'Open', 'open')`,
		`INSERT INTO crm_deals (id, workspace_id, display_id, name, pipeline_id, stage_id, currency, custom_properties, created_at) VALUES ('deal-linked', 'workspace-1', 'DEAL-1', 'Linked', 'pipeline-1', 'stage-1', 'USD', '{}', CURRENT_TIMESTAMP)`,
		`INSERT INTO crm_deals (id, workspace_id, display_id, name, pipeline_id, stage_id, currency, custom_properties, created_at) VALUES ('deal-other', 'workspace-1', 'DEAL-2', 'Other', 'pipeline-1', 'stage-1', 'USD', '{}', CURRENT_TIMESTAMP)`,
		`INSERT INTO crm_associations (id, workspace_id, from_object_type, from_object_id, to_object_type, to_object_id) VALUES ('association-1', 'workspace-1', 'contact', 'contact-1', 'deal', 'deal-linked')`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed test database: %v", err)
		}
	}

	contactID := "contact-1"
	items, total, err := NewCRMDealRepository(db).List(
		context.Background(), "workspace-1", model.CRMDealListFilters{ContactID: &contactID}, model.PMPagination{Page: 1, PerPage: 20},
	)
	if err != nil {
		t.Fatalf("list contact deals: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != "deal-linked" {
		t.Fatalf("items = %#v, total = %d", items, total)
	}
}
