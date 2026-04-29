package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// SupportCoverageClusterer groups support events into durable topic-scoped
// coverage gaps.
type SupportCoverageClusterer struct {
	coverageRepo *repository.SupportCoverageRepository
	now          func() time.Time
}

func NewSupportCoverageClusterer(coverageRepo *repository.SupportCoverageRepository) *SupportCoverageClusterer {
	return &SupportCoverageClusterer{
		coverageRepo: coverageRepo,
		now:          time.Now,
	}
}

// stopwords is intentionally conservative. Over-normalizing support queries
// creates false cluster collisions that are harder to unwind than duplicates.
var coverageClusterStopwords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "as": {}, "at": {}, "be": {}, "by": {},
	"do": {}, "for": {}, "from": {}, "have": {}, "how": {}, "i": {}, "if": {},
	"in": {}, "is": {}, "it": {}, "my": {}, "of": {}, "on": {}, "or": {},
	"that": {}, "the": {}, "this": {}, "to": {}, "was": {}, "what": {},
	"when": {}, "where": {}, "why": {}, "with": {},
}

// normalizeForCluster lowercases, strips punctuation, removes stopwords,
// dedupes, and token-sorts so simple word-order variants cluster together.
func normalizeForCluster(s string) string {
	if s == "" {
		return ""
	}

	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, s)

	seen := map[string]struct{}{}
	tokens := make([]string, 0)
	for _, token := range strings.Fields(cleaned) {
		if _, skip := coverageClusterStopwords[token]; skip {
			continue
		}
		if _, duplicate := seen[token]; duplicate {
			continue
		}
		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}

	sort.Strings(tokens)
	return strings.Join(tokens, " ")
}

// computeClusterKey isolates clusters by workspace, signal type, document
// scope, and optional product issue key before applying text normalization.
func computeClusterKey(workspaceID, signalType, documentID, issueKey, summary string) string {
	parts := []string{
		workspaceID,
		signalType,
		documentID,
		issueKey,
		normalizeForCluster(summary),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func ComputeSupportCoverageClusterKey(workspaceID, signalType, documentID, issueKey, summary string) string {
	return computeClusterKey(workspaceID, signalType, documentID, issueKey, summary)
}

func (c *SupportCoverageClusterer) UpsertTopicGap(ctx context.Context, event *model.SupportEvent) (*model.SupportCoverageGap, error) {
	if event == nil {
		return nil, nil
	}
	rule := classifyEvent(event)
	if rule == nil {
		return nil, nil
	}

	now := c.now()
	documentID := coverageDeref(event.DocumentID)
	clusterKey := computeClusterKey(event.WorkspaceID, event.EventType, documentID, event.IssueKey, event.IssueSummary)
	title := event.IssueSummary
	if title == "" {
		title = rule.Title
	}

	topic, err := c.coverageRepo.UpsertTopicByClusterKey(ctx, event.WorkspaceID, clusterKey, coverageTruncate(title, 160))
	if err != nil {
		return nil, err
	}

	gap := &model.SupportCoverageGap{
		WorkspaceID:  event.WorkspaceID,
		TopicID:      &topic.ID,
		DedupeKey:    clusterKey,
		GapKind:      "content",
		GapCategory:  rule.GapCategory,
		V1GapType:    rule.V1GapType,
		Title:        rule.Title,
		IssueKey:     event.IssueKey,
		Status:       model.SupportCoverageGapStatusOpen,
		Confidence:   rule.Confidence,
		FailureMode:  event.FailureMode,
		SourceSignal: event.SourceSignal,
		CanAnswer:    event.CanAnswer,
		CanResolve:   event.CanResolve,
		Metadata:     []byte(`{"source":"event_detection"}`),
		FirstSeenAt:  now,
		LastSeenAt:   now,
	}

	upserted, _, err := c.coverageRepo.UpsertOpenGapByTopic(ctx, gap)
	if err != nil {
		return nil, err
	}

	evidence := &model.SupportGapEvidence{
		GapID:           upserted.ID,
		WorkspaceID:     event.WorkspaceID,
		EvidenceType:    event.EventType,
		ConversationID:  event.ConversationID,
		MessageID:       event.MessageID,
		WidgetSessionID: event.WidgetSessionID,
		DocumentID:      event.DocumentID,
		ArticlePublicID: event.ArticlePublicID,
		SourceSignal:    event.SourceSignal,
		Excerpt:         coverageTruncate(event.IssueSummary, 500),
		CreatedAt:       now,
	}
	if err := c.coverageRepo.CreateEvidence(ctx, evidence); err != nil {
		return nil, err
	}

	if event.DocumentID != nil && *event.DocumentID != "" {
		if err := c.coverageRepo.LinkGapArticle(ctx, upserted.ID, *event.DocumentID, event.WorkspaceID); err != nil {
			return nil, err
		}
	}

	return upserted, nil
}
