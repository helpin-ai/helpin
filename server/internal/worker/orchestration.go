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
		return nil, fmt.Errorf("planning proposal did not include any stories")
	}
	for idx, story := range proposal.ProposedStories {
		if strings.TrimSpace(story.Name) == "" {
			return nil, fmt.Errorf("planning proposal story %d is missing a name", idx+1)
		}
		if strings.TrimSpace(story.Ref) == "" {
			proposal.ProposedStories[idx].Ref = fmt.Sprintf("story_%d", idx+1)
		}
	}
	return &proposal, nil
}

func extractOrchestrationProposal(messages []Message, epicID string, tokensUsed int) (*model.OrchestrationProposal, error) {
	responseText := latestAssistantText(messages)
	return extractPlanningProposalFromResponseText(responseText, epicID, "", tokensUsed)
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
