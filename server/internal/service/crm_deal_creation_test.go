package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDealCreationRevenueAndParticipants(t *testing.T) {
	for _, tc := range []struct {
		name, period, contacts string
		wantErr                bool
	}{
		{"defaults", "", `["c2","c2","c1"]`, false},
		{"monthly", "monthly", `["c2"]`, false},
		{"annual", "annual", `[]`, false},
		{"invalid period", "weekly", `[]`, true},
		{"foreign participant", "monthly", `["foreign"]`, true},
		{"participant write failure", "monthly", `["c2"]`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, _ := setupCRMPipelineTest(t)
			for _, sql := range []string{
				`DROP TABLE crm_deals`,
				`CREATE TABLE crm_deals (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT, display_id TEXT, name TEXT, pipeline_id TEXT, stage_id TEXT, amount REAL, currency TEXT, revenue_type TEXT DEFAULT 'one_time', close_date DATETIME, owner_member_id TEXT, commercial_motion TEXT, probability INTEGER, custom_properties TEXT, created_at DATETIME, updated_at DATETIME)`,
				`CREATE TABLE crm_contacts (id TEXT PRIMARY KEY, workspace_id TEXT)`,
				`INSERT INTO crm_contacts VALUES ('c1','ws'),('c2','ws'),('foreign','other')`,
				`CREATE TABLE crm_companies (id TEXT PRIMARY KEY, workspace_id TEXT)`,
				`INSERT INTO crm_companies VALUES ('co','ws')`,
				`CREATE TABLE crm_associations (id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id TEXT, from_object_type TEXT, from_object_id TEXT, to_object_type TEXT, to_object_id TEXT, association_label TEXT, created_at DATETIME)`,
			} {
				if err := db.Exec(sql).Error; err != nil {
					t.Fatal(err)
				}
			}
			if tc.name == "participant write failure" {
				if err := db.Exec(`CREATE TRIGGER reject_participant BEFORE INSERT ON crm_associations WHEN NEW.to_object_id = 'c2' BEGIN SELECT RAISE(ABORT, 'simulated association failure'); END`).Error; err != nil {
					t.Fatal(err)
				}
			}
			var req model.CreateCRMDealRequest
			payload := `{"workspace_id":"ws","name":"Deal","company_id":"co","contact_id":"c1","pipeline_id":"p","stage_id":"a","revenue_type":"` + tc.period + `","contact_ids":` + tc.contacts + `}`
			if err := json.Unmarshal([]byte(payload), &req); err != nil {
				t.Fatal(err)
			}
			deal, err := svc.Create(context.Background(), req)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected validation error")
				}
				var count int64
				db.Table("crm_deals").Count(&count)
				if count != 0 {
					t.Fatal("invalid request persisted deal")
				}
				db.Table("crm_associations").Count(&count)
				if count != 0 {
					t.Fatal("failed create left partial associations")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var stored struct{ RevenueType string }
			db.Table("crm_deals").Where("id = ?", deal.ID).Scan(&stored)
			expected := tc.period
			if expected == "" {
				expected = "one_time"
			}
			if stored.RevenueType != expected {
				t.Fatalf("period: got %q want %q", stored.RevenueType, expected)
			}
			var associations []model.CRMAssociation
			db.Find(&associations)
			expectedCount := 2
			if tc.contacts != `[]` {
				expectedCount = 3
			}
			if len(associations) != expectedCount {
				t.Fatalf("associations: got %d want %d", len(associations), expectedCount)
			}
			for _, a := range associations {
				if a.ToObjectID == "c1" && (a.AssociationLabel == nil || *a.AssociationLabel != model.CRMAssociationLabelDealPrimaryContact) {
					t.Fatal("primary contact label overwritten")
				}
			}
			period := "monthly"
			updated, err := svc.Update(context.Background(), deal.ID, model.UpdateCRMDealRequest{RevenueType: &period})
			if err != nil || updated.RevenueType != period {
				t.Fatalf("update period: %v, %+v", err, updated)
			}
			invalid := "weekly"
			if _, err := svc.Update(context.Background(), deal.ID, model.UpdateCRMDealRequest{RevenueType: &invalid}); err == nil {
				t.Fatal("accepted invalid updated period")
			}

		})
	}
}
