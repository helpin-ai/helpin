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
