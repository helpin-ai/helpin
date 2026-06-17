package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	coverageClusterRebuildLimit          = 1000
	coverageClusterSuggestThreshold      = 0.75
	coverageClusterAutoMergeThreshold    = 0.92
	coverageClusterEmbeddingProviderName = "openai"
	coverageClusterDefaultEmbeddingModel = "text-embedding-3-small"
)

type SupportCoverageClusterRebuildService struct {
	coverageRepo   *repository.SupportCoverageRepository
	embedder       llm.EmbeddingProvider
	embeddingModel string
}

type SupportCoverageClusterRebuildResult struct {
	RunID              string     `json:"run_id"`
	Status             string     `json:"status"`
	GapsScanned        int        `json:"gaps_scanned"`
	ClustersFound      int        `json:"clusters_found"`
	AutoMerged         int        `json:"auto_merged"`
	SuggestionsCreated int        `json:"suggestions_created"`
	Skipped            int        `json:"skipped"`
	StartedAt          time.Time  `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
}

type coverageClusterCandidate struct {
	Gap       model.SupportCoverageGapListItem
	Text      string
	Tokens    map[string]struct{}
	Embedding []float32
}

func NewSupportCoverageClusterRebuildService(coverageRepo *repository.SupportCoverageRepository, embedder llm.EmbeddingProvider, embeddingModel string) *SupportCoverageClusterRebuildService {
	return &SupportCoverageClusterRebuildService{
		coverageRepo:   coverageRepo,
		embedder:       embedder,
		embeddingModel: strings.TrimSpace(embeddingModel),
	}
}

func (s *SupportCoverageClusterRebuildService) RebuildWorkspace(ctx context.Context, workspaceID string) (*SupportCoverageClusterRebuildResult, error) {
	if s == nil || s.coverageRepo == nil {
		return nil, fmt.Errorf("coverage cluster rebuild service is not configured")
	}
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	now := time.Now().UTC()
	run := &model.SupportCoverageClusterRebuildRun{
		WorkspaceID: workspaceID,
		Status:      model.SupportCoverageClusterRebuildStatusRunning,
		StartedAt:   now,
		Metadata:    []byte("{}"),
	}
	if err := s.coverageRepo.CreateClusterRebuildRun(ctx, run); err != nil {
		return nil, err
	}

	result := &SupportCoverageClusterRebuildResult{
		RunID:     run.ID,
		Status:    model.SupportCoverageClusterRebuildStatusRunning,
		StartedAt: now,
	}

	items, err := s.coverageRepo.ListOpenGapsForClusterRebuild(ctx, workspaceID, coverageClusterRebuildLimit)
	if err != nil {
		_ = s.coverageRepo.FailClusterRebuildRun(ctx, run.ID, err)
		return nil, err
	}
	result.GapsScanned = len(items)
	candidates := make([]coverageClusterCandidate, 0, len(items))
	for _, item := range items {
		text := coverageClusterComparisonText(item)
		candidates = append(candidates, coverageClusterCandidate{
			Gap:    item,
			Text:   text,
			Tokens: coverageClusterTokens(text),
		})
	}
	s.addEmbeddings(ctx, candidates)

	merged := map[string]bool{}
	seenCluster := map[string]bool{}
	for i := 0; i < len(candidates); i++ {
		left := candidates[i]
		if merged[left.Gap.ID] {
			continue
		}
		for j := i + 1; j < len(candidates); j++ {
			right := candidates[j]
			if merged[right.Gap.ID] || !coverageClusterCompatible(left.Gap, right.Gap) {
				continue
			}
			score := coverageClusterSimilarity(left, right)
			if score < coverageClusterSuggestThreshold {
				continue
			}
			primary, duplicate := chooseCoverageClusterPrimary(left.Gap, right.Gap)
			clusterKey := primary.ID + ":" + duplicate.ID
			seenCluster[clusterKey] = true
			reason := coverageClusterReason(score, left.Gap, right.Gap)

			if score >= coverageClusterAutoMergeThreshold && coverageClusterStrongTargetMatch(left.Gap, right.Gap) {
				if err := s.coverageRepo.MergeGaps(ctx, workspaceID, duplicate.ID, primary.ID); err != nil {
					result.Skipped++
					continue
				}
				merged[duplicate.ID] = true
				result.AutoMerged++
				continue
			}

			metadata, _ := json.Marshal(map[string]any{
				"left_title":  left.Gap.Title,
				"right_title": right.Gap.Title,
			})
			created, err := s.coverageRepo.UpsertMergeSuggestion(ctx, &model.SupportCoverageGapMergeSuggestion{
				WorkspaceID:           workspaceID,
				RunID:                 &run.ID,
				SourceGapID:           duplicate.ID,
				TargetGapID:           primary.ID,
				Status:                model.SupportCoverageMergeSuggestionStatusPending,
				SimilarityScore:       score,
				Reason:                reason,
				CombinedEvidenceCount: primary.EvidenceCount + duplicate.EvidenceCount,
				Metadata:              metadata,
			})
			if err != nil {
				result.Skipped++
				continue
			}
			if created {
				result.SuggestionsCreated++
			}
		}
	}
	result.ClustersFound = len(seenCluster) + result.AutoMerged
	completedAt := time.Now().UTC()
	result.CompletedAt = &completedAt
	result.Status = model.SupportCoverageClusterRebuildStatusCompleted
	if err := s.coverageRepo.CompleteClusterRebuildRun(ctx, run.ID, result.GapsScanned, result.ClustersFound, result.AutoMerged, result.SuggestionsCreated, result.Skipped); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *SupportCoverageClusterRebuildService) LatestRun(ctx context.Context, workspaceID string) (*model.SupportCoverageClusterRebuildRun, error) {
	if s == nil || s.coverageRepo == nil {
		return nil, fmt.Errorf("coverage cluster rebuild service is not configured")
	}
	return s.coverageRepo.LatestClusterRebuildRun(ctx, workspaceID)
}

func (s *SupportCoverageClusterRebuildService) ListMergeSuggestionsForGap(ctx context.Context, workspaceID, gapID string) ([]model.SupportCoverageGapMergeSuggestion, error) {
	if s == nil || s.coverageRepo == nil {
		return nil, fmt.Errorf("coverage cluster rebuild service is not configured")
	}
	return s.coverageRepo.ListMergeSuggestionsForGap(ctx, workspaceID, gapID)
}

func (s *SupportCoverageClusterRebuildService) ApplyMergeSuggestion(ctx context.Context, workspaceID, suggestionID, userID string) error {
	if s == nil || s.coverageRepo == nil {
		return fmt.Errorf("coverage cluster rebuild service is not configured")
	}
	suggestion, err := s.coverageRepo.GetMergeSuggestion(ctx, workspaceID, suggestionID)
	if err != nil {
		return err
	}
	if suggestion.Status != model.SupportCoverageMergeSuggestionStatusPending {
		return fmt.Errorf("merge suggestion is no longer pending")
	}
	if err := s.coverageRepo.MergeGaps(ctx, workspaceID, suggestion.SourceGapID, suggestion.TargetGapID); err != nil {
		return err
	}
	return s.coverageRepo.MarkMergeSuggestionReviewed(ctx, workspaceID, suggestionID, model.SupportCoverageMergeSuggestionStatusApplied, userID)
}

func (s *SupportCoverageClusterRebuildService) DismissMergeSuggestion(ctx context.Context, workspaceID, suggestionID, userID string) error {
	if s == nil || s.coverageRepo == nil {
		return fmt.Errorf("coverage cluster rebuild service is not configured")
	}
	return s.coverageRepo.MarkMergeSuggestionReviewed(ctx, workspaceID, suggestionID, model.SupportCoverageMergeSuggestionStatusDismissed, userID)
}

func (s *SupportCoverageClusterRebuildService) addEmbeddings(ctx context.Context, candidates []coverageClusterCandidate) {
	if s == nil || s.embedder == nil || len(candidates) == 0 {
		return
	}
	modelName := s.embeddingModel
	if modelName == "" {
		modelName = coverageClusterDefaultEmbeddingModel
	}
	inputs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		inputs = append(inputs, candidate.Text)
	}
	resp, err := s.embedder.CreateEmbeddings(ctx, llm.EmbeddingRequest{
		Provider: coverageClusterEmbeddingProviderName,
		Model:    modelName,
		Inputs:   inputs,
	})
	if err != nil || resp == nil || len(resp.Vectors) != len(candidates) {
		return
	}
	for i := range candidates {
		candidates[i].Embedding = resp.Vectors[i]
	}
}

func coverageClusterComparisonText(gap model.SupportCoverageGapListItem) string {
	return strings.Join([]string{
		gap.CanonicalTitle,
		gap.Title,
		gap.TopicTitle,
		gap.GapKind,
		gap.GapCategory,
		gap.V1GapType,
		gap.FailureMode,
		gap.SourceSignal,
	}, " ")
}

func coverageClusterCompatible(a, b model.SupportCoverageGapListItem) bool {
	if coverageDisplayGapKind(a.GapKind) != coverageDisplayGapKind(b.GapKind) {
		return false
	}
	if a.RelatedArticleID != nil && b.RelatedArticleID != nil && *a.RelatedArticleID != *b.RelatedArticleID {
		return false
	}
	return true
}

func coverageClusterStrongTargetMatch(a, b model.SupportCoverageGapListItem) bool {
	return a.RelatedArticleID != nil && b.RelatedArticleID != nil && *a.RelatedArticleID == *b.RelatedArticleID
}

func coverageDisplayGapKind(kind string) string {
	if kind == "policy" {
		return "action"
	}
	if strings.TrimSpace(kind) == "" {
		return "content"
	}
	return strings.TrimSpace(kind)
}

func coverageClusterSimilarity(a, b coverageClusterCandidate) float64 {
	if len(a.Embedding) > 0 && len(a.Embedding) == len(b.Embedding) {
		return cosineSimilarity(a.Embedding, b.Embedding)
	}
	return lexicalCoverageSimilarity(a.Tokens, b.Tokens)
}

func cosineSimilarity(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return math.Max(0, dot/(math.Sqrt(normA)*math.Sqrt(normB)))
}

func lexicalCoverageSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for token := range a {
		if _, ok := b[token]; ok {
			intersection++
		}
	}
	containment := float64(intersection) / float64(min(len(a), len(b)))
	jaccard := float64(intersection) / float64(len(a)+len(b)-intersection)
	return (containment * 0.85) + (jaccard * 0.15)
}

func coverageClusterTokens(input string) map[string]struct{} {
	normalized := normalizeForCluster(input)
	tokens := map[string]struct{}{}
	for _, token := range strings.Fields(normalized) {
		token = strings.TrimSuffix(token, "s")
		if token == "" || coverageClusterRebuildStopword(token) {
			continue
		}
		tokens[token] = struct{}{}
	}
	return tokens
}

func coverageClusterRebuildStopword(token string) bool {
	switch token {
	case "article", "doc", "docs", "documentation", "guide", "instruction", "missing", "user", "customer", "cannot", "cant", "need", "needs":
		return true
	default:
		return false
	}
}

func chooseCoverageClusterPrimary(a, b model.SupportCoverageGapListItem) (model.SupportCoverageGapListItem, model.SupportCoverageGapListItem) {
	if b.EvidenceCount > a.EvidenceCount {
		return b, a
	}
	if b.EvidenceCount == a.EvidenceCount && b.LastSeenAt.After(a.LastSeenAt) {
		return b, a
	}
	return a, b
}

func coverageClusterReason(score float64, a, b model.SupportCoverageGapListItem) string {
	parts := []string{fmt.Sprintf("%.0f%% similar customer need", score*100)}
	if coverageDisplayGapKind(a.GapKind) == coverageDisplayGapKind(b.GapKind) {
		parts = append(parts, "same gap type")
	}
	if a.RelatedArticleID != nil && b.RelatedArticleID != nil && *a.RelatedArticleID == *b.RelatedArticleID {
		parts = append(parts, "same related article")
	}
	sort.Strings(parts[1:])
	return strings.Join(parts, "; ")
}
