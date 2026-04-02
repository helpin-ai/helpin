package worker

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func extractProductSpecDraft(messages []Message) (*model.ProductSpecDraft, error) {
	responseText := latestAssistantText(messages)
	return extractProductSpecDraftFromResponseText(responseText)
}

func extractProductSpecDraftFromResponseText(responseText string) (*model.ProductSpecDraft, error) {
	if strings.TrimSpace(responseText) == "" {
		return nil, fmt.Errorf("product planner returned no spec draft")
	}

	var draft model.ProductSpecDraft
	if err := unmarshalLatestJSON(responseText, &draft); err != nil {
		return nil, fmt.Errorf("failed to parse product spec draft: %w", err)
	}
	if strings.TrimSpace(draft.Title) == "" {
		return nil, fmt.Errorf("product spec draft is missing a title")
	}
	if strings.TrimSpace(draft.SpecMarkdown) == "" {
		return nil, fmt.Errorf("product spec draft is missing spec_markdown")
	}
	return &draft, nil
}

func extractPlanningProposal(messages []Message, epicID, specVersionID string, tokensUsed int) (*model.OrchestrationProposal, error) {
	responseText := latestAssistantText(messages)
	return extractPlanningProposalFromResponseText(responseText, epicID, specVersionID, tokensUsed)
}

func extractPlanningProposalFromResponseText(responseText, epicID, specVersionID string, tokensUsed int) (*model.OrchestrationProposal, error) {
	if strings.TrimSpace(responseText) == "" {
		return nil, fmt.Errorf("product planner returned no planning proposal text")
	}

	var proposal model.OrchestrationProposal
	if err := unmarshalLatestJSON(responseText, &proposal); err != nil {
		return nil, fmt.Errorf("failed to parse planning proposal: %w", err)
	}

	proposal.EpicID = epicID
	proposal.SpecVersionID = strings.TrimSpace(firstNonEmpty(proposal.SpecVersionID, specVersionID))
	proposal.TokensUsed = tokensUsed
	if len(proposal.ProposedStories) == 0 {
		return nil, fmt.Errorf("planning proposal did not include any tasks")
	}
	for idx, story := range proposal.ProposedStories {
		if strings.TrimSpace(story.Name) == "" {
			return nil, fmt.Errorf("planning proposal task %d is missing a name", idx+1)
		}
		if strings.TrimSpace(story.Ref) == "" {
			proposal.ProposedStories[idx].Ref = fmt.Sprintf("task_%d", idx+1)
		}
	}
	return &proposal, nil
}

func NormalizeStoryPlanPreviewContent(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, fmt.Errorf("task plan content is empty")
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err == nil {
		if err := validateCanonicalStoryPlanPreviewPayload(payload); err != nil {
			return nil, err
		}
		normalized, _ := json.Marshal(payload)
		return normalized, nil
	}

	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, fmt.Errorf("task plan content must be a JSON object with summary and proposed_tasks")
	}
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, fmt.Errorf("task plan content is empty")
	}

	if err := unmarshalLatestJSON(encoded, &payload); err != nil {
		return nil, fmt.Errorf("task plan content must be a JSON object with summary and proposed_tasks")
	}
	if err := validateCanonicalStoryPlanPreviewPayload(payload); err != nil {
		return nil, err
	}

	normalized, _ := json.Marshal(payload)
	return normalized, nil
}

func validateCanonicalStoryPlanPreviewPayload(payload map[string]any) error {
	if len(payload) == 0 {
		return fmt.Errorf("task plan content must be a JSON object with summary and proposed_tasks")
	}

	summary, ok := payload["summary"].(string)
	if !ok || strings.TrimSpace(summary) == "" {
		return fmt.Errorf("task plan content must include a non-empty summary")
	}

	if _, ok := payload["proposed_tasks"].([]any); !ok {
		if _, legacyOK := payload["proposed_stories"].([]any); !legacyOK {
			return fmt.Errorf("task plan content must include proposed_tasks as an array")
		}
	}

	return nil
}

func extractOrchestrationProposal(messages []Message, epicID string, tokensUsed int) (*model.OrchestrationProposal, error) {
	responseText := latestAssistantText(messages)
	return extractPlanningProposalFromResponseText(responseText, epicID, "", tokensUsed)
}

func extractStoryCompletionAssessmentFromResponseText(responseText string) (*model.StoryCompletionAssessment, error) {
	if strings.TrimSpace(responseText) == "" {
		return nil, fmt.Errorf("task completion assessment returned no text")
	}
	var assessment model.StoryCompletionAssessment
	if err := unmarshalLatestJSON(responseText, &assessment); err != nil {
		return nil, fmt.Errorf("failed to parse task completion assessment: %w", err)
	}
	if strings.TrimSpace(assessment.Summary) == "" {
		return nil, fmt.Errorf("task completion assessment is missing a summary")
	}
	return &assessment, nil
}

func extractCRMDealReviewActionPlanFromResponseText(responseText string) (*model.CRMDealReviewActionPlan, error) {
	if strings.TrimSpace(responseText) == "" {
		return nil, fmt.Errorf("CRM deal review returned no text")
	}
	var plan model.CRMDealReviewActionPlan
	if err := unmarshalLatestJSON(responseText, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse CRM deal review plan: %w", err)
	}
	if strings.TrimSpace(plan.Summary) == "" {
		return nil, fmt.Errorf("CRM deal review plan is missing a summary")
	}
	return &plan, nil
}

func latestAssistantText(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "assistant" {
			continue
		}
		blocks, ok := messages[i].Content.([]ContentBlock)
		if !ok {
			if text, ok := messages[i].Content.(string); ok {
				return text
			}
			continue
		}
		var parts []string
		for _, block := range blocks {
			if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
				parts = append(parts, block.Text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}
	return ""
}

func unmarshalLatestJSON(raw string, target any) error {
	if err := json.Unmarshal([]byte(raw), target); err == nil {
		return nil
	}

	trimmed := trimJSONFences(raw)
	if err := json.Unmarshal([]byte(trimmed), target); err == nil {
		return nil
	}

	for _, candidate := range []string{
		extractJSONObject(raw),
		extractJSONObject(trimmed),
	} {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if err := json.Unmarshal([]byte(candidate), target); err == nil {
			return nil
		}
	}

	return json.Unmarshal([]byte(trimmed), target)
}

func trimJSONFences(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) < 3 {
		return trimmed
	}
	lines = lines[1:]
	if lines[len(lines)-1] == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	start := -1
	depth := 0
	inString := false
	escaped := false

	for idx, r := range trimmed {
		if start == -1 {
			if r == '{' {
				start = idx
				depth = 1
			}
			continue
		}

		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch r {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch r {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(trimmed[start : idx+1])
			}
		}
	}

	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func flowOutputKindFromRunInput(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var payload struct {
		FlowOutputKind string `json:"flow_output_kind"`
	}
	if err := json.Unmarshal(input, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.FlowOutputKind)
}
