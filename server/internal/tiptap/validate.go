package tiptap

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateDocument reports whether raw is a structurally sound TipTap document.
// Agent tools accept document JSON directly, and a malformed node (for example
// "content" holding a string instead of an array of child nodes) is persisted
// verbatim and then breaks every reader of the document. Rejecting it at the
// boundary keeps bad shapes out of storage.
func ValidateDocument(raw json.RawMessage) error {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return fmt.Errorf("content is required")
	}
	var doc Node
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("content is not valid TipTap document JSON: %w", err)
	}
	if doc.Type != "doc" {
		return fmt.Errorf("content must be a TipTap document with type %q, got %q", "doc", doc.Type)
	}
	return validateChildren(doc.Content, "doc")
}

// ValidateBlockNode reports whether raw is a structurally sound single block
// node, as accepted by the block-level agent tools.
func ValidateBlockNode(raw json.RawMessage) error {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return fmt.Errorf("content is required")
	}
	var node Node
	if err := json.Unmarshal(raw, &node); err != nil {
		return fmt.Errorf("content is not valid TipTap block JSON: %w", err)
	}
	if strings.TrimSpace(node.Type) == "" {
		return fmt.Errorf("block content must include type")
	}
	if node.Type == "doc" {
		return fmt.Errorf("block content must be a single block node, not a document")
	}
	return validateChildren(node.Content, node.Type)
}

func validateChildren(nodes []Node, parentType string) error {
	for _, child := range nodes {
		if strings.TrimSpace(child.Type) == "" {
			return fmt.Errorf("every node inside %q must include type", parentType)
		}
		if err := validateChildren(child.Content, child.Type); err != nil {
			return err
		}
	}
	return nil
}
