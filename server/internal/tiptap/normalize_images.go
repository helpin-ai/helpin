package tiptap

import "encoding/json"

// NormalizeImageNodes converts the standard image node name to the image node
// registered by Helpin's editor. Only content-tree nodes are visited; attributes,
// embedded JSON, text, and unknown fields are preserved. Validation remains the
// caller's responsibility, including malformed JSON and invalid image placement.
func NormalizeImageNodes(raw json.RawMessage) json.RawMessage {
	var node map[string]json.RawMessage
	if json.Unmarshal(raw, &node) != nil || node == nil {
		return raw
	}
	changed := false
	var nodeType string
	if json.Unmarshal(node["type"], &nodeType) == nil && nodeType == "image" {
		node["type"] = json.RawMessage(`"resizableImage"`)
		changed = true
	}
	var children []json.RawMessage
	if json.Unmarshal(node["content"], &children) == nil {
		for i, child := range children {
			next := NormalizeImageNodes(child)
			if string(next) != string(child) {
				children[i] = next
				changed = true
			}
		}
		if changed && children != nil {
			encoded, err := json.Marshal(children)
			if err != nil {
				return raw
			}
			node["content"] = encoded
		}
	}
	if !changed {
		return raw
	}
	encoded, err := json.Marshal(node)
	if err != nil {
		return raw
	}
	return encoded
}
