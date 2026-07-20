package service

import (
	"encoding/json"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type supportConversationState struct {
	ActiveIssueKey       string   `json:"active_issue_key,omitempty"`
	ActiveIssueSummary   string   `json:"active_issue_summary,omitempty"`
	LastAIReplyKind      string   `json:"last_ai_reply_kind,omitempty"`
	LastAIAnswer         string   `json:"last_ai_answer,omitempty"`
	ConfirmationEligible bool     `json:"confirmation_eligible"`
	RecentCustomerFacts  []string `json:"recent_customer_facts"`
}

func buildSupportConversationState(history []model.SupportMessage) supportConversationState {
	state := supportConversationState{RecentCustomerFacts: []string{}}
	for idx := len(history) - 1; idx >= 0; idx-- {
		message := history[idx]
		if message.SenderType == "customer" && len(state.RecentCustomerFacts) < 3 {
			if content := strings.TrimSpace(supportMessagePromptText(message)); content != "" {
				state.RecentCustomerFacts = append(state.RecentCustomerFacts, truncateLog(content, 320))
			}
		}
		if message.SenderType != "ai" {
			continue
		}
		var metadata AIMessageMetadata
		if strings.TrimSpace(message.Metadata) != "" {
			_ = json.Unmarshal([]byte(message.Metadata), &metadata)
		}
		if state.LastAIReplyKind == "" {
			state.LastAIReplyKind = metadata.AIReplyKind
			if metadata.AIReplyKind == supportReplyKindAnswer {
				state.LastAIAnswer = truncateLog(strings.TrimSpace(message.Content), 500)
			}
		}
		if state.ActiveIssueKey == "" && metadata.AIIssueKey != "" {
			state.ActiveIssueKey = metadata.AIIssueKey
			state.ActiveIssueSummary = metadata.AIIssueSummary
		}
	}

	for idx := len(history) - 1; idx >= 0; idx-- {
		message := history[idx]
		if message.MessageType == "system" || message.IsInternal || strings.TrimSpace(message.Content) == "" {
			continue
		}
		if message.SenderType == "ai" {
			var metadata AIMessageMetadata
			_ = json.Unmarshal([]byte(message.Metadata), &metadata)
			state.ConfirmationEligible = metadata.AIReplyKind == supportReplyKindAnswer
		}
		break
	}
	return state
}

func marshalSupportConversationState(history []model.SupportMessage) string {
	encoded, err := json.Marshal(buildSupportConversationState(history))
	if err != nil {
		return `{}`
	}
	return string(encoded)
}
