package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type documentEditOperation struct {
	Type          string          `json:"type"`
	BlockID       string          `json:"block_id,omitempty"`
	StartBlockID  string          `json:"start_block_id,omitempty"`
	EndBlockID    string          `json:"end_block_id,omitempty"`
	BeforeBlockID string          `json:"before_block_id,omitempty"`
	AfterBlockID  string          `json:"after_block_id,omitempty"`
	Position      string          `json:"position,omitempty"`
	OldText       string          `json:"old_text,omitempty"`
	NewText       *string         `json:"new_text,omitempty"`
	Content       json.RawMessage `json:"content,omitempty"`
}

type documentEditRequest struct {
	DocumentID      string                  `json:"document_id"`
	ExpectedVersion string                  `json:"expected_version"`
	Operations      []documentEditOperation `json:"operations"`
}

type documentSplice struct {
	start, end int
	nodes      []map[string]any
}

func (s *InternalCommandService) executeEditDocument(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req documentEditRequest
	if err := decodeStrictInternalCommandInput(input, &req); err != nil {
		return nil, fmt.Errorf("parse document edit: %w", err)
	}
	req.DocumentID = strings.TrimSpace(firstNonEmptyCommand(req.DocumentID, currentDocumentTargetID(meta)))
	if req.DocumentID == "" || strings.TrimSpace(req.ExpectedVersion) == "" {
		return nil, errCommandInput("document_id and expected_version are required")
	}
	if len(req.Operations) < 1 || len(req.Operations) > 20 {
		return nil, errCommandInput("provide between 1 and 20 operations")
	}
	if s.docsContentService == nil || s.docsBlockService == nil {
		return nil, fmt.Errorf("document editing is not available")
	}
	if err := s.requireCommandDocumentInWorkspace(ctx, meta.WorkspaceID, req.DocumentID); err != nil {
		return nil, err
	}
	if _, err := s.docsBlockService.loadEditableDocument(ctx, req.DocumentID); err != nil {
		return nil, err
	}
	var before json.RawMessage
	saved, err := s.docsContentService.mutateContent(ctx, req.DocumentID, meta.ActorID, req.ExpectedVersion, func(raw json.RawMessage) (json.RawMessage, error) {
		before = append(json.RawMessage(nil), raw...)
		next, err := applyDocumentEdits(raw, req.Operations)
		if err != nil {
			// applyDocumentEdits is a pure transform of the caller's snapshot
			// and operations; its messages describe only the caller's input.
			return nil, errCommandInput("%s", err.Error())
		}
		return next, nil
	})
	if err != nil {
		return nil, err
	}
	s.docsBlockService.logBlockActivity(ctx, &model.DocsDocument{ID: req.DocumentID, WorkspaceID: meta.WorkspaceID}, meta.ActorID, "document_edited", map[string]any{"operation_count": len(req.Operations)})
	// Derive IDs from the saved snapshot, not a later read that may have changed again.
	changed, deleted := changedDocumentBlocks(before, saved.Content)
	for _, block := range changed {
		block["revision"] = saved.BlockRevisions[block["id"].(string)]
	}
	response := map[string]any{"document_id": req.DocumentID, "content_id": saved.ID, "version": tiptap.DocumentVersion(saved.Content), "changed_blocks": changed, "deleted_block_ids": deleted, "operation_count": len(req.Operations), "_agent_runtime_compaction": map[string]any{"exempt": true, "max_runes": documentOutputBudget}}
	return boundDocumentEditResult(response)
}

func applyDocumentEdits(raw json.RawMessage, operations []documentEditOperation) (json.RawMessage, error) {
	var aggregate map[string]any
	if len(raw) == 0 {
		raw = json.RawMessage(`{"type":"doc","content":[]}`)
	}
	if err := json.Unmarshal(raw, &aggregate); err != nil || aggregate["type"] != "doc" {
		return nil, fmt.Errorf("document is not block-editable; use a versioned whole-document write")
	}
	children, _ := aggregate["content"].([]any)
	nodes := make([]map[string]any, 0, len(children))
	positions := map[string]int{}
	for _, child := range children {
		node, ok := child.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid document block")
		}
		positions[aggregateBlockID(node)] = len(nodes)
		nodes = append(nodes, node)
	}
	find := func(id string) (int, error) {
		i, ok := positions[strings.TrimSpace(id)]
		if !ok || strings.TrimSpace(id) == "" {
			return 0, fmt.Errorf("block %s not found; reread the section", id)
		}
		return i, nil
	}
	claimed := map[int]bool{}
	textOps := map[int][]documentEditOperation{}
	insertions := map[int]bool{}
	splices := []documentSplice{}
	for index, op := range operations {
		if err := validateDocumentEditOperation(op); err != nil {
			return nil, fmt.Errorf("operation %d: %w", index+1, err)
		}
		if op.Type == "replace_text" {
			i, err := find(op.BlockID)
			if err != nil {
				return nil, err
			}
			if claimed[i] {
				return nil, fmt.Errorf("overlapping operations on block %s", op.BlockID)
			}
			textOps[i] = append(textOps[i], op)
			continue
		}
		start, end := 0, 0
		if op.Type == "insert" {
			switch {
			case op.BeforeBlockID != "":
				i, err := find(op.BeforeBlockID)
				if err != nil {
					return nil, err
				}
				start = i
			case op.AfterBlockID != "":
				i, err := find(op.AfterBlockID)
				if err != nil {
					return nil, err
				}
				start = i + 1
			case op.Position == "end":
				start = len(nodes)
			}
			end = start
			if insertions[start] {
				return nil, fmt.Errorf("multiple insertions at one boundary; combine their content")
			}
			insertions[start] = true
		} else {
			var err error
			start, err = find(op.StartBlockID)
			if err != nil {
				return nil, err
			}
			end, err = find(op.EndBlockID)
			if err != nil {
				return nil, err
			}
			end++
			if end <= start {
				return nil, errCommandInput("end_block_id must follow start_block_id")
			}
			for i := start; i < end; i++ {
				if claimed[i] || len(textOps[i]) > 0 {
					return nil, fmt.Errorf("overlapping block ranges")
				}
				claimed[i] = true
			}
		}
		var replacement []map[string]any
		if op.Type != "delete_range" {
			var err error
			replacement, err = documentReplacementNodes(op.Content)
			if err != nil {
				return nil, err
			}
		}
		splices = append(splices, documentSplice{start, end, replacement})
	}
	for _, splice := range splices {
		for boundary := range insertions {
			if boundary > splice.start && boundary < splice.end {
				return nil, fmt.Errorf("insertion is inside a replaced or deleted range")
			}
		}
	}
	for i, ops := range textOps {
		if err := replaceDocumentText(nodes[i], ops); err != nil {
			return nil, fmt.Errorf("block %s: %w", aggregateBlockID(nodes[i]), err)
		}
	}
	// Descending boundaries preserve all snapshot coordinates. At a shared boundary,
	// replace first and insert second so an insertion precedes the replacement.
	sort.SliceStable(splices, func(i, j int) bool {
		if splices[i].start == splices[j].start {
			return splices[i].end > splices[j].end
		}
		return splices[i].start > splices[j].start
	})
	for _, splice := range splices {
		next := append([]map[string]any{}, nodes[:splice.start]...)
		next = append(next, splice.nodes...)
		next = append(next, nodes[splice.end:]...)
		nodes = next
	}
	aggregate["content"] = nodes
	return json.Marshal(aggregate)
}

func validateDocumentEditOperation(op documentEditOperation) error {
	hasContent := len(op.Content) > 0 && string(op.Content) != "null"
	anchor := 0
	for _, v := range []string{op.BeforeBlockID, op.AfterBlockID, op.Position} {
		if v != "" {
			anchor++
		}
	}
	switch op.Type {
	case "replace_text":
		if strings.TrimSpace(op.BlockID) == "" {
			return errCommandInput("replace_text: block_id is required; use the block id from read_document or get_document_blocks")
		}
		if op.OldText == "" {
			return errCommandInput("replace_text: old_text must be nonempty and match text in the selected block")
		}
		if op.NewText == nil {
			return errCommandInput("replace_text: new_text is required; use an empty string to remove the match")
		}
		if hasContent || anchor > 0 || op.StartBlockID != "" || op.EndBlockID != "" {
			return fmt.Errorf("replace_text accepts only block_id, old_text, and new_text")
		}
	case "insert":
		if anchor != 1 || (op.Position != "" && op.Position != "start" && op.Position != "end") || !hasContent {
			return fmt.Errorf("insert requires content and exactly one of before_block_id, after_block_id, or position=start/end")
		}
		if op.BlockID != "" || op.StartBlockID != "" || op.EndBlockID != "" || op.OldText != "" || op.NewText != nil {
			return fmt.Errorf("insert does not accept range or text selectors")
		}
	case "replace_range", "delete_range":
		if op.StartBlockID == "" || op.EndBlockID == "" {
			return fmt.Errorf("range operations require start_block_id and end_block_id")
		}
		if anchor > 0 || op.BlockID != "" || op.OldText != "" || op.NewText != nil {
			return fmt.Errorf("range operations do not accept insertion or text selectors")
		}
		if (op.Type == "replace_range") != hasContent {
			return errCommandInput("replace_range requires content; delete_range must omit content")
		}
	default:
		return errCommandInput("type must be replace_text, replace_range, insert, or delete_range")
	}
	return nil
}

func documentReplacementNodes(raw json.RawMessage) ([]map[string]any, error) {
	var markdown string
	var nodes []json.RawMessage
	if json.Unmarshal(raw, &markdown) == nil {
		var doc struct {
			Content []json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(tiptap.MarkdownToJSON(markdown), &doc); err != nil {
			return nil, err
		}
		nodes = doc.Content
	} else if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, errCommandInput("content must be Markdown or an array of block nodes")
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("replacement content is empty; use delete_range to delete")
	}
	result := make([]map[string]any, 0, len(nodes))
	for _, rawNode := range nodes {
		node, err := decodeBlockNode(rawNode)
		if err != nil {
			return nil, err
		}
		setAggregateBlockID(node, "")
		result = append(result, node)
	}
	return result, nil
}
