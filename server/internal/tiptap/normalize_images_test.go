package tiptap

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeImageNodes(t *testing.T) {
	tests := []struct {
		name, raw  string
		wantChange bool
	}{
		{"nested images", `{"type":"doc","content":[{"type":"image","attrs":{"src":"https://example.test/a.png","metadata":{"type":"image"},"size":9007199254740993}},{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph"},{"type":"image","attrs":{"alt":"Keep me"}}]}]}]}`, true},
		{"canonical", `{"type":"doc","content":[{"type":"resizableImage","attrs":{"src":"a"}}]}`, false},
		{"attributes are not nodes", `{"type":"doc","attrs":{"type":"image"},"content":[{"type":"paragraph"}]}`, false},
		{"malformed", `{"type":"doc","content":`, false},
		{"markdown envelope", `{"_markdown_source":"image"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeImageNodes(json.RawMessage(tt.raw))
			if (string(got) != tt.raw) != tt.wantChange {
				t.Fatalf("unexpected conversion: %s", got)
			}
			if string(NormalizeImageNodes(got)) != string(got) {
				t.Fatal("conversion is not idempotent")
			}
			if tt.wantChange {
				if err := ValidateDocument(got); err != nil {
					t.Fatal(err)
				}
				if strings.Count(string(got), `"resizableImage"`) != 2 || !strings.Contains(string(got), `9007199254740993`) || !strings.Contains(string(got), `"metadata":{"type":"image"}`) {
					t.Fatalf("content not preserved: %s", got)
				}
			}
		})
	}
}

func TestValidateDocumentUnknownNode(t *testing.T) {
	for _, raw := range []string{`{"type":"doc","content":[{"type":"unknownWidget"}]}`, `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"image"}]}]}`} {
		if ValidateDocument(json.RawMessage(raw)) == nil {
			t.Fatalf("accepted invalid document: %s", raw)
		}
	}
	if err := ValidateDocument(json.RawMessage(`{"type":"doc","content":[{"type":"image","attrs":{"src":"a"}}]}`)); err != nil {
		t.Fatal(err)
	}
	if ValidateBlockNode(json.RawMessage(`{"type":"unknownWidget"}`)) == nil {
		t.Fatal("accepted unknown block")
	}
}
