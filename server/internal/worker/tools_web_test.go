package worker

import "testing"

func TestFormatWebSearchQueryIncludesDomainAllowlist(t *testing.T) {
	got := formatWebSearchQuery("pricing benchmarks", []string{"docs.example.com", "https://brave.com/"})
	expected := "(site:docs.example.com OR site:brave.com) pricing benchmarks"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
