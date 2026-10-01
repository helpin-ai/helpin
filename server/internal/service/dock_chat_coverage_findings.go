package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Keep the explanation and its provenance together in the immutable opening message.
func coverageDockFindings(detail *model.SupportCoverageGapDetail) *model.DockContextMessage {
	nextStep := coverageDockNextStep(detail)
	need, failure, resolution, reason := "Review the sources to confirm what customers need.", "The evidence needs review to confirm what is missing.", "", ""
	if explanation := detail.AnalysisExplanation; explanation != nil {
		need = firstNonEmptyCoverageContext(strings.TrimSpace(explanation.CustomerNeed), need)
		failure = firstNonEmptyCoverageContext(strings.TrimSpace(explanation.AIFailure), failure)
		resolution, reason = strings.TrimSpace(explanation.HumanResolution), strings.TrimSpace(explanation.DecisionReason)
	}
	parts := []string{"**Customer need:** " + need, "**What's missing:** " + failure}
	if resolution != "" {
		parts = append(parts, "**What worked:** "+resolution)
	}
	parts = append(parts, "**Next step:** "+nextStep)
	if detail.ImpactExplanation != "" {
		parts = append(parts, detail.ImpactExplanation+".")
	}
	if detail.Confidence < 0.6 {
		parts = append(parts, "The cause is uncertain. Confirm it with the sources before applying a fix.")
	}
	analysis := []string{}
	if reason != "" {
		analysis = append(analysis, reason)
	}
	confidence := "Low confidence"
	if detail.Confidence >= 0.7 {
		confidence = "High confidence"
	} else if detail.Confidence >= 0.4 {
		confidence = "Medium confidence"
	}
	analysis = append(analysis, fmt.Sprintf("%s · First seen %s · Last seen %s", confidence, detail.FirstSeenAt.Format(time.DateOnly), detail.LastSeenAt.Format(time.DateOnly)))
	for _, rec := range detail.Recommendations {
		label := firstNonEmptyCoverageContext(rec.TargetTitle, strings.ReplaceAll(rec.RecommendationType, "_", " "))
		if rec.Priority == "primary" {
			label += " · recommended"
		}
		fields := []string{"**" + label + "**", rec.Rationale}
		if rec.SuggestedChange != nextStep {
			fields = append(fields, rec.SuggestedChange)
		}
		fields = append(fields, rec.ImplementationNotes)
		if rec.TargetURL != "" {
			fields = append(fields, "[Open target]("+rec.TargetURL+")")
		}
		analysis = append(analysis, joinCoverageFindingParts(fields))
	}
	sources, refs := coverageDockSources(detail)
	total := detail.EvidenceAll
	if total == 0 {
		total = detail.EvidenceCount
	}
	sourceSummary := ""
	if len(detail.Evidence) < total {
		sourceSummary = fmt.Sprintf("Showing the latest %d of %d evidence records.", len(detail.Evidence), total)
	} else if len(detail.Evidence) == 0 {
		sourceSummary = "No source excerpts are available for this gap."
	}
	capturedAt := detail.LastSeenAt
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	return &model.DockContextMessage{
		Content: strings.Join(parts, "\n\n"), CapturedAt: capturedAt, References: refs,
		Sources: sources, SourceSummary: sourceSummary,
		Details: []model.DockContextDetail{{Label: "Analysis details", Content: strings.Join(analysis, "\n\n")}},
	}
}

func coverageDockSources(detail *model.SupportCoverageGapDetail) ([]model.DockContextSource, []model.DockEntityReference) {
	articleTitles := map[string]string{}
	for _, article := range detail.RelatedArticles {
		articleTitles[article.DocumentID] = article.ArticleTitle
	}
	sources := make([]model.DockContextSource, 0, len(detail.Evidence)+len(detail.RelatedArticles))
	seenDocuments := map[string]bool{}
	for index, evidence := range detail.Evidence {
		source := model.DockContextSource{ID: evidence.ID, Label: coverageDockEvidenceLabel(evidence.EvidenceType), Content: evidence.Excerpt}
		if evidence.MessageID != nil {
			roles := map[string]string{"ai": "AI reply", "user": "Team reply", "agent": "Team reply", "customer": "Customer message", "visitor": "Customer message"}
			source.Label = firstNonEmptyCoverageContext(roles[evidence.SenderRole], "Message")
		}
		if !evidence.CreatedAt.IsZero() {
			capturedAt := evidence.CreatedAt
			source.CapturedAt = &capturedAt
		}
		if evidence.ConversationID != nil {
			source.Reference = &model.DockEntityReference{EntityType: "support_conversation", EntityID: *evidence.ConversationID, DisplayTitle: coverageDockSourceTitle(evidence.Excerpt, fmt.Sprintf("Conversation %d", index+1))}
		} else if evidence.DocumentID != nil {
			seenDocuments[*evidence.DocumentID] = true
			source.Reference = &model.DockEntityReference{EntityType: "document", EntityID: *evidence.DocumentID, DisplayTitle: firstNonEmptyCoverageContext(articleTitles[*evidence.DocumentID], "Source article")}
		}
		sources = append(sources, source)
	}
	for _, article := range detail.RelatedArticles {
		if seenDocuments[article.DocumentID] {
			continue
		}
		seenDocuments[article.DocumentID] = true
		sources = append(sources, model.DockContextSource{ID: "document:" + article.DocumentID, Label: "Related guidance", Reference: &model.DockEntityReference{EntityType: "document", EntityID: article.DocumentID, DisplayTitle: firstNonEmptyCoverageContext(article.ArticleTitle, "Untitled article")}})
	}
	refs := make([]model.DockEntityReference, 0, 4)
	seen := map[string]bool{}
	for _, kind := range []string{"support_conversation", "document"} {
		count := 0
		for _, source := range sources {
			ref := source.Reference
			if ref == nil || ref.EntityType != kind || seen[kind+":"+ref.EntityID] {
				continue
			}
			seen[kind+":"+ref.EntityID] = true
			refs = append(refs, *ref)
			count++
			if count == 2 {
				break
			}
		}
	}
	return sources, refs
}

func coverageDockSourceTitle(excerpt, fallback string) string {
	text := strings.Join(strings.Fields(strings.NewReplacer("#", "", "*", "", "_", "", "`", "", ">", "").Replace(excerpt)), " ")
	runes := []rune(text)
	if len(runes) > 90 {
		return strings.TrimSpace(string(runes[:87])) + "…"
	}
	return firstNonEmptyCoverageContext(text, fallback)
}

func coverageDockEvidenceLabel(kind string) string {
	labels := map[string]string{"ai_answer_feedback": "Visitor Feedback", "ai_handoff_triggered": "AI Handoff", "article_feedback_submitted": "Article Feedback", "widget_search_performed": "Widget Search", "docs_issue_feedback": "Agent Feedback", "human_reply_after_ai": "Human Reply", "daily_conversation_analysis": "Daily Analysis"}
	return firstNonEmptyCoverageContext(labels[kind], kind)
}

func joinCoverageFindingParts(parts []string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, "\n\n")
}
