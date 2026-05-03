package docsi18n

import (
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type NodeTranslationHandler interface {
	NodeType() string
	Extract(node *tiptap.Node, path NodePath, ctx *ExtractContext) error
	Reinsert(node *tiptap.Node, ref nodeSegmentRef, translatedText string) error
	Validate(source, translated *tiptap.Node, path NodePath) error
}

type Registry struct {
	handlers map[string]NodeTranslationHandler
}

func NewRegistry(handlers ...NodeTranslationHandler) *Registry {
	registry := &Registry{handlers: make(map[string]NodeTranslationHandler, len(handlers))}
	for _, handler := range handlers {
		registry.handlers[handler.NodeType()] = handler
	}
	return registry
}

func DefaultRegistry() *Registry {
	return NewRegistry(
		newContainerHandler("doc"),
		newContainerHandler("paragraph"),
		newContainerHandler("heading"),
		newContainerHandler("bulletList"),
		newContainerHandler("orderedList"),
		newContainerHandler("listItem"),
		newContainerHandler("blockquote"),
		newContainerHandler("callout"),
		newContainerHandler("aiSection"),
		newContainerHandler("table"),
		newContainerHandler("tableRow"),
		newContainerHandler("tableHeader"),
		newContainerHandler("tableCell"),
		newContainerHandler("taskList"),
		newContainerHandler("taskItem"),
		textHandler{},
		imageHandler{nodeType: "image"},
		imageHandler{nodeType: "resizableImage"},
		preserveOnlyHandler{nodeType: "codeBlock"},
		preserveOnlyHandler{nodeType: "htmlBlock"},
		preserveOnlyHandler{nodeType: "videoEmbed"},
		preserveOnlyHandler{nodeType: "entityEmbed"},
		preserveOnlyHandler{nodeType: "citationBlock"},
		preserveOnlyHandler{nodeType: "horizontalRule"},
		preserveOnlyHandler{nodeType: "hardBreak"},
	)
}

func (r *Registry) Handler(nodeType string) (NodeTranslationHandler, error) {
	handler, ok := r.handlers[nodeType]
	if !ok {
		return nil, fmt.Errorf("unsupported node type %q", nodeType)
	}
	return handler, nil
}
