package automationcatalog

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAllSeededEntriesUseApprovedKinds(t *testing.T) {
	t.Parallel()

	entries := All()
	if len(entries) == 0 {
		t.Fatal("expected seeded catalog entries")
	}

	for _, entry := range entries {
		switch entry.Kind {
		case model.AutomationKindBuiltIn, model.AutomationKindRule:
		default:
			t.Fatalf("entry %s has unexpected kind %q", entry.ID, entry.Kind)
		}
	}

	pmRuleIDs := []string{
		"pm.epic_auto_start",
		"pm.epic_auto_complete",
		"pm.sprint_auto_create",
		"pm.sprint_move_unfinished",
	}
	for _, id := range pmRuleIDs {
		entry, ok := ByID(id)
		if !ok {
			t.Fatalf("missing seeded entry %s", id)
		}
		if entry.Kind != model.AutomationKindBuiltIn {
			t.Fatalf("expected %s to be built-in automation, got %s", id, entry.Kind)
		}
	}
}
