package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func seedFilterCompanies(t *testing.T, env *crmOperationalCommandTestEnv) {
	t.Helper()
	ctx := context.Background()
	for _, seed := range []struct {
		name, domain, fit, country string
	}{
		{name: "Fit DE", domain: "fit-de.io", fit: "yes", country: "DE"},
		{name: "Fit FR", domain: "fit-fr.io", fit: "yes", country: "FR"},
		{name: "Skip DE", domain: "skip-de.io", fit: "no", country: "DE"},
	} {
		domain := seed.domain
		if _, err := env.company.Create(ctx, model.CreateCRMCompanyRequest{
			WorkspaceID: "ws-crm-1", Name: seed.name, Domain: &domain,
			CustomProperties: map[string]any{"fit": seed.fit, "country": seed.country},
		}); err != nil {
			t.Fatalf("seed company %s: %v", seed.name, err)
		}
	}
}

func listFilteredCompanies(t *testing.T, env *crmOperationalCommandTestEnv, input string) map[string]any {
	t.Helper()
	output, err := env.commands.Execute(context.Background(), env.meta("workspace", ""), "crm.list_companies", json.RawMessage(input))
	if err != nil {
		t.Fatalf("list companies %s: %v", input, err)
	}
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCRMListCompaniesFiltersOnCustomProperties(t *testing.T) {
	env := newCRMOperationalCommandTestEnv(t, setupCRMOperationalCommandTestDB(t))
	seedFilterCompanies(t, env)

	result := listFilteredCompanies(t, env, `{"filter":{"logic":"and","rules":[
		{"field":"custom_properties.fit","operator":"is","value":"YES"},
		{"field":"custom_properties.country","operator":"is","value":"de"}
	]}}`)
	companies, _ := result["companies"].([]any)
	if len(companies) != 1 || result["total"] != float64(1) {
		t.Fatalf("result = %#v, want only Fit DE", result)
	}
	company, _ := companies[0].(map[string]any)
	properties, _ := company["custom_properties"].(map[string]any)
	if company["name"] != "Fit DE" || properties["fit"] != "yes" {
		t.Fatalf("company = %#v", company)
	}
}

func TestCRMListCompaniesPagesWithOffset(t *testing.T) {
	env := newCRMOperationalCommandTestEnv(t, setupCRMOperationalCommandTestDB(t))
	seedFilterCompanies(t, env)

	filter := `"filter":{"rules":[{"field":"custom_properties.fit","operator":"is_not_empty"}]}`
	first := listFilteredCompanies(t, env, `{"limit":2,`+filter+`}`)
	second := listFilteredCompanies(t, env, `{"limit":2,"offset":2,`+filter+`}`)
	firstItems, _ := first["companies"].([]any)
	secondItems, _ := second["companies"].([]any)
	if len(firstItems) != 2 || first["has_more"] != true || first["next_offset"] != float64(2) {
		t.Fatalf("first page = %#v", first)
	}
	if len(secondItems) != 1 || second["has_more"] != false {
		t.Fatalf("second page = %#v", second)
	}
}

func TestCRMListCompaniesRejectsUnsafeFilterFields(t *testing.T) {
	env := newCRMOperationalCommandTestEnv(t, setupCRMOperationalCommandTestDB(t))
	for _, field := range []string{"custom_properties.fit') OR 1=1 --", "custom_properties.Fit", "password"} {
		input := mustJSON(map[string]any{"filter": map[string]any{"rules": []any{
			map[string]any{"field": field, "operator": "is", "value": "x"},
		}}})
		_, err := env.commands.Execute(context.Background(), env.meta("workspace", ""), "crm.list_companies", input)
		var commandErr *CommandError
		if !errors.As(err, &commandErr) || commandErr.Kind != CommandErrorInvalidInput {
			t.Errorf("field %q error = %v, want invalid input", field, err)
		}
	}
}

func TestCRMListContactsFiltersOnCustomProperties(t *testing.T) {
	env := newCRMOperationalCommandTestEnv(t, setupCRMOperationalCommandTestDB(t))
	ctx := context.Background()
	for _, seed := range []struct{ name, tier string }{{"Grace", "gold"}, {"Linus", "silver"}} {
		if _, err := env.contact.Create(ctx, model.CreateCRMContactRequest{
			WorkspaceID: "ws-crm-1", FirstName: seed.name, CustomProperties: map[string]any{"tier": seed.tier},
		}); err != nil {
			t.Fatalf("seed contact: %v", err)
		}
	}
	output, err := env.commands.Execute(ctx, env.meta("workspace", ""), "crm.list_contacts", json.RawMessage(`{
		"filter":{"rules":[{"field":"custom_properties.tier","operator":"is","value":"gold"}]}
	}`))
	if err != nil {
		t.Fatalf("list contacts: %v", err)
	}
	var result struct {
		Contacts []struct {
			FirstName        string         `json:"first_name"`
			CustomProperties map[string]any `json:"custom_properties"`
		} `json:"contacts"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Contacts) != 1 || result.Contacts[0].FirstName != "Grace" || result.Contacts[0].CustomProperties["tier"] != "gold" {
		t.Fatalf("contacts = %#v", result.Contacts)
	}
}

func TestCRMListCompaniesAnyOfMatchesScalarsAndListElements(t *testing.T) {
	env := newCRMOperationalCommandTestEnv(t, setupCRMOperationalCommandTestDB(t))
	ctx := context.Background()
	for _, seed := range []struct {
		name  string
		tools any
		fit   any
	}{
		{name: "Zendesk shop", tools: []any{"Zendesk", "Intercom"}, fit: "yes"},
		{name: "Freshdesk shop", tools: []any{"freshdesk"}, fit: "maybe"},
		{name: "Plain value", tools: "zendesk", fit: true},
		{name: "No tools", fit: "no"},
	} {
		properties := map[string]any{"fit": seed.fit}
		if seed.tools != nil {
			properties["support_tools"] = seed.tools
		}
		if _, err := env.company.Create(ctx, model.CreateCRMCompanyRequest{WorkspaceID: "ws-crm-1", Name: seed.name, CustomProperties: properties}); err != nil {
			t.Fatalf("seed %s: %v", seed.name, err)
		}
	}
	names := func(input string) map[string]bool {
		t.Helper()
		result := listFilteredCompanies(t, env, input)
		found := map[string]bool{}
		companies, _ := result["companies"].([]any)
		for _, item := range companies {
			found[item.(map[string]any)["name"].(string)] = true
		}
		return found
	}

	tools := names(`{"filter":{"rules":[{"field":"custom_properties.support_tools","operator":"is_any_of","values":["ZENDESK"]}]}}`)
	if len(tools) != 2 || !tools["Zendesk shop"] || !tools["Plain value"] {
		t.Fatalf("support_tools is any of zendesk = %v, want list and scalar matches", tools)
	}
	fit := names(`{"filter":{"rules":[{"field":"custom_properties.fit","operator":"is_any_of","values":["yes","true"]}]}}`)
	if len(fit) != 2 || !fit["Zendesk shop"] || !fit["Plain value"] {
		t.Fatalf("fit is any of yes/true = %v", fit)
	}
	both := names(`{"filter":{"logic":"and","rules":[
		{"field":"custom_properties.support_tools","operator":"is_any_of","values":["zendesk","freshdesk"]},
		{"field":"custom_properties.fit","operator":"is_any_of","values":["maybe"]}
	]}}`)
	if len(both) != 1 || !both["Freshdesk shop"] {
		t.Fatalf("combined filter = %v", both)
	}
}

func TestCRMListCompaniesAnyOfRequiresBoundedValues(t *testing.T) {
	env := newCRMOperationalCommandTestEnv(t, setupCRMOperationalCommandTestDB(t))
	tooMany := make([]string, 51)
	for index := range tooMany {
		tooMany[index] = "v"
	}
	for _, values := range [][]string{{}, {"  "}, tooMany} {
		input := mustJSON(map[string]any{"filter": map[string]any{"rules": []any{
			map[string]any{"field": "industry", "operator": "is_any_of", "values": values},
		}}})
		_, err := env.commands.Execute(context.Background(), env.meta("workspace", ""), "crm.list_companies", input)
		var commandErr *CommandError
		if !errors.As(err, &commandErr) || commandErr.Kind != CommandErrorInvalidInput {
			t.Errorf("values %d error = %v, want invalid input", len(values), err)
		}
	}
}

func TestCRMListCompaniesNumericComparisonsSkipNonNumericValues(t *testing.T) {
	env := newCRMOperationalCommandTestEnv(t, setupCRMOperationalCommandTestDB(t))
	ctx := context.Background()
	for _, seed := range []struct {
		name      string
		employees any
	}{
		{name: "Number", employees: float64(120)},
		{name: "Numeric string", employees: " 75 "},
		{name: "Decimal", employees: 50.5},
		{name: "Range text", employees: "11-50"},
		{name: "Unknown", employees: "unknown"},
		{name: "List", employees: []any{"200"}},
		{name: "Small", employees: float64(8)},
		{name: "Missing"},
	} {
		properties := map[string]any{"batch": "b1"}
		if seed.employees != nil {
			properties["employees_est"] = seed.employees
		}
		if _, err := env.company.Create(ctx, model.CreateCRMCompanyRequest{WorkspaceID: "ws-crm-1", Name: seed.name, CustomProperties: properties}); err != nil {
			t.Fatalf("seed %s: %v", seed.name, err)
		}
	}
	names := func(rule string) map[string]bool {
		t.Helper()
		result := listFilteredCompanies(t, env, `{"filter":{"rules":[`+rule+`]}}`)
		found := map[string]bool{}
		companies, _ := result["companies"].([]any)
		for _, item := range companies {
			found[item.(map[string]any)["name"].(string)] = true
		}
		return found
	}

	greater := names(`{"field":"custom_properties.employees_est","operator":"gt","value":"50"}`)
	if len(greater) != 3 || !greater["Number"] || !greater["Numeric string"] || !greater["Decimal"] {
		t.Fatalf("employees_est > 50 = %v, want numbers and numeric strings only", greater)
	}
	between := names(`{"field":"custom_properties.employees_est","operator":"between","values":["8","75"]}`)
	if len(between) != 3 || !between["Small"] || !between["Decimal"] || !between["Numeric string"] {
		t.Fatalf("employees_est between 8 and 75 = %v", between)
	}
	empty := names(`{"field":"custom_properties.employees_est","operator":"is_empty"}`)
	if !empty["Missing"] || empty["Unknown"] || empty["Range text"] || empty["List"] {
		t.Fatalf("is_empty must mean missing or blank, not non-numeric; got %v", empty)
	}
}

func TestCRMListCompaniesNoneOfIncludesMissingAndChecksListElements(t *testing.T) {
	env := newCRMOperationalCommandTestEnv(t, setupCRMOperationalCommandTestDB(t))
	ctx := context.Background()
	for _, seed := range []struct {
		name  string
		tools any
	}{
		{name: "Zendesk list", tools: []any{"Intercom", "Zendesk"}},
		{name: "Zendesk plain", tools: "zendesk"},
		{name: "Crisp list", tools: []any{"Crisp"}},
		{name: "No tools"},
	} {
		properties := map[string]any{"batch": "b1"}
		if seed.tools != nil {
			properties["support_tools"] = seed.tools
		}
		if _, err := env.company.Create(ctx, model.CreateCRMCompanyRequest{WorkspaceID: "ws-crm-1", Name: seed.name, CustomProperties: properties}); err != nil {
			t.Fatalf("seed %s: %v", seed.name, err)
		}
	}
	result := listFilteredCompanies(t, env, `{"filter":{"rules":[
		{"field":"custom_properties.batch","operator":"is","value":"b1"},
		{"field":"custom_properties.support_tools","operator":"is_none_of","values":["ZENDESK"]}
	]}}`)
	found := map[string]bool{}
	for _, item := range result["companies"].([]any) {
		found[item.(map[string]any)["name"].(string)] = true
	}
	if len(found) != 2 || !found["Crisp list"] || !found["No tools"] {
		t.Fatalf("support_tools is none of zendesk = %v, want Crisp list and No tools", found)
	}
}
