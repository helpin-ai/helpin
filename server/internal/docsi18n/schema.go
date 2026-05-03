package docsi18n

import (
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func ValidateDocument(root *tiptap.Node) error {
	if root == nil {
		return fmt.Errorf("document is nil")
	}
	return validateNodeSchema(root, NodePath{})
}

func validateNodeSchema(node *tiptap.Node, path NodePath) error {
	switch node.Type {
	case "doc":
		if len(node.Content) == 0 {
			return fmt.Errorf("doc has no content")
		}
	case "paragraph", "blockquote", "tableHeader", "tableCell":
		// Content may be empty for placeholders, but children must still validate.
	case "heading":
		level, ok := intLikeAttr(node.Attrs, "level")
		if !ok || level < 1 || level > 6 {
			return fmt.Errorf("invalid heading level at %s", path.String())
		}
	case "bulletList", "orderedList", "taskList":
		for _, child := range node.Content {
			if child.Type != "listItem" && child.Type != "taskItem" {
				return fmt.Errorf("invalid list child %s at %s", child.Type, path.String())
			}
		}
	case "listItem", "taskItem", "callout", "aiSection":
		if len(node.Content) == 0 {
			return fmt.Errorf("%s has no content at %s", node.Type, path.String())
		}
	case "table":
		if len(node.Content) == 0 {
			return fmt.Errorf("table has no rows at %s", path.String())
		}
		for _, child := range node.Content {
			if child.Type != "tableRow" {
				return fmt.Errorf("invalid table child %s at %s", child.Type, path.String())
			}
		}
	case "tableRow":
		if len(node.Content) == 0 {
			return fmt.Errorf("table row has no cells at %s", path.String())
		}
		for _, child := range node.Content {
			if child.Type != "tableHeader" && child.Type != "tableCell" {
				return fmt.Errorf("invalid table row child %s at %s", child.Type, path.String())
			}
		}
	case "text":
		if len(node.Content) > 0 {
			return fmt.Errorf("text node has children at %s", path.String())
		}
		for _, mark := range node.Marks {
			if err := validateMark(mark, path); err != nil {
				return err
			}
		}
	case "codeBlock":
		if len(node.Content) > 1 {
			return fmt.Errorf("code block has too many children at %s", path.String())
		}
	case "resizableImage", "image":
		if src := strAttr(node.Attrs, "src"); src == "" {
			return fmt.Errorf("image src is required at %s", path.String())
		}
	case "htmlBlock":
		if _, ok := node.Attrs["html"].(string); !ok {
			return fmt.Errorf("htmlBlock html is required at %s", path.String())
		}
	case "videoEmbed":
		if strAttr(node.Attrs, "embedUrl") == "" {
			return fmt.Errorf("video embed url is required at %s", path.String())
		}
	case "entityEmbed":
		entityType := strAttr(node.Attrs, "entityType")
		switch entityType {
		case "task", "story", "epic", "support_conversation", "deal", "contact", "company":
		default:
			return fmt.Errorf("invalid entity embed type at %s", path.String())
		}
		if strAttr(node.Attrs, "entityId") == "" {
			return fmt.Errorf("entity embed id is required at %s", path.String())
		}
	case "citationBlock":
		if err := validateCitationBlock(node, path); err != nil {
			return err
		}
	case "horizontalRule", "hardBreak":
		if len(node.Content) > 0 {
			return fmt.Errorf("%s should not have children at %s", node.Type, path.String())
		}
	default:
		return fmt.Errorf("unsupported node type %q at %s", node.Type, path.String())
	}

	for i := range node.Content {
		if err := validateNodeSchema(&node.Content[i], path.Child(i)); err != nil {
			return err
		}
	}
	return nil
}

func validateCitationBlock(node *tiptap.Node, path NodePath) error {
	raw, ok := node.Attrs["sources"]
	if !ok || raw == nil {
		return nil
	}
	sources, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("citation sources must be an array at %s", path.String())
	}
	for i, item := range sources {
		source, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("citation source %d must be an object at %s", i, path.String())
		}
		sourceType := strAttr(source, "sourceType")
		switch sourceType {
		case "docs_chunk", "support_conversation":
		default:
			return fmt.Errorf("invalid citation source type at %s", path.String())
		}
		if strAttr(source, "sourceId") == "" {
			return fmt.Errorf("citation source id is required at %s", path.String())
		}
	}
	return nil
}

func validateMark(mark tiptap.Mark, path NodePath) error {
	switch mark.Type {
	case "bold", "strong", "italic", "em", "strike", "code", "underline", "subscript", "superscript", "highlight":
		return nil
	case "link":
		if href := strAttr(mark.Attrs, "href"); href == "" {
			return fmt.Errorf("link mark missing href at %s", path.String())
		}
		return nil
	default:
		return fmt.Errorf("unsupported mark type %q at %s", mark.Type, path.String())
	}
}

func intLikeAttr(attrs map[string]any, key string) (int, bool) {
	if attrs == nil {
		return 0, false
	}
	value, ok := attrs[key]
	if !ok || value == nil {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}
