package service

import (
	"encoding/json"
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
	shown := 0
	for _, source := range sources {
		if source.Reference != nil && source.Reference.EntityType == "support_conversation" {
			shown++
		}
	}
	sourceSummary := ""
	if shown > 0 && shown < detail.ConversationsAll {
		sourceSummary = fmt.Sprintf("Showing the latest %d of %d conversations.", shown, detail.ConversationsAll)
	} else if len(detail.Evidence) == 0 {
		sourceSummary = "No source conversations are available for this gap."
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
	conversationSource := map[string]int{}
	conversationEvidence := map[string][]model.SupportGapEvidenceView{}
	for _, evidence := range detail.Evidence {
		if evidence.ConversationID != nil {
			id := *evidence.ConversationID
			if _, seen := conversationSource[id]; !seen {
				conversationSource[id] = len(sources)
				sources = append(sources, model.DockContextSource{})
			}
			conversationEvidence[id] = append(conversationEvidence[id], evidence)
			continue
		}
		source := model.DockContextSource{ID: evidence.ID, Label: coverageDockEvidenceLabel(evidence.EvidenceType), Content: evidence.Excerpt}
		if !evidence.CreatedAt.IsZero() {
			capturedAt := evidence.CreatedAt
			source.CapturedAt = &capturedAt
		}
		if evidence.DocumentID != nil {
			seenDocuments[*evidence.DocumentID] = true
			source.Reference = &model.DockEntityReference{EntityType: "document", EntityID: *evidence.DocumentID, DisplayTitle: firstNonEmptyCoverageContext(articleTitles[*evidence.DocumentID], "Source article")}
		}
		sources = append(sources, source)
	}
	for id, index := range conversationSource {
		sources[index] = coverageDockConversationSource(id, conversationEvidence[id])
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

// coverageDockConversationSource summarizes one source conversation — who
// asked, what they needed and why the AI could not resolve it — instead of
// replaying its messages. Items arrive newest first.
func coverageDockConversationSource(conversationID string, items []model.SupportGapEvidenceView) model.DockContextSource {
	var need, failure, customerMessage, earliestExcerpt, name, email string
	var latest time.Time
	for _, evidence := range items {
		if evidence.CreatedAt.After(latest) {
			latest = evidence.CreatedAt
		}
		name = firstNonEmptyCoverageContext(name, strings.TrimSpace(evidence.CustomerName))
		email = firstNonEmptyCoverageContext(email, strings.TrimSpace(evidence.CustomerEmail))
		if need == "" && failure == "" {
			need, failure = coverageEvidenceExplanation(evidence.Metadata)
		}
		excerpt := strings.Join(strings.Fields(evidence.Excerpt), " ")
		if excerpt == "" {
			continue
		}
		earliestExcerpt = excerpt
		if evidence.MessageID != nil && (evidence.SenderRole == "customer" || evidence.SenderRole == "visitor") {
			customerMessage = excerpt
		}
	}
	asked := firstNonEmptyCoverageContext(customerMessage, earliestExcerpt)
	parts := []string{}
	if need != "" {
		parts = append(parts, "**Summary:** "+need)
	} else if asked != "" {
		parts = append(parts, "**Customer asked:** "+coverageDockTruncate(asked, 280))
	}
	if failure != "" {
		parts = append(parts, "**Why the AI couldn't solve it:** "+failure)
	}
	label := "Customer"
	switch {
	case name != "" && email != "" && !strings.EqualFold(name, email):
		label = name + " · " + email
	case name != "" || email != "":
		label = firstNonEmptyCoverageContext(name, email)
	}
	source := model.DockContextSource{
		ID:      "conversation:" + conversationID,
		Label:   label,
		Content: strings.Join(parts, "\n\n"),
		Reference: &model.DockEntityReference{
			EntityType:   "support_conversation",
			EntityID:     conversationID,
			DisplayTitle: coverageDockSourceTitle(firstNonEmptyCoverageContext(need, asked), "Conversation"),
		},
	}
	if !latest.IsZero() {
		source.CapturedAt = &latest
	}
	return source
}

// coverageEvidenceExplanation reads the analysis summary stored on
// daily-analysis evidence.
func coverageEvidenceExplanation(raw json.RawMessage) (need, failure string) {
	var metadata struct {
		CustomerNeed string `json:"customer_need"`
		AIFailure    string `json:"ai_failure"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &metadata) != nil {
		return "", ""
	}
	return strings.TrimSpace(metadata.CustomerNeed), strings.TrimSpace(metadata.AIFailure)
}

func coverageDockSourceTitle(excerpt, fallback string) string {
	text := strings.Join(strings.Fields(strings.NewReplacer("#", "", "*", "", "_", "", "`", "", ">", "").Replace(excerpt)), " ")
	return firstNonEmptyCoverageContext(coverageDockTruncate(text, 90), fallback)
}

func coverageDockTruncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) > limit {
		return strings.TrimSpace(string(runes[:limit-3])) + "…"
	}
	return text
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
