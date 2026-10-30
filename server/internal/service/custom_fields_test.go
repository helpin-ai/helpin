package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const _customFieldTestWorkspace = "ws-crm-1"

// newCustomFieldTestEnv returns the CRM command env with seeded companies
// (custom properties fit and country) and the custom field service wired.
func newCustomFieldTestEnv(t *testing.T) (*crmOperationalCommandTestEnv, *CustomFieldService) {
	t.Helper()
	env, db := newCRMFieldCatalogTestEnv(t)
	fields := NewCustomFieldService(repository.NewCustomFieldRepository(db))
	env.company.SetCustomFieldService(fields)
	env.contact.SetCustomFieldService(fields)
	catalog := NewCRMFieldCatalogService(repository.NewCRMCompanyRepository(db), env.contact, repository.NewCRMBehaviorProfileRepository(db))
	catalog.SetCustomFieldService(fields)
	env.commands.SetCRMFieldCatalog(catalog)
	seedFilterCompanies(t, env)
	return env, fields
}

func defineCompanyField(t *testing.T, fields *CustomFieldService, req model.CustomFieldDefinitionRequest) *model.CustomFieldDefinition {
	t.Helper()
	definition, err := fields.Create(context.Background(), _customFieldTestWorkspace, "", model.CustomFieldObjectCRMCompany, req)
	if err != nil {
		t.Fatalf("define %s: %v", req.Key, err)
	}
	return definition
}

func companyIDByName(t *testing.T, env *crmOperationalCommandTestEnv, name string) string {
	t.Helper()
	companies, _, err := env.company.List(context.Background(), _customFieldTestWorkspace, model.CRMCompanyListFilters{Search: &name}, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil || len(companies) == 0 {
		t.Fatalf("find company %s: %v", name, err)
	}
	return companies[0].ID
}

func TestCustomFieldCreateRejectsInvalidDefinitions(t *testing.T) {
	_, fields := newCustomFieldTestEnv(t)
	ctx := context.Background()
	defineCompanyField(t, fields, model.CustomFieldDefinitionRequest{Key: "plan", FieldType: "text"})
	archived := defineCompanyField(t, fields, model.CustomFieldDefinitionRequest{Key: "old_plan", FieldType: "text"})
	if err := fields.SetArchived(ctx, _customFieldTestWorkspace, model.CustomFieldObjectCRMCompany, archived.ID, true); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		object string
		req    model.CustomFieldDefinitionRequest
		want   error
	}{
		{"uppercase key", model.CustomFieldObjectCRMCompany, model.CustomFieldDefinitionRequest{Key: "Plan", FieldType: "text"}, ErrCustomFieldInvalid},
		{"reserved key", model.CustomFieldObjectCRMCompany, model.CustomFieldDefinitionRequest{Key: "agent_enrichment", FieldType: "text"}, ErrCustomFieldInvalid},
		{"unknown type", model.CustomFieldObjectCRMCompany, model.CustomFieldDefinitionRequest{Key: "size", FieldType: "file"}, ErrCustomFieldInvalid},
		{"select without options", model.CustomFieldObjectCRMCompany, model.CustomFieldDefinitionRequest{Key: "size", FieldType: "select"}, ErrCustomFieldInvalid},
		{"unknown object", "deal", model.CustomFieldDefinitionRequest{Key: "size", FieldType: "text"}, ErrCustomFieldInvalid},
		{"duplicate key", model.CustomFieldObjectCRMCompany, model.CustomFieldDefinitionRequest{Key: "plan", FieldType: "number"}, ErrCustomFieldExists},
		{"archived key", model.CustomFieldObjectCRMCompany, model.CustomFieldDefinitionRequest{Key: "old_plan", FieldType: "text"}, ErrCustomFieldExists},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := fields.Create(ctx, _customFieldTestWorkspace, "", tc.object, tc.req); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
	// The same key is free on another object type.
	if _, err := fields.Create(ctx, _customFieldTestWorkspace, "", model.CustomFieldObjectCRMContact, model.CustomFieldDefinitionRequest{Key: "plan", FieldType: "text"}); err != nil {
		t.Fatalf("contact plan: %v", err)
	}
}

func TestCustomFieldUpdateArchiveAndReorder(t *testing.T) {
	_, fields := newCustomFieldTestEnv(t)
	ctx := context.Background()
	first := defineCompanyField(t, fields, model.CustomFieldDefinitionRequest{Key: "plan", FieldType: "text"})
	second := defineCompanyField(t, fields, model.CustomFieldDefinitionRequest{Key: "seats", FieldType: "number"})
	if first.DisplayLabel != "Plan" || second.Position <= first.Position {
		t.Fatalf("created = %#v / %#v", first, second)
	}

	updated, err := fields.Update(ctx, _customFieldTestWorkspace, model.CustomFieldObjectCRMCompany, first.ID, model.CustomFieldDefinitionRequest{
		Key: "renamed", Label: "  Billing   plan ", FieldType: "select",
		Options: model.CustomFieldOptions{{Value: "pro"}, {Value: "free", Label: "Free tier"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Key != "plan" || updated.Label != "Billing plan" || len(updated.Options) != 2 || updated.Options[0].Label != "Pro" {
		t.Fatalf("updated = %#v", updated)
	}
	if _, err := fields.Update(ctx, _customFieldTestWorkspace, model.CustomFieldObjectCRMContact, first.ID, model.CustomFieldDefinitionRequest{FieldType: "text"}); !errors.Is(err, repository.ErrCustomFieldNotFound) {
		t.Fatalf("update through another object = %v, want not found", err)
	}

	ordered, err := fields.Reorder(ctx, _customFieldTestWorkspace, model.CustomFieldObjectCRMCompany, []string{second.ID, first.ID})
	if err != nil || len(ordered) != 2 || ordered[0].Key != "seats" {
		t.Fatalf("reorder = %#v, %v", ordered, err)
	}

	if err := fields.SetArchived(ctx, _customFieldTestWorkspace, model.CustomFieldObjectCRMCompany, second.ID, true); err != nil {
		t.Fatal(err)
	}
	active, _ := fields.List(ctx, _customFieldTestWorkspace, model.CustomFieldObjectCRMCompany, false, false)
	all, _ := fields.List(ctx, _customFieldTestWorkspace, model.CustomFieldObjectCRMCompany, true, false)
	if len(active) != 1 || len(all) != 2 {
		t.Fatalf("active = %d, all = %d", len(active), len(all))
	}
	if err := fields.SetArchived(ctx, _customFieldTestWorkspace, model.CustomFieldObjectCRMCompany, second.ID, false); err != nil {
		t.Fatal(err)
	}
	if active, _ = fields.List(ctx, _customFieldTestWorkspace, model.CustomFieldObjectCRMCompany, false, false); len(active) != 2 {
		t.Fatalf("restored active = %d", len(active))
	}
}

func TestCustomFieldListCountsUsageAndMismatches(t *testing.T) {
	_, fields := newCustomFieldTestEnv(t)
	defineCompanyField(t, fields, model.CustomFieldDefinitionRequest{
		Key: "country", FieldType: "select", Options: model.CustomFieldOptions{{Value: "DE"}},
	})
	defineCompanyField(t, fields, model.CustomFieldDefinitionRequest{Key: "unused", FieldType: "text"})
	definitions, err := fields.List(context.Background(), _customFieldTestWorkspace, model.CustomFieldObjectCRMCompany, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if definitions[0].UsageCount != 3 || definitions[0].MismatchCount != 1 || definitions[0].UsageSampled {
		t.Fatalf("country usage = %#v", definitions[0])
	}
	if definitions[1].UsageCount != 0 {
		t.Fatalf("unused usage = %#v", definitions[1])
	}
}

func TestCustomFieldSetValuesWritesKeysOnly(t *testing.T) {
	env, fields := newCustomFieldTestEnv(t)
	ctx := context.Background()
	defineCompanyField(t, fields, model.CustomFieldDefinitionRequest{Key: "seats", FieldType: "number"})
	defineCompanyField(t, fields, model.CustomFieldDefinitionRequest{Key: "fit", FieldType: "text"})
	archived := defineCompanyField(t, fields, model.CustomFieldDefinitionRequest{Key: "legacy", FieldType: "text"})
	if err := fields.SetArchived(ctx, _customFieldTestWorkspace, model.CustomFieldObjectCRMCompany, archived.ID, true); err != nil {
		t.Fatal(err)
	}
	id := companyIDByName(t, env, "Fit DE")

	if err := fields.SetValues(ctx, _customFieldTestWorkspace, model.CustomFieldObjectCRMCompany, id, map[string]any{"seats": "12", "fit": nil}); err != nil {
		t.Fatal(err)
	}
	company, err := env.company.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	properties := company.CustomProperties
	if properties["seats"] != float64(12) || properties["country"] != "DE" {
		t.Fatalf("properties = %#v, want seats 12 and country kept", properties)
	}
	if _, ok := properties["fit"]; ok {
		t.Fatalf("fit = %#v, want removed", properties["fit"])
	}

	for _, tc := range []struct {
		name   string
		values map[string]any
		record string
		want   error
	}{
		{"undefined key", map[string]any{"country": "FR"}, id, ErrCustomFieldInvalid},
		{"archived key", map[string]any{"legacy": "x"}, id, ErrCustomFieldInvalid},
		{"wrong type", map[string]any{"seats": "many"}, id, ErrCustomFieldInvalid},
		{"no values", map[string]any{}, id, ErrCustomFieldInvalid},
		{"missing record", map[string]any{"seats": 1}, "00000000-0000-0000-0000-000000000000", ErrCustomFieldRecordNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := fields.SetValues(ctx, _customFieldTestWorkspace, model.CustomFieldObjectCRMCompany, tc.record, tc.values); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
	if err := fields.SetValues(ctx, "ws-other", model.CustomFieldObjectCRMCompany, id, map[string]any{"seats": 1}); !errors.Is(err, ErrCustomFieldInvalid) {
		t.Fatalf("other workspace = %v, want invalid (no such field there)", err)
	}
}

func TestCRMListCompaniesFiltersDefinedDateFields(t *testing.T) {
	env, fields := newCustomFieldTestEnv(t)
	ctx := context.Background()
	defineCompanyField(t, fields, model.CustomFieldDefinitionRequest{Key: "renews_on", FieldType: "date"})
	for name, date := range map[string]string{"Fit DE": "2026-10-05", "Fit FR": "2026-12-01"} {
		if err := fields.SetValues(ctx, _customFieldTestWorkspace, model.CustomFieldObjectCRMCompany, companyIDByName(t, env, name), map[string]any{"renews_on": date}); err != nil {
			t.Fatal(err)
		}
	}
	result := listFilteredCompanies(t, env, `{"filter":{"rules":[{"field":"custom_properties.renews_on","operator":"before","value":"2026-11-01"}]}}`)
	companies, _ := result["companies"].([]any)
	if len(companies) != 1 || companies[0].(map[string]any)["name"] != "Fit DE" {
		t.Fatalf("result = %#v, want only Fit DE", result)
	}
}

func TestDescribeCRMFieldsPutsDefinedFieldsFirst(t *testing.T) {
	env, fields := newCustomFieldTestEnv(t)
	defineCompanyField(t, fields, model.CustomFieldDefinitionRequest{
		Key: "country", Label: "Country", FieldType: "select", Options: model.CustomFieldOptions{{Value: "DE"}, {Value: "FR"}},
	})
	defineCompanyField(t, fields, model.CustomFieldDefinitionRequest{Key: "renews_on", FieldType: "date"})

	result := describeCRMFields(t, env, `{"object_type":"company"}`)
	properties, _ := result["custom_properties"].([]any)
	keys := make([]string, 0, len(properties))
	for _, raw := range properties {
		keys = append(keys, raw.(map[string]any)["key"].(string))
	}
	if len(keys) != 3 || keys[0] != "country" || keys[1] != "renews_on" || keys[2] != "fit" {
		t.Fatalf("keys = %v, want defined country, renews_on, then fit", keys)
	}
	country := properties[0].(map[string]any)
	definition, _ := country["definition"].(map[string]any)
	if definition["field_type"] != "select" || country["count"] != float64(3) {
		t.Fatalf("country = %#v", country)
	}
	renews := properties[1].(map[string]any)
	operators, _ := renews["operators"].([]any)
	if renews["count"] != float64(0) || len(operators) == 0 || operators[0] != "on" {
		t.Fatalf("renews_on = %#v, want unused date field with date operators", renews)
	}
	if _, ok := properties[2].(map[string]any)["definition"]; ok {
		t.Fatalf("fit should have no definition: %#v", properties[2])
	}
}
