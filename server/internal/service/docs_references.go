package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type DocsReferencesService struct {
	linkRepo       *repository.DocsLinkRepository
	blockRepo      *repository.DocsBlockRepository
	docRepo        *repository.DocsDocumentRepository
	commentService *PMCommentService
	agentService   *AgentService
	entityResolver *DocsEntityReferenceResolverService
}

func NewDocsReferencesService(linkRepo *repository.DocsLinkRepository, blockRepo *repository.DocsBlockRepository, docRepo *repository.DocsDocumentRepository, commentService *PMCommentService, agentService *AgentService) *DocsReferencesService {
	return &DocsReferencesService{linkRepo: linkRepo, blockRepo: blockRepo, docRepo: docRepo, commentService: commentService, agentService: agentService}
}

func (s *DocsReferencesService) SetEntityReferenceResolver(resolver *DocsEntityReferenceResolverService) {
	s.entityResolver = resolver
}

func (s *DocsReferencesService) List(ctx context.Context, workspaceID, documentID string) (*model.DocsReferencesResponse, error) {
	if workspaceID == "" || documentID == "" {
		return nil, fmt.Errorf("workspace_id and document_id are required")
	}
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("document not found")
	}

	items := make([]model.DocsReferenceItem, 0)
	items = append(items, s.linkReferences(ctx, workspaceID, documentID)...)
	items = append(items, s.blockReferences(ctx, workspaceID, documentID)...)
	items = append(items, s.commentReferences(ctx, documentID)...)
	items = append(items, s.agentRunReferences(ctx, workspaceID, documentID)...)
	s.enrichEntityReferences(ctx, workspaceID, items)

	return &model.DocsReferencesResponse{Items: items}, nil
}

func (s *DocsReferencesService) linkReferences(ctx context.Context, workspaceID, documentID string) []model.DocsReferenceItem {
	if s.linkRepo == nil {
		return nil
	}
	links, err := s.linkRepo.ListByObject(ctx, workspaceID, "doc", documentID)
	if err != nil {
		return nil
	}
	if more, err := s.linkRepo.ListByObject(ctx, workspaceID, "document", documentID); err == nil {
		links = append(links, more...)
	}
	items := make([]model.DocsReferenceItem, 0, len(links))
	for _, link := range links {
		title := strings.TrimSpace(link.DocumentTitle)
		if title == "" && s.docRepo != nil {
			if sourceDoc, err := s.docRepo.GetByID(ctx, link.DocumentID); err == nil && sourceDoc != nil {
				title = sourceDoc.Title
			}
		}
		if title == "" {
			title = "Document link"
		}
		items = append(items, model.DocsReferenceItem{
			ID:          "link:" + link.ID,
			Kind:        "doc_link",
			Title:       title,
			Description: "Links to this document",
			DocumentID:  link.DocumentID,
			BlockID:     derefString(link.BlockID),
			ReferenceID: link.ID,
			CreatedAt:   link.CreatedAt,
		})
	}
	return items
}

func (s *DocsReferencesService) blockReferences(ctx context.Context, workspaceID, documentID string) []model.DocsReferenceItem {
	if s.blockRepo == nil {
		return nil
	}
	blocks, err := s.blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		return nil
	}
	items := make([]model.DocsReferenceItem, 0)
	for _, block := range blocks {
		var node struct {
			Type  string         `json:"type"`
			Attrs map[string]any `json:"attrs"`
		}
		if err := json.Unmarshal(block.Content, &node); err != nil {
			continue
		}
		switch docsBlockKind(node.Type) {
		case docsBlockKindEntityEmbed:
			def, _ := docsBlockDefinitionFor(node.Type)
			entityType := stringAttr(node.Attrs, "entityType")
			entityID := stringAttr(node.Attrs, "entityId")
			title := firstNonBlank(stringAttr(node.Attrs, "title"), stringAttr(node.Attrs, "label"), entityType)
			items = append(items, model.DocsReferenceItem{
				ID:          "embed:" + block.ID,
				Kind:        def.AgentReadableKind,
				Title:       title,
				Description: def.Label,
				DocumentID:  documentID,
				BlockID:     block.ID,
				EntityType:  entityType,
				EntityID:    entityID,
				CreatedAt:   block.CreatedAt,
			})
		case docsBlockKindCitation:
			def, _ := docsBlockDefinitionFor(node.Type)
			sourceType := stringAttr(node.Attrs, "sourceType")
			sourceID := stringAttr(node.Attrs, "sourceId")
			title := firstNonBlank(stringAttr(node.Attrs, "title"), stringAttr(node.Attrs, "label"), sourceType)
			items = append(items, model.DocsReferenceItem{
				ID:          "citation:" + block.ID,
				Kind:        def.AgentReadableKind,
				Title:       title,
				Description: def.Label,
				DocumentID:  documentID,
				BlockID:     block.ID,
				EntityType:  sourceType,
				EntityID:    sourceID,
				CreatedAt:   block.CreatedAt,
			})
		}
		for index, ref := range inlineEntityMentionRefs(block.Content) {
			items = append(items, model.DocsReferenceItem{
				ID:          fmt.Sprintf("mention:%s:%d:%s:%s", block.ID, index, ref.entityType, ref.entityID),
				Kind:        "entity_mention",
				Title:       firstNonBlank(ref.label, ref.entityType),
				Description: "Inline entity mention",
				DocumentID:  documentID,
				BlockID:     block.ID,
				EntityType:  ref.entityType,
				EntityID:    ref.entityID,
				CreatedAt:   block.CreatedAt,
			})
		}
	}
	return items
}

func (s *DocsReferencesService) commentReferences(ctx context.Context, documentID string) []model.DocsReferenceItem {
	if s.commentService == nil {
		return nil
	}
	comments, err := s.commentService.List(ctx, "doc", documentID)
	if err != nil {
		return nil
	}
	items := make([]model.DocsReferenceItem, 0, len(comments))
	for _, entry := range comments {
		if entry.Comment.BlockID == nil && strings.TrimSpace(entry.Comment.AnchorText) == "" {
			continue
		}
		items = append(items, model.DocsReferenceItem{
			ID:          "comment:" + entry.Comment.ID,
			Kind:        "comment",
			Title:       firstNonBlank(entry.Comment.AnchorText, "Block comment"),
			Description: truncate(entry.Comment.Body, 120),
			DocumentID:  documentID,
			BlockID:     derefString(entry.Comment.BlockID),
			ReferenceID: entry.Comment.ID,
			CreatedAt:   entry.Comment.CreatedAt,
		})
	}
	return items
}

func (s *DocsReferencesService) agentRunReferences(ctx context.Context, workspaceID, documentID string) []model.DocsReferenceItem {
	if s.agentService == nil {
		return nil
	}
	runs, err := s.agentService.ListTargetRuns(ctx, workspaceID, "document", documentID)
	if err != nil {
		return nil
	}
	items := make([]model.DocsReferenceItem, 0, len(runs))
	for _, run := range runs {
		items = append(items, model.DocsReferenceItem{
			ID:          "agent_run:" + run.ID,
			Kind:        "agent_run",
			Title:       "Agent run " + run.Status,
			Description: derefString(run.ExecutionStage),
			DocumentID:  documentID,
			ReferenceID: run.ID,
			CreatedAt:   run.CreatedAt,
		})
	}
	return items
}

func (s *DocsReferencesService) enrichEntityReferences(ctx context.Context, workspaceID string, items []model.DocsReferenceItem) {
	if s.entityResolver == nil || len(items) == 0 {
		return
	}
	req := model.ResolveDocsEntityRefsRequest{Refs: make([]model.DocsEntityRefRequest, 0)}
	itemIndexes := make([]int, 0)
	for index, item := range items {
		if item.EntityType == "" || item.EntityID == "" {
			continue
		}
		req.Refs = append(req.Refs, model.DocsEntityRefRequest{
			EntityType: item.EntityType,
			EntityID:   item.EntityID,
			Label:      item.Title,
		})
		itemIndexes = append(itemIndexes, index)
	}
	if len(req.Refs) == 0 {
		return
	}
	resolved, err := s.entityResolver.Resolve(ctx, workspaceID, req)
	if err != nil || resolved == nil {
		return
	}
	for index, ref := range resolved.Refs {
		if index >= len(itemIndexes) {
			break
		}
		item := &items[itemIndexes[index]]
		item.Status = ref.Status
		item.Access = ref.Access
		if ref.Access == docsEntityRefAccessRedacted {
			item.Title = ref.Title
			item.Description = ref.Meta
			item.ReferenceURL = ""
			continue
		}
		if ref.Status == docsEntityRefStatusAvailable {
			item.Title = firstNonBlank(ref.Title, item.Title)
			if ref.Meta != "" {
				item.Description = ref.Meta
			}
			continue
		}
		item.Title = firstNonBlank(item.Title, ref.Title)
		item.Description = "Reference unavailable"
	}
}

type inlineEntityMentionRef struct {
	entityType string
	entityID   string
	label      string
}

func inlineEntityMentionRefs(raw json.RawMessage) []inlineEntityMentionRef {
	var node interface{}
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil
	}
	refs := make([]inlineEntityMentionRef, 0)
	var walk func(value interface{})
	walk = func(value interface{}) {
		current, ok := value.(map[string]interface{})
		if !ok {
			return
		}
		if nodeType, _ := current["type"].(string); nodeType == "entityMention" {
			if attrs, ok := current["attrs"].(map[string]interface{}); ok {
				entityType := jsonStringAttr(attrs, "entityType")
				entityID := jsonStringAttr(attrs, "entityId")
				label := jsonStringAttr(attrs, "label")
				if entityType != "" && entityID != "" {
					refs = append(refs, inlineEntityMentionRef{entityType: entityType, entityID: entityID, label: label})
				}
			}
		}
		if content, ok := current["content"].([]interface{}); ok {
			for _, child := range content {
				walk(child)
			}
		}
	}
	walk(node)
	return refs
}

func jsonStringAttr(attrs map[string]interface{}, key string) string {
	value, ok := attrs[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func stringAttr(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	value, _ := attrs[key].(string)
	return strings.TrimSpace(value)
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
