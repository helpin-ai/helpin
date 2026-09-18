package service

import (
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func aiMsgWithKind(agentID, content, kind string, confidence float64) model.SupportMessage {
	meta := AIMessageMetadata{
		AIAutoReply:  true,
		AIConfidence: confidence,
		AIAgentID:    agentID,
		AIReplyKind:  kind,
	}
	b, _ := json.Marshal(meta)
	return model.SupportMessage{
		SenderType:    "ai",
		SenderAgentID: strPtr(agentID),
		Content:       content,
		Metadata:      string(b),
	}
}

func customerMsg(content string) model.SupportMessage {
	return model.SupportMessage{SenderType: "customer", Content: content}
}

func TestCountMaxFollowupAITurnsExcludesGreetingAndClarify(t *testing.T) {
	agentID := "agent-1"
	history := []model.SupportMessage{
		customerMsg("Hi there"),
		aiMsgWithKind(agentID, "How can I help you today?", supportReplyKindGreeting, 0.95),
		customerMsg("What is this thing?"),
		aiMsgWithKind(agentID, "What product or page are you referring to?", supportReplyKindClarify, 0.92),
		customerMsg("ContentStudio"),
		aiMsgWithKind(agentID, "ContentStudio is a social media management platform.", supportReplyKindAnswer, 0.82),
	}

	if got := countAgentAITurns(history, agentID); got != 3 {
		t.Fatalf("countAgentAITurns() = %d, want 3", got)
	}
	if got := countMaxFollowupAITurns(history, agentID); got != 1 {
		t.Fatalf("countMaxFollowupAITurns() = %d, want 1", got)
	}
}

func TestCountMaxFollowupAITurnsFallsBackToHeuristicsForLegacyMessages(t *testing.T) {
	agentID := "agent-1"
	history := []model.SupportMessage{
		{
			SenderType:    "ai",
			SenderAgentID: strPtr(agentID),
			Content:       "How can I help you today?",
			Metadata:      `{"ai_auto_reply":true,"ai_confidence":0.95,"ai_agent_id":"agent-1"}`,
		},
		{
			SenderType:    "ai",
			SenderAgentID: strPtr(agentID),
			Content:       "What product or page are you referring to?",
			Metadata:      `{"ai_auto_reply":true,"ai_confidence":0.92,"ai_agent_id":"agent-1"}`,
		},
		{
			SenderType:    "ai",
			SenderAgentID: strPtr(agentID),
			Content:       "ContentStudio is a social media management platform.",
			Metadata:      `{"ai_auto_reply":true,"ai_confidence":0.82,"ai_agent_id":"agent-1"}`,
		},
	}

	if got := countMaxFollowupAITurns(history, agentID); got != 1 {
		t.Fatalf("countMaxFollowupAITurns() = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// checkHardEscalation (moved function, verify still works)
// ---------------------------------------------------------------------------

func TestCheckHardEscalation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "no match", content: "How do I reset?", want: ""},
		{name: "human request", content: "I want to talk to a human", want: "customer_requested_human"},
		{name: "live agent", content: "Connect me to a live agent please", want: "customer_requested_human"},
		{name: "billing goes through planner", content: "I need a refund", want: ""},
		{name: "account cancellation goes through planner", content: "Cancel my account now", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkHardEscalation(tt.content)
			if got != tt.want {
				t.Errorf("checkHardEscalation(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}
