package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildAISystemPrompt_IncludesOutOfScopeFallbackGuidance(t *testing.T) {
	prompt := buildAISystemPrompt(&model.Agent{Name: "Support Bot"}, "")

	if !strings.Contains(prompt, "third-party tool recommendations/comparisons") {
		t.Fatalf("prompt missing out-of-scope comparison guidance: %s", prompt)
	}

	if !strings.Contains(prompt, "acknowledging the limitation and redirecting back to supported questions") {
		t.Fatalf("prompt missing safe fallback guidance: %s", prompt)
	}
}

func TestBuildAISystemPrompt_IncludesHandoffRepeatGuidance(t *testing.T) {
	prompt := buildAISystemPrompt(&model.Agent{Name: "Support Bot"}, "")

	if !strings.Contains(prompt, "do not repeat that message") {
		t.Fatalf("prompt missing repeated handoff guidance: %s", prompt)
	}

	if !strings.Contains(prompt, "A team member will be with you shortly") {
		t.Fatalf("prompt missing short follow-up acknowledgment guidance: %s", prompt)
	}
}

func TestBuildSupportPlannerSystemPromptIncludesWelcomeMessage(t *testing.T) {
	prompt := buildSupportPlannerSystemPrompt("Hi there! How can we help you today?")
	if !strings.Contains(prompt, "Automatic welcome message already shown to the visitor:\nHi there! How can we help you today?") {
		t.Fatalf("prompt missing welcome context:\n%s", prompt)
	}
	if !strings.Contains(prompt, `"greet"`) || !strings.Contains(prompt, `"greeting_reply"`) {
		t.Fatal("prompt missing greet decision or greeting_reply field")
	}
}

func TestBuildSupportPlannerSystemPromptOmitsEmptyWelcome(t *testing.T) {
	prompt := buildSupportPlannerSystemPrompt("   ")
	if strings.Contains(prompt, "Automatic welcome message already shown to the visitor:") {
		t.Fatal("prompt should omit welcome section when none is configured")
	}
}

func TestHasEscalationMessageInHistoryDetectsSystemEvent(t *testing.T) {
	history := []model.SupportMessage{
		{SenderType: "customer", Content: "Can you help?"},
		{
			SenderType:      "agent",
			MessageType:     "system",
			Content:         "Let me connect you with a team member who can help further.",
			SystemEventType: model.SupportSystemEventTypeStrPtr(model.SystemEventAIEscalated),
		},
	}

	if !hasEscalationMessageInHistory(history) {
		t.Fatal("hasEscalationMessageInHistory() = false, want true for escalation system event")
	}

	if !hasEscalationSystemEventInHistory(history) {
		t.Fatal("hasEscalationSystemEventInHistory() = false, want true")
	}
}

func TestHasEscalationMessageInHistoryDetectsPriorAIHandoffText(t *testing.T) {
	history := []model.SupportMessage{
		{SenderType: "customer", Content: "I need billing help"},
		{SenderType: "ai", MessageType: "reply", Content: "Let me connect you to a team member who can help with billing."},
	}

	if !hasEscalationMessageInHistory(history) {
		t.Fatal("hasEscalationMessageInHistory() = false, want true for AI handoff text")
	}

	if hasEscalationSystemEventInHistory(history) {
		t.Fatal("hasEscalationSystemEventInHistory() = true, want false without system event")
	}
}

func TestContainsHandoffLanguage(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "team member", content: "Let me connect you with a team member who can help further.", want: true},
		{name: "transfer", content: "I'll transfer you to our support team.", want: true},
		{name: "ordinary answer", content: "You can update your profile from settings.", want: false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsHandoffLanguage(tt.content); got != tt.want {
				t.Fatalf("containsHandoffLanguage(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}
