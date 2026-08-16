package service

import (
	"encoding/json"
	"testing"
)

func TestNormalizeInternalCommandDocumentContentRemovesMatchingLeadingTitle(t *testing.T) {
	got := normalizeInternalCommandDocumentContent(json.RawMessage(`"# Release notes\n\n## Highlights\n\n- Shipped"`), "Release notes")
	var document map[string]any
	if err := json.Unmarshal(got, &document); err != nil {
		t.Fatalf("decode normalized document: %v", err)
	}
	content := document["content"].([]any)
	if len(content) != 2 || content[0].(map[string]any)["type"] != "heading" {
		t.Fatalf("content = %#v, want leading duplicate title removed", content)
	}
}

func TestNormalizeInternalCommandDocumentContentKeepsDifferentLeadingHeading(t *testing.T) {
	got := normalizeInternalCommandDocumentContent(json.RawMessage(`"# Overview\n\nBody"`), "Release notes")
	var document map[string]any
	if err := json.Unmarshal(got, &document); err != nil {
		t.Fatalf("decode normalized document: %v", err)
	}
	content := document["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("content length = %d, want 2", len(content))
	}
}
