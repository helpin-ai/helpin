package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type mcpHelpcenterAdmin interface {
	GetArticle(context.Context, string) (*model.DocsHelpcenterArticle, error)
	EnrichDocumentPublishState(context.Context, *model.DocsDocument) error
	UpdateArticleMetadata(context.Context, string, string, model.UpdateDocsHelpcenterArticleMetadataRequest) (*model.DocsHelpcenterArticle, error)
	ListRedirects(context.Context, string, model.DocsRedirectFilter) ([]model.DocsRedirect, int64, error)
	CreateRedirect(context.Context, string, model.CreateDocsRedirectRequest) (*model.DocsRedirect, error)
}

func helpcenterMCPToolDefinitions() []MCPToolDefinition {
	object := func(properties map[string]any, required ...string) map[string]any {
		if required == nil {
			required = []string{}
		}
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	documentID := map[string]any{"type": "string", "minLength": 1}
	optionalText := func(maxLength int, description string) map[string]any {
		return map[string]any{"type": []string{"string", "null"}, "maxLength": maxLength, "description": description}
	}
	docs := func(definition MCPToolDefinition) MCPToolDefinition {
		definition.Toolset, definition.Module = MCPToolsetDocs, model.ModuleDocs
		return definition
	}
	return []MCPToolDefinition{
		docs(MCPToolDefinition{
			Name:  "get_help_center_article",
			Title: "Get Help Center article",
			Description: "Return a document's Help Center state: live slug, live time, unpublished changes, " +
				"social preview metadata, and reader feedback (helpful, not helpful, views).",
			InputSchema: object(map[string]any{"document_id": documentID}, "document_id"),
			Scope:       MCPScopeDocsRead, Permission: authorization.PermDocsRead,
		}),
		docs(MCPToolDefinition{
			Name:  "update_help_center_article_metadata",
			Title: "Update Help Center article metadata",
			Description: "Set the social preview title, description, image URL, and image alt text for a Help Center " +
				"article. Omit a field to keep it; null clears it. Changes public output.",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{
				"document_id":    documentID,
				"og_title":       optionalText(200, "Social preview title."),
				"og_description": optionalText(500, "Social preview description."),
				"og_image_url":   optionalText(2048, "HTTPS image URL for social previews."),
				"og_image_alt":   optionalText(300, "Alt text for the social preview image."),
			}, "document_id")),
			Scope: MCPScopeDocsPublish, Permission: authorization.PermDocsEdit, Mutating: true, IdempotentHint: true,
		}),
		docs(MCPToolDefinition{
			Name:        "list_help_center_redirects",
			Title:       "List Help Center redirects",
			Description: "List Help Center redirects, optionally filtered by a search term.",
			InputSchema: object(map[string]any{
				"search":   map[string]any{"type": "string", "maxLength": 200},
				"page":     map[string]any{"type": "integer", "minimum": 1, "default": 1},
				"per_page": map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 25},
			}),
			Scope: MCPScopeDocsRead, Permission: authorization.PermDocsAdmin,
		}),
		docs(MCPToolDefinition{
			Name:  "create_help_center_redirect",
			Title: "Create Help Center redirect",
			Description: "Redirect an old public path to a collection or article. Use it after merging or " +
				"archiving articles so existing links keep working. Changes public output.",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{
				"source_path":            map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
				"target_collection_slug": map[string]any{"type": "string", "minLength": 1, "maxLength": 200},
				"target_article_slug":    map[string]any{"type": "string", "maxLength": 200},
			}, "source_path", "target_collection_slug")),
			Scope: MCPScopeDocsPublish, Permission: authorization.PermDocsAdmin, Mutating: true, IdempotentHint: true,
		}),
	}
}

func (s *MCPService) executeHelpcenterMCPTool(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	name string,
	arguments json.RawMessage,
) (*MCPToolResult, bool, error) {
	switch name {
	case "get_help_center_article", "update_help_center_article_metadata",
		"list_help_center_redirects", "create_help_center_redirect":
	default:
		return nil, false, nil
	}
	admin, ok := s.helpcenter.(mcpHelpcenterAdmin)
	if !ok {
		return nil, true, ErrMCPForbidden
	}
	var result *MCPToolResult
	var err error
	switch name {
	case "get_help_center_article":
		result, err = s.getMCPHelpcenterArticle(ctx, principal, actor, admin, arguments)
	case "update_help_center_article_metadata":
		result, err = s.updateMCPHelpcenterArticleMetadata(ctx, principal, actor, admin, arguments)
	case "list_help_center_redirects":
		result, err = s.listMCPHelpcenterRedirects(ctx, principal, admin, arguments)
	case "create_help_center_redirect":
		result, err = s.createMCPHelpcenterRedirect(ctx, principal, admin, arguments)
	}
	return result, true, err
}

func (s *MCPService) getMCPHelpcenterArticle(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	admin mcpHelpcenterAdmin,
	arguments json.RawMessage,
) (*MCPToolResult, error) {
	var input struct {
		DocumentID string `json:"document_id"`
	}
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, err
	}
	document, err := s.accessibleMCPDocument(ctx, principal, actor, strings.TrimSpace(input.DocumentID))
	if err != nil {
		return nil, err
	}
	if document == nil {
		return nil, ErrMCPNotFound
	}
	enriched := *document
	if err := admin.EnrichDocumentPublishState(ctx, &enriched); err != nil {
		return nil, err
	}
	article, err := admin.GetArticle(ctx, document.ID)
	if err != nil {
		return nil, err
	}
	data := map[string]any{
		"document_id":             document.ID,
		"title":                   document.Title,
		"help_center_live":        enriched.LivePublishedAt != nil,
		"live_published_at":       enriched.LivePublishedAt,
		"has_unpublished_changes": enriched.HasUnpublishedChanges,
	}
	if enriched.LiveSlug != nil {
		data["live_slug"] = *enriched.LiveSlug
	}
	if article != nil {
		data["public_id"] = article.PublicID
		data["og_title"], data["og_description"] = article.OGTitle, article.OGDescription
		data["og_image_url"], data["og_image_alt"] = article.OGImageURL, article.OGImageAlt
		data["feedback"] = map[string]int{
			"helpful": article.HelpfulCount, "not_helpful": article.NotHelpfulCount, "views": article.ViewCount,
		}
	}
	return &MCPToolResult{Summary: "Help Center state for " + document.Title + " loaded.", Data: data}, nil
}

func (s *MCPService) updateMCPHelpcenterArticleMetadata(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	admin mcpHelpcenterAdmin,
	arguments json.RawMessage,
) (*MCPToolResult, error) {
	var input struct {
		DocumentID string `json:"document_id"`
	}
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(arguments, &raw); err != nil {
		return nil, fmt.Errorf("%w: arguments must be a JSON object", ErrMCPInvalidArguments)
	}
	document, err := s.requireMCPEditableDocument(ctx, principal, actor, input.DocumentID)
	if err != nil {
		return nil, err
	}
	request := model.UpdateDocsHelpcenterArticleMetadataRequest{
		OGTitle:       mcpClearableString(raw, "og_title"),
		OGDescription: mcpClearableString(raw, "og_description"),
		OGImageURL:    mcpClearableString(raw, "og_image_url"),
		OGImageAlt:    mcpClearableString(raw, "og_image_alt"),
	}
	if request.OGImageURL != nil && *request.OGImageURL != "" && !strings.HasPrefix(*request.OGImageURL, "https://") {
		return nil, newMCPToolError(MCPErrorCodeURLNotPublic, "og_image_url must be an HTTPS URL.")
	}
	article, err := admin.UpdateArticleMetadata(ctx, principal.WorkspaceID, document.ID, request)
	if err != nil {
		return nil, err
	}
	return &MCPToolResult{Summary: "Help Center metadata for " + document.Title + " updated.", Data: article}, nil
}

func (s *MCPService) listMCPHelpcenterRedirects(
	ctx context.Context,
	principal *model.MCPPrincipal,
	admin mcpHelpcenterAdmin,
	arguments json.RawMessage,
) (*MCPToolResult, error) {
	var input model.DocsRedirectFilter
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, err
	}
	if input.Page < 1 {
		input.Page = 1
	}
	input.PerPage = normalizeMCPLimit(input.PerPage)
	items, total, err := admin.ListRedirects(ctx, principal.WorkspaceID, input)
	if err != nil {
		return nil, err
	}
	return &MCPToolResult{
		Summary: "Returned " + strconv.Itoa(len(items)) + " of " + strconv.FormatInt(total, 10) + " redirects.",
		Data:    map[string]any{"items": items, "total": total, "page": input.Page, "per_page": input.PerPage},
	}, nil
}

func (s *MCPService) createMCPHelpcenterRedirect(
	ctx context.Context,
	principal *model.MCPPrincipal,
	admin mcpHelpcenterAdmin,
	arguments json.RawMessage,
) (*MCPToolResult, error) {
	var input model.CreateDocsRedirectRequest
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, err
	}
	if input.TargetArticleSlug != nil && strings.TrimSpace(*input.TargetArticleSlug) == "" {
		input.TargetArticleSlug = nil
	}
	redirect, err := admin.CreateRedirect(ctx, principal.WorkspaceID, input)
	if err != nil {
		return nil, err
	}
	return &MCPToolResult{Summary: "Redirect from " + input.SourcePath + " created.", Data: redirect}, nil
}

// mcpClearableString maps an omitted field to nil (keep), JSON null to an
// empty string (clear), and a string to its trimmed value.
func mcpClearableString(raw map[string]json.RawMessage, key string) *string {
	value, ok := raw[key]
	if !ok {
		return nil
	}
	cleared := ""
	if string(value) == "null" {
		return &cleared
	}
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return nil
	}
	text = strings.TrimSpace(text)
	return &text
}
