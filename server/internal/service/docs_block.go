package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// DocsBlockService exposes block-level operations while keeping docs_contents
// as the materialized compatibility aggregate.
type DocsBlockService struct {
	blockRepo  *repository.DocsBlockRepository
	contentSvc *DocsContentService
	docRepo    *repository.DocsDocumentRepository
}

func NewDocsBlockService(blockRepo *repository.DocsBlockRepository, contentSvc *DocsContentService, docRepo *repository.DocsDocumentRepository) *DocsBlockService {
	return &DocsBlockService{blockRepo: blockRepo, contentSvc: contentSvc, docRepo: docRepo}
}

func (s *DocsBlockService) List(ctx context.Context, documentID string) ([]model.DocsBlock, error) {
	return s.blockRepo.ListByDocument(ctx, documentID, false)
}

func (s *DocsBlockService) Patch(ctx context.Context, documentID, blockID string, expectedRevision int, content json.RawMessage, actorID string) (*model.DocsContent, error) {
	if expectedRevision <= 0 {
		return nil, fmt.Errorf("revision is required")
	}
	if err := s.checkEditable(ctx, documentID); err != nil {
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
			aggregate.Content[i] = node
			replaced = true
			break
		}
	}
	if !replaced {
		return nil, fmt.Errorf("block not found in document")
	}
	raw, _ := json.Marshal(aggregate)
	return s.contentSvc.Save(ctx, documentID, raw, actorID)
}

func (s *DocsBlockService) Create(ctx context.Context, documentID string, afterBlockID *string, content json.RawMessage, actorID string) (*model.DocsContent, error) {
	if err := s.checkEditable(ctx, documentID); err != nil {
		return nil, err
	}
	node, err := decodeBlockNode(content)
	if err != nil {
		return nil, err
	}
	setAggregateBlockID(node, "")
	aggregate, err := s.currentAggregate(ctx, documentID)
	if err != nil {
		return nil, err
	}
	insertAt := len(aggregate.Content)
	if afterBlockID != nil && strings.TrimSpace(*afterBlockID) != "" {
		insertAt = -1
		for i, child := range aggregate.Content {
			if aggregateBlockID(child) == strings.TrimSpace(*afterBlockID) {
				insertAt = i + 1
				break
			}
		}
		if insertAt < 0 {
			return nil, fmt.Errorf("after block not found")
		}
	}
	aggregate.Content = append(aggregate.Content, nil)
	copy(aggregate.Content[insertAt+1:], aggregate.Content[insertAt:])
	aggregate.Content[insertAt] = node
	raw, _ := json.Marshal(aggregate)
	return s.contentSvc.Save(ctx, documentID, raw, actorID)
}

func (s *DocsBlockService) Reorder(ctx context.Context, documentID string, blockIDs []string, actorID string) (*model.DocsContent, error) {
	if len(blockIDs) == 0 {
		return nil, fmt.Errorf("block_ids is required")
	}
	if err := s.checkEditable(ctx, documentID); err != nil {
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
	return s.contentSvc.Save(ctx, documentID, raw, actorID)
}

func (s *DocsBlockService) Delete(ctx context.Context, documentID, blockID, actorID string) (*model.DocsContent, error) {
	if err := s.checkEditable(ctx, documentID); err != nil {
		return nil, err
	}
	aggregate, err := s.currentAggregate(ctx, documentID)
	if err != nil {
		return nil, err
	}
	next := make([]map[string]any, 0, len(aggregate.Content))
	removed := false
	for _, child := range aggregate.Content {
		if aggregateBlockID(child) == blockID {
			removed = true
			continue
		}
		next = append(next, child)
	}
	if !removed {
		return nil, fmt.Errorf("block not found")
	}
	aggregate.Content = next
	raw, _ := json.Marshal(aggregate)
	return s.contentSvc.Save(ctx, documentID, raw, actorID)
}

func (s *DocsBlockService) checkEditable(ctx context.Context, documentID string) error {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}
	return checkLocked(doc)
}

func (s *DocsBlockService) currentAggregate(ctx context.Context, documentID string) (*aggregateDoc, error) {
	content, err := s.contentSvc.Get(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if content == nil || len(content.Content) == 0 {
		return &aggregateDoc{Type: "doc", Content: []map[string]any{}}, nil
	}
	var aggregate aggregateDoc
	if err := json.Unmarshal(content.Content, &aggregate); err != nil || aggregate.Type != "doc" {
		return nil, fmt.Errorf("document content is not block-editable")
	}
	if aggregate.Content == nil {
		aggregate.Content = []map[string]any{}
	}
	return &aggregate, nil
}

type aggregateDoc struct {
	Type    string           `json:"type"`
	Content []map[string]any `json:"content,omitempty"`
}

func decodeBlockNode(raw json.RawMessage) (map[string]any, error) {
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("invalid block content: %w", err)
	}
	if strings.TrimSpace(asBlockString(node["type"])) == "" {
		return nil, fmt.Errorf("block content must include type")
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

func asBlockString(v any) string {
	s, _ := v.(string)
	return s
}
