package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	coverageClusterRebuildLimit           = 1000
	coverageClusterEmbeddingSuggest       = 0.72
	coverageClusterEmbeddingAutoMerge     = 0.88
	coverageClusterCreationDedupeLimit    = 200
	coverageClusterDecisionResurfaceDelta = 0.05
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
	EmbeddingsCreated  int        `json:"embeddings_created"`
	MissingEmbeddings  int        `json:"missing_embeddings"`
	StartedAt          time.Time  `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
}

type coverageClusterCandidate struct {
	Gap       model.SupportCoverageGapListItem
	Text      string
	Embedding []float32
}

type coverageClusterScoredPair struct {
	left    int
	right   int
	score   float64
	pairKey string
}

type coverageClusterAcceptedPair struct {
	left  int
	right int
	score float64
}

type coverageClusterEmbeddingSummary struct {
	status         string
	errMessage     string
	created        int
	missing        int
	candidatesUsed int
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
		text := coverageGapEmbeddingText(item)
		candidates = append(candidates, coverageClusterCandidate{
			Gap:       item,
			Text:      text,
			Embedding: coverageParseVectorLiteral(item.Embedding),
		})
	}
	embeddingSummary, err := s.ensureGapEmbeddings(ctx, candidates)
	result.EmbeddingStatus = embeddingSummary.status
	result.EmbeddingError = embeddingSummary.errMessage
	result.EmbeddingsCreated = embeddingSummary.created
	result.MissingEmbeddings = embeddingSummary.missing
	if err != nil {
		metadata := coverageClusterRunMetadata(embeddingSummary)
		_ = s.coverageRepo.FailClusterRebuildRunWithMetadata(ctx, run.ID, err, metadata)
		return nil, err
	}

	decisions, err := s.coverageRepo.ListKeepSeparatePairDecisions(ctx, workspaceID)
	if err != nil {
		metadata := coverageClusterRunMetadata(embeddingSummary)
		_ = s.coverageRepo.FailClusterRebuildRunWithMetadata(ctx, run.ID, err, metadata)
		return nil, err
	}
	decisionByPair := map[string]model.SupportCoverageGapPairDecision{}
	for _, decision := range decisions {
		decisionByPair[decision.PairKey] = decision
	}
	cannotLink := map[string]map[string]bool{}
	pairs := []coverageClusterScoredPair{}
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if !coverageClusterCompatible(candidates[i].Gap, candidates[j].Gap) {
				continue
			}
			pairKey := repository.CoverageGapPairKey(candidates[i].Gap.ID, candidates[j].Gap.ID)
			score := coverageClusterSimilarity(candidates[i], candidates[j])
			if decision, ok := decisionByPair[pairKey]; ok && !coverageClusterDecisionIsStale(decision, candidates[i], candidates[j], score) {
				coverageClusterAddCannotLink(cannotLink, candidates[i].Gap.ID, candidates[j].Gap.ID)
				continue
			}
			if score < coverageClusterSuggestThresholdFor(candidates[i], candidates[j]) {
				continue
			}
			pairs = append(pairs, coverageClusterScoredPair{left: i, right: j, score: score, pairKey: pairKey})
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].score == pairs[j].score {
			return pairs[i].pairKey < pairs[j].pairKey
		}
		return pairs[i].score > pairs[j].score
	})
	uf := newCoverageClusterUnionFind(len(candidates))
	members := make(map[int][]int, len(candidates))
	acceptedPairs := make([]coverageClusterAcceptedPair, 0, len(pairs))
	for i := range candidates {
		members[i] = []int{i}
	}
	for _, pair := range pairs {
		leftRoot := uf.find(pair.left)
		rightRoot := uf.find(pair.right)
		if leftRoot == rightRoot {
			continue
		}
		if len(members[leftRoot]) < len(members[rightRoot]) {
			leftRoot, rightRoot = rightRoot, leftRoot
		}
		if !coverageClusterCanUnion(members[leftRoot], members[rightRoot], candidates, cannotLink) {
			continue
		}
		uf.parent[rightRoot] = leftRoot
		members[leftRoot] = append(members[leftRoot], members[rightRoot]...)
		delete(members, rightRoot)
		acceptedPairs = append(acceptedPairs, coverageClusterAcceptedPair{
			left:  pair.left,
			right: pair.right,
			score: pair.score,
		})
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
			directScore := coverageClusterBestPairScore(primaryIndex, idx, pairs)
			if directScore >= coverageClusterAutoMergeThresholdFor(candidates[primaryIndex], candidates[idx]) && coverageClusterAutoMergeAllowed(primary, duplicate) {
				if err := s.coverageRepo.MergeGaps(ctx, workspaceID, duplicate.ID, primary.ID); err != nil {
					result.Skipped++
					continue
				}
				merged[duplicate.ID] = true
				result.AutoMerged++
			}
		}
	}

	// Suggestions must always describe the directly scored pair. The accepted
	// pairs form a cannot-link-safe spanning forest, so they retain useful
	// cluster coverage without projecting a neighbor's score onto a transitive
	// primary/member pair.
	for _, pair := range acceptedPairs {
		left := candidates[pair.left].Gap
		right := candidates[pair.right].Gap
		if merged[left.ID] || merged[right.ID] {
			continue
		}
		target, source := chooseCoverageClusterPrimary(left, right)
		reason := coverageClusterReason(pair.score, target, source)
		metadata, _ := json.Marshal(map[string]any{
			"left_title":       left.Title,
			"right_title":      right.Title,
			"similarity_basis": "direct_pair",
		})
		created, err := s.coverageRepo.UpsertMergeSuggestion(ctx, &model.SupportCoverageGapMergeSuggestion{
			WorkspaceID:           workspaceID,
			RunID:                 &run.ID,
			SourceGapID:           source.ID,
			TargetGapID:           target.ID,
			Status:                model.SupportCoverageMergeSuggestionStatusPending,
			SimilarityScore:       pair.score,
			Reason:                reason,
			CombinedEvidenceCount: left.EvidenceCount + right.EvidenceCount,
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
	completedAt := time.Now().UTC()
	result.CompletedAt = &completedAt
	result.Status = model.SupportCoverageClusterRebuildStatusCompleted
	metadata := coverageClusterRunMetadata(embeddingSummary)
	if err := s.coverageRepo.CompleteClusterRebuildRun(ctx, run.ID, result.GapsScanned, result.ClustersFound, result.AutoMerged, result.SuggestionsCreated, result.Skipped, metadata); err != nil {
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
	return s.coverageRepo.KeepSeparateMergeSuggestion(ctx, workspaceID, suggestionID, userID)
}

func (s *SupportCoverageClusterRebuildService) ensureGapEmbeddings(ctx context.Context, candidates []coverageClusterCandidate) (coverageClusterEmbeddingSummary, error) {
	summary := coverageClusterEmbeddingSummary{
		status:         "complete",
		candidatesUsed: len(candidates),
	}
	if len(candidates) == 0 {
		return summary, nil
	}
	modelName := coverageEmbeddingModel(s.embeddingModel)
	type pendingGapEmbedding struct {
		index int
		text  string
		hash  string
	}
	pending := []pendingGapEmbedding{}
	for idx := range candidates {
		textHash := coverageEmbeddingTextHash(candidates[idx].Text)
		if textHash == "" {
			summary.missing++
			continue
		}
		gap := candidates[idx].Gap.SupportCoverageGap
		if gap.Embedding != "" &&
			gap.EmbeddingProvider == coverageEmbeddingProviderName &&
			gap.EmbeddingModel == modelName &&
			gap.EmbeddingVersion == coverageGapEmbeddingVersion &&
			gap.EmbeddingTextHash == textHash &&
			gap.EmbeddingDimensions > 0 &&
			len(candidates[idx].Embedding) == gap.EmbeddingDimensions {
			continue
		}
		pending = append(pending, pendingGapEmbedding{index: idx, text: candidates[idx].Text, hash: textHash})
	}
	if len(pending) == 0 {
		return summary, nil
	}
	if s == nil || s.embedder == nil {
		summary.status = "failed"
		summary.missing = len(pending)
		summary.errMessage = "embedding provider is not configured"
		return summary, errors.New(summary.errMessage)
	}
	inputs := make([]string, 0, len(pending))
	for _, item := range pending {
		inputs = append(inputs, item.text)
	}
	resp, err := s.embedder.CreateEmbeddings(ctx, llm.EmbeddingRequest{
		Provider: coverageEmbeddingProviderName,
		Model:    modelName,
		Inputs:   inputs,
	})
	if err != nil {
		summary.status = "failed"
		summary.missing = len(pending)
		summary.errMessage = err.Error()
		return summary, fmt.Errorf("embed coverage gaps for cluster rebuild: %w", err)
	}
	if resp == nil || len(resp.Vectors) != len(pending) {
		summary.status = "failed"
		summary.missing = len(pending)
		summary.errMessage = "embedding response did not match candidate count"
		return summary, errors.New(summary.errMessage)
	}
	now := time.Now().UTC()
	for idx, item := range pending {
		vector := resp.Vectors[idx]
		if len(vector) == 0 {
			summary.status = "failed"
			summary.missing = len(pending) - summary.created
			summary.errMessage = "embedding response contained an empty vector"
			return summary, errors.New(summary.errMessage)
		}
		if err := s.coverageRepo.UpdateGapEmbedding(ctx, candidates[item.index].Gap.ID, coverageVectorLiteral(vector), coverageEmbeddingProviderName, modelName, coverageGapEmbeddingVersion, len(vector), item.hash, now); err != nil {
			summary.status = "failed"
			summary.missing = len(pending) - summary.created
			summary.errMessage = err.Error()
			return summary, err
		}
		candidates[item.index].Embedding = vector
		candidates[item.index].Gap.Embedding = coverageVectorLiteral(vector)
		candidates[item.index].Gap.EmbeddingProvider = coverageEmbeddingProviderName
		candidates[item.index].Gap.EmbeddingModel = modelName
		candidates[item.index].Gap.EmbeddingVersion = coverageGapEmbeddingVersion
		candidates[item.index].Gap.EmbeddingDimensions = len(vector)
		candidates[item.index].Gap.EmbeddingTextHash = item.hash
		candidates[item.index].Gap.EmbeddingUpdatedAt = &now
		summary.created++
	}
	return summary, nil
}

func coverageClusterDecisionIsStale(decision model.SupportCoverageGapPairDecision, a, b coverageClusterCandidate, score float64) bool {
	currentHashes := map[string]string{
		a.Gap.ID: a.Gap.EmbeddingTextHash,
		b.Gap.ID: b.Gap.EmbeddingTextHash,
	}
	leftChanged := currentHashes[decision.GapAID] != decision.GapATextHash
	rightChanged := currentHashes[decision.GapBID] != decision.GapBTextHash
	if !leftChanged && !rightChanged {
		return false
	}
	if score >= decision.SimilarityAtDecision+coverageClusterDecisionResurfaceDelta {
		return true
	}
	autoThreshold := coverageClusterAutoMergeThresholdFor(a, b)
	return score >= autoThreshold && decision.SimilarityAtDecision < autoThreshold
}

func coverageClusterAddCannotLink(cannotLink map[string]map[string]bool, leftID, rightID string) {
	if cannotLink[leftID] == nil {
		cannotLink[leftID] = map[string]bool{}
	}
	if cannotLink[rightID] == nil {
		cannotLink[rightID] = map[string]bool{}
	}
	cannotLink[leftID][rightID] = true
	cannotLink[rightID][leftID] = true
}

func coverageClusterCanUnion(leftMembers, rightMembers []int, candidates []coverageClusterCandidate, cannotLink map[string]map[string]bool) bool {
	if len(leftMembers) == 0 || len(rightMembers) == 0 || len(cannotLink) == 0 {
		return true
	}
	smaller := leftMembers
	larger := rightMembers
	if len(smaller) > len(larger) {
		smaller, larger = larger, smaller
	}
	largerIDs := map[string]bool{}
	for _, idx := range larger {
		largerIDs[candidates[idx].Gap.ID] = true
	}
	for _, idx := range smaller {
		leftID := candidates[idx].Gap.ID
		for blockedID := range cannotLink[leftID] {
			if largerIDs[blockedID] {
				return false
			}
		}
	}
	return true
}

func coverageClusterStrongTargetMatch(a, b model.SupportCoverageGapListItem) bool {
	return a.RelatedArticleID != nil && b.RelatedArticleID != nil && *a.RelatedArticleID == *b.RelatedArticleID
}

func coverageClusterAutoMergeAllowed(a, b model.SupportCoverageGapListItem) bool {
	if coverageClusterStrongTargetMatch(a, b) {
		return true
	}
	aNeed := normalizeForCluster(a.CustomerNeedText)
	bNeed := normalizeForCluster(b.CustomerNeedText)
	return aNeed != "" && aNeed == bNeed
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
		return coverageCosineSimilarity(a.Embedding, b.Embedding)
	}
	return 0
}

func coverageClusterSuggestThresholdFor(a, b coverageClusterCandidate) float64 {
	if coverageClusterUsesEmbeddings(a, b) {
		return coverageClusterEmbeddingSuggest
	}
	return 1.01
}

func coverageClusterAutoMergeThresholdFor(a, b coverageClusterCandidate) float64 {
	if coverageClusterUsesEmbeddings(a, b) {
		return coverageClusterEmbeddingAutoMerge
	}
	return 1.01
}

func coverageClusterUsesEmbeddings(a, b coverageClusterCandidate) bool {
	return len(a.Embedding) > 0 && len(a.Embedding) == len(b.Embedding)
}

func coverageClusterRunMetadata(summary coverageClusterEmbeddingSummary) json.RawMessage {
	if summary.status == "" {
		summary.status = "complete"
	}
	payload := map[string]any{
		"embedding_status":          summary.status,
		"embeddings_created":        summary.created,
		"missing_embeddings":        summary.missing,
		"vector_candidates_scanned": summary.candidatesUsed,
		"auto_merge_policy":         "strict-v1",
	}
	if summary.errMessage != "" {
		payload["embedding_error"] = summary.errMessage
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
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
