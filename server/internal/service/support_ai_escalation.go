package service

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func countAgentAITurns(history []model.SupportMessage, agentID string) int {
	count := 0
	for _, msg := range history {
		if isAIReplyFromAgent(msg, agentID) {
			count++
		}
	}
	return count
}

func countMaxFollowupAITurns(history []model.SupportMessage, agentID string) int {
	count := 0
	for _, msg := range history {
		if !isAIReplyFromAgent(msg, agentID) {
			continue
		}
		if inferAIReplyKind(msg) != supportReplyKindAnswer {
			continue
		}
		count++
	}
	return count
}

func isAIReplyFromAgent(msg model.SupportMessage, agentID string) bool {
	if msg.SenderType != "ai" {
		return false
	}
	if agentID == "" {
		return true
	}
	if msg.SenderAgentID == nil {
		return false
	}
	return strings.TrimSpace(*msg.SenderAgentID) == agentID
}

func parseAIMessageMetadata(msg model.SupportMessage) (AIMessageMetadata, bool) {
	if strings.TrimSpace(msg.Metadata) == "" {
		return AIMessageMetadata{}, false
	}
	var meta AIMessageMetadata
	if err := json.Unmarshal([]byte(msg.Metadata), &meta); err != nil {
		return AIMessageMetadata{}, false
	}
	if !meta.AIAutoReply {
		return AIMessageMetadata{}, false
	}
	return meta, true
}

func inferAIReplyKind(msg model.SupportMessage) string {
	var meta AIMessageMetadata
	if strings.TrimSpace(msg.Metadata) != "" {
		if err := json.Unmarshal([]byte(msg.Metadata), &meta); err == nil && strings.TrimSpace(meta.AIReplyKind) != "" {
			return strings.TrimSpace(meta.AIReplyKind)
		}
	}

	content := strings.ToLower(strings.TrimSpace(msg.Content))
	switch {
	case isLikelyGreetingReply(content):
		return supportReplyKindGreeting
	case isLikelyClarificationReply(content):
		return supportReplyKindClarify
	default:
		return supportReplyKindAnswer
	}
}

func isLikelyGreetingReply(content string) bool {
	if !strings.HasSuffix(content, "?") {
		return false
	}
	prefixes := []string{
		"how can i help",
		"how can i assist",
		"what can i help",
		"what can i assist",
		"hello how can i help",
		"hi how can i help",
		"hey how can i help",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(content, prefix) {
			return true
		}
	}
	return false
}

func isLikelyClarificationReply(content string) bool {
	if !strings.HasSuffix(content, "?") {
		return false
	}
	patterns := []string{
		"what product or page are you referring to",
		"are you referring to",
		"do you mean",
		"could you clarify",
		"can you clarify",
		"which product",
		"which page",
		"what do you mean by",
		"what exactly do you mean",
		"what part are you asking about",
	}
	for _, pattern := range patterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}
	return false
}

// tokenizeWords splits text into lowercase alphanumeric words.
func tokenizeWords(text string) []string {
	lower := strings.ToLower(text)
	// Strip non-alphanumeric except spaces.
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			return r
		}
		return ' '
	}, lower)
	return strings.Fields(cleaned)
}

// multiExclamation matches 3+ consecutive exclamation marks.
var multiExclamation = regexp.MustCompile(`!{3,}`)

// parseAIConfidence extracts the ai_confidence value from a message's metadata JSON.
func parseAIConfidence(metadata string) (float64, bool) {
	if strings.TrimSpace(metadata) == "" {
		return 0, false
	}
	meta, ok := parseAIMessageMetadata(model.SupportMessage{Metadata: metadata})
	if !ok {
		return 0, false
	}
	return meta.AIConfidence, true
}

// ---------------------------------------------------------------------------
// Hard escalation rules (moved from support_ai.go)
// ---------------------------------------------------------------------------

// checkHardEscalation checks if a message matches hard escalation rules.
func checkHardEscalation(content string) string {
	lower := strings.ToLower(strings.TrimSpace(content))

	// Customer explicitly asks for a human.
	humanPatterns := []string{
		"talk to someone", "real person", "human agent", "talk to a human",
		"speak to someone", "real agent", "live agent", "connect me",
	}
	for _, pattern := range humanPatterns {
		if strings.Contains(lower, pattern) {
			return "customer_requested_human"
		}
	}

	return ""
}
