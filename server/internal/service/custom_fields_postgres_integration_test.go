//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCustomFieldValuesAndDateFilterPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, _ := seedSupportOpsWorkspace(t, db)
	fields := NewCustomFieldService(repository.NewCustomFieldRepository(db))
	for _, req := range []model.CustomFieldDefinitionRequest{
		{Key: "renews_on", FieldType: "date"},
		{Key: "seats", FieldType: "number"},
		{Key: "regions", FieldType: "multi_select", Options: model.CustomFieldOptions{{Value: "EU"}, {Value: "US"}}},
	} {
		if _, err := fields.Create(ctx, workspaceID, "", model.CustomFieldObjectCRMCompany, req); err != nil {
			t.Fatalf("define %s: %v", req.Key, err)
		}
	}
	ids := map[string]string{}
	for i, name := range []string{"Early", "Late", "Broken"} {
		ids[name] = uuid.NewString()
		mustExec(t, db, `INSERT INTO crm_companies (id, workspace_id, display_id, name, custom_properties) VALUES (?, ?, ?, ?, '{"source":"sdk"}')`,
			ids[name], workspaceID, "CO-"+string(rune('1'+i)), name)
	}
	set := func(name string, values map[string]any) {
		t.Helper()
		if err := fields.SetValues(ctx, workspaceID, model.CustomFieldObjectCRMCompany, ids[name], values); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	set("Early", map[string]any{"renews_on": "2026-10-05", "seats": 5, "regions": []any{"EU", "US", "EU"}})
	set("Late", map[string]any{"renews_on": "2026-12-01"})
	set("Late", map[string]any{"renews_on": nil, "seats": "7"})
	// Integration data that does not fit stays stored and never breaks the cast.
	mustExec(t, db, `UPDATE crm_companies SET custom_properties = '{"renews_on":"soon"}' WHERE id = ?`, ids["Broken"])

	var early, late model.JSONB
	if err := db.Raw(`SELECT custom_properties FROM crm_companies WHERE id = ?`, ids["Early"]).Scan(&early).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw(`SELECT custom_properties FROM crm_companies WHERE id = ?`, ids["Late"]).Scan(&late).Error; err != nil {
		t.Fatal(err)
	}
	if early["source"] != "sdk" || early["seats"] != float64(5) || len(early["regions"].([]any)) != 2 {
		t.Fatalf("early = %#v", early)
	}
	if _, ok := late["renews_on"]; ok || late["seats"] != float64(7) || late["source"] != "sdk" {
		t.Fatalf("late = %#v, want renews_on removed and seats 7", late)
	}

	companies := NewCRMCompanyService(repository.NewCRMCompanyRepository(db))
	companies.SetCustomFieldService(fields)
	before := "2026-11-01"
	query := &model.QueryFilterGroup{Logic: "and", Rules: []model.QueryFilterRule{{Field: "custom_properties.renews_on", Operator: "before", Value: &before}}}
	found, total, err := companies.List(ctx, workspaceID, model.CRMCompanyListFilters{Query: query}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || found[0].ID != ids["Early"] {
		t.Fatalf("before filter = %d companies, want Early only", total)
	}
}
