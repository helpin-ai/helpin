package automationcatalog

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestResolveBindingForManualRepositoryRun(t *testing.T) {
	bindingID, bindingKind, ok := ResolveBindingForTrigger(
		model.AgentRunTriggerSourceManual,
		model.AgentRunTriggerTypeManual,
		"repository",
	)
	if !ok {
		t.Fatal("expected manual repository trigger binding to resolve")
	}
	if bindingID != "manual.repository_run" {
		t.Fatalf("bindingID = %q, want manual.repository_run", bindingID)
	}
	if bindingKind != "manual" {
		t.Fatalf("bindingKind = %q, want manual", bindingKind)
	}
}

func TestTriggerCatalogIncludesManualRepositoryRun(t *testing.T) {
	for _, entry := range TriggerCatalog() {
		if entry.ID != "manual.repository_run" {
			continue
		}
		if entry.BindingKind != "manual" {
			t.Fatalf("BindingKind = %q, want manual", entry.BindingKind)
		}
		if entry.TriggerType != model.AgentRunTriggerTypeManual {
			t.Fatalf("TriggerType = %q, want %q", entry.TriggerType, model.AgentRunTriggerTypeManual)
		}
		if !entry.SupportsAgentRuns {
			t.Fatal("expected manual.repository_run to support agent runs")
		}
		return
	}
	t.Fatal("manual.repository_run missing from trigger catalog")
}
