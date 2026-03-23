package worker

import (
	"encoding/json"
	"fmt"
	"strings"
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
	result, _ := json.MarshalIndent(summaries, "", "  ")
	return string(result), nil
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

	result, _ := json.MarshalIndent(detail, "", "  ")
	return string(result), nil
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

	result, _ := json.MarshalIndent(hits, "", "  ")
	return string(result), nil
}

func toolWriteDocumentContent(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.WriteDocumentContent == nil {
		return "", fmt.Errorf("docs mutation is not available for this agent")
	}
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
	if err := ctx.Services.WriteDocumentContent(ctx.Context, ctx.WorkspaceID, params.DocumentID, params.Content); err != nil {
		return "", fmt.Errorf("write document content: %w", err)
	}
	return fmt.Sprintf("Document %s updated.", params.DocumentID), nil
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
	if ctx.Services == nil || ctx.Services.LinkDocumentToObject == nil {
		return "", fmt.Errorf("docs mutation is not available for this agent")
	}
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
	if err := ctx.Services.LinkDocumentToObject(ctx.Context, ctx.WorkspaceID, params.DocumentID, params.LinkedObjectType, params.LinkedObjectID, linkContext, ctx.AgentID); err != nil {
		return "", fmt.Errorf("link document: %w", err)
	}
	return "Document linked successfully.", nil
}
