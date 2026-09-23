package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// Typed public MCP error codes. Clients can branch on these stable values.
const (
	MCPErrorCodeDocumentLocked    = "DOCUMENT_LOCKED"
	MCPErrorCodeDocumentArchived  = "DOCUMENT_ARCHIVED"
	MCPErrorCodeDocumentPublished = "DOC_IS_PUBLISHED"
	MCPErrorCodeNotArchived       = "DOCUMENT_NOT_ARCHIVED"
	MCPErrorCodeNotPublished      = "DOCUMENT_NOT_PUBLISHED"
	MCPErrorCodeRunNotOwned       = "RUN_NOT_OWNED"
)

// MCPToolError is a public, client-actionable tool failure with a stable code.
type MCPToolError struct {
	Code    string
	Message string
}

// Error implements error.
func (e *MCPToolError) Error() string { return e.Code + ": " + e.Message }

func newMCPToolError(code, message string) error {
	return &MCPToolError{Code: code, Message: message}
}

type mcpDocsLifecycleService interface {
	Update(context.Context, string, model.UpdateDocsDocumentRequest) (*model.DocsDocument, error)
	Publish(context.Context, string) (*model.DocsDocument, error)
	Unpublish(context.Context, string) (*model.DocsDocument, error)
	Archive(context.Context, string) (*model.DocsDocument, error)
	Unarchive(context.Context, string) (*model.DocsDocument, error)
}

type mcpHelpcenterPublisher interface {
	PublishExternally(context.Context, string, string, json.RawMessage) error
	UnpublishExternally(context.Context, string) error
	GetArticle(context.Context, string) (*model.DocsHelpcenterArticle, error)
}

type mcpDocsEmbeddingQueue interface {
	QueueDocumentSync(context.Context, string) error
}

// SetDocsLifecycle wires document lifecycle, Help Center publishing, and
// embedding refresh dependencies used by the Docs lifecycle tools.
func (s *MCPService) SetDocsLifecycle(
	documents *DocsDocumentService,
	helpcenter *DocsHelpcenterService,
	embeddings *DocsEmbeddingService,
) {
	if documents != nil {
		s.docsLifecycle = documents
	}
	if helpcenter != nil {
		s.helpcenter = helpcenter
	}
	if embeddings != nil {
		s.docsEmbedding = embeddings
	}
}

func docsLifecycleMCPToolDefinitions() []MCPToolDefinition {
	object := func(properties map[string]any, required ...string) map[string]any {
		schema := map[string]any{
			"type":                 "object",
			"properties":           properties,
			"additionalProperties": false,
		}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	}
	documentID := map[string]any{"type": "string", "minLength": 1}
	return []MCPToolDefinition{
		{
			Name:  "update_document",
			Title: "Update document",
			Description: "Rename a document or update its excerpt and tags. " +
				"Use get_document_blocks and block tools to change body content.",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{
				"document_id": documentID,
				"title":       map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
				"excerpt":     map[string]any{"type": []string{"string", "null"}, "maxLength": 1000, "description": "Short summary; null clears it."},
				"tags":        map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 100}, "maxItems": 50, "uniqueItems": true},
			}, "document_id")),
			Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit,
			Module: model.ModuleDocs, Mutating: true, IdempotentHint: true,
		},
		{
			Name:  "publish_document",
			Title: "Publish document",
			Description: "Publish a document. In an external-capable (Help Center) space this also makes " +
				"it a live public article; ongoing edits stay private until you publish again. " +
				"Requires the helpin.docs.publish scope and the docs.publish permission.",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{
				"document_id": documentID,
				"slug":        map[string]any{"type": "string", "maxLength": 200, "description": "Public URL slug; omit to keep the current slug or derive one from the title."},
			}, "document_id")),
			Toolset: MCPToolsetDocs, Scope: MCPScopeDocsPublish, Permission: authorization.PermDocsPublish,
			Module: model.ModuleDocs, Mutating: true, IdempotentHint: true,
		},
		{
			Name:  "unpublish_document",
			Title: "Unpublish document",
			Description: "Remove a document from the public Help Center and return it to draft. " +
				"Content and history are kept.",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{"document_id": documentID}, "document_id")),
			Toolset:     MCPToolsetDocs, Scope: MCPScopeDocsPublish, Permission: authorization.PermDocsPublish,
			Module: model.ModuleDocs, Mutating: true, Destructive: true, IdempotentHint: true,
		},
		{
			Name:  "archive_document",
			Title: "Archive document",
			Description: "Archive a document so it leaves default lists. Reversible with restore_document. " +
				"A live Help Center article must be unpublished first.",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{"document_id": documentID}, "document_id")),
			Toolset:     MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit,
			Module: model.ModuleDocs, Mutating: true, Destructive: true, IdempotentHint: true,
		},
		{
			Name:        "restore_document",
			Title:       "Restore document",
			Description: "Restore an archived document to draft.",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{"document_id": documentID}, "document_id")),
			Toolset:     MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit,
			Module: model.ModuleDocs, Mutating: true, IdempotentHint: true,
		},
	}
}

// executeDocsLifecycleMCPTool runs a Docs lifecycle tool. handled is false
// when name is not a lifecycle tool.
func (s *MCPService) executeDocsLifecycleMCPTool(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	name string,
	arguments json.RawMessage,
) (result *MCPToolResult, handled bool, err error) {
	switch name {
	case "update_document", "publish_document", "unpublish_document", "archive_document", "restore_document":
	default:
		return nil, false, nil
	}
	if s.docsLifecycle == nil {
		return nil, true, ErrMCPForbidden
	}
	var input struct {
		DocumentID string   `json:"document_id"`
		Title      *string  `json:"title"`
		Excerpt    *string  `json:"excerpt"`
		Tags       []string `json:"tags"`
		Slug       string   `json:"slug"`
	}
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, true, err
	}
	var rawValues map[string]json.RawMessage
	if err := json.Unmarshal(arguments, &rawValues); err != nil {
		return nil, true, fmt.Errorf("%w: arguments must be a JSON object", ErrMCPInvalidArguments)
	}
	document, err := s.accessibleMCPDocument(ctx, principal, actor, input.DocumentID)
	if err != nil {
		return nil, true, err
	}
	if document == nil {
		return nil, true, ErrMCPNotFound
	}
	if document.IsLocked {
		return nil, true, newMCPToolError(MCPErrorCodeDocumentLocked, "The document is locked. Unlock it in Helpin first.")
	}

	switch name {
	case "update_document":
		result, err = s.updateMCPDocument(ctx, document, input.Title, input.Excerpt, input.Tags, rawValues)
	case "publish_document":
		result, err = s.publishMCPDocument(ctx, principal, actor, document, input.Slug)
	case "unpublish_document":
		result, err = s.unpublishMCPDocument(ctx, document)
	case "archive_document":
		result, err = s.archiveMCPDocument(ctx, document)
	case "restore_document":
		result, err = s.restoreMCPDocument(ctx, document)
	}
	return result, true, err
}

func (s *MCPService) updateMCPDocument(
	ctx context.Context,
	document *model.DocsDocument,
	title, excerpt *string,
	tags []string,
	rawValues map[string]json.RawMessage,
) (*MCPToolResult, error) {
	request := model.UpdateDocsDocumentRequest{Tags: tags}
	if title != nil {
		trimmed := strings.TrimSpace(*title)
		if trimmed == "" {
			return nil, fmt.Errorf("%w: title cannot be blank", ErrMCPInvalidArguments)
		}
		request.Title = &trimmed
	}
	if raw, ok := rawValues["excerpt"]; ok {
		if string(raw) == "null" {
			request.ClearExcerpt = true
		} else {
			request.Excerpt = excerpt
		}
	}
	if request.Title == nil && request.Excerpt == nil && !request.ClearExcerpt && request.Tags == nil {
		return nil, fmt.Errorf("%w: provide title, excerpt, or tags", ErrMCPInvalidArguments)
	}
	updated, err := s.docsLifecycle.Update(ctx, document.ID, request)
	if err != nil {
		return nil, mapMCPDocsLifecycleError(err)
	}
	return &MCPToolResult{Summary: "Document " + updated.Title + " updated.", Data: mcpDocumentState(updated, nil)}, nil
}

func (s *MCPService) publishMCPDocument(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	document *model.DocsDocument,
	slug string,
) (*MCPToolResult, error) {
	if document.Status == model.DocStatusArchived {
		return nil, newMCPToolError(MCPErrorCodeDocumentArchived, "Restore the document before publishing it.")
	}
	space, err := s.accessibleMCPDocsSpace(ctx, principal, actor, document.SpaceID)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, ErrMCPNotFound
	}
	published, err := s.docsLifecycle.Publish(ctx, document.ID)
	if err != nil {
		return nil, mapMCPDocsLifecycleError(err)
	}
	if space.Type != model.SpaceTypeExternalCapable || s.helpcenter == nil {
		s.queueMCPDocumentEmbedding(ctx, published.ID)
		return &MCPToolResult{
			Summary: "Document " + published.Title + " published inside the workspace. Its space is not a Help Center space, so it is not public.",
			Data:    mcpDocumentState(published, nil),
		}, nil
	}
	if err := s.helpcenter.PublishExternally(ctx, published.ID, strings.TrimSpace(slug), nil); err != nil {
		return nil, fmt.Errorf("publish to help center: %w", err)
	}
	s.queueMCPDocumentEmbedding(ctx, published.ID)
	article, err := s.helpcenter.GetArticle(ctx, published.ID)
	if err != nil {
		return nil, err
	}
	return &MCPToolResult{
		Summary: "Document " + published.Title + " is live in the Help Center.",
		Data:    mcpDocumentState(published, article),
	}, nil
}

func (s *MCPService) unpublishMCPDocument(ctx context.Context, document *model.DocsDocument) (*MCPToolResult, error) {
	live, err := s.mcpDocumentIsLive(ctx, document.ID)
	if err != nil {
		return nil, err
	}
	if !live && document.Status != model.DocStatusPublished {
		return nil, newMCPToolError(MCPErrorCodeNotPublished, "The document is not published.")
	}
	if live {
		if err := s.helpcenter.UnpublishExternally(ctx, document.ID); err != nil {
			return nil, fmt.Errorf("unpublish from help center: %w", err)
		}
	}
	updated := document
	if document.Status == model.DocStatusPublished {
		updated, err = s.docsLifecycle.Unpublish(ctx, document.ID)
		if err != nil {
			return nil, mapMCPDocsLifecycleError(err)
		}
	}
	s.queueMCPDocumentEmbedding(ctx, updated.ID)
	return &MCPToolResult{Summary: "Document " + updated.Title + " unpublished and returned to draft.", Data: mcpDocumentState(updated, nil)}, nil
}

func (s *MCPService) archiveMCPDocument(ctx context.Context, document *model.DocsDocument) (*MCPToolResult, error) {
	live, err := s.mcpDocumentIsLive(ctx, document.ID)
	if err != nil {
		return nil, err
	}
	if live {
		return nil, newMCPToolError(MCPErrorCodeDocumentPublished, "The document is live in the Help Center. Call unpublish_document first.")
	}
	archived, err := s.docsLifecycle.Archive(ctx, document.ID)
	if err != nil {
		return nil, mapMCPDocsLifecycleError(err)
	}
	return &MCPToolResult{Summary: "Document " + archived.Title + " archived.", Data: mcpDocumentState(archived, nil)}, nil
}

func (s *MCPService) restoreMCPDocument(ctx context.Context, document *model.DocsDocument) (*MCPToolResult, error) {
	if document.Status != model.DocStatusArchived {
		return nil, newMCPToolError(MCPErrorCodeNotArchived, "The document is not archived.")
	}
	restored, err := s.docsLifecycle.Unarchive(ctx, document.ID)
	if err != nil {
		return nil, mapMCPDocsLifecycleError(err)
	}
	return &MCPToolResult{Summary: "Document " + restored.Title + " restored to draft.", Data: mcpDocumentState(restored, nil)}, nil
}

func (s *MCPService) mcpDocumentIsLive(ctx context.Context, documentID string) (bool, error) {
	if s.helpcenter == nil {
		return false, nil
	}
	article, err := s.helpcenter.GetArticle(ctx, documentID)
	if err != nil {
		return false, err
	}
	return article != nil && article.PublicPublishedAt != nil, nil
}

func (s *MCPService) queueMCPDocumentEmbedding(ctx context.Context, documentID string) {
	if s.docsEmbedding == nil {
		return
	}
	if err := s.docsEmbedding.QueueDocumentSync(ctx, documentID); err != nil {
		slog.WarnContext(ctx, "queue docs embedding sync after MCP lifecycle change failed",
			"document_id", documentID, "error", err)
	}
}

func mapMCPDocsLifecycleError(err error) error {
	if errors.Is(err, ErrDocsDocumentLocked) {
		return newMCPToolError(MCPErrorCodeDocumentLocked, "The document is locked. Unlock it in Helpin first.")
	}
	return err
}

func mcpDocumentState(document *model.DocsDocument, article *model.DocsHelpcenterArticle) map[string]any {
	state := map[string]any{
		"document_id":   document.ID,
		"title":         document.Title,
		"status":        document.Status,
		"space_id":      document.SpaceID,
		"collection_id": document.CollectionID,
		"markdown_link": "[" + document.Title + "](helpin://documents/" + document.ID + ")",
		"help_center":   false,
	}
	if article != nil && article.PublicPublishedAt != nil {
		state["help_center"] = true
		state["live_slug"] = article.Slug
		state["public_id"] = article.PublicID
		state["live_published_at"] = article.PublicPublishedAt
	}
	return state
}
