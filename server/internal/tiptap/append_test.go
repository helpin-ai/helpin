package tiptap

import (
	"encoding/json"
	"testing"
)

func TestAppendContent_BothValid(t *testing.T) {
	existing := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`)
	additions := json.RawMessage(`{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"New Section"}]}]}`)

	merged, err := AppendContent(existing, additions)
	if err != nil {
		t.Fatalf("AppendContent: %v", err)
	}

	var doc struct {
		Type    string            `json:"type"`
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(merged, &doc); err != nil {
		t.Fatalf("unmarshal merged: %v", err)
	}
	if doc.Type != "doc" {
		t.Errorf("expected type=doc, got %q", doc.Type)
	}
	if len(doc.Content) != 2 {
		t.Fatalf("expected 2 content nodes, got %d", len(doc.Content))
	}
}

func TestAppendContent_ExistingNil(t *testing.T) {
	additions := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"New"}]}]}`)
	merged, err := AppendContent(nil, additions)
	if err != nil {
		t.Fatalf("AppendContent: %v", err)
	}
	if string(merged) != string(additions) {
		t.Errorf("expected additions returned as-is")
	}
}

func TestAppendContent_AdditionsNil(t *testing.T) {
	existing := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Existing"}]}]}`)
	merged, err := AppendContent(existing, nil)
	if err != nil {
		t.Fatalf("AppendContent: %v", err)
	}
	if string(merged) != string(existing) {
		t.Errorf("expected existing returned as-is")
	}
}

func TestAppendContent_PreservesExistingNodes(t *testing.T) {
	existing := json.RawMessage(`{"type":"doc","content":[{"type":"resizableImage","attrs":{"src":"img.png"}},{"type":"paragraph","content":[{"type":"text","text":"Caption"}]}]}`)
	additions := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Appended"}]}]}`)

	merged, err := AppendContent(existing, additions)
	if err != nil {
		t.Fatalf("AppendContent: %v", err)
	}

	var doc struct {
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(merged, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Content) != 3 {
		t.Fatalf("expected 3 content nodes (image + caption + appended), got %d", len(doc.Content))
	}
}

func TestAppendContent_BothNil(t *testing.T) {
	merged, err := AppendContent(nil, nil)
	if err != nil {
		t.Fatalf("AppendContent: %v", err)
	}
	if merged != nil {
		t.Errorf("expected nil, got %s", string(merged))
	}
}
