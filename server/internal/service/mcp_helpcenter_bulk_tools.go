package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	// _mcpPublishBatchMaxDocuments bounds one publish_documents call.
	_mcpPublishBatchMaxDocuments = 50
	// _mcpPublishBatchBudget is the working budget of one publish_documents
	// call, kept under the 30 second synchronous tool deadline.
	_mcpPublishBatchBudget = 25 * time.Second
	// _mcpPublishBatchReserve is left before the caller's deadline so the
	// result can still be encoded and returned.
	_mcpPublishBatchReserve = 3 * time.Second
)

// Per-document publish_documents result statuses.
const (
	mcpBatchPublishStatusPublished = "published"
	mcpBatchPublishStatusSkipped   = "skipped"
	mcpBatchPublishStatusError     = "error"
)

type mcpHelpcenterCollectionSlugUpdater interface {
	UpdateCollectionSlug(context.Context, string, string) (*model.DocsCollection, error)
}

func helpcenterBulkMCPToolDefinitions() []MCPToolDefinition {
	object := func(properties map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	return []MCPToolDefinition{
		{
			Name:  "publish_documents",
			Title: "Publish documents",
			Description: "Publish up to 50 documents in one call, with the same behavior as publish_document for each. " +
				"Set only_if_changed to skip documents whose live Help Center article has no unpublished changes. " +
				"Continues past per-document failures and returns one result per document. " +
				"Documents not reached before the time budget are listed in remaining_document_ids; call again with them.",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{
				"document_ids": map[string]any{
					"type": "array", "minItems": 1, "maxItems": _mcpPublishBatchMaxDocuments, "uniqueItems": true,
					"items": map[string]any{"type": "string", "minLength": 1},
				},
				"only_if_changed": map[string]any{
					"type": "boolean", "default": false,
					"description": "Skip documents that are already published and have no unpublished changes.",
				},
			}, "document_ids")),
			Toolset: MCPToolsetDocs, Scope: MCPScopeDocsPublish, Permission: authorization.PermDocsPublish,
			Module: model.ModuleDocs, Mutating: true, IdempotentHint: true,
		},
		{
			Name:  "update_help_center_collection_slug",
			Title: "Update Help Center collection slug",
			Description: "Change a collection's public URL slug. Old collection and article paths are redirected " +
				"to the new slug automatically. Changes public output.",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{
				"collection_id": map[string]any{"type": "string", "minLength": 1},
				"slug": map[string]any{
					"type": "string", "minLength": 1, "maxLength": 200,
					"description": "New slug, such as getting-started. It is lowercased and non-alphanumeric characters become hyphens.",
				},
			}, "collection_id", "slug")),
			Toolset: MCPToolsetDocs, Scope: MCPScopeDocsPublish, Permission: authorization.PermDocsAdmin,
			Module: model.ModuleDocs, Mutating: true, IdempotentHint: true,
		},
	}
}

func (s *MCPService) executeHelpcenterBulkMCPTool(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	name string,
	arguments json.RawMessage,
) (*MCPToolResult, bool, error) {
	switch name {
	case "publish_documents":
		result, err := s.publishMCPDocuments(ctx, principal, actor, arguments)
		return result, true, err
	case "update_help_center_collection_slug":
		result, err := s.updateMCPHelpcenterCollectionSlug(ctx, principal, actor, arguments)
		return result, true, err
	default:
		return nil, false, nil
	}
}

func (s *MCPService) publishMCPDocuments(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	arguments json.RawMessage,
) (*MCPToolResult, error) {
	if s.docsLifecycle == nil {
		return nil, ErrMCPForbidden
	}
	var input struct {
		DocumentIDs   []string `json:"document_ids"`
		OnlyIfChanged bool     `json:"only_if_changed"`
	}
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(input.DocumentIDs))
	seen := make(map[string]bool, len(input.DocumentIDs))
	for _, id := range input.DocumentIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: document_ids must contain at least one id", ErrMCPInvalidArguments)
	}
	if len(ids) > _mcpPublishBatchMaxDocuments {
		return nil, fmt.Errorf("%w: at most %d document_ids per call", ErrMCPInvalidArguments, _mcpPublishBatchMaxDocuments)
	}

	stopAt := time.Now().Add(_mcpPublishBatchBudget)
	if deadline, ok := ctx.Deadline(); ok && deadline.Add(-_mcpPublishBatchReserve).Before(stopAt) {
		stopAt = deadline.Add(-_mcpPublishBatchReserve)
	}
	results := make([]map[string]any, 0, len(ids))
	remaining := []string{}
	counts := map[string]int{}
	for index, id := range ids {
		if ctx.Err() != nil || !time.Now().Before(stopAt) {
			remaining = append(remaining, ids[index:]...)
			break
		}
		entry := s.publishMCPBatchDocument(ctx, principal, actor, id, input.OnlyIfChanged)
		counts[entry["status"].(string)]++
		results = append(results, entry)
	}

	summary := fmt.Sprintf("Published %d, skipped %d, failed %d of %d documents.",
		counts[mcpBatchPublishStatusPublished], counts[mcpBatchPublishStatusSkipped], counts[mcpBatchPublishStatusError], len(ids))
	if len(remaining) > 0 {
		summary += fmt.Sprintf(" %d documents were not attempted before the time budget; call again with remaining_document_ids.", len(remaining))
	}
	return &MCPToolResult{
		Summary: summary,
		Data: map[string]any{
			"results":                results,
			"published":              counts[mcpBatchPublishStatusPublished],
			"skipped":                counts[mcpBatchPublishStatusSkipped],
			"failed":                 counts[mcpBatchPublishStatusError],
			"complete":               len(remaining) == 0,
			"remaining_document_ids": remaining,
		},
	}, nil
}

// publishMCPBatchDocument publishes one document for publish_documents and
// never returns an error: failures become a per-document error entry.
func (s *MCPService) publishMCPBatchDocument(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	documentID string,
	onlyIfChanged bool,
) map[string]any {
	entry := map[string]any{"document_id": documentID}
	fail := func(err error) map[string]any {
		entry["status"] = mcpBatchPublishStatusError
		entry["code"], entry["message"] = mcpBatchErrorCode(err)
		if entry["code"] == "PUBLISH_FAILED" {
			slog.WarnContext(ctx, "MCP batch publish failed", "document_id", documentID, "error", err)
		}
		return entry
	}
	document, err := s.accessibleMCPDocument(ctx, principal, actor, documentID)
	if err != nil {
		return fail(err)
	}
	if document == nil {
		return fail(ErrMCPNotFound)
	}
	if document.IsLocked {
		return fail(newMCPToolError(MCPErrorCodeDocumentLocked, "The document is locked. Unlock it in Helpin first."))
	}
	if onlyIfChanged {
		unchanged, liveSlug, err := s.mcpDocumentPublishedAndUnchanged(ctx, principal, actor, document)
		if err != nil {
			return fail(err)
		}
		if unchanged {
			entry["status"] = mcpBatchPublishStatusSkipped
			entry["code"] = "NO_UNPUBLISHED_CHANGES"
			if liveSlug != "" {
				entry["live_slug"] = liveSlug
			}
			return entry
		}
	}
	result, err := s.publishMCPDocument(ctx, principal, actor, document, "")
	if err != nil {
		return fail(err)
	}
	entry["status"] = mcpBatchPublishStatusPublished
	if data, ok := result.Data.(map[string]any); ok {
		entry["help_center"] = data["help_center"]
		if slug, ok := data["live_slug"]; ok {
			entry["live_slug"] = slug
		}
	}
	return entry
}

// mcpDocumentPublishedAndUnchanged reports whether publishing the document
// again would change nothing: it is published and, in a Help Center space,
// its live snapshot has no unpublished changes.
func (s *MCPService) mcpDocumentPublishedAndUnchanged(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	document *model.DocsDocument,
) (bool, string, error) {
	if document.Status != model.DocStatusPublished {
		return false, "", nil
	}
	space, err := s.accessibleMCPDocsSpace(ctx, principal, actor, document.SpaceID)
	if err != nil {
		return false, "", err
	}
	if space == nil {
		return false, "", ErrMCPNotFound
	}
	if space.Type != model.SpaceTypeExternalCapable || s.helpcenter == nil {
		return true, "", nil
	}
	admin, ok := s.helpcenter.(mcpHelpcenterAdmin)
	if !ok {
		return false, "", nil
	}
	enriched := *document
	if err := admin.EnrichDocumentPublishState(ctx, &enriched); err != nil {
		return false, "", err
	}
	if enriched.LivePublishedAt == nil || enriched.HasUnpublishedChanges {
		return false, "", nil
	}
	liveSlug := ""
	if enriched.LiveSlug != nil {
		liveSlug = *enriched.LiveSlug
	}
	return true, liveSlug, nil
}

// mcpBatchErrorCode maps an error to a stable code and a client-safe message
// for per-item batch results.
func mcpBatchErrorCode(err error) (string, string) {
	var toolErr *MCPToolError
	switch {
	case errors.As(err, &toolErr):
		return toolErr.Code, toolErr.Message
	case errors.Is(err, ErrMCPNotFound):
		return "NOT_FOUND", "The document was not found or is not accessible."
	case errors.Is(err, ErrMCPForbidden):
		return "FORBIDDEN", "The operation is not allowed."
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "TIMEOUT", "The operation did not finish in time; retry this document."
	default:
		return "PUBLISH_FAILED", "The document could not be published."
	}
}

func (s *MCPService) updateMCPHelpcenterCollectionSlug(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	arguments json.RawMessage,
) (*MCPToolResult, error) {
	updater, ok := s.helpcenter.(mcpHelpcenterCollectionSlugUpdater)
	if !ok || s.collections == nil {
		return nil, ErrMCPForbidden
	}
	var input struct {
		CollectionID string `json:"collection_id"`
		Slug         string `json:"slug"`
	}
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, err
	}
	slug := normalizeSlug(input.Slug)
	if slug == "" {
		return nil, fmt.Errorf("%w: slug must contain letters or digits", ErrMCPInvalidArguments)
	}
	collection, err := s.accessibleMCPDocsCollection(ctx, principal, actor, strings.TrimSpace(input.CollectionID))
	if err != nil {
		return nil, err
	}
	if collection == nil {
		return nil, ErrMCPNotFound
	}
	oldSlug := collection.Slug
	updated, err := updater.UpdateCollectionSlug(ctx, collection.ID, slug)
	if err != nil {
		if errors.Is(err, ErrDocsCollectionNotFound) {
			return nil, ErrMCPNotFound
		}
		return nil, err
	}
	data := map[string]any{
		"collection_id": updated.ID,
		"name":          updated.Name,
		"old_slug":      oldSlug,
		"slug":          updated.Slug,
		"changed":       oldSlug != updated.Slug,
	}
	summary := "Collection " + updated.Name + " already uses slug " + updated.Slug + "."
	if oldSlug != updated.Slug {
		summary = "Collection " + updated.Name + " slug changed to " + updated.Slug + "."
		if oldSlug != "" {
			data["redirected_from"] = buildDocsRedirectPath(oldSlug, nil)
			summary += " Old paths under /" + oldSlug + " redirect to the new slug."
		}
	}
	return &MCPToolResult{Summary: summary, Data: data}, nil
}
