package docsimport

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// LinkRewrite decides what happens to one link address. It returns the
// address to store and whether the link changed.
type LinkRewrite func(href string) (replacement string, changed bool)

// RewriteTiptapLinks walks a stored Tiptap document and passes every link
// address to rewrite: link marks on text and the link an image opens. It
// returns the document, re-encoded only when something changed, with the
// number of links it changed. Fields the walker does not know are kept.
func RewriteTiptapLinks(raw json.RawMessage, rewrite LinkRewrite) (json.RawMessage, int, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	// Keep numbers as written so attributes such as widths survive the
	// round trip unchanged.
	decoder.UseNumber()
	var doc any
	if err := decoder.Decode(&doc); err != nil {
		return nil, 0, fmt.Errorf("decode document: %w", err)
	}
	changed := rewriteTiptapNodeLinks(doc, rewrite, 0)
	if changed == 0 {
		return raw, 0, nil
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		return nil, 0, fmt.Errorf("encode document: %w", err)
	}
	return encoded, changed, nil
}

// tiptapLinkMaxDepth bounds the walk on malformed, self-nesting documents.
const tiptapLinkMaxDepth = 64

func rewriteTiptapNodeLinks(value any, rewrite LinkRewrite, depth int) int {
	node, ok := value.(map[string]any)
	if !ok || depth > tiptapLinkMaxDepth {
		return 0
	}
	changed := 0
	if marks, ok := node["marks"].([]any); ok {
		for _, entry := range marks {
			mark, ok := entry.(map[string]any)
			if !ok || mark["type"] != "link" {
				continue
			}
			if attrs, ok := mark["attrs"].(map[string]any); ok && rewriteLinkAttr(attrs, "href", rewrite) {
				// A link that now stays inside the help center opens in place.
				delete(attrs, "target")
				delete(attrs, "rel")
				changed++
			}
		}
	}
	if node["type"] == "resizableImage" || node["type"] == "image" {
		if attrs, ok := node["attrs"].(map[string]any); ok && rewriteLinkAttr(attrs, "linkUrl", rewrite) {
			attrs["linkNewTab"] = false
			changed++
		}
	}
	if children, ok := node["content"].([]any); ok {
		for _, child := range children {
			changed += rewriteTiptapNodeLinks(child, rewrite, depth+1)
		}
	}
	return changed
}

func rewriteLinkAttr(attrs map[string]any, key string, rewrite LinkRewrite) bool {
	href, ok := attrs[key].(string)
	if !ok || href == "" {
		return false
	}
	replacement, changed := rewrite(href)
	if !changed || replacement == href {
		return false
	}
	attrs[key] = replacement
	return true
}
