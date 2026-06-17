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
	coverageClusterLexicalSuggest        = 0.45
	coverageClusterLexicalAutoMerge      = 0.82
	coverageClusterEmbeddingSuggest      = 0.72
	coverageClusterEmbeddingAutoMerge    = 0.88
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
	EmbeddingStatus    string     `json:"embedding_status"`
	EmbeddingError     string     `json:"embedding_error,omitempty"`
	StartedAt          time.Time  `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
}

type coverageClusterCandidate struct {
	Gap       model.SupportCoverageGapListItem
	Text      string
	Tokens    map[string]struct{}
	Embedding []float32
}

type coverageClusterScoredPair struct {
	left  int
	right int
	score float64
}

type coverageClusterUnionFind struct {
	parent []int
}

func newCoverageClusterUnionFind(size int) *coverageClusterUnionFind {
	parent := make([]int, size)
	for i := range parent {
		parent[i] = i
	}
	return &coverageClusterUnionFind{parent: parent}
}

func (u *coverageClusterUnionFind) find(index int) int {
	if u.parent[index] != index {
		u.parent[index] = u.find(u.parent[index])
	}
	return u.parent[index]
}

func (u *coverageClusterUnionFind) union(a, b int) {
	rootA := u.find(a)
	rootB := u.find(b)
	if rootA != rootB {
		u.parent[rootB] = rootA
	}
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
		RunID:           run.ID,
		Status:          model.SupportCoverageClusterRebuildStatusRunning,
		EmbeddingStatus: "unavailable",
		StartedAt:       now,
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
	embeddingStatus, embeddingError := s.addEmbeddings(ctx, candidates)
	result.EmbeddingStatus = embeddingStatus
	result.EmbeddingError = embeddingError

	uf := newCoverageClusterUnionFind(len(candidates))
	pairs := []coverageClusterScoredPair{}
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if !coverageClusterCompatible(candidates[i].Gap, candidates[j].Gap) {
				continue
			}
			score := coverageClusterSimilarity(candidates[i], candidates[j])
			if score < coverageClusterSuggestThresholdFor(candidates[i], candidates[j]) {
				continue
			}
			uf.union(i, j)
			pairs = append(pairs, coverageClusterScoredPair{left: i, right: j, score: score})
		}
	}
	groups := map[int][]int{}
	for i := range candidates {
		groups[uf.find(i)] = append(groups[uf.find(i)], i)
	}
	merged := map[string]bool{}
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		result.ClustersFound++
		primaryIndex := coverageClusterPrimaryIndex(group, candidates)
		primary := candidates[primaryIndex].Gap
		for _, idx := range group {
			if idx == primaryIndex || merged[candidates[idx].Gap.ID] {
				continue
			}
			duplicate := candidates[idx].Gap
			score := coverageClusterBestPairScore(primaryIndex, idx, pairs)
			if score == 0 {
				score = coverageClusterBestGroupScore(idx, group, pairs)
			}
			if score >= coverageClusterAutoMergeThresholdFor(candidates[primaryIndex], candidates[idx]) {
				if err := s.coverageRepo.MergeGaps(ctx, workspaceID, duplicate.ID, primary.ID); err != nil {
					result.Skipped++
					continue
				}
				merged[duplicate.ID] = true
				result.AutoMerged++
				continue
			}
			reason := coverageClusterReason(score, primary, duplicate)
			metadata, _ := json.Marshal(map[string]any{
				"left_title":  primary.Title,
				"right_title": duplicate.Title,
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

func (s *SupportCoverageClusterRebuildService) addEmbeddings(ctx context.Context, candidates []coverageClusterCandidate) (string, string) {
	if s == nil || s.embedder == nil || len(candidates) == 0 {
		return "unavailable", ""
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
		if err != nil {
			return "failed", err.Error()
		}
		return "failed", "embedding response did not match candidate count"
	}
	for i := range candidates {
		candidates[i].Embedding = resp.Vectors[i]
	}
	return "ok", ""
}

func coverageClusterComparisonText(gap model.SupportCoverageGapListItem) string {
	return strings.Join([]string{
		gap.CustomerNeedText,
		gap.EvidenceText,
		gap.CanonicalTitle,
		gap.Title,
		gap.TopicTitle,
	}, " ")
}

func coverageClusterCompatible(a, b model.SupportCoverageGapListItem) bool {
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
	aNeed := normalizeForCluster(a.Gap.CustomerNeedText)
	bNeed := normalizeForCluster(b.Gap.CustomerNeedText)
	if aNeed != "" && aNeed == bNeed {
		return 1
	}
	if len(a.Embedding) > 0 && len(a.Embedding) == len(b.Embedding) {
		return cosineSimilarity(a.Embedding, b.Embedding)
	}
	return lexicalCoverageSimilarity(a.Tokens, b.Tokens)
}

func coverageClusterSuggestThresholdFor(a, b coverageClusterCandidate) float64 {
	if coverageClusterUsesEmbeddings(a, b) {
		return coverageClusterEmbeddingSuggest
	}
	return coverageClusterLexicalSuggest
}

func coverageClusterAutoMergeThresholdFor(a, b coverageClusterCandidate) float64 {
	if coverageClusterUsesEmbeddings(a, b) {
		return coverageClusterEmbeddingAutoMerge
	}
	return coverageClusterLexicalAutoMerge
}

func coverageClusterUsesEmbeddings(a, b coverageClusterCandidate) bool {
	return len(a.Embedding) > 0 && len(a.Embedding) == len(b.Embedding)
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

func coverageClusterPrimaryIndex(group []int, candidates []coverageClusterCandidate) int {
	primaryIndex := group[0]
	for _, idx := range group[1:] {
		current := candidates[primaryIndex].Gap
		next := candidates[idx].Gap
		primary, _ := chooseCoverageClusterPrimary(current, next)
		if primary.ID == next.ID {
			primaryIndex = idx
		}
	}
	return primaryIndex
}

func coverageClusterBestPairScore(a, b int, pairs []coverageClusterScoredPair) float64 {
	best := 0.0
	for _, pair := range pairs {
		if (pair.left == a && pair.right == b) || (pair.left == b && pair.right == a) {
			if pair.score > best {
				best = pair.score
			}
		}
	}
	return best
}

func coverageClusterBestGroupScore(index int, group []int, pairs []coverageClusterScoredPair) float64 {
	best := 0.0
	for _, other := range group {
		if other == index {
			continue
		}
		if score := coverageClusterBestPairScore(index, other, pairs); score > best {
			best = score
		}
	}
	return best
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
