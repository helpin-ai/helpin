package service

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/docsimport"
)

func (s *DocsContentService) restoreImportedToggleAttrs(ctx context.Context, documentID string, content json.RawMessage) json.RawMessage {
	if s == nil || s.contentRepo == nil || !bytes.Contains(content, []byte(`"toggleSection"`)) {
		return content
	}

	existing, err := s.contentRepo.GetByDocumentID(ctx, documentID)
	if err != nil {
		slog.WarnContext(ctx, "failed to load docs import source before restoring toggle metadata", "document_id", documentID, "error", err)
		return content
	}
	if existing == nil || existing.ImportSourceHTML == nil || *existing.ImportSourceHTML == "" {
		return content
	}

	restored, changed, err := restoreImportedToggleAttrsFromSource(content, *existing.ImportSourceHTML)
	if err != nil {
		slog.WarnContext(ctx, "failed to restore imported toggle metadata", "document_id", documentID, "error", err)
		return content
	}
	if !changed {
		return content
	}
	return restored
}

func restoreImportedToggleAttrsFromSource(content json.RawMessage, sourceHTML string) (json.RawMessage, bool, error) {
	sourceAttrs, err := importedToggleAttrsByTitle(sourceHTML)
	if err != nil || len(sourceAttrs) == 0 {
		return content, false, err
	}

	var doc map[string]any
	if err := json.Unmarshal(content, &doc); err != nil {
		return content, false, err
	}

	changed := restoreToggleAttrsInNode(doc, sourceAttrs)
	if !changed {
		return content, false, nil
	}

	restored, err := json.Marshal(doc)
	if err != nil {
		return content, false, err
	}
	return restored, true, nil
}

func importedToggleAttrsByTitle(sourceHTML string) (map[string]map[string]any, error) {
	normalized, _ := docsimport.PreprocessHelpScoutHTML(sourceHTML)
	converted, err := docsimport.ConvertHTML(normalized)
	if err != nil {
		return nil, err
	}

	result := map[string]map[string]any{}
	var walk func(docsimport.Node)
	walk = func(node docsimport.Node) {
		if node.Type == "toggleSection" {
			title, _ := node.Attrs["title"].(string)
			if title != "" && node.Attrs["sourceStyle"] == "helpScoutCard" {
				result[title] = node.Attrs
			}
		}
		for _, child := range node.Content {
			walk(child)
		}
	}
	walk(converted.Doc)
	return result, nil
}

func restoreToggleAttrsInNode(node map[string]any, sourceAttrs map[string]map[string]any) bool {
	changed := false
	if node["type"] == "toggleSection" {
		attrs, _ := node["attrs"].(map[string]any)
		if attrs == nil {
			attrs = map[string]any{}
			node["attrs"] = attrs
		}
		title, _ := attrs["title"].(string)
		if title != "" {
			if source, ok := sourceAttrs[title]; ok {
				for _, key := range []string{"icon", "badgeText", "sourceStyle"} {
					if attrIsEmpty(attrs[key]) && !attrIsEmpty(source[key]) {
						attrs[key] = source[key]
						changed = true
					}
				}
			}
		}
	}

	children, _ := node["content"].([]any)
	for _, child := range children {
		childNode, ok := child.(map[string]any)
		if !ok {
			continue
		}
		if restoreToggleAttrsInNode(childNode, sourceAttrs) {
			changed = true
		}
	}
	return changed
}

func attrIsEmpty(value any) bool {
	if value == nil {
		return true
	}
	if s, ok := value.(string); ok {
		return s == ""
	}
	return false
}
