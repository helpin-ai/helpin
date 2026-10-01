package service

import (
	"context"
	"errors"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrKnowledgeSourceNotFound means the source is unavailable in the requested scope.
var ErrKnowledgeSourceNotFound = errors.New("knowledge source not found")

// ListIndexedDocuments lists actual indexed articles within one selected source.
func (s *AgentKnowledgeSourceService) ListIndexedDocuments(ctx context.Context, workspaceID, agentID, sourceID string) ([]model.IndexedKnowledgeDocument, error) {
	source, err := s.repo.GetByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	if source == nil || source.WorkspaceID != workspaceID || source.AgentID != agentID {
		return nil, ErrKnowledgeSourceNotFound
	}
	space, err := s.spaceRepo.GetByID(ctx, source.SpaceID)
	if err != nil {
		return nil, err
	}
	if space == nil || space.WorkspaceID != workspaceID || !isDocsKnowledgeSpaceType(space.Type) {
		return nil, ErrKnowledgeSourceNotFound
	}
	documents, err := s.repo.ListIndexedDocuments(ctx, workspaceID, source.SpaceID)
	if err != nil {
		return nil, err
	}
	descendants := map[string]map[string]struct{}{}
	if source.ScopeType == model.KnowledgeSourceScopeCollection && source.CollectionID != nil && s.collectionRepo != nil {
		collections, err := s.collectionRepo.ListBySpace(ctx, source.SpaceID)
		if err != nil {
			return nil, err
		}
		descendants[*source.CollectionID] = docsCollectionDescendantSet(*source.CollectionID, collections)
	}
	result := []model.IndexedKnowledgeDocument{}
	for _, document := range documents {
		if docsKnowledgeSourceMatchesDocument(*source, document, descendants) {
			result = append(result, model.IndexedKnowledgeDocument{ID: document.ID, Title: document.Title})
		}
	}
	return result, nil
}
