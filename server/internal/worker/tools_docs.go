package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

func toolListSpaces(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.ListSpaces == nil {
		return "", fmt.Errorf("docs access is not available for this agent")
	}
	spaces, err := ctx.Services.ListSpaces(ctx.Context, ctx.WorkspaceID)
	if err != nil {
		return "", fmt.Errorf("list spaces: %w", err)
	}
	if len(spaces) == 0 {
		return "No spaces found. Create a docs space before creating documents.", nil
	}

	type spaceSummary struct {
		ID     string  `json:"id"`
		Name   string  `json:"name"`
		Slug   string  `json:"slug"`
		Type   string  `json:"type"`
		TeamID *string `json:"team_id,omitempty"`
	}
	summaries := make([]spaceSummary, 0, len(spaces))
	for _, sp := range spaces {
		summaries = append(summaries, spaceSummary{ID: sp.ID, Name: sp.Name, Slug: sp.Slug, Type: sp.Type, TeamID: sp.TeamID})
	}
	return toCompactJSONString(summaries), nil
}

// resolveDefaultDocsSpace picks a space for create_document when the caller did
// not supply one. A single space is used automatically; when several exist the
// agent must choose, so we return the options in the error for it to surface to
// the user (e.g. via request_user_input) or pass an explicit space_id.
func resolveDefaultDocsSpace(ctx *ExecutionContext) (string, error) {
	if ctx.Services == nil || ctx.Services.ListSpaces == nil {
		return "", fmt.Errorf("space_id is required")
	}
	spaces, err := ctx.Services.ListSpaces(ctx.Context, ctx.WorkspaceID)
	if err != nil {
		return "", fmt.Errorf("resolve space: %w", err)
	}
	if len(spaces) == 0 {
		return "", fmt.Errorf("no docs spaces exist in this workspace; create a space first")
	}
	if len(spaces) == 1 {
		return spaces[0].ID, nil
	}
	type spaceOption struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	options := make([]spaceOption, 0, len(spaces))
	for _, sp := range spaces {
		options = append(options, spaceOption{ID: sp.ID, Name: sp.Name, Slug: sp.Slug})
	}
	return "", fmt.Errorf(
		"space_id is required: this workspace has multiple docs spaces, ask the user which one to use and pass its space_id. Available spaces: %s",
		toCompactJSONString(options),
	)
}

func toolListCollections(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.ListCollections == nil {
		return "", fmt.Errorf("docs access is not available for this agent")
	}
	var params struct {
		SpaceID *string `json:"space_id"`
	}
	_ = json.Unmarshal(input, &params)

	collections, err := ctx.Services.ListCollections(ctx.Context, ctx.WorkspaceID, params.SpaceID)
	if err != nil {
		return "", fmt.Errorf("list collections: %w", err)
	}
	if len(collections) == 0 {
		return "No collections found.", nil
	}

	type collectionSummary struct {
		ID                 string  `json:"id"`
		Name               string  `json:"name"`
		Slug               string  `json:"slug"`
		SpaceID            string  `json:"space_id"`
		ParentCollectionID *string `json:"parent_collection_id,omitempty"`
	}
	summaries := make([]collectionSummary, 0, len(collections))
	for _, c := range collections {
		summaries = append(summaries, collectionSummary{
			ID:                 c.ID,
			Name:               c.Name,
			Slug:               c.Slug,
			SpaceID:            c.SpaceID,
			ParentCollectionID: c.ParentCollectionID,
		})
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
		ID                   string              `json:"id"`
		Title                string              `json:"title"`
		Status               string              `json:"status"`
		TeamID               *string             `json:"team_id,omitempty"`
		ContentText          string              `json:"content_text,omitempty"`
		ContentTextRunes     int                 `json:"content_text_runes,omitempty"`
		ContentTextTruncated bool                `json:"content_text_truncated,omitempty"`
		BlocksTotal          int                 `json:"blocks_total,omitempty"`
		BlocksNextOffset     *int                `json:"blocks_next_offset,omitempty"`
		Blocks               []documentBlockView `json:"blocks,omitempty"`
	}
	detail := docDetail{ID: doc.ID, Title: doc.Title, Status: doc.Status, TeamID: doc.TeamID}

	if ctx.Services.GetDocumentContent != nil {
		if text, err := ctx.Services.GetDocumentContent(ctx.Context, params.DocumentID); err == nil {
			detail.ContentText, detail.ContentTextTruncated = truncateDocsToolTextWithFlag(text, maxReadDocumentTextRunes)
			detail.ContentTextRunes = len([]rune(text))
		}
	}
	if ctx.Services.ListDocumentBlocks != nil {
		if blocks, err := ctx.Services.ListDocumentBlocks(ctx.Context, params.DocumentID); err == nil {
			detail.BlocksTotal = len(blocks)
			page := blocks
			if len(page) > readDocumentBlockPreviewLimit {
				next := readDocumentBlockPreviewLimit
				detail.BlocksNextOffset = &next
				page = page[:readDocumentBlockPreviewLimit]
			}
			detail.Blocks = compactDocumentBlocks(page)
		}
	}

	return toCompactJSONString(detail), nil
}

type documentBlockView struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Revision    int    `json:"revision"`
	ContentText string `json:"content_text,omitempty"`
}

type documentBlockDetailView struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Revision    int             `json:"revision"`
	ContentText string          `json:"content_text,omitempty"`
	Content     json.RawMessage `json:"content,omitempty"`
}

type documentBlocksResponse struct {
	DocumentID string                    `json:"document_id"`
	Total      int                       `json:"total"`
	Offset     int                       `json:"offset"`
	Limit      int                       `json:"limit"`
	NextOffset *int                      `json:"next_offset,omitempty"`
	Blocks     []documentBlockDetailView `json:"blocks"`
}

const (
	maxReadDocumentTextRunes       = 2400
	readDocumentBlockPreviewLimit  = 40
	defaultDocumentBlocksToolLimit = 40
	maxDocumentBlocksToolLimit     = 100
	defaultDocumentBlocksAround    = 5
	maxDocumentBlocksAround        = 25
	maxFullDocumentBlocksToolFetch = 20
)

func compactDocumentBlocks(blocks []model.DocsBlock) []documentBlockView {
	out := make([]documentBlockView, 0, len(blocks))
	for _, block := range blocks {
		out = append(out, documentBlockView{
			ID:          block.ID,
			Type:        block.Type,
			Revision:    block.Revision,
			ContentText: truncateDocsToolText(block.ContentText, 140),
		})
	}
	return out
}

func truncateDocsToolText(value string, max int) string {
	out, _ := truncateDocsToolTextWithFlag(value, max)
	return out
}

func truncateDocsToolTextWithFlag(value string, max int) (string, bool) {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if max <= 0 || len(runes) <= max {
		return value, false
	}
	return strings.TrimSpace(string(runes[:max])) + "...", true
}

func toolGetDocumentBlocks(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.ListDocumentBlocks == nil {
		return "", fmt.Errorf("docs block access is not available for this agent")
	}
	var params struct {
		DocumentID    string   `json:"document_id"`
		BlockIDs      []string `json:"block_ids"`
		Include       bool     `json:"include_content"`
		Offset        int      `json:"offset"`
		Limit         int      `json:"limit"`
		AnchorBlockID string   `json:"anchor_block_id"`
		Around        int      `json:"around"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	params.DocumentID = strings.TrimSpace(params.DocumentID)
	if params.DocumentID == "" {
		return "", fmt.Errorf("document_id is required")
	}

	blocks, err := ctx.Services.ListDocumentBlocks(ctx.Context, params.DocumentID)
	if err != nil {
		return "", fmt.Errorf("get document blocks: %w", err)
	}

	filtered := blocks
	offset := 0
	limit := defaultDocumentBlocksToolLimit
	if len(params.BlockIDs) > 0 {
		requested := make(map[string]struct{}, len(params.BlockIDs))
		for _, id := range params.BlockIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			requested[id] = struct{}{}
		}
		filtered = make([]model.DocsBlock, 0, len(requested))
		found := make(map[string]struct{}, len(requested))
		for _, block := range blocks {
			if _, ok := requested[block.ID]; !ok {
				continue
			}
			filtered = append(filtered, block)
			found[block.ID] = struct{}{}
		}
		for id := range requested {
			if _, ok := found[id]; !ok {
				return "", fmt.Errorf("block %s not found", id)
			}
		}
		offset = 0
		limit = len(filtered)
	} else if strings.TrimSpace(params.AnchorBlockID) != "" {
		anchorID := strings.TrimSpace(params.AnchorBlockID)
		anchorIndex := -1
		for i, block := range blocks {
			if block.ID == anchorID {
				anchorIndex = i
				break
			}
		}
		if anchorIndex < 0 {
			return "", fmt.Errorf("anchor block %s not found", anchorID)
		}
		around := params.Around
		if around <= 0 {
			around = defaultDocumentBlocksAround
		}
		if around > maxDocumentBlocksAround {
			around = maxDocumentBlocksAround
		}
		start := anchorIndex - around
		if start < 0 {
			start = 0
		}
		end := anchorIndex + around + 1
		if end > len(blocks) {
			end = len(blocks)
		}
		filtered = blocks[start:end]
		offset = start
		limit = end - start
	} else {
		if params.Offset < 0 {
			return "", fmt.Errorf("offset must be >= 0")
		}
		if params.Limit > 0 {
			limit = params.Limit
		}
		if limit > maxDocumentBlocksToolLimit {
			limit = maxDocumentBlocksToolLimit
		}
		offset = params.Offset
		if offset >= len(blocks) {
			filtered = nil
		} else {
			end := offset + limit
			if end > len(blocks) {
				end = len(blocks)
			}
			filtered = blocks[offset:end]
		}
	}

	nextOffset := (*int)(nil)
	if offset+len(filtered) < len(blocks) {
		next := offset + len(filtered)
		nextOffset = &next
	}
	if params.Include && len(filtered) > maxFullDocumentBlocksToolFetch {
		return "", fmt.Errorf("include_content is limited to %d blocks; provide block_ids or a smaller limit/window", maxFullDocumentBlocksToolFetch)
	}

	out := make([]documentBlockDetailView, 0, len(filtered))
	for _, block := range filtered {
		view := documentBlockDetailView{
			ID:          block.ID,
			Type:        block.Type,
			Revision:    block.Revision,
			ContentText: truncateDocsToolText(block.ContentText, 140),
		}
		if params.Include {
			view.ContentText = block.ContentText
			view.Content = block.Content
		}
		out = append(out, view)
	}
	return toCompactJSONString(documentBlocksResponse{
		DocumentID: params.DocumentID,
		Total:      len(blocks),
		Offset:     offset,
		Limit:      limit,
		NextOffset: nextOffset,
		Blocks:     out,
	}), nil
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
	if params.Title == "" {
		return "", fmt.Errorf("title is required")
	}
	if params.SpaceID == "" {
		resolved, err := resolveDefaultDocsSpace(ctx)
		if err != nil {
			return "", err
		}
		params.SpaceID = resolved
	}
	if existing, ok, err := existingOutputDocument(ctx, params.SpaceID, params.CollectionID); err != nil {
		return "", err
	} else if ok {
		return existing, nil
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
		if err := persistOutputDocumentKey(ctx, output); err != nil {
			return "", err
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
	output := toCompactJSONString(map[string]any{
		"id":       doc.ID,
		"title":    doc.Title,
		"status":   doc.Status,
		"space_id": doc.SpaceID,
	})
	if err := persistOutputDocumentKey(ctx, json.RawMessage(output)); err != nil {
		return "", err
	}
	return output, nil
}

func existingOutputDocument(ctx *ExecutionContext, spaceID string, collectionID *string) (string, bool, error) {
	if ctx == nil || ctx.RunInput == nil || ctx.RunInput.Output == nil || ctx.Services == nil || ctx.Services.GetDocumentKey == nil || ctx.Services.GetDocument == nil {
		return "", false, nil
	}
	output := ctx.RunInput.Output
	if !strings.EqualFold(strings.TrimSpace(output.Type), "docs_document") || strings.TrimSpace(output.IdempotencyKey) == "" {
		return "", false, nil
	}
	if strings.TrimSpace(output.SpaceID) != "" && strings.TrimSpace(output.SpaceID) != strings.TrimSpace(spaceID) {
		return "", false, nil
	}
	if output.CollectionID != nil && collectionID != nil && strings.TrimSpace(*output.CollectionID) != strings.TrimSpace(*collectionID) {
		return "", false, nil
	}
	record, err := ctx.Services.GetDocumentKey(ctx.Context, ctx.WorkspaceID, model.DocsDocumentKeyTypeReleaseNotes, output.IdempotencyKey)
	if err != nil {
		return "", false, fmt.Errorf("get existing output document: %w", err)
	}
	if record == nil || record.DocumentID == nil || strings.TrimSpace(*record.DocumentID) == "" {
		return "", false, nil
	}
	doc, err := ctx.Services.GetDocument(ctx.Context, strings.TrimSpace(*record.DocumentID))
	if err != nil {
		return "", false, fmt.Errorf("load existing output document: %w", err)
	}
	if doc == nil {
		return "", false, nil
	}
	return toCompactJSONString(map[string]any{
		"id":       doc.ID,
		"title":    doc.Title,
		"status":   doc.Status,
		"space_id": doc.SpaceID,
	}), true, nil
}

func persistOutputDocumentKey(ctx *ExecutionContext, raw json.RawMessage) error {
	if ctx == nil || ctx.RunInput == nil || ctx.RunInput.Output == nil || ctx.Services == nil || ctx.Services.UpsertDocumentKey == nil {
		return nil
	}
	output := ctx.RunInput.Output
	if !strings.EqualFold(strings.TrimSpace(output.Type), "docs_document") || strings.TrimSpace(output.IdempotencyKey) == "" {
		return nil
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("parse created document output: %w", err)
	}
	if strings.TrimSpace(payload.ID) == "" {
		return fmt.Errorf("created document output did not include id")
	}
	documentID := strings.TrimSpace(payload.ID)
	return ctx.Services.UpsertDocumentKey(ctx.Context, &model.DocsDocumentKey{
		WorkspaceID: ctx.WorkspaceID,
		KeyType:     model.DocsDocumentKeyTypeReleaseNotes,
		Key:         strings.TrimSpace(output.IdempotencyKey),
		DocumentID:  &documentID,
	})
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

func toolUpdateDocumentBlock(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		DocumentID string          `json:"document_id"`
		BlockID    string          `json:"block_id"`
		Revision   int             `json:"revision"`
		Content    json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	params.DocumentID = strings.TrimSpace(params.DocumentID)
	params.BlockID = strings.TrimSpace(params.BlockID)
	if params.DocumentID == "" {
		return "", fmt.Errorf("document_id is required")
	}
	if params.BlockID == "" {
		return "", fmt.Errorf("block_id is required")
	}
	if params.Revision <= 0 {
		return "", fmt.Errorf("revision is required")
	}
	if len(params.Content) == 0 || strings.TrimSpace(string(params.Content)) == "" || strings.TrimSpace(string(params.Content)) == "null" {
		return "", fmt.Errorf("content is required")
	}
	commandInput, _ := json.Marshal(map[string]any{
		"document_id": params.DocumentID,
		"block_id":    params.BlockID,
		"revision":    params.Revision,
		"content":     params.Content,
	})
	if output, ok, err := executeInternalCommand(ctx, "document", params.DocumentID, "docs.update_document_block", commandInput); ok {
		if err != nil {
			return "", fmt.Errorf("update document block: %w", err)
		}
		return string(output), nil
	}
	if ctx.Services == nil || ctx.Services.UpdateDocumentBlock == nil {
		return "", fmt.Errorf("docs block mutation is not available for this agent")
	}
	if _, err := ctx.Services.UpdateDocumentBlock(ctx.Context, params.DocumentID, params.BlockID, params.Revision, params.Content, ctx.AgentID); err != nil {
		return "", fmt.Errorf("update document block: %w", err)
	}
	return fmt.Sprintf("Block %s updated.", params.BlockID), nil
}

func toolPublishAISectionCandidate(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("execution context is required")
	}
	if ctx.RunInput == nil || ctx.RunInput.Output == nil || !strings.EqualFold(strings.TrimSpace(ctx.RunInput.Output.Type), "docs_ai_section_candidate") {
		return "", fmt.Errorf("publish_ai_section_candidate is only available for Docs AI section candidate runs")
	}
	var params struct {
		DocumentID string           `json:"document_id"`
		BlockID    string           `json:"block_id"`
		Content    string           `json:"content"`
		Sources    []map[string]any `json:"sources"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	params.DocumentID = strings.TrimSpace(params.DocumentID)
	params.BlockID = strings.TrimSpace(params.BlockID)
	params.Content = strings.TrimSpace(params.Content)
	if params.DocumentID == "" {
		return "", fmt.Errorf("document_id is required")
	}
	if params.BlockID == "" {
		return "", fmt.Errorf("block_id is required")
	}
	if params.Content == "" {
		return "", fmt.Errorf("content is required")
	}
	if outputBlockID := strings.TrimSpace(ctx.RunInput.Output.IdempotencyKey); outputBlockID != "" && outputBlockID != params.BlockID {
		return "", fmt.Errorf("block_id does not match this AI section run")
	}
	if ctx.TargetType != "" && ctx.TargetType != "document" {
		return "", fmt.Errorf("AI section candidate runs must target a document")
	}
	if strings.TrimSpace(ctx.TargetID) != "" && strings.TrimSpace(ctx.TargetID) != params.DocumentID {
		return "", fmt.Errorf("document_id does not match this run target")
	}
	if ctx.Services == nil || ctx.Services.ListDocumentBlocks == nil || ctx.Services.PublishAISectionCandidate == nil {
		return "", fmt.Errorf("AI section candidate publishing is not available for this agent")
	}
	blocks, err := ctx.Services.ListDocumentBlocks(ctx.Context, params.DocumentID)
	if err != nil {
		return "", fmt.Errorf("load document blocks: %w", err)
	}
	var block *model.DocsBlock
	for i := range blocks {
		if strings.TrimSpace(blocks[i].ID) == params.BlockID {
			block = &blocks[i]
			break
		}
	}
	if block == nil || block.DeletedAt != nil {
		return "", fmt.Errorf("block not found")
	}
	if block.Type != "aiSection" {
		return "", fmt.Errorf("block is not an AI section")
	}
	sources := normalizeAISectionCandidateSources(params.Sources)
	candidateContent, err := aiSectionCandidateNodeFromMarkdown(block.Content, params.Content, ctx, len(sources))
	if err != nil {
		return "", err
	}
	promptHash := hashAISectionCandidateToolInput(params.Content, sources)
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	sourceRefs := model.JSONB{
		"items":         sources,
		"agent_run_id":  ctx.RunID,
		"agent_id":      ctx.AgentID,
		"prompt_hash":   promptHash,
		"generated_at":  generatedAt,
		"candidate_for": params.BlockID,
	}
	prompt := strings.TrimSpace(ctx.InitialInstructions)
	candidate, err := ctx.Services.PublishAISectionCandidate(ctx.Context, ctx.WorkspaceID, params.DocumentID, params.BlockID, ctx.AgentID, ctx.RunID, block.Content, candidateContent, tiptap.StripHTML(params.Content), sourceRefs, nilIfBlankWorker(prompt), &promptHash, agentModelName(ctx.Agent))
	if err != nil {
		return "", fmt.Errorf("publish AI section candidate: %w", err)
	}
	return toCompactJSONString(map[string]any{
		"status":       "published",
		"candidate_id": candidate.ID,
		"document_id":  candidate.DocumentID,
		"block_id":     candidate.BlockID,
	}), nil
}

type documentChangeProposalPreview struct {
	Scope           string                           `json:"scope"`
	DocumentID      string                           `json:"document_id"`
	BlockID         string                           `json:"block_id,omitempty"`
	Revision        int                              `json:"revision,omitempty"`
	Summary         string                           `json:"summary"`
	ContentMarkdown string                           `json:"content_markdown"`
	Content         json.RawMessage                  `json:"content,omitempty"`
	Sources         []model.DocsChangeProposalSource `json:"sources,omitempty"`
}

func toolPublishDocumentChangeProposal(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("execution context is required")
	}
	if ctx.Services == nil || ctx.Services.PublishDocumentChangeProposal == nil {
		return "", fmt.Errorf("document change proposal storage is not available for this agent")
	}
	proposal, err := buildDocumentChangeProposal(input, ctx)
	if err != nil {
		return "", err
	}
	sources, err := json.Marshal(proposal.Sources)
	if err != nil {
		return "", fmt.Errorf("marshal proposal sources: %w", err)
	}
	created, err := ctx.Services.PublishDocumentChangeProposal(ctx.Context, ctx.WorkspaceID, model.CreateDocsChangeProposalRequest{
		Scope:           proposal.Scope,
		DocumentID:      proposal.DocumentID,
		BlockID:         nilIfBlankWorker(proposal.BlockID),
		AgentID:         nilIfBlankWorker(ctx.AgentID),
		AgentRunID:      nilIfBlankWorker(ctx.RunID),
		Revision:        proposal.Revision,
		Summary:         proposal.Summary,
		ContentMarkdown: proposal.ContentMarkdown,
		Content:         proposal.Content,
		Sources:         sources,
		CreatedBy:       firstNonEmptyStringWorker(ctx.AgentID, ctx.RunID),
	})
	if err != nil {
		return "", err
	}
	return toCompactJSONString(map[string]any{
		"status":      "submitted",
		"proposal_id": created.ID,
		"scope":       proposal.Scope,
		"document_id": proposal.DocumentID,
		"block_id":    proposal.BlockID,
		"next_action": "Finish. The proposal is now visible in Docs for a human to apply or discard.",
	}), nil
}

func buildDocumentChangeProposal(input json.RawMessage, ctx *ExecutionContext) (documentChangeProposalPreview, error) {
	var params struct {
		Scope      string                           `json:"scope"`
		DocumentID string                           `json:"document_id"`
		BlockID    string                           `json:"block_id"`
		Revision   int                              `json:"revision"`
		Content    string                           `json:"content"`
		Summary    string                           `json:"summary"`
		Sources    []model.DocsChangeProposalSource `json:"sources"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return documentChangeProposalPreview{}, fmt.Errorf("parse input: %w", err)
	}
	params.Scope = strings.ToLower(strings.TrimSpace(params.Scope))
	params.DocumentID = strings.TrimSpace(params.DocumentID)
	params.BlockID = strings.TrimSpace(params.BlockID)
	params.Content = strings.TrimSpace(params.Content)
	params.Summary = strings.TrimSpace(params.Summary)
	switch params.Scope {
	case "document", "block":
	default:
		return documentChangeProposalPreview{}, fmt.Errorf("scope must be document or block")
	}
	if params.DocumentID == "" {
		return documentChangeProposalPreview{}, fmt.Errorf("document_id is required")
	}
	if params.Content == "" {
		return documentChangeProposalPreview{}, fmt.Errorf("content is required")
	}
	if params.Summary == "" {
		return documentChangeProposalPreview{}, fmt.Errorf("summary is required")
	}
	if params.Scope == "block" {
		if params.BlockID == "" {
			return documentChangeProposalPreview{}, fmt.Errorf("block_id is required for block proposals")
		}
		if params.Revision <= 0 {
			return documentChangeProposalPreview{}, fmt.Errorf("revision is required for block proposals")
		}
	}
	if ctx != nil {
		if ctx.TargetType != "" && ctx.TargetType != "document" {
			return documentChangeProposalPreview{}, fmt.Errorf("document change proposals must target a document")
		}
		if strings.TrimSpace(ctx.TargetID) != "" && strings.TrimSpace(ctx.TargetID) != params.DocumentID {
			return documentChangeProposalPreview{}, fmt.Errorf("document_id does not match this run target")
		}
	}
	proposal := documentChangeProposalPreview{
		Scope:           params.Scope,
		DocumentID:      params.DocumentID,
		BlockID:         params.BlockID,
		Revision:        params.Revision,
		Summary:         params.Summary,
		ContentMarkdown: params.Content,
		Sources:         params.Sources,
	}
	switch params.Scope {
	case "document":
		proposal.Content = tiptap.MarkdownToJSON(params.Content)
	case "block":
		if ctx != nil {
			block, err := requireDocumentProposalBlock(ctx, params.DocumentID, params.BlockID, params.Revision)
			if err != nil {
				return documentChangeProposalPreview{}, err
			}
			content, err := documentProposalBlockContentFromMarkdown(block.Content, params.Content)
			if err != nil {
				return documentChangeProposalPreview{}, err
			}
			proposal.Content = content
		}
	}
	return proposal, nil
}

func requireDocumentProposalBlock(ctx *ExecutionContext, documentID, blockID string, revision int) (*model.DocsBlock, error) {
	if ctx.Services == nil || ctx.Services.ListDocumentBlocks == nil {
		return nil, fmt.Errorf("document block access is not available for this agent")
	}
	blocks, err := ctx.Services.ListDocumentBlocks(ctx.Context, documentID)
	if err != nil {
		return nil, fmt.Errorf("load document blocks: %w", err)
	}
	for i := range blocks {
		block := &blocks[i]
		if strings.TrimSpace(block.ID) != blockID {
			continue
		}
		if block.DeletedAt != nil {
			return nil, fmt.Errorf("block not found")
		}
		if block.Revision != revision {
			return nil, fmt.Errorf("revision is stale; fetch the latest block revision")
		}
		return block, nil
	}
	return nil, fmt.Errorf("block not found")
}

func documentProposalBlockContentFromMarkdown(current json.RawMessage, markdown string) (json.RawMessage, error) {
	var currentNode map[string]any
	if err := json.Unmarshal(current, &currentNode); err != nil {
		return nil, fmt.Errorf("parse current block: %w", err)
	}
	var generated struct {
		Content []map[string]any `json:"content"`
	}
	if err := json.Unmarshal(tiptap.MarkdownToJSON(markdown), &generated); err != nil {
		return nil, fmt.Errorf("parse proposal markdown: %w", err)
	}
	if len(generated.Content) == 0 {
		return nil, fmt.Errorf("content must not be empty")
	}
	next := generated.Content[0]
	if currentAttrs, _ := currentNode["attrs"].(map[string]any); currentAttrs != nil {
		attrs, _ := next["attrs"].(map[string]any)
		if attrs == nil {
			attrs = map[string]any{}
			next["attrs"] = attrs
		}
		for _, key := range []string{"blockId", "staleState", "staleReason", "staleSource", "staleGapId", "staleMarkedAt"} {
			if value, ok := currentAttrs[key]; ok {
				attrs[key] = value
			}
		}
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return nil, fmt.Errorf("marshal proposal block: %w", err)
	}
	return raw, nil
}

func firstNonEmptyStringWorker(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeAISectionCandidateSources(raw []map[string]any) []map[string]any {
	if len(raw) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for idx, source := range raw {
		if source == nil {
			continue
		}
		next := make(map[string]any, len(source)+3)
		for key, value := range source {
			next[key] = value
		}
		if _, ok := next["sourceType"]; !ok {
			if value, ok := next["source_type"]; ok {
				next["sourceType"] = value
			} else {
				next["sourceType"] = "web_page"
			}
		}
		if _, ok := next["sourceId"]; !ok {
			if value, ok := next["source_id"]; ok {
				next["sourceId"] = value
			} else if value, ok := next["url"]; ok {
				next["sourceId"] = value
			} else {
				next["sourceId"] = fmt.Sprintf("source-%d", idx+1)
			}
		}
		if _, ok := next["access"]; !ok {
			next["access"] = "granted"
		}
		out = append(out, next)
	}
	return out
}

func aiSectionCandidateNodeFromMarkdown(current json.RawMessage, markdown string, ctx *ExecutionContext, sourceCount int) (json.RawMessage, error) {
	var node map[string]any
	if err := json.Unmarshal(current, &node); err != nil {
		return nil, fmt.Errorf("parse AI section block: %w", err)
	}
	attrs, _ := node["attrs"].(map[string]any)
	if attrs == nil {
		attrs = map[string]any{}
		node["attrs"] = attrs
	}
	attrs["status"] = "needs_review"
	attrs["lastGeneratedAt"] = time.Now().UTC().Format(time.RFC3339)
	attrs["sourceCount"] = sourceCount
	if strings.TrimSpace(ctx.AgentID) != "" {
		attrs["ownerAgentId"] = ctx.AgentID
	}
	if ctx.Agent != nil && strings.TrimSpace(ctx.Agent.Name) != "" {
		attrs["ownerAgentName"] = strings.TrimSpace(ctx.Agent.Name)
	}
	if modelName := agentModelName(ctx.Agent); modelName != nil {
		attrs["model"] = *modelName
	}

	var generated struct {
		Type    string           `json:"type"`
		Content []map[string]any `json:"content"`
	}
	if err := json.Unmarshal(tiptap.MarkdownToJSON(markdown), &generated); err != nil {
		return nil, fmt.Errorf("parse generated markdown: %w", err)
	}
	if len(generated.Content) == 0 {
		generated.Content = []map[string]any{{"type": "paragraph"}}
	}
	node["content"] = generated.Content
	raw, err := json.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("marshal AI section candidate: %w", err)
	}
	return raw, nil
}

func hashAISectionCandidateToolInput(content string, sources []map[string]any) string {
	sourceBytes, _ := json.Marshal(sources)
	sum := sha256.Sum256([]byte(strings.TrimSpace(content) + "\n" + string(sourceBytes)))
	return hex.EncodeToString(sum[:])
}

func agentModelName(agent *model.Agent) *string {
	if agent == nil {
		return nil
	}
	modelName := strings.TrimSpace(derefString(agent.Model))
	provider := strings.TrimSpace(derefString(agent.Provider))
	value := ""
	switch {
	case provider != "" && modelName != "":
		value = provider + "/" + modelName
	case provider != "":
		value = provider
	default:
		value = modelName
	}
	return nilIfBlankWorker(value)
}

func nilIfBlankWorker(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
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
