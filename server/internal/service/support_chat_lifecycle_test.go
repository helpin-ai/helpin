package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func supportChatLifecycleRun(now time.Time) *model.AgentRun {
	return &model.AgentRun{
		TargetType:  "support_conversation",
		Status:      model.AgentRunStatusPaused,
		PauseReason: model.AgentRunPauseReasonUserMessage,
		Input:       json.RawMessage(`{"trigger":{"source":"system","trigger_type":"support_chat"}}`),
		UpdatedAt:   now,
	}
}

func TestSupportChatRunClosureDecision(t *testing.T) {
	now := time.Now().UTC()
	t.Run("human handoff closes immediately", func(t *testing.T) {
		run := supportChatLifecycleRun(now)
		takeover := true
		closeRun, reason := supportChatRunClosureDecision(run, &model.SupportConversation{HumanTakeover: &takeover}, now)
		if !closeRun || reason != "human_handoff" {
			t.Fatalf("decision = %v/%q, want human_handoff", closeRun, reason)
		}
	})

	t.Run("ordinary paused chat stays reusable before timeout", func(t *testing.T) {
		run := supportChatLifecycleRun(now.Add(-time.Hour))
		closeRun, reason := supportChatRunClosureDecision(run, &model.SupportConversation{}, now)
		if closeRun || reason != "" {
			t.Fatalf("decision = %v/%q, want open", closeRun, reason)
		}
	})

	t.Run("ordinary paused chat closes at 24 hour timeout", func(t *testing.T) {
		run := supportChatLifecycleRun(now.Add(-24 * time.Hour))
		closeRun, reason := supportChatRunClosureDecision(run, &model.SupportConversation{}, now)
		if !closeRun || reason != "idle_timeout" {
			t.Fatalf("decision = %v/%q, want idle_timeout", closeRun, reason)
		}
	})

	t.Run("non support-chat run is untouched", func(t *testing.T) {
		run := supportChatLifecycleRun(now.Add(-48 * time.Hour))
		run.Input = json.RawMessage(`{"trigger":{"source":"manual","trigger_type":"manual"}}`)
		closeRun, _ := supportChatRunClosureDecision(run, &model.SupportConversation{}, now)
		if closeRun {
			t.Fatal("manual support run must not be closed by chat lifecycle")
		}
	})
}
