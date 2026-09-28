package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	supportAIRetrievalTraceMaxQueries = 8
	supportAIRetrievalTraceMaxResults = 8
	supportAIRetrievalTraceMaxQuery   = 200
	supportAIRetrievalTraceMaxSnippet = 500
)

type SupportAIRetrievalTraceRecorder interface {
	RecordSupportAIRetrievalTrace(ctx context.Context, trace *model.SupportAIRetrievalTrace) error
}

type SupportAIRetrievalTraceInput struct {
	WorkspaceID    string
	ConversationID string
	MessageID      string
	SearchQueries  []string
	SearchResults  []KnowledgeSearchResult
	CitedSourceIDs []string
	AIConfidence   float64
	CanAnswer      *string
	CanResolve     *string
	FailureMode    string
	Metadata       map[string]any
}

type supportAIRetrievalTraceRepository interface {
	UpsertRetrievalTrace(ctx context.Context, trace *model.SupportAIRetrievalTrace) error
}

type SupportCoverageRetrievalTraceService struct {
	repo supportAIRetrievalTraceRepository
}

func NewSupportCoverageRetrievalTraceService(repo supportAIRetrievalTraceRepository) *SupportCoverageRetrievalTraceService {
	return &SupportCoverageRetrievalTraceService{repo: repo}
}

func (s *SupportCoverageRetrievalTraceService) RecordSupportAIRetrievalTrace(ctx context.Context, trace *model.SupportAIRetrievalTrace) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("support AI retrieval trace repository is not configured")
	}
	return s.repo.UpsertRetrievalTrace(ctx, trace)
}

func BuildSupportAIRetrievalTrace(input SupportAIRetrievalTraceInput) (*model.SupportAIRetrievalTrace, error) {
	if strings.TrimSpace(input.WorkspaceID) == "" || strings.TrimSpace(input.ConversationID) == "" || strings.TrimSpace(input.MessageID) == "" {
		return nil, fmt.Errorf("workspace_id, conversation_id, and message_id are required")
	}

	searchQueries, err := json.Marshal(compactStringList(input.SearchQueries, supportAIRetrievalTraceMaxQueries, supportAIRetrievalTraceMaxQuery))
	if err != nil {
		return nil, fmt.Errorf("marshal search queries: %w", err)
	}
	results, err := json.Marshal(compactKnowledgeSearchResults(input.SearchResults))
	if err != nil {
		return nil, fmt.Errorf("marshal retrieval results: %w", err)
	}
	citedSourceIDs, err := json.Marshal(compactStringList(input.CitedSourceIDs, supportAIRetrievalTraceMaxResults, 200))
	if err != nil {
		return nil, fmt.Errorf("marshal cited source ids: %w", err)
	}
	metadata := input.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal trace metadata: %w", err)
	}

	return &model.SupportAIRetrievalTrace{
		WorkspaceID:    strings.TrimSpace(input.WorkspaceID),
		ConversationID: strings.TrimSpace(input.ConversationID),
		MessageID:      strings.TrimSpace(input.MessageID),
		SearchQueries:  searchQueries,
		Results:        results,
		CitedSourceIDs: citedSourceIDs,
		AIConfidence:   input.AIConfidence,
		CanAnswer:      input.CanAnswer,
		CanResolve:     input.CanResolve,
		FailureMode:    strings.TrimSpace(input.FailureMode),
		Metadata:       metadataJSON,
	}, nil
}

type compactKnowledgeSearchResult struct {
	ID            string  `json:"id,omitempty"`
	ReferenceID   string  `json:"reference_id,omitempty"`
	SourceType    string  `json:"source_type,omitempty"`
	DocumentID    string  `json:"document_id,omitempty"`
	BlockID       string  `json:"block_id,omitempty"`
	SourceID      string  `json:"source_id,omitempty"`
	ChunkIndex    int     `json:"chunk_index"`
	Title         string  `json:"title,omitempty"`
	URL           string  `json:"url,omitempty"`
	Snippet       string  `json:"snippet,omitempty"`
	LexicalScore  float64 `json:"lexical_score,omitempty"`
	VectorScore   float64 `json:"vector_score,omitempty"`
	CombinedScore float64 `json:"combined_score,omitempty"`
}

func compactKnowledgeSearchResults(results []KnowledgeSearchResult) []compactKnowledgeSearchResult {
	limit := len(results)
	if limit > supportAIRetrievalTraceMaxResults {
		limit = supportAIRetrievalTraceMaxResults
	}
	compact := make([]compactKnowledgeSearchResult, 0, limit)
	for _, result := range results {
		if result.SourceType == "external_mcp" {
			continue
		}
		if len(compact) == limit {
			break
		}
		compact = append(compact, compactKnowledgeSearchResult{
			ID:            strings.TrimSpace(result.ID),
			ReferenceID:   strings.TrimSpace(result.ReferenceID),
			SourceType:    strings.TrimSpace(result.SourceType),
			DocumentID:    strings.TrimSpace(result.DocumentID),
			BlockID:       strings.TrimSpace(result.BlockID),
			SourceID:      strings.TrimSpace(result.SourceID),
			ChunkIndex:    result.ChunkIndex,
			Title:         truncateTraceString(result.Title, supportAIRetrievalTraceMaxQuery),
			URL:           truncateTraceString(result.URL, supportAIRetrievalTraceMaxQuery),
			Snippet:       truncateTraceString(result.Content, supportAIRetrievalTraceMaxSnippet),
			LexicalScore:  result.LexicalScore,
			VectorScore:   result.VectorScore,
			CombinedScore: result.CombinedScore,
		})
	}
	return compact
}

func compactStringList(values []string, maxItems, maxChars int) []string {
	compact := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		trimmed := truncateTraceString(value, maxChars)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		compact = append(compact, trimmed)
		if len(compact) >= maxItems {
			break
		}
	}
	return compact
}

func truncateTraceString(value string, maxChars int) string {
	trimmed := strings.TrimSpace(value)
	if maxChars <= 0 || len(trimmed) <= maxChars {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:maxChars]) + "..."
}
