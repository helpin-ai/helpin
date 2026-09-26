package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const _mcpBatchReadMaxDocuments = 50

type mcpDocsContentReader interface {
	GetByDocumentID(context.Context, string) (*model.DocsContent, error)
}

type mcpDocsPublishStateEnricher interface {
	EnrichDocumentPublishState(context.Context, *model.DocsDocument) error
}

// SetDocsContentReader wires document content lookups for batch summaries.
func (s *MCPService) SetDocsContentReader(reader mcpDocsContentReader) {
	if reader != nil {
		s.docsContent = reader
	}
}

func docsBatchMCPToolDefinitions() []MCPToolDefinition {
	return []MCPToolDefinition{{
		Name:  "read_documents",
		Title: "Read document summaries",
		Description: "Summarize up to 50 documents in one call: title, status, word count, whether the " +
			"body is empty, and Help Center state (live, live slug, unpublished changes). " +
			"Inaccessible or unknown IDs are listed in not_found. Use read_document for content.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"document_ids": map[string]any{
					"type": "array", "minItems": 1, "maxItems": _mcpBatchReadMaxDocuments, "uniqueItems": true,
					"items": map[string]any{"type": "string", "minLength": 1},
				},
			},
			"required":             []string{"document_ids"},
			"additionalProperties": false,
		},
		Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs,
	}}
}

func (s *MCPService) executeDocsBatchMCPTool(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	name string,
	arguments json.RawMessage,
) (*MCPToolResult, bool, error) {
	if name != "read_documents" {
		return nil, false, nil
	}
	var input struct {
		DocumentIDs []string `json:"document_ids"`
	}
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, true, err
	}
	if len(input.DocumentIDs) == 0 || len(input.DocumentIDs) > _mcpBatchReadMaxDocuments {
		return nil, true, ErrMCPInvalidArguments
	}
	items := make([]map[string]any, 0, len(input.DocumentIDs))
	var notFound []string
	for _, rawID := range input.DocumentIDs {
		documentID := strings.TrimSpace(rawID)
		document, err := s.accessibleMCPDocument(ctx, principal, actor, documentID)
		if err != nil {
			return nil, true, err
		}
		if document == nil {
			notFound = append(notFound, documentID)
			continue
		}
		item, err := s.mcpDocumentSummary(ctx, document)
		if err != nil {
			return nil, true, err
		}
		items = append(items, item)
	}
	return &MCPToolResult{
		Summary: "Summarized " + strconv.Itoa(len(items)) + " documents.",
		Data:    map[string]any{"items": items, "not_found": notFound},
	}, true, nil
}

func (s *MCPService) mcpDocumentSummary(ctx context.Context, document *model.DocsDocument) (map[string]any, error) {
	summary := map[string]any{
		"document_id":   document.ID,
		"title":         document.Title,
		"status":        document.Status,
		"space_id":      document.SpaceID,
		"collection_id": document.CollectionID,
		"updated_at":    document.UpdatedAt,
		"markdown_link": "[" + document.Title + "](helpin://documents/" + document.ID + ")",
	}
	if s.docsContent != nil {
		content, err := s.docsContent.GetByDocumentID(ctx, document.ID)
		if err != nil {
			return nil, err
		}
		words := 0
		if content != nil {
			words = content.WordCount
		}
		summary["word_count"] = words
		summary["is_empty"] = words == 0
	}
	if enricher, ok := s.helpcenter.(mcpDocsPublishStateEnricher); ok {
		enriched := *document
		if err := enricher.EnrichDocumentPublishState(ctx, &enriched); err != nil {
			return nil, err
		}
		summary["help_center_live"] = enriched.LivePublishedAt != nil
		summary["has_unpublished_changes"] = enriched.HasUnpublishedChanges
		if enriched.LiveSlug != nil {
			summary["live_slug"] = *enriched.LiveSlug
		}
	}
	return summary, nil
}
