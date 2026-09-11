package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

const documentOutputBudget = 30000

type documentReadRequest struct {
	DocumentID    string   `json:"document_id"`
	Mode          string   `json:"mode,omitempty"`
	Format        string   `json:"format,omitempty"`
	BlockIDs      []string `json:"block_ids,omitempty"`
	AnchorBlockID string   `json:"anchor_block_id,omitempty"`
	Around        *int     `json:"around,omitempty"`
	SectionID     string   `json:"section_id,omitempty"`
	Query         string   `json:"query,omitempty"`
	Offset        int      `json:"offset,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	Include       *bool    `json:"include_content,omitempty"`
	Cursor        string   `json:"cursor,omitempty"`
}

type documentReadCursor struct {
	Request   documentReadRequest `json:"request"`
	Version   string              `json:"version"`
	Tool      string              `json:"tool"`
	Position  int                 `json:"position"`
	Character int                 `json:"character"`
}

func (s *InternalCommandService) executeReadDocument(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	return s.executeDocumentRead(ctx, meta, input, "read_document")
}
func (s *InternalCommandService) executeGetDocumentBlocks(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	return s.executeDocumentRead(ctx, meta, input, "get_document_blocks")
}

func (s *InternalCommandService) executeDocumentRead(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage, tool string) (json.RawMessage, error) {
	var req documentReadRequest
	if err := decodeStrictInternalCommandInput(input, &req); err != nil {
		return nil, fmt.Errorf("parse document read: %w", err)
	}
	var cursor documentReadCursor
	if req.Cursor != "" {
		if len(req.Cursor) > 16000 {
			return nil, fmt.Errorf("invalid document cursor")
		}
		raw, err := base64.RawURLEncoding.DecodeString(req.Cursor)
		if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Tool != tool || len(cursor.Version) != 64 || cursor.Request.DocumentID == "" || cursor.Position < 0 || cursor.Character < 0 {
			return nil, fmt.Errorf("invalid document cursor")
		}
		if req.DocumentID != "" && req.DocumentID != cursor.Request.DocumentID {
			return nil, fmt.Errorf("cursor belongs to another document")
		}
		supplied := req
		supplied.DocumentID = ""
		supplied.Cursor = ""
		empty, _ := json.Marshal(supplied)
		if string(empty) != `{"document_id":""}` {
			return nil, fmt.Errorf("use cursor with document_id only; do not change its selection")
		}
		req = cursor.Request
		if req.Cursor != "" {
			return nil, fmt.Errorf("invalid nested document cursor")
		}
	}
	req.DocumentID = strings.TrimSpace(firstNonEmptyCommand(req.DocumentID, currentDocumentTargetID(meta)))
	req.Query = strings.TrimSpace(req.Query)
	req.SectionID = strings.TrimSpace(req.SectionID)
	req.AnchorBlockID = strings.TrimSpace(req.AnchorBlockID)
	if req.DocumentID == "" {
		return nil, fmt.Errorf("document_id is required")
	}
	if err := validateDocumentReadRequest(req, tool); err != nil {
		return nil, err
	}
	if s.docsDocumentService == nil || s.docsContentService == nil {
		return nil, fmt.Errorf("document reading is not available")
	}
	doc, err := s.docsDocumentService.Get(ctx, req.DocumentID)
	if err != nil {
		return nil, err
	}
	if doc == nil || doc.WorkspaceID != meta.WorkspaceID {
		return nil, fmt.Errorf("document not found")
	}
	content, blocks, err := s.docsContentService.contentRepo.ReadSnapshot(ctx, req.DocumentID)
	if err != nil {
		return nil, err
	}
	version := tiptap.DocumentVersion(content.Content)
	if cursor.Version != "" && cursor.Version != version {
		return nil, fmt.Errorf("document changed; restart the read to avoid mixing versions")
	}
	var stored struct {
		Type    string            `json:"type"`
		Content []json.RawMessage `json:"content"`
	}
	_ = json.Unmarshal(content.Content, &stored)
	emptyDocument := stored.Type == "doc" && len(stored.Content) == 0
	editable := len(blocks) > 0 || len(content.Content) == 0 || emptyDocument
	if len(blocks) == 0 && len(content.Content) > 0 && !emptyDocument {
		blocks = []model.DocsBlock{{Type: "document", Content: content.Content, ContentText: tiptap.RichTextToMarkdown(string(content.Content))}}
	}
	sections := documentOutline(blocks)
	response := map[string]any{
		"id": doc.ID, "document_id": doc.ID, "title": truncateCommandBarText(doc.Title, 500), "status": doc.Status,
		"markdown_link": helpinMarkdownLink(truncateCommandBarText(doc.Title, 500), "documents", doc.ID),
		"version":       version, "blocks_total": len(blocks), "total": len(blocks), "block_editable": editable,
		"content_characters": len([]rune(content.ContentText)), "word_count": content.WordCount,
		"complete": true, "next_cursor": nil, "next_offset": nil,
		"_agent_runtime_compaction": map[string]any{"exempt": true, "max_runes": documentOutputBudget},
	}
	if tool == "read_document" {
		excerpt, truncated := truncateCommandBarTextWithFlag(content.ContentText, 2400)
		response["content_text"] = excerpt
		response["content_text_runes"] = len([]rune(strings.TrimSpace(content.ContentText)))
		response["content_text_truncated"] = truncated
	}
	if doc.TeamID != nil {
		response["team_id"] = *doc.TeamID
	}
	mode := req.Mode
	if mode == "" {
		mode = "auto"
	}
	if tool == "read_document" && mode == "auto" {
		all := make([]documentReadBlock, 0, len(blocks))
		response["mode"] = "full"
		response["content_complete"] = true
		fits := true
		for i, b := range blocks {
			projected := readableDocumentBlock(b, i, sections, "markdown")
			if b.Type == "document" {
				projected.Markdown = b.ContentText
			}
			all = append(all, projected)
			response["blocks"] = all
			if !documentOutputFits(response) {
				fits = false
				break
			}
		}
		response["blocks"] = all
		if fits {
			return marshalBoundedDocument(response)
		}
		delete(response, "blocks")
		mode = "outline"
	}
	if tool == "read_document" && mode == "outline" {
		response["mode"] = "outline"
		response["content_complete"] = false
		counts := map[string]int{}
		for _, b := range blocks {
			counts[b.Type]++
		}
		response["block_types"] = counts
		items := make([]any, 0, len(sections))
		for _, section := range sections {
			section.Title = truncateCommandBarText(section.Title, 300)
			items = append(items, section)
		}
		req.Mode = "outline"
		return paginateDocumentItems(response, items, req, version, tool, cursor, "sections")
	}
	indices, matches, err := selectDocumentBlocks(blocks, sections, req, tool)
	if err != nil {
		return nil, err
	}
	format := req.Format
	if format == "" {
		format = "markdown"
		if tool == "get_document_blocks" && req.Include != nil && !*req.Include {
			format = "summary"
		}
	}
	if req.Include != nil && *req.Include {
		format = "json"
	}
	response["mode"] = "full"
	response["format"] = format
	items := make([]any, 0, len(indices))
	for _, index := range indices {
		b := readableDocumentBlock(blocks[index], index, sections, format)
		b.Match = matches[index]
		if req.Include != nil && *req.Include {
			b.ContentText = blocks[index].ContentText
		}
		if blocks[index].Type == "document" && format == "markdown" {
			b.Markdown = blocks[index].ContentText
		}
		items = append(items, b)
	}
	response["selected_total"] = len(indices)
	response["content_complete"] = format != "summary" && len(indices) == len(blocks)
	response["offset"] = req.Offset
	response["limit"] = len(indices)
	if tool == "get_document_blocks" && len(req.BlockIDs) == 0 && req.AnchorBlockID == "" && req.SectionID == "" && req.Query == "" {
		limit := req.Limit
		if limit == 0 {
			limit = 40
		}
		if req.Offset+limit < len(blocks) {
			response["next_offset"] = req.Offset + limit
		}
	}
	if req.Query != "" {
		headingMatches := []string{}
		for _, index := range indices {
			if matches[index] && blocks[index].Type == "heading" {
				headingMatches = append(headingMatches, blocks[index].ID)
			}
		}
		// A short heading shortlist gives useful destinations without disrupting merged windows.
		response["heading_matches"] = headingMatches[:min(10, len(headingMatches))]
	}
	req.Mode = "full"
	return paginateDocumentItems(response, items, req, version, tool, cursor, "blocks")
}

func validateDocumentReadRequest(req documentReadRequest, tool string) error {
	if req.Mode != "" && req.Mode != "auto" && req.Mode != "full" && req.Mode != "outline" {
		return fmt.Errorf("mode must be auto, full, or outline")
	}
	if req.Format != "" && req.Include != nil {
		return fmt.Errorf("choose format or include_content, not both")
	}
	if req.Around != nil && req.AnchorBlockID == "" && req.Query == "" {
		return fmt.Errorf("around requires anchor_block_id or query")
	}
	if req.Format != "" && req.Format != "markdown" && req.Format != "json" && req.Format != "summary" {
		return fmt.Errorf("format must be markdown, json, or summary")
	}
	if req.Offset < 0 || req.Limit < 0 || req.Limit > 100 {
		return fmt.Errorf("offset must be >= 0 and limit between 1 and 100 when provided")
	}
	if req.Around != nil && (*req.Around < 0 || *req.Around > 25) {
		return fmt.Errorf("around must be between 0 and 25")
	}
	if len(req.BlockIDs) > 100 || len(req.Query) > 500 {
		return fmt.Errorf("provide at most 100 block_ids and a query of at most 500 characters")
	}
	selectors := 0
	for _, set := range []bool{len(req.BlockIDs) > 0, req.AnchorBlockID != "", req.SectionID != "", strings.TrimSpace(req.Query) != "", req.Offset > 0} {
		if set {
			selectors++
		}
	}
	if selectors > 1 {
		return fmt.Errorf("choose one selector: block_ids, anchor_block_id, section_id, query, or offset")
	}
	if tool == "read_document" && (selectors > 0 || req.Include != nil || req.Around != nil || req.Format != "" || req.Limit != 0) {
		return fmt.Errorf("use get_document_blocks for selected blocks, sections, search, or JSON")
	}
	return nil
}

func selectDocumentBlocks(blocks []model.DocsBlock, sections []documentSection, req documentReadRequest, tool string) ([]int, map[int]bool, error) {
	selected := map[int]bool{}
	matches := map[int]bool{}
	find := func(id string) int {
		for i, b := range blocks {
			if b.ID == strings.TrimSpace(id) && b.ID != "" {
				return i
			}
		}
		return -1
	}
	window := func(i, n int) {
		for j := max(0, i-n); j < min(len(blocks), i+n+1); j++ {
			selected[j] = true
		}
	}
	switch {
	case len(req.BlockIDs) > 0:
		for _, id := range req.BlockIDs {
			i := find(id)
			if i < 0 {
				return nil, nil, fmt.Errorf("block %s not found", id)
			}
			selected[i] = true
		}
	case req.AnchorBlockID != "":
		i := find(req.AnchorBlockID)
		if i < 0 {
			return nil, nil, fmt.Errorf("anchor block not found")
		}
		n := 5
		if req.Around != nil {
			n = *req.Around
		}
		window(i, n)
	case req.SectionID != "":
		found := false
		for _, section := range sections {
			if section.ID == req.SectionID {
				for i := section.Start; i <= section.End; i++ {
					selected[i] = true
				}
				found = true
				break
			}
		}
		if !found {
			return nil, nil, fmt.Errorf("section not found; read the outline")
		}
	case strings.TrimSpace(req.Query) != "":
		n := 1
		if req.Around != nil {
			n = *req.Around
		}
		for i, b := range blocks {
			if documentBlockMatches(b, req.Query) {
				matches[i] = true
				window(i, n)
			}
		}
	default:
		end := len(blocks)
		if tool == "get_document_blocks" {
			limit := req.Limit
			if limit == 0 {
				limit = 40
			}
			end = min(end, req.Offset+limit)
		}
		for i := req.Offset; i < end; i++ {
			selected[i] = true
		}
	}
	indices := []int{}
	for i := range blocks {
		if selected[i] {
			indices = append(indices, i)
		}
	}
	// Keep merged search windows in document order; put heading hits first only in metadata.
	return indices, matches, nil
}
