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

func TestResolveBindingForManualDocRun(t *testing.T) {
	tests := []string{"doc", "document"}
	for _, targetType := range tests {
		t.Run(targetType, func(t *testing.T) {
			bindingID, bindingKind, ok := ResolveBindingForTrigger(
				model.AgentRunTriggerSourceManual,
				model.AgentRunTriggerTypeManual,
				targetType,
			)
			if !ok {
				t.Fatal("expected manual doc trigger binding to resolve")
			}
			if bindingID != "manual.doc_run" {
				t.Fatalf("bindingID = %q, want manual.doc_run", bindingID)
			}
			if bindingKind != "manual" {
				t.Fatalf("bindingKind = %q, want manual", bindingKind)
			}
		})
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

func TestTriggerCatalogIncludesDocsRuleTriggers(t *testing.T) {
	want := map[string]string{
		"doc.published":          model.TriggerDocPublished,
		"ai_section.regenerated": model.TriggerAISectionRegenerated,
		"ai_section.approved":    model.TriggerAISectionApproved,
	}
	seen := map[string]bool{}

	for _, entry := range TriggerCatalog() {
		triggerType, ok := want[entry.ID]
		if !ok {
			continue
		}
		seen[entry.ID] = true
		if entry.BindingKind != "automation_rule" {
			t.Fatalf("%s BindingKind = %q, want automation_rule", entry.ID, entry.BindingKind)
		}
		if entry.TriggerType != triggerType {
			t.Fatalf("%s TriggerType = %q, want %q", entry.ID, entry.TriggerType, triggerType)
		}
		if !entry.SupportsAgentRuns {
			t.Fatalf("expected %s to support agent runs", entry.ID)
		}
	}

	for id := range want {
		if !seen[id] {
			t.Fatalf("%s missing from trigger catalog", id)
		}
	}
}
