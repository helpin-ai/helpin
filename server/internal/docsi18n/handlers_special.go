package docsi18n

import (
	"fmt"
	"reflect"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type preserveOnlyHandler struct {
	nodeType string
}

func (h preserveOnlyHandler) NodeType() string { return h.nodeType }

func (h preserveOnlyHandler) Extract(_ *tiptap.Node, _ NodePath, _ *ExtractContext) error { return nil }

func (h preserveOnlyHandler) Reinsert(_ *tiptap.Node, ref nodeSegmentRef, _ string) error {
	return fmt.Errorf("preserve-only node %s does not support translated field %s", h.nodeType, ref.FieldName)
}

func (h preserveOnlyHandler) Validate(source, translated *tiptap.Node, path NodePath) error {
	if !reflect.DeepEqual(source, translated) {
		return fmt.Errorf("preserve-only node changed at %s for %s", path.String(), source.Type)
	}
	return nil
}
