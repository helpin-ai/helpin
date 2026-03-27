package docsi18n

import (
	"fmt"
	"reflect"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type imageHandler struct {
	nodeType string
}

func (h imageHandler) NodeType() string { return h.nodeType }

func (h imageHandler) Extract(node *tiptap.Node, path NodePath, ctx *ExtractContext) error {
	if alt := strAttr(node.Attrs, "alt"); alt != "" {
		ctx.addNodeSegment(path, node.Type, "alt", alt, "image alt")
	}
	if title := strAttr(node.Attrs, "title"); title != "" {
		ctx.addNodeSegment(path, node.Type, "title", title, "image title")
	}
	return nil
}

func (h imageHandler) Reinsert(node *tiptap.Node, ref nodeSegmentRef, translatedText string) error {
	switch ref.FieldName {
	case "alt", "title":
		if node.Attrs == nil {
			node.Attrs = map[string]any{}
		}
		node.Attrs[ref.FieldName] = translatedText
		return nil
	default:
		return fmt.Errorf("unsupported image field %q", ref.FieldName)
	}
}

func (h imageHandler) Validate(source, translated *tiptap.Node, path NodePath) error {
	sourceAttrs := cloneMap(source.Attrs)
	translatedAttrs := cloneMap(translated.Attrs)
	delete(sourceAttrs, "alt")
	delete(sourceAttrs, "title")
	delete(translatedAttrs, "alt")
	delete(translatedAttrs, "title")
	if !reflect.DeepEqual(sourceAttrs, translatedAttrs) {
		return fmt.Errorf("non-translatable image attrs changed at %s", path.String())
	}
	return nil
}
