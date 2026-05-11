package service

import (
	"encoding/json"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

const publicRedactedEntityLabel = "Restricted reference"

// RedactPublicDocsContent returns a copy of document content that is safe for
// anonymous/public readers. Entity references are workspace-private even when a
// document is externally shared.
func RedactPublicDocsContent(content *model.DocsContent) *model.DocsContent {
	if content == nil {
		return nil
	}

	redacted := *content
	redacted.Content = RedactPublicDocsJSON(content.Content)
	redacted.ContentText = ExtractPublicDocsText(redacted.Content)
	redacted.WordCount = len(strings.Fields(redacted.ContentText))
	return &redacted
}

func RedactPublicDocsJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}

	out, err := json.Marshal(redactPublicDocsValue(value))
	if err != nil {
		return raw
	}
	return out
}

func RenderPublicDocsHTML(raw json.RawMessage) (string, error) {
	return tiptap.RenderHTML(RedactPublicDocsJSON(raw))
}

func ExtractPublicDocsText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}

	var parts []string
	collectPublicDocsText(value, &parts)
	return strings.Join(parts, " ")
}

func redactPublicDocsValue(value any) any {
	switch current := value.(type) {
	case map[string]any:
		nodeType, _ := current["type"].(string)
		switch nodeType {
		case "entityMention":
			current["attrs"] = redactedPublicEntityAttrs(current["attrs"], true)
			return current
		case "entityEmbed":
			current["attrs"] = redactedPublicEntityAttrs(current["attrs"], false)
			return current
		}
		for key, child := range current {
			current[key] = redactPublicDocsValue(child)
		}
	case []any:
		for i, child := range current {
			current[i] = redactPublicDocsValue(child)
		}
	}
	return value
}

func redactedPublicEntityAttrs(_ any, inline bool) map[string]any {
	attrs := map[string]any{
		"entityType": "reference",
		"entityId":   nil,
		"displayId":  nil,
		"href":       nil,
		"access":     "redacted",
	}
	if inline {
		attrs["label"] = publicRedactedEntityLabel
		return attrs
	}

	attrs["title"] = publicRedactedEntityLabel
	attrs["status"] = nil
	return attrs
}

func collectPublicDocsText(value any, parts *[]string) {
	switch current := value.(type) {
	case map[string]any:
		switch current["type"] {
		case "entityMention", "entityEmbed":
			*parts = append(*parts, publicRedactedEntityLabel)
			return
		}
		if text, _ := current["text"].(string); text != "" {
			*parts = append(*parts, text)
		}
		if content, ok := current["content"]; ok {
			collectPublicDocsText(content, parts)
		}
	case []any:
		for _, child := range current {
			collectPublicDocsText(child, parts)
		}
	}
}
