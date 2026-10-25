package dbmigrate

import (
	"strings"
	"testing"
)

func TestIconSelectionMigrationPreservesExistingDecisionFeatures(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations {
		if migration.Version != "202610250003" {
			continue
		}
		if migration.NoTransaction() {
			t.Fatal("constraint replacement must be atomic")
		}
		for _, feature := range []string{"meeting_routing", "coverage_classification", "coverage_topic_matching", "automation_condition", "answer_evidence", "translation_review", "language_detection", "icon_selection"} {
			if !strings.Contains(migration.SQL, "'"+feature+"'") {
				t.Fatalf("missing allowed feature %s", feature)
			}
		}
		return
	}
	t.Fatal("icon selection migration not embedded")
}
