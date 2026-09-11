package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// DocsBlockService exposes block-level operations while keeping docs_contents
// as the materialized compatibility aggregate.
type DocsBlockService struct {
	blockRepo  *repository.DocsBlockRepository
	contentSvc *DocsContentService
	docRepo    *repository.DocsDocumentRepository
	activity   *PMActivityService
}

func NewDocsBlockService(blockRepo *repository.DocsBlockRepository, contentSvc *DocsContentService, docRepo *repository.DocsDocumentRepository) *DocsBlockService {
	return &DocsBlockService{blockRepo: blockRepo, contentSvc: contentSvc, docRepo: docRepo}
}

func (s *DocsBlockService) SetActivityService(activity *PMActivityService) {
	s.activity = activity
}

func (s *DocsBlockService) List(ctx context.Context, workspaceID, documentID string) ([]model.DocsBlock, error) {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("document not found")
	}
	blocks, err := s.blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		return nil, err
	}
	attachDocsBlockAgentReadable(blocks)
	return blocks, nil
}

func attachDocsBlockAgentReadable(blocks []model.DocsBlock) {
	for idx := range blocks {
		blocks[idx].AgentReadable = docsBlockAgentProjection(blocks[idx])
	}
}

// Get returns one live block of the document, with its agent projection.
func (s *DocsBlockService) Get(ctx context.Context, documentID, blockID string) (*model.DocsBlock, error) {
	block, err := s.blockRepo.GetByID(ctx, blockID)
	if err != nil {
		return nil, err
	}
	if block == nil || block.DocumentID != documentID || block.DeletedAt != nil {
		return nil, fmt.Errorf("block not found")
	}
	block.AgentReadable = docsBlockAgentProjection(*block)
	return block, nil
}

func (s *DocsBlockService) Patch(ctx context.Context, documentID, blockID string, expectedRevision int, content json.RawMessage, actorID string) (*model.DocsContent, error) {
	if expectedRevision <= 0 {
		return nil, fmt.Errorf("revision is required")
	}
	doc, err := s.loadEditableDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}
	block, err := s.blockRepo.GetByID(ctx, blockID)
	if err != nil {
		return nil, err
	}
	if block == nil || block.DocumentID != documentID || block.DeletedAt != nil {
		return nil, fmt.Errorf("block not found")
	}
	if block.Revision != expectedRevision {
		return nil, ErrDocsStaleBlockRevision
	}
	node, err := decodeBlockNode(content)
	if err != nil {
		return nil, err
	}
	setAggregateBlockID(node, blockID)
	aggregate, err := s.currentAggregate(ctx, documentID)
	if err != nil {
		return nil, err
	}
	replaced := false
	for i, child := range aggregate.Content {
		if aggregateBlockID(child) == blockID {
			current, err := json.Marshal(child)
			if err != nil {
				return nil, err
			}
			if tiptap.DocumentVersion(current) != tiptap.DocumentVersion(block.Content) {
				return nil, ErrDocsStaleBlockRevision
			}
			aggregate.Content[i] = node
			replaced = true
			break
		}
	}
	if !replaced {
		return nil, fmt.Errorf("block not found in document")
	}
	raw, _ := json.Marshal(aggregate)
	saved, err := s.contentSvc.SaveVersioned(ctx, documentID, raw, actorID, aggregate.snapshotVersion)
	if err != nil {
		return nil, err
	}
	s.logBlockActivity(ctx, doc, actorID, "block_updated", map[string]interface{}{
		"block_id":          block.ID,
		"block_type":        block.Type,
		"previous_revision": block.Revision,
		"next_revision":     block.Revision + 1,
	})
	return saved, nil
}

func (s *DocsBlockService) Create(ctx context.Context, documentID string, afterBlockID *string, content json.RawMessage, actorID string) (*model.DocsContent, error) {
	saved, _, err := s.CreateBlocks(ctx, documentID, afterBlockID, false, []json.RawMessage{content}, actorID)
	return saved, err
}

// CreateBlocks inserts one or more block nodes at a single position. When
// afterBlockID is set the nodes are inserted right after it; otherwise they are
// prepended when atStart is true or appended at the end of the document. It
// returns the saved aggregate plus the stable IDs assigned to the inserted
// blocks, in insertion order.
func (s *DocsBlockService) CreateBlocks(ctx context.Context, documentID string, afterBlockID *string, atStart bool, contents []json.RawMessage, actorID string) (*model.DocsContent, []string, error) {
	if len(contents) == 0 {
		return nil, nil, fmt.Errorf("at least one block is required")
	}
	doc, err := s.loadEditableDocument(ctx, documentID)
	if err != nil {
		return nil, nil, err
	}
	nodes := make([]map[string]any, 0, len(contents))
	for _, content := range contents {
		node, err := decodeBlockNode(content)
		if err != nil {
			return nil, nil, err
		}
		setAggregateBlockID(node, "")
		nodes = append(nodes, node)
	}
	aggregate, err := s.currentAggregate(ctx, documentID)
	if err != nil {
		return nil, nil, err
	}
	insertAt := len(aggregate.Content)
	if atStart {
		insertAt = 0
	}
	if afterBlockID != nil && strings.TrimSpace(*afterBlockID) != "" {
		insertAt = -1
		for i, child := range aggregate.Content {
			if aggregateBlockID(child) == strings.TrimSpace(*afterBlockID) {
				insertAt = i + 1
				break
			}
		}
		if insertAt < 0 {
			return nil, nil, fmt.Errorf("after block not found")
		}
	}
	next := make([]map[string]any, 0, len(aggregate.Content)+len(nodes))
	next = append(next, aggregate.Content[:insertAt]...)
	next = append(next, nodes...)
	next = append(next, aggregate.Content[insertAt:]...)
	aggregate.Content = next
	raw, _ := json.Marshal(aggregate)
	saved, err := s.contentSvc.SaveVersioned(ctx, documentID, raw, actorID, aggregate.snapshotVersion)
	if err != nil {
		return nil, nil, err
	}
	createdIDs := make([]string, 0, len(nodes))
	for i, node := range nodes {
		createdID, createdType := aggregateBlockAt(saved.Content, insertAt+i)
		createdIDs = append(createdIDs, createdID)
		s.logBlockActivity(ctx, doc, actorID, "block_created", map[string]interface{}{
			"block_id":       createdID,
			"block_type":     firstNonBlank(createdType, asBlockString(node["type"])),
			"after_block_id": derefString(afterBlockID),
		})
	}
	return saved, createdIDs, nil
}

func (s *DocsBlockService) Reorder(ctx context.Context, documentID string, blockIDs []string, actorID string) (*model.DocsContent, error) {
	if len(blockIDs) == 0 {
		return nil, fmt.Errorf("block_ids is required")
	}
	doc, err := s.loadEditableDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}
	aggregate, err := s.currentAggregate(ctx, documentID)
	if err != nil {
		return nil, err
	}
	byID := map[string]map[string]any{}
	for _, child := range aggregate.Content {
		if id := aggregateBlockID(child); id != "" {
			byID[id] = child
		}
	}
	next := make([]map[string]any, 0, len(aggregate.Content))
	seen := map[string]struct{}{}
	for _, id := range blockIDs {
		id = strings.TrimSpace(id)
		child, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("block %s not found", id)
		}
		next = append(next, child)
		seen[id] = struct{}{}
	}
	for _, child := range aggregate.Content {
		id := aggregateBlockID(child)
		if id == "" {
			next = append(next, child)
			continue
		}
		if _, ok := seen[id]; !ok {
			next = append(next, child)
		}
	}
	aggregate.Content = next
	raw, _ := json.Marshal(aggregate)
	saved, err := s.contentSvc.SaveVersioned(ctx, documentID, raw, actorID, aggregate.snapshotVersion)
	if err != nil {
		return nil, err
	}
	s.logBlockActivity(ctx, doc, actorID, "blocks_reordered", map[string]interface{}{
		"block_ids": blockIDs,
	})
	return saved, nil
}

func (s *DocsBlockService) Delete(ctx context.Context, documentID, blockID, actorID string) (*model.DocsContent, error) {
	doc, err := s.loadEditableDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}
	aggregate, err := s.currentAggregate(ctx, documentID)
	if err != nil {
		return nil, err
	}
	next := make([]map[string]any, 0, len(aggregate.Content))
	removed := false
	removedType := ""
	for _, child := range aggregate.Content {
		if aggregateBlockID(child) == blockID {
			removed = true
			removedType = asBlockString(child["type"])
			continue
		}
		next = append(next, child)
	}
	if !removed {
		return nil, fmt.Errorf("block not found")
	}
	aggregate.Content = next
	raw, _ := json.Marshal(aggregate)
	saved, err := s.contentSvc.SaveVersioned(ctx, documentID, raw, actorID, aggregate.snapshotVersion)
	if err != nil {
		return nil, err
	}
	s.logBlockActivity(ctx, doc, actorID, "block_deleted", map[string]interface{}{
		"block_id":   blockID,
		"block_type": removedType,
	})
	return saved, nil
}

func (s *DocsBlockService) MarkStaleFromSupport(ctx context.Context, workspaceID, documentID, blockID, gapID, reason, sourceSignal string) error {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return err
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return fmt.Errorf("document not found")
	}
	aggregate, err := s.currentAggregate(ctx, documentID)
	if err != nil {
		return err
	}
	targetIndex := -1
	blockID = strings.TrimSpace(blockID)
	if blockID != "" {
		for idx, child := range aggregate.Content {
			if aggregateBlockID(child) == blockID {
				targetIndex = idx
				break
			}
		}
	}
	if targetIndex < 0 && len(aggregate.Content) > 0 {
		targetIndex = 0
	}
	if targetIndex < 0 {
		return nil
	}

	target := aggregate.Content[targetIndex]
	attrs, _ := target["attrs"].(map[string]any)
	if attrs == nil {
		attrs = map[string]any{}
		target["attrs"] = attrs
	}
	if aggregateBlockID(target) == "" && blockID != "" {
		attrs["blockId"] = blockID
	}
	markedAt := time.Now().UTC().Format(time.RFC3339)
	attrs["staleState"] = "support_gap"
	attrs["staleReason"] = firstNonBlank(reason, "Support feedback indicates this content may be stale.")
	attrs["staleSource"] = firstNonBlank(sourceSignal, "support")
	attrs["staleGapId"] = strings.TrimSpace(gapID)
	attrs["staleMarkedAt"] = markedAt

	raw, _ := json.Marshal(aggregate)
	if _, err := s.contentSvc.SaveVersioned(ctx, documentID, raw, "", aggregate.snapshotVersion); err != nil {
		return err
	}
	s.logBlockActivity(ctx, doc, "", "block_marked_stale", map[string]interface{}{
		"block_id":          aggregateBlockID(target),
		"block_type":        asBlockString(target["type"]),
		"gap_id":            strings.TrimSpace(gapID),
		"reason":            attrs["staleReason"],
		"source_signal":     attrs["staleSource"],
		"stale_marked_at":   markedAt,
		"fallback_to_first": blockID == "",
	})
	return nil
}

func (s *DocsBlockService) checkEditable(ctx context.Context, documentID string) error {
	_, err := s.loadEditableDocument(ctx, documentID)
	return err
}

func (s *DocsBlockService) loadEditableDocument(ctx context.Context, documentID string) (*model.DocsDocument, error) {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}
	if err := checkLocked(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *DocsBlockService) currentAggregate(ctx context.Context, documentID string) (*aggregateDoc, error) {
	content, err := s.contentSvc.Get(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if content == nil || len(content.Content) == 0 {
		var raw json.RawMessage
		if content != nil {
			raw = content.Content
		}
		return &aggregateDoc{Type: "doc", Content: []map[string]any{}, snapshotVersion: tiptap.DocumentVersion(raw)}, nil
	}
	var aggregate aggregateDoc
	if err := json.Unmarshal(content.Content, &aggregate); err != nil || aggregate.Type != "doc" {
		return nil, fmt.Errorf("document content is not block-editable")
	}
	if aggregate.Content == nil {
		aggregate.Content = []map[string]any{}
	}
	aggregate.snapshotVersion = tiptap.DocumentVersion(content.Content)
	return &aggregate, nil
}

type aggregateDoc struct {
	snapshotVersion string
	Type            string           `json:"type"`
	Content         []map[string]any `json:"content,omitempty"`
}

func decodeBlockNode(raw json.RawMessage) (map[string]any, error) {
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("invalid block content: %w", err)
	}
	if strings.TrimSpace(asBlockString(node["type"])) == "" {
		return nil, fmt.Errorf("block content must include type")
	}
	// Reject structurally invalid nodes here rather than storing them: a bad
	// shape (for example "content" as a string) survives the round trip and
	// then breaks every reader of the document.
	if err := tiptap.ValidateBlockNode(raw); err != nil {
		return nil, err
	}
	return node, nil
}

func aggregateBlockID(node map[string]any) string {
	attrs, _ := node["attrs"].(map[string]any)
	if attrs == nil {
		return ""
	}
	return strings.TrimSpace(asBlockString(attrs["blockId"]))
}

func setAggregateBlockID(node map[string]any, id string) {
	attrs, _ := node["attrs"].(map[string]any)
	if attrs == nil {
		attrs = map[string]any{}
		node["attrs"] = attrs
	}
	if strings.TrimSpace(id) == "" {
		delete(attrs, "blockId")
		return
	}
	attrs["blockId"] = strings.TrimSpace(id)
}

func aggregateBlockAt(raw json.RawMessage, index int) (string, string) {
	if index < 0 {
		return "", ""
	}
	var aggregate aggregateDoc
	if err := json.Unmarshal(raw, &aggregate); err != nil || index >= len(aggregate.Content) {
		return "", ""
	}
	block := aggregate.Content[index]
	return aggregateBlockID(block), asBlockString(block["type"])
}

func (s *DocsBlockService) logBlockActivity(ctx context.Context, doc *model.DocsDocument, actorID, action string, metadata map[string]interface{}) {
	if s.activity == nil || doc == nil {
		return
	}
	if err := s.activity.Log(ctx, doc.WorkspaceID, "doc", doc.ID, optionalActor(actorID), action, nil, nil, nil, metadata); err != nil {
		slog.WarnContext(ctx, "failed to log docs block activity", "error", err, "document_id", doc.ID, "action", action)
	}
}

func asBlockString(v any) string {
	s, _ := v.(string)
	return s
}
