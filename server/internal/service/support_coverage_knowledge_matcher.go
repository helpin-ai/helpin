package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type CoverageKnowledgeMatcher struct {
	docsChunkRepo     *repository.DocsChunkRepository
	contentChunkRepo  *repository.SupportContentChunkRepository
	embeddingProvider llm.EmbeddingProvider
	embeddingModel    string
}

func NewCoverageKnowledgeMatcher(
	docsChunkRepo *repository.DocsChunkRepository,
	contentChunkRepo *repository.SupportContentChunkRepository,
	embeddingProvider llm.EmbeddingProvider,
	embeddingModel string,
) *CoverageKnowledgeMatcher {
	return &CoverageKnowledgeMatcher{
		docsChunkRepo:     docsChunkRepo,
		contentChunkRepo:  contentChunkRepo,
		embeddingProvider: embeddingProvider,
		embeddingModel:    strings.TrimSpace(embeddingModel),
	}
}

func (m *CoverageKnowledgeMatcher) MatchKnowledge(ctx context.Context, workspaceID string, spaceIDs []string, contentSourceIDs []string, query string, limit int) ([]CoverageKnowledgeCandidate, error) {
	if m == nil {
		return nil, nil
	}
	query = strings.TrimSpace(query)
	if workspaceID == "" || query == "" {
		return []CoverageKnowledgeCandidate{}, nil
	}
	if limit <= 0 {
		limit = 8
	}

	queryEmbedding := ""
	if m.embeddingProvider != nil {
		embeddingModel := m.embeddingModel
		if embeddingModel == "" {
			embeddingModel = defaultDocsEmbeddingModel
		}
		resp, err := m.embeddingProvider.CreateEmbeddings(ctx, llm.EmbeddingRequest{
			Model:  embeddingModel,
			Inputs: []string{query},
		})
		if err != nil {
			slog.WarnContext(ctx, "coverage knowledge matcher embedding failed; falling back to lexical retrieval",
				"error", err,
				"workspace_id", workspaceID,
				"query_preview", safeLogPreview(query, 120),
			)
		} else if len(resp.Vectors) > 0 {
			queryEmbedding = formatVector(resp.Vectors[0])
		}
	}

	return m.matchKnowledge(ctx, workspaceID, spaceIDs, contentSourceIDs, query, queryEmbedding, limit)
}

func (m *CoverageKnowledgeMatcher) MatchGapKnowledge(ctx context.Context, workspaceID string, spaceIDs []string, contentSourceIDs []string, query string, queryEmbedding string, limit int) ([]CoverageKnowledgeCandidate, error) {
	if m == nil {
		return nil, nil
	}
	query = strings.TrimSpace(query)
	if workspaceID == "" || query == "" {
		return []CoverageKnowledgeCandidate{}, nil
	}
	if limit <= 0 {
		limit = 8
	}
	return m.matchKnowledge(ctx, workspaceID, spaceIDs, contentSourceIDs, query, strings.TrimSpace(queryEmbedding), limit)
}

func (m *CoverageKnowledgeMatcher) matchKnowledge(ctx context.Context, workspaceID string, spaceIDs []string, contentSourceIDs []string, query string, queryEmbedding string, limit int) ([]CoverageKnowledgeCandidate, error) {
	candidates := make([]CoverageKnowledgeCandidate, 0, limit)
	embeddingModel := strings.TrimSpace(m.embeddingModel)
	if embeddingModel == "" {
		embeddingModel = defaultDocsEmbeddingModel
	}
	if m.docsChunkRepo != nil && len(spaceIDs) > 0 {
		results, err := m.docsChunkRepo.HybridSearchWithEmbeddingModel(ctx, workspaceID, spaceIDs, query, queryEmbedding, embeddingModel, limit)
		if err != nil {
			return nil, fmt.Errorf("docs hybrid search: %w", err)
		}
		for _, result := range results {
			candidates = append(candidates, CoverageKnowledgeCandidate{
				SourceType:    knowledgeSourceTypeDocs,
				TargetType:    "docs",
				DocumentID:    result.DocumentID,
				BlockID:       derefString(result.BlockID),
				Title:         result.Title,
				Excerpt:       truncateCoverageAnalysisContent(result.Content, supportAIRetrievalTraceMaxSnippet),
				CombinedScore: result.CombinedScore,
			})
		}
	}

	if m.contentChunkRepo != nil && len(contentSourceIDs) > 0 {
		results, err := m.contentChunkRepo.HybridSearchWithEmbeddingModel(ctx, workspaceID, contentSourceIDs, query, queryEmbedding, embeddingModel, limit)
		if err != nil {
			return nil, fmt.Errorf("content hybrid search: %w", err)
		}
		for _, result := range results {
			candidates = append(candidates, CoverageKnowledgeCandidate{
				SourceType:    "website",
				TargetType:    "website_page",
				PageID:        result.PageID,
				Title:         result.Title,
				URL:           result.URL,
				Excerpt:       truncateCoverageAnalysisContent(result.Content, supportAIRetrievalTraceMaxSnippet),
				CombinedScore: result.CombinedScore,
			})
		}
	}

	candidates = dedupeCoverageKnowledgeCandidates(candidates)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].CombinedScore == candidates[j].CombinedScore {
			return candidates[i].Title < candidates[j].Title
		}
		return candidates[i].CombinedScore > candidates[j].CombinedScore
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

func dedupeCoverageKnowledgeCandidates(candidates []CoverageKnowledgeCandidate) []CoverageKnowledgeCandidate {
	byKey := map[string]CoverageKnowledgeCandidate{}
	order := []string{}
	for _, candidate := range candidates {
		key := coverageKnowledgeCandidateKey(candidate)
		if key == "" {
			continue
		}
		existing, ok := byKey[key]
		if !ok {
			byKey[key] = candidate
			order = append(order, key)
			continue
		}
		if candidate.CombinedScore > existing.CombinedScore || (candidate.CombinedScore == existing.CombinedScore && len(candidate.Excerpt) > len(existing.Excerpt)) {
			byKey[key] = candidate
		}
	}
	deduped := make([]CoverageKnowledgeCandidate, 0, len(order))
	for _, key := range order {
		deduped = append(deduped, byKey[key])
	}
	return deduped
}

func coverageKnowledgeCandidateKey(candidate CoverageKnowledgeCandidate) string {
	switch candidate.TargetType {
	case "docs":
		if candidate.DocumentID == "" {
			return ""
		}
		if strings.TrimSpace(candidate.BlockID) != "" {
			return "docs:" + candidate.DocumentID + ":" + strings.TrimSpace(candidate.BlockID)
		}
		return "docs:" + candidate.DocumentID
	case "website_page":
		if candidate.PageID == "" {
			return ""
		}
		return "website_page:" + candidate.PageID
	default:
		return strings.TrimSpace(candidate.SourceType) + ":" + strings.TrimSpace(candidate.Title) + ":" + strings.TrimSpace(candidate.URL)
	}
}
