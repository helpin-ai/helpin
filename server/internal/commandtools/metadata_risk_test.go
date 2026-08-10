package commandtools

import "testing"

func TestRuntimeToolRiskMetadata(t *testing.T) {
	tests := map[string]string{
		"create_task":              RiskLevelRoutine,
		"create_document":          RiskLevelRoutine,
		"write_document_content":   RiskLevelRoutine,
		"insert_document_artifact": RiskLevelRoutine,
	}
	for alias, want := range tests {
		meta, ok := ToolMetadataForAlias(alias)
		if !ok {
			t.Fatalf("tool %q missing", alias)
		}
		if meta.RiskLevel != want {
			t.Errorf("tool %q risk = %q, want %q", alias, meta.RiskLevel, want)
		}
	}
}
