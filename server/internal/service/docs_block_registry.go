package service

import (
	"encoding/json"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type docsBlockKind string

const (
	docsBlockKindAISection   docsBlockKind = "aiSection"
	docsBlockKindCitation    docsBlockKind = "citationBlock"
	docsBlockKindEntityEmbed docsBlockKind = "entityEmbed"
	docsBlockKindSavedView   docsBlockKind = "savedViewEmbed"
	docsBlockKindToggle      docsBlockKind = "toggleSection"
	docsBlockKindFile        docsBlockKind = "fileAttachment"
	docsBlockKindTOC         docsBlockKind = "tableOfContents"
	docsBlockKindRichEmbed   docsBlockKind = "richEmbed"
	docsBlockKindCallout     docsBlockKind = "callout"
	docsBlockKindImage       docsBlockKind = "resizableImage"
	docsBlockKindVideo       docsBlockKind = "videoEmbed"
	docsBlockKindTable       docsBlockKind = "table"
	docsBlockKindCode        docsBlockKind = "codeBlock"
	docsBlockKindHTML        docsBlockKind = "htmlBlock"
)

type docsBlockDefinition struct {
	Kind               docsBlockKind
	Label              string
	AgentReadableKind  string
	Attrs              []string
	PermissionBehavior string
	AuditBehavior      string
	Actions            []model.DocsBlockAgentAction
}

var docsBlockRegistry = map[docsBlockKind]docsBlockDefinition{
	docsBlockKindAISection: {
		Kind:              docsBlockKindAISection,
		Label:             "Legacy AI section",
		AgentReadableKind: "section",
		Attrs:             []string{"status", "ownerAgentId", "lastGeneratedAt", "sourceCount"},
		AuditBehavior:     "legacy_section",
	},
	docsBlockKindCitation: {
		Kind:               docsBlockKindCitation,
		Label:              "Citation",
		AgentReadableKind:  "citation",
		Attrs:              []string{"title", "sourceType", "sourceId", "confidence", "access"},
		PermissionBehavior: "source_ref",
		AuditBehavior:      "source_linked",
	},
	docsBlockKindEntityEmbed: {
		Kind:               docsBlockKindEntityEmbed,
		Label:              "Entity embed",
		AgentReadableKind:  "entity_embed",
		Attrs:              []string{"entityType", "entityId", "title", "status", "access"},
		PermissionBehavior: "entity_permission",
		AuditBehavior:      "entity_linked",
	},
	docsBlockKindSavedView: {
		Kind:               docsBlockKindSavedView,
		Label:              "Saved view",
		AgentReadableKind:  "saved_view_embed",
		Attrs:              []string{"module", "viewId", "viewName"},
		PermissionBehavior: "view_permission",
		AuditBehavior:      "view_linked",
	},
	docsBlockKindToggle: {
		Kind:              docsBlockKindToggle,
		Label:             "Toggle",
		AgentReadableKind: "toggle_section",
		Attrs:             []string{"title", "open"},
	},
	docsBlockKindFile: {
		Kind:              docsBlockKindFile,
		Label:             "File",
		AgentReadableKind: "file_attachment",
		Attrs:             []string{"fileName", "fileSize", "contentType", "url", "attachmentId"},
	},
	docsBlockKindTOC: {
		Kind:              docsBlockKindTOC,
		Label:             "Table of contents",
		AgentReadableKind: "table_of_contents",
		Attrs:             []string{},
	},
	docsBlockKindRichEmbed: {
		Kind:              docsBlockKindRichEmbed,
		Label:             "Embed",
		AgentReadableKind: "rich_embed",
		Attrs:             []string{"url", "provider", "title", "description", "image_url"},
	},
	docsBlockKindCallout: {
		Kind:              docsBlockKindCallout,
		Label:             "Callout",
		AgentReadableKind: "callout",
		Attrs:             []string{"variant"},
	},
	docsBlockKindImage: {
		Kind:              docsBlockKindImage,
		Label:             "Image",
		AgentReadableKind: "image",
		Attrs:             []string{"src", "alt", "caption", "align"},
	},
	docsBlockKindVideo: {
		Kind:              docsBlockKindVideo,
		Label:             "Video",
		AgentReadableKind: "video",
		Attrs:             []string{"provider", "sourceUrl", "embedUrl", "title"},
	},
	docsBlockKindTable: {
		Kind:              docsBlockKindTable,
		Label:             "Table",
		AgentReadableKind: "table",
		Attrs:             []string{},
	},
	docsBlockKindCode: {
		Kind:              docsBlockKindCode,
		Label:             "Code block",
		AgentReadableKind: "code",
		Attrs:             []string{"language"},
	},
	docsBlockKindHTML: {
		Kind:              docsBlockKindHTML,
		Label:             "HTML",
		AgentReadableKind: "html",
		Attrs:             []string{"html"},
	},
}

func docsBlockDefinitionFor(kind string) (docsBlockDefinition, bool) {
	def, ok := docsBlockRegistry[docsBlockKind(kind)]
	return def, ok
}

func docsBlockAgentProjection(block model.DocsBlock) *model.DocsBlockAgentProjection {
	def, ok := docsBlockDefinitionFor(block.Type)
	if !ok {
		def = docsBlockDefinition{
			Kind:              docsBlockKind(block.Type),
			Label:             block.Type,
			AgentReadableKind: strings.TrimSpace(block.Type),
		}
	}

	attrs := attrsFromBlockContent(block.Content)
	projectedAttrs := make(map[string]interface{}, len(def.Attrs))
	for _, key := range def.Attrs {
		if value, ok := attrs[key]; ok {
			projectedAttrs[key] = value
		}
	}
	for _, key := range []string{"staleState", "staleReason", "staleSource", "staleGapId", "staleMarkedAt"} {
		if value, ok := attrs[key]; ok {
			projectedAttrs[key] = value
		}
	}

	projection := &model.DocsBlockAgentProjection{
		Kind:    def.AgentReadableKind,
		BlockID: block.ID,
		Text:    block.ContentText,
		Attrs:   projectedAttrs,
		Actions: append([]model.DocsBlockAgentAction(nil), def.Actions...),
	}

	switch def.Kind {
	case docsBlockKindEntityEmbed:
		access := normalizedAccess(stringAttrFromMap(attrs, "access"))
		ref := model.DocsBlockAgentRef{
			Type:     stringAttrFromMap(attrs, "entityType"),
			ID:       stringAttrFromMap(attrs, "entityId"),
			Title:    firstNonBlank(stringAttrFromMap(attrs, "title"), stringAttrFromMap(attrs, "label")),
			Access:   access,
			Redacted: access == "redacted",
		}
		if ref.Type != "" && ref.ID != "" {
			projection.EntityRefs = []model.DocsBlockAgentRef{ref}
		}
	case docsBlockKindCitation:
		projection.Citations = citationRefsFromAttrs(attrs)
	case docsBlockKindSavedView:
		ref := model.DocsBlockAgentRef{
			Type:   firstNonBlank(stringAttrFromMap(attrs, "module"), "pm") + "_view",
			ID:     stringAttrFromMap(attrs, "viewId"),
			Title:  firstNonBlank(stringAttrFromMap(attrs, "viewName"), stringAttrFromMap(attrs, "title")),
			Access: "unknown",
		}
		if ref.ID != "" {
			projection.EntityRefs = []model.DocsBlockAgentRef{ref}
		}
	}

	return projection
}

func citationRefsFromAttrs(attrs map[string]interface{}) []model.DocsBlockAgentRef {
	if rawSources, ok := attrs["sources"].([]interface{}); ok {
		refs := make([]model.DocsBlockAgentRef, 0, len(rawSources))
		for _, rawSource := range rawSources {
			source, ok := rawSource.(map[string]interface{})
			if !ok {
				continue
			}
			access := normalizedAccess(stringAttrFromMap(source, "access"))
			ref := model.DocsBlockAgentRef{
				Type:     stringAttrFromMap(source, "sourceType"),
				ID:       stringAttrFromMap(source, "sourceId"),
				Title:    firstNonBlank(stringAttrFromMap(source, "title"), stringAttrFromMap(source, "label")),
				Access:   access,
				Redacted: access == "redacted",
			}
			if ref.Type != "" && ref.ID != "" {
				refs = append(refs, ref)
			}
		}
		return refs
	}

	access := normalizedAccess(stringAttrFromMap(attrs, "access"))
	ref := model.DocsBlockAgentRef{
		Type:     stringAttrFromMap(attrs, "sourceType"),
		ID:       stringAttrFromMap(attrs, "sourceId"),
		Title:    firstNonBlank(stringAttrFromMap(attrs, "title"), stringAttrFromMap(attrs, "label")),
		Access:   access,
		Redacted: access == "redacted",
	}
	if ref.Type == "" || ref.ID == "" {
		return nil
	}
	return []model.DocsBlockAgentRef{ref}
}

func attrsFromBlockContent(raw json.RawMessage) map[string]interface{} {
	var node struct {
		Attrs map[string]interface{} `json:"attrs"`
	}
	if err := json.Unmarshal(raw, &node); err != nil || node.Attrs == nil {
		return map[string]interface{}{}
	}
	return node.Attrs
}

func stringAttrFromMap(attrs map[string]interface{}, key string) string {
	if attrs == nil {
		return ""
	}
	value, _ := attrs[key].(string)
	return strings.TrimSpace(value)
}

func normalizedAccess(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "granted", "redacted":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}
