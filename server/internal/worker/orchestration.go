package worker

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func extractOrchestrationProposal(messages []Message, epicID string, tokensUsed int) (*model.OrchestrationProposal, error) {
	responseText := latestAssistantText(messages)
	if strings.TrimSpace(responseText) == "" {
		return nil, fmt.Errorf("orchestrator returned no proposal text")
	}

	var proposal model.OrchestrationProposal
	if err := json.Unmarshal([]byte(responseText), &proposal); err != nil {
		trimmed := trimJSONFences(responseText)
		if trimmed == responseText {
			return nil, fmt.Errorf("failed to parse orchestration proposal: %w", err)
		}
		if err := json.Unmarshal([]byte(trimmed), &proposal); err != nil {
			return nil, fmt.Errorf("failed to parse orchestration proposal: %w", err)
		}
	}

	proposal.EpicID = epicID
	proposal.TokensUsed = tokensUsed
	if len(proposal.ProposedStories) == 0 {
		return nil, fmt.Errorf("orchestration proposal did not include any stories")
	}
	for idx, story := range proposal.ProposedStories {
		if strings.TrimSpace(story.Name) == "" {
			return nil, fmt.Errorf("orchestration proposal story %d is missing a name", idx+1)
		}
	}
	return &proposal, nil
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
