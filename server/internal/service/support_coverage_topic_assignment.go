package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	coverageTopicAutoConfidence = 0.75
	coverageTopicAttachScore    = 0.55
)

type CoverageTopicAssignmentDecision struct {
	Outcome       string
	TopicID       string
	Similarity    float64
	Compatibility float64
}

func assignCoverageV2Finding(ctx context.Context, repo *repository.CoverageV2Repository, finding *model.CoverageFinding) error {
	if repo == nil || finding == nil {
		return nil
	}
	topics, err := repo.ListOpenTopics(ctx, finding.WorkspaceID, 500)
	if err != nil {
		return fmt.Errorf("list canonical coverage topics: %w", err)
	}
	decision := DecideCoverageTopicAssignment(*finding, topics)
	attempt := &model.CoverageAssignmentAttempt{
		WorkspaceID: finding.WorkspaceID, FindingID: finding.ID, PolicyVersion: "v1",
		Outcome: decision.Outcome, Similarity: decision.Similarity, Compatibility: decision.Compatibility,
	}
	if decision.TopicID != "" {
		attempt.CandidateTopicID = &decision.TopicID
	}
	if err := repo.AppendAssignmentAttempt(ctx, attempt); err != nil {
		return fmt.Errorf("record coverage assignment: %w", err)
	}
	if decision.Outcome == model.CoverageAssignmentReview {
		if err := repo.UpdateFindingAssignmentStatus(ctx, finding.WorkspaceID, finding.ID, "review"); err != nil {
			return err
		}
		return repo.UpsertUnreviewedSignal(ctx, &model.CoverageUnreviewedSignal{
			WorkspaceID: finding.WorkspaceID, SourceKind: finding.SourceKind, SourceID: finding.SourceID,
			NormalizedQuery: finding.CustomerNeed, SignalKey: aiUsageStableHash("finding:" + finding.ID),
			Status: model.CoverageSignalUnreviewed, Confidence: finding.Confidence, FindingID: &finding.ID,
			ObservedAt: finding.CreatedAt,
		})
	}
	topicID := decision.TopicID
	if decision.Outcome == model.CoverageAssignmentCreate {
		canonicalKey := aiUsageStableHash(strings.Join(sortedCoverageTopicTokens(finding.CustomerNeed), " "))
		topic, createErr := repo.CreateTopic(ctx, &model.CoverageTopicV2{
			WorkspaceID: finding.WorkspaceID, CanonicalKey: canonicalKey,
			Title: coverageTruncate(finding.CustomerNeed, 160), CustomerNeed: finding.CustomerNeed,
			Status: model.CoverageTopicOpen, AssignmentPolicy: "v1",
		})
		if createErr != nil {
			return fmt.Errorf("create canonical coverage topic: %w", createErr)
		}
		topicID = topic.ID
	}
	if topicID == "" {
		return fmt.Errorf("assignment produced no topic")
	}
	if err := repo.SetCurrentMembership(ctx, &model.CoverageTopicMembership{
		WorkspaceID: finding.WorkspaceID, FindingID: finding.ID, TopicID: topicID,
		DecisionSource: model.CoverageMembershipAutomatic, Confidence: max(finding.Confidence, decision.Similarity),
		PolicyVersion: "v1", AssignmentAttemptID: &attempt.ID,
	}); err != nil {
		return fmt.Errorf("set coverage topic membership: %w", err)
	}
	if err := repo.UpdateFindingAssignmentStatus(ctx, finding.WorkspaceID, finding.ID, "assigned"); err != nil {
		return err
	}
	return repo.RefreshTopicCounts(ctx, finding.WorkspaceID, topicID)
}

func sortedCoverageTopicTokens(text string) []string {
	set := coverageTopicTokens(text)
	tokens := make([]string, 0, len(set))
	for token := range set {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	return tokens
}

// DecideCoverageTopicAssignment is deterministic. It chooses one reversible
// membership target; it never merges or deletes canonical topics.
func DecideCoverageTopicAssignment(finding model.CoverageFinding, candidates []model.CoverageTopicV2) CoverageTopicAssignmentDecision {
	if finding.Confidence < coverageTopicAutoConfidence {
		return CoverageTopicAssignmentDecision{Outcome: model.CoverageAssignmentReview}
	}
	ordered := append([]model.CoverageTopicV2(nil), candidates...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	best := CoverageTopicAssignmentDecision{Outcome: model.CoverageAssignmentCreate}
	for _, candidate := range ordered {
		score := coverageTopicTextSimilarity(finding.CustomerNeed, candidate.CustomerNeed)
		if score > best.Similarity {
			best.TopicID, best.Similarity, best.Compatibility = candidate.ID, score, 1
		}
	}
	if best.Similarity >= coverageTopicAttachScore {
		best.Outcome = model.CoverageAssignmentAttach
	} else {
		best.TopicID = ""
	}
	return best
}

var coverageTopicTokenPattern = regexp.MustCompile(`[a-z0-9]+`)

func coverageTopicTextSimilarity(left, right string) float64 {
	leftTokens := coverageTopicTokens(left)
	rightTokens := coverageTopicTokens(right)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}
	intersection := 0
	union := map[string]struct{}{}
	for token := range leftTokens {
		union[token] = struct{}{}
	}
	for token := range rightTokens {
		if _, ok := leftTokens[token]; ok {
			intersection++
		}
		union[token] = struct{}{}
	}
	return float64(intersection) / float64(len(union))
}

func coverageTopicTokens(text string) map[string]struct{} {
	stop := map[string]bool{"a": true, "an": true, "the": true, "their": true, "to": true, "of": true, "is": true, "are": true}
	tokens := map[string]struct{}{}
	for _, token := range coverageTopicTokenPattern.FindAllString(strings.ToLower(text), -1) {
		if stop[token] {
			continue
		}
		if strings.HasSuffix(token, "s") && len(token) > 4 {
			token = strings.TrimSuffix(token, "s")
		}
		tokens[token] = struct{}{}
	}
	return tokens
}
