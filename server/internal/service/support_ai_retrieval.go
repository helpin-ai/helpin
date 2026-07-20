package service

import (
	"context"
	"fmt"
)

// expandKnowledgeNeighbors adds exact adjacent chunks after reranking. The
// repository repeats workspace, agent-link, publication, and source filters so
// neighbor expansion cannot bypass eligibility.
func (s *SupportAIService) expandKnowledgeNeighbors(
	ctx context.Context,
	workspaceID string,
	agentID string,
	ranked []KnowledgeSearchResult,
	limit int,
) ([]KnowledgeSearchResult, error) {
	if len(ranked) == 0 || limit <= 0 {
		return ranked, nil
	}
	baseLimit := min(len(ranked), max(1, limit-4))
	result := append([]KnowledgeSearchResult(nil), ranked[:baseLimit]...)
	seen := make(map[string]struct{}, len(result))
	for _, item := range result {
		seen[item.ID] = struct{}{}
	}

	seedCount := min(len(result), 4)
	for _, seed := range result[:seedCount] {
		if len(result) >= limit {
			break
		}
		if seed.SourceType == knowledgeSourceTypeGuidance {
			continue
		}
		indexes := []int{}
		if seed.ChunkIndex > 0 {
			indexes = append(indexes, seed.ChunkIndex-1)
		}
		indexes = append(indexes, seed.ChunkIndex+1)

		switch seed.SourceType {
		case knowledgeSourceTypeDocs:
			if s.docsChunkRepo == nil {
				continue
			}
			neighbors, err := s.docsChunkRepo.ListDocumentNeighbors(ctx, workspaceID, agentID, seed.DocumentID, indexes)
			if err != nil {
				return nil, fmt.Errorf("expand docs neighbors: %w", err)
			}
			for _, neighbor := range neighbors {
				if _, ok := seen[neighbor.ID]; ok || len(result) >= limit {
					continue
				}
				seen[neighbor.ID] = struct{}{}
				result = append(result, KnowledgeSearchResult{
					ID: neighbor.ID, ReferenceID: seed.ReferenceID, SourceType: seed.SourceType,
					IsInternal: seed.IsInternal, DocumentID: neighbor.DocumentID,
					BlockID: derefString(neighbor.BlockID), SourceID: neighbor.SpaceID,
					ChunkIndex: neighbor.ChunkIndex, SectionKey: neighbor.SectionKey,
					HeadingPath: neighbor.HeadingPath, Title: neighbor.Title,
					Content: neighbor.Content, CombinedScore: seed.CombinedScore - 0.001,
				})
			}
		case knowledgeSourceTypeContent:
			if s.contentChunkRepo == nil {
				continue
			}
			neighbors, err := s.contentChunkRepo.ListPageNeighbors(ctx, workspaceID, agentID, seed.DocumentID, indexes)
			if err != nil {
				return nil, fmt.Errorf("expand content neighbors: %w", err)
			}
			for _, neighbor := range neighbors {
				if _, ok := seen[neighbor.ID]; ok || len(result) >= limit {
					continue
				}
				seen[neighbor.ID] = struct{}{}
				result = append(result, KnowledgeSearchResult{
					ID: neighbor.ID, ReferenceID: seed.ReferenceID, SourceType: seed.SourceType,
					DocumentID: neighbor.PageID, SourceID: neighbor.ContentSourceID,
					ChunkIndex: neighbor.ChunkIndex, SectionKey: neighbor.SectionKey,
					HeadingPath: neighbor.HeadingPath, Title: neighbor.Title, URL: neighbor.URL,
					Content: neighbor.Content, CombinedScore: seed.CombinedScore - 0.001,
				})
			}
		}
	}
	return result, nil
}
