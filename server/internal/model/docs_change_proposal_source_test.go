package model

import (
	"encoding/json"
	"testing"
)

func TestNormalizeDocsChangeProposalSourcesKeepsOnlyCanonicalSources(t *testing.T) {
	raw := json.RawMessage(`[
		{"type":"conversation","id":"conv-1","label":"Billing question"},
		{"source_type":"document","title":"Refund policy","url":"/docs/refunds"},
		{"type":"url","label":"Stripe docs","url":"https://stripe.com/docs"},
		{"type":"document","id":"doc-1"},
		"bad entry"
	]`)

	got := NormalizeDocsChangeProposalSources(raw)

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2: %#v", len(got), got)
	}
	assertSource := func(idx int, typ, id, label, url string) {
		t.Helper()
		if got[idx].Type != typ || got[idx].ID != id || got[idx].Label != label || got[idx].URL != url {
			t.Fatalf("source[%d] = %#v, want type=%q id=%q label=%q url=%q", idx, got[idx], typ, id, label, url)
		}
	}
	assertSource(0, "conversation", "conv-1", "Billing question", "")
	assertSource(1, "url", "", "Stripe docs", "https://stripe.com/docs")
}

func TestNormalizeDocsChangeProposalSourcesReturnsEmptyForMalformedJSON(t *testing.T) {
	got := NormalizeDocsChangeProposalSources(json.RawMessage(`{bad json`))
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
}
