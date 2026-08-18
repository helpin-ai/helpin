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
	for index, child := range nodes {
		if strings.TrimSpace(child.Type) == "" {
			return fmt.Errorf("every node inside %q must include type", parentType)
		}
		if child.Type == "text" && child.Text == "" {
			return fmt.Errorf("text nodes cannot be empty")
		}
		if err := validateMarks(child); err != nil {
			return err
		}
		if err := validateChildPlacement(parentType, index, child.Type); err != nil {
			return err
		}
		if err := validateChildren(child.Content, child.Type); err != nil {
			return err
		}
	}
	return nil
}

func validateMarks(node Node) error {
	seen := make(map[string]struct{}, len(node.Marks))
	hasCode := false
	for _, mark := range node.Marks {
		markType := strings.TrimSpace(mark.Type)
		if markType == "" {
			return fmt.Errorf("marks on %q must include type", node.Type)
		}
		if _, exists := seen[markType]; exists {
			return fmt.Errorf("%q contains duplicate %q marks", node.Type, markType)
		}
		seen[markType] = struct{}{}
		hasCode = hasCode || markType == "code"
	}
	if hasCode && len(node.Marks) > 1 {
		return fmt.Errorf("code marks cannot be combined with other marks")
	}
	return nil
}

func validateChildPlacement(parentType string, index int, childType string) error {
	switch parentType {
	case "paragraph", "heading":
		if childType != "text" && childType != "hardBreak" && childType != "entityMention" {
			return fmt.Errorf("%q cannot contain block node %q", parentType, childType)
		}
	case "codeBlock":
		if childType != "text" {
			return fmt.Errorf("codeBlock can only contain text, got %q", childType)
		}
	case "bulletList", "orderedList":
		if childType != "listItem" {
			return fmt.Errorf("%s children must be listItem nodes, got %q", parentType, childType)
		}
	case "taskList":
		if childType != "taskItem" {
			return fmt.Errorf("taskList children must be taskItem nodes, got %q", childType)
		}
	case "listItem", "taskItem":
		if index == 0 && childType != "paragraph" {
			return fmt.Errorf("%s must start with a paragraph, got %q", parentType, childType)
		}
	case "table":
		if childType != "tableRow" {
			return fmt.Errorf("table children must be tableRow nodes, got %q", childType)
		}
	case "tableRow":
		if childType != "tableCell" && childType != "tableHeader" {
			return fmt.Errorf("tableRow children must be tableCell or tableHeader nodes, got %q", childType)
		}
	}
	return nil
}
