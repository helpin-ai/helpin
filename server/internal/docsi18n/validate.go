package docsi18n

import (
	"fmt"
	"reflect"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func validatePreservation(source, translated *tiptap.Node, path NodePath, registry *Registry) error {
	if source.Type != translated.Type {
		return fmt.Errorf("node type changed at %s: %s -> %s", path.String(), source.Type, translated.Type)
	}

	handler, err := registry.Handler(source.Type)
	if err != nil {
		return err
	}
	if err := handler.Validate(source, translated, path); err != nil {
		return err
	}

	if len(source.Content) != len(translated.Content) {
		return fmt.Errorf("child count changed at %s for %s", path.String(), source.Type)
	}

	for i := range source.Content {
		if err := validatePreservation(&source.Content[i], &translated.Content[i], path.Child(i), registry); err != nil {
			return err
		}
	}
	return nil
}

func cloneNode(node tiptap.Node) tiptap.Node {
	return tiptap.Node{
		Type:    node.Type,
		Attrs:   cloneMap(node.Attrs),
		Content: cloneChildren(node.Content),
		Marks:   cloneMarks(node.Marks),
		Text:    node.Text,
	}
}

func cloneChildren(children []tiptap.Node) []tiptap.Node {
	if len(children) == 0 {
		return nil
	}
	cloned := make([]tiptap.Node, len(children))
	for i, child := range children {
		cloned[i] = cloneNode(child)
	}
	return cloned
}

func cloneMarks(marks []tiptap.Mark) []tiptap.Mark {
	if len(marks) == 0 {
		return nil
	}
	cloned := make([]tiptap.Mark, len(marks))
	for i, mark := range marks {
		cloned[i] = tiptap.Mark{
			Type:  mark.Type,
			Attrs: cloneMap(mark.Attrs),
		}
	}
	return cloned
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneAny(value)
	}
	return out
}

func cloneAny(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneMap(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = cloneAny(item)
		}
		return out
	case []string:
		return append([]string(nil), v...)
	default:
		return v
	}
}

func nodeAtPath(root *tiptap.Node, path NodePath) (*tiptap.Node, error) {
	current := root
	for _, index := range path {
		if index < 0 || index >= len(current.Content) {
			return nil, fmt.Errorf("path %s is out of bounds", path.String())
		}
		current = &current.Content[index]
	}
	return current, nil
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func stringPtr(value string) *string {
	return &value
}

func strAttr(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	value, ok := attrs[key]
	if !ok || value == nil {
		return ""
	}
	s, _ := value.(string)
	return s
}

func attrsEqual(source, translated map[string]any) bool {
	return reflect.DeepEqual(source, translated)
}
