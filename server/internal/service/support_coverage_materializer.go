package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type CoverageMaterializationResult struct {
	FindingsScanned       int
	EmbeddingsCreated     int
	ExistingGapAttached   int
	NewGapsCreated        int
	SameRunFindingsMerged int
	EvidenceInserted      int
	AlreadyMaterialized   int
	MissingEmbeddings     int
}

type coverageMaterializedFinding struct {
	Analysis model.SupportCoverageConversationAnalysis
	Text     string
	Vector   []float32
}

func (s *SupportCoverageDailyAnalyzer) materializeRunFindings(ctx context.Context, workspaceID, runID string) (*CoverageMaterializationResult, error) {
	if s == nil || s.coverageRepo == nil || s.analysisRepo == nil {
		return nil, fmt.Errorf("coverage materializer dependencies are not configured")
	}
	if workspaceID == "" || runID == "" {
		return nil, fmt.Errorf("workspace_id and run_id are required")
	}
	result := &CoverageMaterializationResult{}
	analyses, err := s.analysisRepo.ListUnmaterializedGapAnalysesForRun(ctx, workspaceID, runID, 1000)
	if err != nil {
		return nil, err
	}
	result.FindingsScanned = len(analyses)
	if len(analyses) == 0 {
		return result, nil
	}
	findings, err := s.embedMaterializationFindings(ctx, analyses, result)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	remaining := make([]coverageMaterializedFinding, 0, len(findings))
	for _, finding := range findings {
		existing, err := s.matchExistingMaterializedGap(ctx, workspaceID, finding)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			closed, err := s.matchRecentClosedMaterializedGap(ctx, workspaceID, finding)
			if err != nil {
				return nil, err
			}
			if closed == nil {
				remaining = append(remaining, finding)
				continue
			}
			inserted, err := s.insertMaterializedEvidence(ctx, workspaceID, closed.ID, finding, now)
			if err != nil {
				return nil, err
			}
			result.ExistingGapAttached++
			if inserted {
				result.EvidenceInserted++
				if err := s.coverageRepo.MarkGapRecurrenceWatch(ctx, workspaceID, closed.ID, now); err != nil {
					return nil, err
				}
			} else {
				result.AlreadyMaterialized++
			}
			continue
		}
		if err := s.applyMaterializedGapKnowledgeMatch(ctx, workspaceID, existing.ID, finding, now); err != nil {
			return nil, err
		}
		inserted, err := s.insertMaterializedEvidence(ctx, workspaceID, existing.ID, finding, now)
		if err != nil {
			return nil, err
		}
		result.ExistingGapAttached++
		if inserted {
			result.EvidenceInserted++
		} else {
			result.AlreadyMaterialized++
		}
	}
	groups := coverageMaterializationGroups(remaining)
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		primary := coveragePrimaryFinding(group)
		gap, created, err := s.createMaterializedGap(ctx, workspaceID, primary, now)
		if err != nil {
			return nil, err
		}
		if created {
			result.NewGapsCreated++
		}
		if err := s.applyMaterializedGapKnowledgeMatch(ctx, workspaceID, gap.ID, primary, now); err != nil {
			return nil, err
		}
		if len(group) > 1 {
			result.SameRunFindingsMerged += len(group) - 1
		}
		for _, finding := range group {
			inserted, err := s.insertMaterializedEvidence(ctx, workspaceID, gap.ID, finding, now)
			if err != nil {
				return nil, err
			}
			if inserted {
				result.EvidenceInserted++
			} else {
				result.AlreadyMaterialized++
			}
		}
	}
	return result, nil
}

func (s *SupportCoverageDailyAnalyzer) matchExistingMaterializedGap(ctx context.Context, workspaceID string, finding coverageMaterializedFinding) (*model.SupportCoverageGapListItem, error) {
	vectorLiteral := coverageVectorLiteral(finding.Vector)
	modelName := coverageEmbeddingModel(s.embeddingModel)
	candidates, err := s.coverageRepo.FindNearestOpenGapsByEmbedding(ctx, workspaceID, vectorLiteral, coverageEmbeddingProviderName, modelName, coverageGapEmbeddingVersion, len(finding.Vector), 10)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		items, err := s.coverageRepo.ListOpenGapsForClusterRebuild(ctx, workspaceID, 1000)
		if err != nil {
			return nil, err
		}
		candidates = items
	}
	var best *model.SupportCoverageGapListItem
	bestScore := 0.0
	for _, candidate := range candidates {
		if candidate.Embedding == "" ||
			candidate.EmbeddingProvider != coverageEmbeddingProviderName ||
			candidate.EmbeddingModel != modelName ||
			candidate.EmbeddingVersion != coverageGapEmbeddingVersion ||
			candidate.EmbeddingDimensions != len(finding.Vector) {
			continue
		}
		score := coverageCosineSimilarity(finding.Vector, coverageParseVectorLiteral(candidate.Embedding))
		if score > bestScore {
			candidateCopy := candidate
			best = &candidateCopy
			bestScore = score
		}
	}
	if best == nil || bestScore < coverageSemanticAttachThreshold {
		return nil, nil
	}
	return best, nil
}

func (s *SupportCoverageDailyAnalyzer) matchRecentClosedMaterializedGap(ctx context.Context, workspaceID string, finding coverageMaterializedFinding) (*model.SupportCoverageGapListItem, error) {
	since := time.Now().AddDate(0, 0, -90)
	vectorLiteral := coverageVectorLiteral(finding.Vector)
	modelName := coverageEmbeddingModel(s.embeddingModel)
	candidates, err := s.coverageRepo.FindNearestRecentClosedGapsByEmbedding(ctx, workspaceID, vectorLiteral, coverageEmbeddingProviderName, modelName, coverageGapEmbeddingVersion, len(finding.Vector), since, 10)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		items, err := s.coverageRepo.ListRecentDoneGapsForRecurrence(ctx, workspaceID, since, 1000)
		if err != nil {
			return nil, err
		}
		candidates = items
	}
	var best *model.SupportCoverageGapListItem
	bestScore := 0.0
	for _, candidate := range candidates {
		if candidate.Embedding == "" ||
			candidate.EmbeddingProvider != coverageEmbeddingProviderName ||
			candidate.EmbeddingModel != modelName ||
			candidate.EmbeddingVersion != coverageGapEmbeddingVersion ||
			candidate.EmbeddingDimensions != len(finding.Vector) {
			continue
		}
		score := coverageCosineSimilarity(finding.Vector, coverageParseVectorLiteral(candidate.Embedding))
		if score > bestScore {
			candidateCopy := candidate
			best = &candidateCopy
			bestScore = score
		}
	}
	if best == nil || bestScore < coverageSemanticAttachThreshold {
		return nil, nil
	}
	return best, nil
}

func (s *SupportCoverageDailyAnalyzer) embedMaterializationFindings(ctx context.Context, analyses []model.SupportCoverageConversationAnalysis, result *CoverageMaterializationResult) ([]coverageMaterializedFinding, error) {
	if s.embeddingProvider == nil {
		result.MissingEmbeddings = len(analyses)
		return nil, fmt.Errorf("coverage embedding provider is not configured")
	}
	modelName := coverageEmbeddingModel(s.embeddingModel)
	inputs := make([]string, 0, len(analyses))
	for _, analysis := range analyses {
		inputs = append(inputs, coverageFindingEmbeddingText(analysis))
	}
	resp, err := s.embeddingProvider.CreateEmbeddings(ctx, llm.EmbeddingRequest{
		Provider: coverageEmbeddingProviderName,
		Model:    modelName,
		Inputs:   inputs,
	})
	if err != nil {
		result.MissingEmbeddings = len(analyses)
		return nil, fmt.Errorf("create coverage finding embeddings: %w", err)
	}
	if len(resp.Vectors) != len(analyses) {
		result.MissingEmbeddings = len(analyses)
		return nil, fmt.Errorf("coverage finding embedding count mismatch: got %d want %d", len(resp.Vectors), len(analyses))
	}
	findings := make([]coverageMaterializedFinding, 0, len(analyses))
	now := time.Now()
	for idx, analysis := range analyses {
		text := inputs[idx]
		vector := resp.Vectors[idx]
		vectorLiteral := coverageVectorLiteral(vector)
		textHash := coverageEmbeddingTextHash(text)
		if err := s.analysisRepo.UpdateAnalysisEmbedding(ctx, analysis.ID, vectorLiteral, coverageEmbeddingProviderName, modelName, coverageFindingEmbeddingVersion, len(vector), textHash, now); err != nil {
			return nil, err
		}
		analysis.Embedding = vectorLiteral
		analysis.EmbeddingProvider = coverageEmbeddingProviderName
		analysis.EmbeddingModel = modelName
		analysis.EmbeddingVersion = coverageFindingEmbeddingVersion
		analysis.EmbeddingDimensions = len(vector)
		analysis.EmbeddingTextHash = textHash
		analysis.EmbeddingUpdatedAt = &now
		findings = append(findings, coverageMaterializedFinding{
			Analysis: analysis,
			Text:     text,
			Vector:   vector,
		})
		result.EmbeddingsCreated++
	}
	return findings, nil
}

func coverageMaterializationGroups(findings []coverageMaterializedFinding) [][]coverageMaterializedFinding {
	if len(findings) == 0 {
		return nil
	}
	parent := make([]int, len(findings))
	for idx := range parent {
		parent[idx] = idx
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra := find(a)
		rb := find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}
	for i := 0; i < len(findings); i++ {
		for j := i + 1; j < len(findings); j++ {
			if coverageFindingsSameCluster(findings[i], findings[j]) {
				union(i, j)
			}
		}
	}
	byRoot := map[int][]coverageMaterializedFinding{}
	order := []int{}
	for idx, finding := range findings {
		root := find(idx)
		if _, ok := byRoot[root]; !ok {
			order = append(order, root)
		}
		byRoot[root] = append(byRoot[root], finding)
	}
	groups := make([][]coverageMaterializedFinding, 0, len(order))
	for _, root := range order {
		groups = append(groups, byRoot[root])
	}
	return groups
}

func coverageFindingsSameCluster(a, b coverageMaterializedFinding) bool {
	aNeed := coverageNormalizeEmbeddingText(a.Analysis.CustomerNeed)
	bNeed := coverageNormalizeEmbeddingText(b.Analysis.CustomerNeed)
	if aNeed != "" && aNeed == bNeed {
		return true
	}
	return coverageCosineSimilarity(a.Vector, b.Vector) >= coverageSameRunClusterThreshold
}

func coveragePrimaryFinding(group []coverageMaterializedFinding) coverageMaterializedFinding {
	primary := group[0]
	for _, finding := range group[1:] {
		if finding.Analysis.Confidence > primary.Analysis.Confidence {
			primary = finding
			continue
		}
		if finding.Analysis.Confidence == primary.Analysis.Confidence && finding.Analysis.ID < primary.Analysis.ID {
			primary = finding
		}
	}
	return primary
}

func (s *SupportCoverageDailyAnalyzer) createMaterializedGap(ctx context.Context, workspaceID string, primary coverageMaterializedFinding, now time.Time) (*model.SupportCoverageGap, bool, error) {
	title := coverageTruncate(firstNonEmptyCoverageString(primary.Analysis.CanonicalTitle, primary.Analysis.CustomerNeed, "Coverage gap"), 160)
	dedupeKey := coverageDeterministicClusterKey(workspaceID, primary.Text)
	if dedupeKey == "" {
		dedupeKey = "semantic:" + primary.Analysis.ID
	}
	metadata, _ := json.Marshal(map[string]any{"source": "daily_conversation_analysis", "primary_analysis_id": primary.Analysis.ID})
	gap := &model.SupportCoverageGap{
		ID:                  uuid.New().String(),
		WorkspaceID:         workspaceID,
		DedupeKey:           dedupeKey,
		GapKind:             firstNonEmptyCoverageString(primary.Analysis.GapKind, "content"),
		GapCategory:         firstNonEmptyCoverageString(primary.Analysis.GapCategory, model.SupportCoverageGapCategoryUnknown),
		V1GapType:           firstNonEmptyCoverageString(primary.Analysis.PrimaryRecommendationType, model.SupportCoverageV1GapNeedsReview),
		Title:               title,
		IssueKey:            dedupeKey,
		Status:              model.SupportCoverageGapStatusOpen,
		Confidence:          primary.Analysis.Confidence,
		SourceSignal:        model.SupportCoverageGapSourceDailyConversationAnalysis,
		Metadata:            metadata,
		FirstSeenAt:         now,
		LastSeenAt:          now,
		Embedding:           coverageVectorLiteral(primary.Vector),
		EmbeddingProvider:   coverageEmbeddingProviderName,
		EmbeddingModel:      coverageEmbeddingModel(s.embeddingModel),
		EmbeddingVersion:    coverageGapEmbeddingVersion,
		EmbeddingDimensions: len(primary.Vector),
		EmbeddingTextHash:   coverageEmbeddingTextHash(primary.Text),
		EmbeddingUpdatedAt:  &now,
	}
	return s.coverageRepo.UpsertOpenGapByDedupeKeyNoBump(ctx, gap)
}

func (s *SupportCoverageDailyAnalyzer) applyMaterializedGapKnowledgeMatch(ctx context.Context, workspaceID, gapID string, finding coverageMaterializedFinding, now time.Time) error {
	if s == nil || s.knowledgeMatcher == nil || gapID == "" {
		return nil
	}
	spaceIDs, err := s.externalDocsSpaceIDs(ctx, workspaceID)
	if err != nil {
		return err
	}
	contentSourceIDs, err := s.supportContentSourceIDs(ctx, workspaceID)
	if err != nil {
		return err
	}
	if len(spaceIDs) == 0 && len(contentSourceIDs) == 0 {
		return nil
	}
	query := firstNonEmptyCoverageString(finding.Text, finding.Analysis.CustomerNeed, finding.Analysis.CanonicalTitle)
	if query == "" {
		return nil
	}
	candidates, err := s.knowledgeMatcher.MatchGapKnowledge(ctx, workspaceID, spaceIDs, contentSourceIDs, query, coverageVectorLiteral(finding.Vector), 5)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}
	best := candidates[0]
	documentID := emptyToNil(best.DocumentID)
	failureMode := model.SupportCoverageFailureWeakRetrieval
	if documentID != nil {
		failureMode = model.SupportCoverageFailureNoRetrieval
	}
	if err := s.coverageRepo.UpdateGapKnowledgeMatch(ctx, workspaceID, gapID, failureMode, best.CombinedScore, documentID, best.Title, now); err != nil {
		return err
	}
	if documentID != nil {
		if err := s.coverageRepo.LinkGapArticle(ctx, gapID, *documentID, workspaceID); err != nil {
			return err
		}
	}
	return nil
}

func (s *SupportCoverageDailyAnalyzer) insertMaterializedEvidence(ctx context.Context, workspaceID, gapID string, finding coverageMaterializedFinding, now time.Time) (bool, error) {
	metadata, err := json.Marshal(map[string]any{
		"customer_need":            finding.Analysis.CustomerNeed,
		"ai_failure":               finding.Analysis.AIFailure,
		"human_resolution":         finding.Analysis.HumanResolution,
		"decision_reason":          finding.Analysis.DecisionReason,
		"conversation_analysis_id": finding.Analysis.ID,
	})
	if err != nil {
		return false, fmt.Errorf("marshal materialized evidence metadata: %w", err)
	}
	evidence := &model.SupportGapEvidence{
		GapID:          gapID,
		WorkspaceID:    workspaceID,
		EvidenceType:   model.SupportCoverageGapSourceDailyConversationAnalysis,
		ConversationID: emptyToNil(finding.Analysis.ConversationID),
		SourceSignal:   model.SupportCoverageGapSourceDailyConversationAnalysis,
		SourceKey:      coverageEvidenceSourceKey(finding.Analysis.ID),
		Excerpt:        coverageTruncate(firstNonEmptyCoverageString(finding.Analysis.CustomerNeed, finding.Analysis.DecisionReason, finding.Analysis.CanonicalTitle), 500),
		Metadata:       metadata,
		CreatedAt:      now,
	}
	inserted, err := s.coverageRepo.CreateEvidenceIfAbsent(ctx, evidence)
	if err != nil {
		return false, err
	}
	if inserted {
		if err := s.coverageRepo.IncrementGapEvidenceAfterInsert(ctx, workspaceID, gapID, now); err != nil {
			return false, err
		}
	}
	if err := s.analysisRepo.SetConversationAnalysisGap(ctx, finding.Analysis.ID, gapID, finding.Analysis.PrimaryRecommendationType); err != nil {
		return false, err
	}
	if inserted {
		if err := s.flagPotentialOverAttachment(ctx, workspaceID, gapID); err != nil {
			return false, err
		}
	}
	return inserted, nil
}

func (s *SupportCoverageDailyAnalyzer) flagPotentialOverAttachment(ctx context.Context, workspaceID, gapID string) error {
	if s == nil || s.analysisRepo == nil || s.coverageRepo == nil {
		return nil
	}
	analyses, err := s.analysisRepo.ListGapAnalysisEmbeddings(ctx, workspaceID, gapID, 20)
	if err != nil {
		return err
	}
	if len(analyses) < 6 {
		return nil
	}
	vectors := make([][]float32, 0, len(analyses))
	for _, analysis := range analyses {
		if analysis.EmbeddingProvider != coverageEmbeddingProviderName ||
			analysis.EmbeddingModel != coverageEmbeddingModel(s.embeddingModel) ||
			analysis.EmbeddingVersion != coverageFindingEmbeddingVersion ||
			analysis.EmbeddingDimensions <= 0 {
			continue
		}
		vector := coverageParseVectorLiteral(analysis.Embedding)
		if len(vector) == analysis.EmbeddingDimensions {
			vectors = append(vectors, vector)
		}
	}
	if len(vectors) < 6 {
		return nil
	}
	seedA := vectors[0]
	seedB := vectors[0]
	lowest := 1.0
	for _, vector := range vectors[1:] {
		score := coverageCosineSimilarity(seedA, vector)
		if score < lowest {
			lowest = score
			seedB = vector
		}
	}
	if lowest >= 0.55 {
		return nil
	}
	nearA := 0
	nearB := 0
	for _, vector := range vectors {
		if coverageCosineSimilarity(seedA, vector) >= 0.92 {
			nearA++
		}
		if coverageCosineSimilarity(seedB, vector) >= 0.92 {
			nearB++
		}
	}
	if nearA >= 3 && nearB >= 3 {
		return s.coverageRepo.MarkGapSplitReviewNeeded(ctx, workspaceID, gapID, "Evidence appears to contain two separate semantic clusters")
	}
	return nil
}
