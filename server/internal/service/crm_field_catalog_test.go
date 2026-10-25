package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const _crmBehaviorProfilesSQLite = `CREATE TABLE crm_behavior_profiles (
	entity_type TEXT NOT NULL, entity_id TEXT NOT NULL, workspace_id TEXT NOT NULL,
	first_seen_at DATETIME NOT NULL, last_seen_at DATETIME NOT NULL,
	sessions_30d INTEGER NOT NULL DEFAULT 0, match_methods TEXT NOT NULL DEFAULT '',
	computed_at DATETIME NOT NULL, PRIMARY KEY (entity_type, entity_id))`

// newCRMFieldCatalogTestEnv extends the operational command env with the
// field catalog and behavior profiles.
func newCRMFieldCatalogTestEnv(t *testing.T) (*crmOperationalCommandTestEnv, *gorm.DB) {
	t.Helper()
	db := setupCRMOperationalCommandTestDB(t)
	if err := db.Exec(_crmBehaviorProfilesSQLite).Error; err != nil {
		t.Fatalf("create crm_behavior_profiles: %v", err)
	}
	env := newCRMOperationalCommandTestEnv(t, db)
	behavior := repository.NewCRMBehaviorProfileRepository(db)
	env.contact.SetBehaviorProfiles(behavior)
	env.company.SetBehaviorProfiles(behavior)
	env.commands.SetCRMFieldCatalog(NewCRMFieldCatalogService(repository.NewCRMCompanyRepository(db), env.contact, behavior))
	return env, db
}

func describeCRMFields(t *testing.T, env *crmOperationalCommandTestEnv, input string) map[string]any {
	t.Helper()
	output, err := env.commands.Execute(context.Background(), env.meta("workspace", ""), "crm.describe_fields", json.RawMessage(input))
	if err != nil {
		t.Fatalf("describe fields %s: %v", input, err)
	}
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestDescribeCRMFieldsCatalogListsNativeAndCustomFields(t *testing.T) {
	env, _ := newCRMFieldCatalogTestEnv(t)
	seedFilterCompanies(t, env)

	result := describeCRMFields(t, env, `{"object_type":"company"}`)
	fields, _ := result["fields"].([]any)
	byField := map[string]map[string]any{}
	for _, raw := range fields {
		field, _ := raw.(map[string]any)
		byField[field["field"].(string)] = field
	}
	for _, name := range []string{"name", "employee_count", "owner_member_id", "created_at", "last_seen_at", "sessions_30d"} {
		if byField[name] == nil {
			t.Errorf("catalog is missing native field %q", name)
		}
	}
	if byField["employee_count"]["type"] != "number" || byField["last_seen_at"]["type"] != "date" {
		t.Fatalf("field types = %#v / %#v", byField["employee_count"], byField["last_seen_at"])
	}
	properties, _ := result["custom_properties"].([]any)
	if len(properties) != 2 {
		t.Fatalf("custom_properties = %#v, want fit and country", properties)
	}
	fit, _ := properties[0].(map[string]any)
	if fit["key"] != "country" && fit["key"] != "fit" {
		t.Fatalf("unexpected key %#v", fit)
	}
	if fit["field"] != "custom_properties."+fit["key"].(string) || fit["count"] != float64(3) {
		t.Fatalf("property = %#v", fit)
	}
	operators, _ := fit["operators"].([]any)
	if containsAny(operators, "gt") || !containsAny(operators, "is_any_of") || !containsAny(operators, "contains") {
		t.Fatalf("custom property operators = %#v, want text operators without numeric comparisons", operators)
	}
	behavior, _ := result["behavior"].(map[string]any)
	if behavior["total_records"] != float64(4) || behavior["records_with_behavior"] != float64(0) {
		t.Fatalf("behavior coverage = %#v", behavior)
	}
}

func TestDescribeCRMFieldsListsContactEnumValues(t *testing.T) {
	env, db := newCRMFieldCatalogTestEnv(t)
	if err := db.Model(&model.CRMContact{}).Where("id = ?", "contact-1").
		Update("custom_properties", model.JSONB{"plan_tier": "pro"}).Error; err != nil {
		t.Fatal(err)
	}

	result := describeCRMFields(t, env, `{"object_type":"contact"}`)
	fields, _ := result["fields"].([]any)
	var stage map[string]any
	for _, raw := range fields {
		if field, _ := raw.(map[string]any); field["field"] == "lifecycle_stage" {
			stage = field
		}
	}
	values, _ := stage["values"].([]any)
	if stage["type"] != "enum" || !containsAny(values, model.CRMLifecycleCustomer) {
		t.Fatalf("lifecycle_stage = %#v", stage)
	}
	properties, _ := result["custom_properties"].([]any)
	if len(properties) != 1 || properties[0].(map[string]any)["key"] != "plan_tier" {
		t.Fatalf("contact custom_properties = %#v", properties)
	}
}

func TestDescribeCRMFieldsSearchesOneKeysValues(t *testing.T) {
	env, _ := newCRMFieldCatalogTestEnv(t)
	seedFilterCompanies(t, env)

	result := describeCRMFields(t, env, `{"object_type":"company","key":"custom_properties.country","value_query":"d"}`)
	values, _ := result["values"].([]any)
	if result["found"] != true || result["key"] != "country" || result["count"] != float64(3) || len(values) != 1 {
		t.Fatalf("values = %#v", result)
	}
	if first, _ := values[0].(map[string]any); first["value"] != "DE" || first["count"] != float64(2) {
		t.Fatalf("first value = %#v", values[0])
	}

	missing := describeCRMFields(t, env, `{"object_type":"company","key":"plan_tier"}`)
	if missing["found"] != false || len(missing["values"].([]any)) != 0 {
		t.Fatalf("missing key = %#v", missing)
	}
}

func TestDescribeCRMFieldsIsWorkspaceScoped(t *testing.T) {
	env, _ := newCRMFieldCatalogTestEnv(t)
	if _, err := env.company.Create(context.Background(), model.CreateCRMCompanyRequest{
		WorkspaceID: "ws-crm-2", Name: "Other tenant", CustomProperties: map[string]any{"secret_plan": "enterprise"},
	}); err != nil {
		t.Fatal(err)
	}
	result := describeCRMFields(t, env, `{"object_type":"company"}`)
	if properties, _ := result["custom_properties"].([]any); len(properties) != 0 {
		t.Fatalf("catalog leaked another workspace's keys: %#v", properties)
	}
	values := describeCRMFields(t, env, `{"object_type":"company","key":"secret_plan"}`)
	if values["found"] != false {
		t.Fatalf("values leaked another workspace's key: %#v", values)
	}
}

func TestDescribeCRMFieldsRejectsInvalidInput(t *testing.T) {
	env, _ := newCRMFieldCatalogTestEnv(t)
	for _, input := range []string{
		`{"object_type":"deal"}`,
		`{"object_type":"company","key":"Bad Key"}`,
		`{"object_type":"company","value_query":"x"}`,
		`{"object_type":"company","key":"fit","limit":101}`,
	} {
		_, err := env.commands.Execute(context.Background(), env.meta("workspace", ""), "crm.describe_fields", json.RawMessage(input))
		var commandErr *CommandError
		if !errors.As(err, &commandErr) || commandErr.Kind != CommandErrorInvalidInput {
			t.Errorf("input %s: error = %v, want invalid input", input, err)
		}
	}
}

func TestContactFilterOptionsSummarizesCustomProperties(t *testing.T) {
	env, db := newCRMFieldCatalogTestEnv(t)
	if err := db.Model(&model.CRMContact{}).Where("id = ?", "contact-1").
		Update("custom_properties", model.JSONB{"seats": float64(12), "tools": []any{"zendesk"}}).Error; err != nil {
		t.Fatal(err)
	}
	options, err := env.contact.FilterOptions(context.Background(), "ws-crm-1")
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]model.CRMCustomPropertySummary{}
	for _, summary := range options.CustomProperties {
		byKey[summary.Key] = summary
	}
	if options.ScannedContacts != 1 || byKey["seats"].ValueType != "number" || byKey["seats"].NumericMax == nil ||
		*byKey["seats"].NumericMax != 12 || byKey["tools"].ValueType != "list" {
		t.Fatalf("options = %+v", options)
	}
}

func TestCRMListsFilterOnBehaviorAndReturnBehavior(t *testing.T) {
	env, db := newCRMFieldCatalogTestEnv(t)
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	if err := repository.NewCRMBehaviorProfileRepository(db).ReplaceWorkspaceProfiles(context.Background(), "ws-crm-1", []model.CRMBehaviorProfile{{
		EntityType: model.CRMFieldObjectContact, EntityID: "contact-1", WorkspaceID: "ws-crm-1",
		FirstSeenAt: now.AddDate(0, -2, 0), LastSeenAt: now, Sessions30d: 7,
		MatchMethods: "email,identity_link", ComputedAt: now,
	}}); err != nil {
		t.Fatal(err)
	}

	output, err := env.commands.Execute(context.Background(), env.meta("workspace", ""), "crm.list_contacts",
		json.RawMessage(`{"filter":{"rules":[{"field":"sessions_30d","operator":"gte","value":"5"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Total    int `json:"total"`
		Contacts []struct {
			ID       string                    `json:"id"`
			Behavior *model.CRMBehaviorSummary `json:"behavior"`
		} `json:"contacts"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || result.Contacts[0].Behavior == nil || result.Contacts[0].Behavior.Sessions30d != 7 ||
		strings.Join(result.Contacts[0].Behavior.MatchMethods, ",") != "email,identity_link" {
		t.Fatalf("result = %s", output)
	}

	empty, err := env.commands.Execute(context.Background(), env.meta("workspace", ""), "crm.list_companies",
		json.RawMessage(`{"filter":{"rules":[{"field":"last_seen_at","operator":"is_empty"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(empty), `"total":1`) || strings.Contains(string(empty), `"behavior"`) {
		t.Fatalf("companies without profiles = %s", empty)
	}

	catalog := describeCRMFields(t, env, `{"object_type":"contact"}`)
	coverage, _ := catalog["behavior"].(map[string]any)
	if coverage["records_with_behavior"] != float64(1) || coverage["last_computed_at"] == nil {
		t.Fatalf("contact coverage = %#v", coverage)
	}
}

func containsAny(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestCRMCustomPropertyOperatorsMatchValueType(t *testing.T) {
	list := crmCustomPropertyOperatorsFor("list", 0)
	if len(list) != 4 || list[0] != model.QueryFilterOpIsAnyOf {
		t.Fatalf("list operators = %v, want only value-list and empty checks", list)
	}
	text := crmCustomPropertyOperatorsFor("text", 0)
	for _, operator := range text {
		if operator == model.QueryFilterOpGT {
			t.Fatalf("text operators = %v, must not offer numeric comparisons without numeric values", text)
		}
	}
	number := crmCustomPropertyOperatorsFor("number", 3)
	if !containsOperator(number, model.QueryFilterOpBetween) || !containsOperator(number, model.QueryFilterOpIsAnyOf) {
		t.Fatalf("number operators = %v", number)
	}
}

func containsOperator(operators []model.QueryFilterOperator, want model.QueryFilterOperator) bool {
	for _, operator := range operators {
		if operator == want {
			return true
		}
	}
	return false
}
