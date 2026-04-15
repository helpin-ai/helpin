package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// gapRule is the output of the v1 rule classifier.
type gapRule struct {
	DedupeKey   string
	GapCategory string
	V1GapType   string
	Title       string
	Confidence  float64
}

// classifyEvent applies deterministic v1 rules to a support event
// and returns a gap rule if the event should create/upsert a gap.
// Returns nil if no rule matches.
func classifyEvent(event *model.SupportEvent) *gapRule {
	switch event.EventType {

	case model.SupportEventAIHandoffTriggered:
		return classifyAIHandoff(event)

	case model.SupportEventArticleFeedback:
		return classifyArticleFeedback(event)

	case model.SupportEventWidgetSearchPerformed:
		return classifyWidgetSearch(event)

	case model.SupportEventDocsIssueFeedback:
		return classifyDocsIssueFeedback(event)

	case model.SupportEventHumanReplyAfterAI:
		return classifyHumanReplyAfterAI(event)

	default:
		return nil
	}
}

// classifyAIHandoff handles AI handoff events.
func classifyAIHandoff(event *model.SupportEvent) *gapRule {
	switch event.FailureMode {

	case model.SupportCoverageFailureNoRetrieval:
		if event.IssueKey != "" {
			return &gapRule{
				DedupeKey:   buildDedupeKey(event.WorkspaceID, model.SupportCoverageV1GapMissingArticle, event.IssueKey, ""),
				GapCategory: model.SupportCoverageGapCategoryKnowledge,
				V1GapType:   model.SupportCoverageV1GapMissingArticle,
				Title:       titleFromIssueKey(event.IssueKey, "Missing article"),
				Confidence:  0.8,
			}
		}
		return &gapRule{
			DedupeKey:   buildDedupeKey(event.WorkspaceID, model.SupportCoverageV1GapNeedsReview, "", hashExcerpt(event.IssueSummary)),
			GapCategory: model.SupportCoverageGapCategoryUnknown,
			V1GapType:   model.SupportCoverageV1GapNeedsReview,
			Title:       "Needs review: AI handoff with no retrieval",
			Confidence:  0.4,
		}

	case model.SupportCoverageFailureWeakRetrieval:
		docID := coverageDeref(event.DocumentID)
		if event.IssueKey != "" {
			return &gapRule{
				DedupeKey:   buildDedupeKey(event.WorkspaceID, model.SupportCoverageV1GapWeakArticle, event.IssueKey, docID),
				GapCategory: model.SupportCoverageGapCategoryStructure,
				V1GapType:   model.SupportCoverageV1GapWeakArticle,
				Title:       titleFromIssueKey(event.IssueKey, "Weak article"),
				Confidence:  0.7,
			}
		}
		return &gapRule{
			DedupeKey:   buildDedupeKey(event.WorkspaceID, model.SupportCoverageV1GapNeedsReview, "", hashExcerpt(event.IssueSummary)),
			GapCategory: model.SupportCoverageGapCategoryUnknown,
			V1GapType:   model.SupportCoverageV1GapNeedsReview,
			Title:       "Needs review: AI handoff with weak retrieval",
			Confidence:  0.4,
		}

	case model.SupportCoverageFailureLowConfidence:
		if event.IssueKey != "" {
			return &gapRule{
				DedupeKey:   buildDedupeKey(event.WorkspaceID, model.SupportCoverageV1GapNeedsReview, event.IssueKey, ""),
				GapCategory: model.SupportCoverageGapCategoryUnknown,
				V1GapType:   model.SupportCoverageV1GapNeedsReview,
				Title:       titleFromIssueKey(event.IssueKey, "Low confidence"),
				Confidence:  0.5,
			}
		}
		return nil

	case model.SupportCoverageFailureStuck:
		if event.IssueKey != "" {
			return &gapRule{
				DedupeKey:   buildDedupeKey(event.WorkspaceID, model.SupportCoverageV1GapMissingArticle, event.IssueKey, ""),
				GapCategory: model.SupportCoverageGapCategoryKnowledge,
				V1GapType:   model.SupportCoverageV1GapMissingArticle,
				Title:       titleFromIssueKey(event.IssueKey, "Repeated issue"),
				Confidence:  0.7,
			}
		}
		return nil

	default:
		return nil
	}
}

// classifyArticleFeedback handles unhelpful article feedback.
func classifyArticleFeedback(event *model.SupportEvent) *gapRule {
	// Only create gaps for negative feedback.
	if event.SourceSignal != "not_helpful" && event.SourceSignal != model.SupportCoverageSourceArticleFeedback {
		// Check metadata for rating.
		// For now, all article feedback events are assumed negative since
		// the emitter should only fire for unhelpful feedback.
	}

	docID := coverageDeref(event.DocumentID)
	if event.IssueKey != "" {
		return &gapRule{
			DedupeKey:   buildDedupeKey(event.WorkspaceID, model.SupportCoverageV1GapWeakArticle, event.IssueKey, docID),
			GapCategory: model.SupportCoverageGapCategoryStructure,
			V1GapType:   model.SupportCoverageV1GapWeakArticle,
			Title:       titleFromIssueKey(event.IssueKey, "Article marked unhelpful"),
			Confidence:  0.6,
		}
	}
	if docID != "" {
		return &gapRule{
			DedupeKey:   buildDedupeKey(event.WorkspaceID, model.SupportCoverageV1GapWeakArticle, "", docID),
			GapCategory: model.SupportCoverageGapCategoryStructure,
			V1GapType:   model.SupportCoverageV1GapWeakArticle,
			Title:       "Article marked unhelpful",
			Confidence:  0.5,
		}
	}
	return nil
}

// classifyWidgetSearch handles no-result searches.
func classifyWidgetSearch(event *model.SupportEvent) *gapRule {
	// Only trigger on zero results — check metadata.
	// The emitter should set source_signal to indicate no results.
	if event.SourceSignal != "no_results" {
		return nil
	}

	if event.IssueKey != "" {
		return &gapRule{
			DedupeKey:   buildDedupeKey(event.WorkspaceID, model.SupportCoverageV1GapMissingArticle, event.IssueKey, ""),
			GapCategory: model.SupportCoverageGapCategoryKnowledge,
			V1GapType:   model.SupportCoverageV1GapMissingArticle,
			Title:       titleFromIssueKey(event.IssueKey, "No search results"),
			Confidence:  0.6,
		}
	}
	return &gapRule{
		DedupeKey:   buildDedupeKey(event.WorkspaceID, model.SupportCoverageV1GapNeedsReview, "", hashExcerpt(event.IssueSummary)),
		GapCategory: model.SupportCoverageGapCategoryUnknown,
		V1GapType:   model.SupportCoverageV1GapNeedsReview,
		Title:       "No search results: " + coverageTruncate(event.IssueSummary, 80),
		Confidence:  0.3,
	}
}

// classifyDocsIssueFeedback handles agent "Docs issue? Yes" feedback.
func classifyDocsIssueFeedback(event *model.SupportEvent) *gapRule {
	if event.IssueKey != "" {
		return &gapRule{
			DedupeKey:   buildDedupeKey(event.WorkspaceID, model.SupportCoverageV1GapNeedsReview, event.IssueKey, ""),
			GapCategory: model.SupportCoverageGapCategoryUnknown,
			V1GapType:   model.SupportCoverageV1GapNeedsReview,
			Title:       titleFromIssueKey(event.IssueKey, "Agent flagged docs issue"),
			Confidence:  0.7,
		}
	}
	return &gapRule{
		DedupeKey:   buildDedupeKey(event.WorkspaceID, model.SupportCoverageV1GapNeedsReview, "", hashExcerpt(event.IssueSummary)),
		GapCategory: model.SupportCoverageGapCategoryUnknown,
		V1GapType:   model.SupportCoverageV1GapNeedsReview,
		Title:       "Agent flagged docs issue",
		Confidence:  0.6,
	}
}

// classifyHumanReplyAfterAI handles human resolution after AI failure.
func classifyHumanReplyAfterAI(event *model.SupportEvent) *gapRule {
	if event.IssueKey == "" {
		return nil // Not enough signal without issue key.
	}
	return &gapRule{
		DedupeKey:   buildDedupeKey(event.WorkspaceID, model.SupportCoverageV1GapNeedsReview, event.IssueKey, ""),
		GapCategory: model.SupportCoverageGapCategoryUnknown,
		V1GapType:   model.SupportCoverageV1GapNeedsReview,
		Title:       titleFromIssueKey(event.IssueKey, "Human resolved after AI failure"),
		Confidence:  0.5,
	}
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func buildDedupeKey(workspaceID, v1GapType, issueKey, extra string) string {
	parts := []string{v1GapType}
	if issueKey != "" {
		parts = append(parts, issueKey)
	}
	if extra != "" {
		parts = append(parts, extra)
	}
	return strings.Join(parts, ":")
}

func titleFromIssueKey(issueKey, prefix string) string {
	clean := strings.ReplaceAll(issueKey, "_", " ")
	clean = strings.ReplaceAll(clean, "-", " ")
	if prefix != "" {
		return fmt.Sprintf("%s: %s", prefix, clean)
	}
	return clean
}

func hashExcerpt(s string) string {
	if s == "" {
		return "empty"
	}
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(s))))
	return hex.EncodeToString(h[:8])
}

func coverageDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
