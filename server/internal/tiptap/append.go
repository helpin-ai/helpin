package tiptap

import (
	"encoding/json"
	"fmt"
)

// AppendContent merges additional TipTap nodes into an existing document.
// Both inputs should be TipTap JSON with { "type": "doc", "content": [...] }.
// Existing content is fully preserved; new nodes are appended after existing ones.
func AppendContent(existing, additions json.RawMessage) (json.RawMessage, error) {
	if len(existing) == 0 || string(existing) == "null" {
		return additions, nil
	}
	if len(additions) == 0 || string(additions) == "null" {
		return existing, nil
	}

	var existingDoc struct {
		Type    string            `json:"type"`
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(existing, &existingDoc); err != nil {
		return nil, fmt.Errorf("parse existing content: %w", err)
	}

	var additionsDoc struct {
		Type    string            `json:"type"`
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(additions, &additionsDoc); err != nil {
		return nil, fmt.Errorf("parse additions: %w", err)
	}

	merged := struct {
		Type    string            `json:"type"`
		Content []json.RawMessage `json:"content"`
	}{
		Type:    "doc",
		Content: append(existingDoc.Content, additionsDoc.Content...),
	}

	return json.Marshal(merged)
}
