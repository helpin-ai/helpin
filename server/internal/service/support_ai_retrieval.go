package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

const canonicalPricingPageChunkScanLimit = 32

// ensureCanonicalPricingLeadChunks repairs a common failure mode of semantic
// retrieval on long interactive pricing pages: navigation, onboarding, or FAQ
// chunks can score above the actual plan cards. Once a canonical pricing page
// is found, inspect its released chunks and promote the first primary
// price-bearing section before applying the per-document cap.
func (s *SupportAIService) ensureCanonicalPricingLeadChunks(
	ctx context.Context,
	workspaceID string,
	agentID string,
	query string,
	ranked []KnowledgeSearchResult,
) ([]KnowledgeSearchResult, error) {
	if s == nil || s.contentChunkRepo == nil || len(ranked) == 0 || !isPricingKnowledgeQuery(query) || isComparativeKnowledgeQuery(query) {
		return ranked, nil
	}
	result := append([]KnowledgeSearchResult(nil), ranked...)
	seenPages := map[string]struct{}{}
	for _, seed := range ranked {
		if seed.SourceType != knowledgeSourceTypeContent || !isCanonicalPricingURL(seed.URL) || strings.TrimSpace(seed.DocumentID) == "" {
			continue
		}
		pageKey := strings.TrimSpace(seed.DocumentID)
		if _, ok := seenPages[pageKey]; ok {
			continue
		}
		seenPages[pageKey] = struct{}{}
		indexes := make([]int, canonicalPricingPageChunkScanLimit)
		for idx := range indexes {
			indexes[idx] = idx
		}
		chunks, err := s.contentChunkRepo.ListPageNeighbors(ctx, workspaceID, agentID, seed.DocumentID, indexes)
		if err != nil {
			return nil, fmt.Errorf("load canonical pricing page chunks: %w", err)
		}
		lead, ok := selectCanonicalPricingLeadChunk(chunks)
		if !ok {
			continue
		}
		promoted := KnowledgeSearchResult{
			ID: lead.ID, ReferenceID: seed.ReferenceID, SourceType: seed.SourceType,
			DocumentID: lead.PageID, SourceID: lead.ContentSourceID,
			ChunkIndex: lead.ChunkIndex, SectionKey: lead.SectionKey,
			HeadingPath: lead.HeadingPath, Title: lead.Title, URL: lead.URL,
			Content: lead.Content, CombinedScore: seed.CombinedScore + 0.25,
		}
		replaced := false
		for idx := range result {
			if result[idx].ID == promoted.ID {
				if promoted.CombinedScore > result[idx].CombinedScore {
					result[idx] = promoted
				}
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, promoted)
		}
	}
	sort.SliceStable(result, func(left, right int) bool {
		return result[left].CombinedScore > result[right].CombinedScore
	})
	return result, nil
}

func selectCanonicalPricingLeadChunk(chunks []repository.SupportContentChunkSearchResult) (repository.SupportContentChunkSearchResult, bool) {
	var fallback repository.SupportContentChunkSearchResult
	hasFallback := false
	for _, chunk := range chunks {
		if !currencyValuePattern.MatchString(chunk.Content) {
			continue
		}
		if !hasFallback {
			fallback = chunk
			hasFallback = true
		}
		content := strings.ToLower(chunk.Content)
		if strings.Contains(content, "one-time") || strings.Contains(content, "one time") ||
			strings.Contains(content, "add-on") || strings.Contains(content, "addon") ||
			strings.Contains(content, "white-label") || strings.Contains(content, "white label") {
			continue
		}
		return chunk, true
	}
	return fallback, hasFallback
}

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
