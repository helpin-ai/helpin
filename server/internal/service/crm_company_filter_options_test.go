package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildCRMCompanyFilterOptions(t *testing.T) {
	software, saas := "Software", "software"
	options := buildCRMCompanyFilterOptions([]model.CRMCompanyFilterSource{
		{Industry: &software, CustomProperties: model.JSONB{"fit": "yes", "support_tools": []any{"zendesk", "intercom"}, "employees_est": float64(40)}},
		{Industry: &saas, CustomProperties: model.JSONB{"fit": "Yes", "support_tools": []any{"zendesk"}, "agent_enrichment": map[string]any{"x": 1}}},
		{CustomProperties: model.JSONB{"fit": true, "notes": "", "crawl": map[string]any{"pages": 3}}},
	})

	if len(options.Industries) != 1 || options.Industries[0] != (model.CRMFilterValueCount{Value: "Software", Count: 2}) {
		t.Fatalf("industries = %+v, want case-insensitive merge", options.Industries)
	}
	byKey := map[string]model.CRMCustomPropertySummary{}
	for _, summary := range options.CustomProperties {
		byKey[summary.Key] = summary
	}
	for _, skipped := range []string{"agent_enrichment", "notes", "crawl"} {
		if _, ok := byKey[skipped]; ok {
			t.Errorf("%s should be skipped (reserved, empty, or nested)", skipped)
		}
	}
	fit := byKey["fit"]
	if fit.Count != 3 || fit.ValueType != "mixed" || len(fit.Values) != 2 || fit.Values[0] != (model.CRMFilterValueCount{Value: "yes", Count: 2}) {
		t.Fatalf("fit = %+v", fit)
	}
	tools := byKey["support_tools"]
	if tools.ValueType != "list" || tools.Count != 2 || tools.Values[0] != (model.CRMFilterValueCount{Value: "zendesk", Count: 2}) {
		t.Fatalf("support_tools = %+v", tools)
	}
	if byKey["employees_est"].ValueType != "number" || byKey["employees_est"].Values[0].Value != "40" {
		t.Fatalf("employees_est = %+v", byKey["employees_est"])
	}
	if options.CustomProperties[0].Key != "fit" || options.ScannedCompanies != 3 {
		t.Fatalf("keys should sort by usage; got %+v", options.CustomProperties)
	}
}

func TestBuildCRMCompanyFilterOptionsCapsValues(t *testing.T) {
	sources := make([]model.CRMCompanyFilterSource, 0, 60)
	for index := 0; index < 60; index++ {
		sources = append(sources, model.CRMCompanyFilterSource{CustomProperties: model.JSONB{"batch": float64(index)}})
	}
	options := buildCRMCompanyFilterOptions(sources)
	batch := options.CustomProperties[0]
	if len(batch.Values) != 50 || !batch.HasMoreValues || batch.Count != 60 {
		t.Fatalf("batch = %d values, more=%v, count=%d", len(batch.Values), batch.HasMoreValues, batch.Count)
	}
}

func TestBuildCRMCompanyFilterOptionsCountsNumericValues(t *testing.T) {
	options := buildCRMCompanyFilterOptions([]model.CRMCompanyFilterSource{
		{CustomProperties: model.JSONB{"employees_est": float64(45)}},
		{CustomProperties: model.JSONB{"employees_est": " 120 "}},
		{CustomProperties: model.JSONB{"employees_est": "11-50"}},
		{CustomProperties: model.JSONB{"employees_est": "unknown"}},
		{CustomProperties: model.JSONB{"employees_est": float64(-3.5)}},
	})
	summary := options.CustomProperties[0]
	if summary.Count != 5 || summary.NumericCount != 3 {
		t.Fatalf("count = %d numeric = %d, want 5 and 3", summary.Count, summary.NumericCount)
	}
	if summary.NumericMin == nil || *summary.NumericMin != -3.5 || summary.NumericMax == nil || *summary.NumericMax != 120 {
		t.Fatalf("range = %v..%v", summary.NumericMin, summary.NumericMax)
	}
}
