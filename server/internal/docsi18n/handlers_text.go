package docsi18n

import (
	"fmt"
	"reflect"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type containerHandler struct {
	nodeType string
}

func newContainerHandler(nodeType string) containerHandler {
	return containerHandler{nodeType: nodeType}
}

func (h containerHandler) NodeType() string { return h.nodeType }

func (h containerHandler) Extract(node *tiptap.Node, path NodePath, ctx *ExtractContext) error {
	return ctx.visitChildren(node, path)
}

func (h containerHandler) Reinsert(_ *tiptap.Node, _ nodeSegmentRef, _ string) error { return nil }

func (h containerHandler) Validate(source, translated *tiptap.Node, path NodePath) error {
	if !reflect.DeepEqual(source.Attrs, translated.Attrs) {
		return fmt.Errorf("attrs changed at %s for node %s", path.String(), source.Type)
	}
	return nil
}

type textHandler struct{}

func (textHandler) NodeType() string { return "text" }

func (textHandler) Extract(node *tiptap.Node, path NodePath, ctx *ExtractContext) error {
	if hasCodeMark(node) || node.Text == "" {
		return nil
	}
	ctx.addNodeSegment(path, node.Type, "text", node.Text, "text run")
	return nil
}

func (textHandler) Reinsert(node *tiptap.Node, ref nodeSegmentRef, translatedText string) error {
	if ref.FieldName != "text" {
		return fmt.Errorf("unsupported text field %q", ref.FieldName)
	}
	node.Text = translatedText
	return nil
}

func (textHandler) Validate(source, translated *tiptap.Node, path NodePath) error {
	if !reflect.DeepEqual(source.Marks, translated.Marks) {
		return fmt.Errorf("marks changed at %s", path.String())
	}
	if hasCodeMark(source) && source.Text != translated.Text {
		return fmt.Errorf("code-mark text changed at %s", path.String())
	}
	return nil
}

func hasCodeMark(node *tiptap.Node) bool {
	for _, mark := range node.Marks {
		if mark.Type == "code" {
			return true
		}
	}
	return false
}
