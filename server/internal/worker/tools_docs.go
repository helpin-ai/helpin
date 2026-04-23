package worker

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func toolListDocuments(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.ListDocuments == nil {
		return "", fmt.Errorf("docs access is not available for this agent")
	}
	var params struct {
		SpaceID *string `json:"space_id"`
	}
	_ = json.Unmarshal(input, &params)

	docs, err := ctx.Services.ListDocuments(ctx.Context, ctx.WorkspaceID, params.SpaceID)
	if err != nil {
		return "", fmt.Errorf("list documents: %w", err)
	}
	if len(docs) == 0 {
		return "No documents found.", nil
	}

	type docSummary struct {
		ID     string  `json:"id"`
		Title  string  `json:"title"`
		Status string  `json:"status"`
		TeamID *string `json:"team_id,omitempty"`
	}
	summaries := make([]docSummary, 0, len(docs))
	for _, d := range docs {
		summaries = append(summaries, docSummary{ID: d.ID, Title: d.Title, Status: d.Status, TeamID: d.TeamID})
	}
	return toCompactJSONString(summaries), nil
}

func toolReadDocument(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.GetDocument == nil {
		return "", fmt.Errorf("docs access is not available for this agent")
	}
	var params struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(params.DocumentID) == "" {
		return "", fmt.Errorf("document_id is required")
	}

	doc, err := ctx.Services.GetDocument(ctx.Context, params.DocumentID)
	if err != nil {
		return "", fmt.Errorf("get document: %w", err)
	}
	if doc == nil {
		return "", fmt.Errorf("document not found")
	}

	type docDetail struct {
		ID          string  `json:"id"`
		Title       string  `json:"title"`
		Status      string  `json:"status"`
		TeamID      *string `json:"team_id,omitempty"`
		ContentText string  `json:"content_text,omitempty"`
	}
	detail := docDetail{ID: doc.ID, Title: doc.Title, Status: doc.Status, TeamID: doc.TeamID}

	if ctx.Services.GetDocumentContent != nil {
		if text, err := ctx.Services.GetDocumentContent(ctx.Context, params.DocumentID); err == nil {
			detail.ContentText = text
		}
	}

	return toCompactJSONString(detail), nil
}

func toolSearchDocuments(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.SearchDocuments == nil {
		return "", fmt.Errorf("docs search is not available for this agent")
	}
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(params.Query) == "" {
		return "", fmt.Errorf("query is required")
	}
	if params.Limit <= 0 || params.Limit > 20 {
		params.Limit = 10
	}

	hits, err := ctx.Services.SearchDocuments(ctx.Context, ctx.WorkspaceID, params.Query, params.Limit)
	if err != nil {
		return "", fmt.Errorf("search documents: %w", err)
	}
	if len(hits) == 0 {
		return "No documents matched your query.", nil
	}

	return toCompactJSONString(hits), nil
}

func toolCreateDocument(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		SpaceID      string          `json:"space_id"`
		Title        string          `json:"title"`
		CollectionID *string         `json:"collection_id"`
		Content      json.RawMessage `json:"content"`
		Icon         *string         `json:"icon"`
		Tags         []string        `json:"tags"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	params.SpaceID = strings.TrimSpace(params.SpaceID)
	params.Title = strings.TrimSpace(params.Title)
	if params.SpaceID == "" {
		return "", fmt.Errorf("space_id is required")
	}
	if params.Title == "" {
		return "", fmt.Errorf("title is required")
	}

	content := normalizeDocumentToolContent(params.Content)
	commandInput, _ := json.Marshal(map[string]any{
		"space_id":      params.SpaceID,
		"title":         params.Title,
		"collection_id": params.CollectionID,
		"content":       content,
		"icon":          params.Icon,
		"tags":          params.Tags,
	})
	if output, ok, err := executeInternalCommand(ctx, "workspace", ctx.WorkspaceID, "docs.create_document", commandInput); ok {
		if err != nil {
			return "", fmt.Errorf("create document: %w", err)
		}
		return string(output), nil
	}

	if ctx.Services == nil || ctx.Services.CreateDocument == nil {
		return "", fmt.Errorf("docs creation is not available for this agent")
	}
	doc, err := ctx.Services.CreateDocument(ctx.Context, ctx.WorkspaceID, ctx.AgentID, model.CreateDocsDocumentRequest{
		SpaceID:      params.SpaceID,
		CollectionID: params.CollectionID,
		Title:        params.Title,
		Icon:         params.Icon,
		Tags:         params.Tags,
	}, content)
	if err != nil {
		return "", fmt.Errorf("create document: %w", err)
	}
	return toCompactJSONString(map[string]any{
		"id":       doc.ID,
		"title":    doc.Title,
		"status":   doc.Status,
		"space_id": doc.SpaceID,
	}), nil
}

func toolWriteDocumentContent(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		DocumentID string          `json:"document_id"`
		Content    json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(params.DocumentID) == "" {
		return "", fmt.Errorf("document_id is required")
	}
	if len(params.Content) == 0 || strings.TrimSpace(string(params.Content)) == "" || strings.TrimSpace(string(params.Content)) == "null" {
		fallbackContent, ok := latestApprovedMarkdownArtifactContent(ctx)
		if !ok {
			return "", fmt.Errorf("content is required")
		}
		params.Content = fallbackContent
	}
	commandInput, _ := json.Marshal(map[string]any{
		"document_id": params.DocumentID,
		"content":     params.Content,
	})
	if output, ok, err := executeInternalCommand(ctx, "document", params.DocumentID, "docs.write_document_content", commandInput); ok {
		if err != nil {
			return "", fmt.Errorf("write document content: %w", err)
		}
		return string(output), nil
	}
	if ctx.Services == nil || ctx.Services.WriteDocumentContent == nil {
		return "", fmt.Errorf("docs mutation is not available for this agent")
	}
	if err := ctx.Services.WriteDocumentContent(ctx.Context, ctx.WorkspaceID, params.DocumentID, params.Content); err != nil {
		return "", fmt.Errorf("write document content: %w", err)
	}
	return fmt.Sprintf("Document %s updated.", params.DocumentID), nil
}

func normalizeDocumentToolContent(raw json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	content := json.RawMessage(trimmed)
	if len(content) > 0 && content[0] == '"' {
		var markdown string
		if err := json.Unmarshal(content, &markdown); err == nil {
			if strings.TrimSpace(markdown) == "" {
				return nil
			}
			return tiptap.MarkdownToJSON(markdown)
		}
	}
	return content
}

func latestApprovedMarkdownArtifactContent(ctx *ExecutionContext) (json.RawMessage, bool) {
	if ctx == nil || ctx.ArtifactContext == nil || len(ctx.ArtifactContext.Entries) == 0 {
		return nil, false
	}

	for i := len(ctx.ArtifactContext.Entries) - 1; i >= 0; i-- {
		entry := ctx.ArtifactContext.Entries[i]
		if strings.TrimSpace(entry.Source) != "approved_preview" {
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(entry.Status), "approved") {
			continue
		}
		if strings.TrimSpace(entry.Format) != PreviewFormatMarkdown {
			continue
		}
		content := strings.TrimSpace(entry.Content)
		if content == "" {
			continue
		}
		payload, err := json.Marshal(content)
		if err != nil {
			return nil, false
		}
		return payload, true
	}
	return nil, false
}

func toolLinkDocumentToObject(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		DocumentID       string  `json:"document_id"`
		LinkedObjectType string  `json:"linked_object_type"`
		LinkedObjectID   string  `json:"linked_object_id"`
		LinkContext      *string `json:"link_context"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(params.DocumentID) == "" || strings.TrimSpace(params.LinkedObjectType) == "" || strings.TrimSpace(params.LinkedObjectID) == "" {
		return "", fmt.Errorf("document_id, linked_object_type, and linked_object_id are required")
	}
	linkContext := "attached"
	if params.LinkContext != nil && strings.TrimSpace(*params.LinkContext) != "" {
		linkContext = strings.TrimSpace(*params.LinkContext)
	}
	commandInput, _ := json.Marshal(map[string]any{
		"document_id":        params.DocumentID,
		"linked_object_type": params.LinkedObjectType,
		"linked_object_id":   params.LinkedObjectID,
		"link_context":       linkContext,
	})
	if output, ok, err := executeInternalCommand(ctx, params.LinkedObjectType, params.LinkedObjectID, "docs.link_document_to_object", commandInput); ok {
		if err != nil {
			return "", fmt.Errorf("link document: %w", err)
		}
		return string(output), nil
	}
	if ctx.Services == nil || ctx.Services.LinkDocumentToObject == nil {
		return "", fmt.Errorf("docs mutation is not available for this agent")
	}
	if err := ctx.Services.LinkDocumentToObject(ctx.Context, ctx.WorkspaceID, params.DocumentID, params.LinkedObjectType, params.LinkedObjectID, linkContext, ctx.AgentID); err != nil {
		return "", fmt.Errorf("link document: %w", err)
	}
	return "Document linked successfully.", nil
}
