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

func TestInternalKnowledgeContextHidesSourceMetadataAndCitations(t *testing.T) {
	results := []KnowledgeSearchResult{
		{
			ReferenceID: "docs:internal-doc",
			SourceType:  knowledgeSourceTypeDocs,
			IsInternal:  true,
			Title:       "Secret enterprise playbook",
			HeadingPath: "Unannounced 2027 launch",
			URL:         "https://internal.example/doc",
			Content:     "Enterprise plans include SAML SSO and audit logs.",
		},
		{
			ReferenceID: "docs:public-doc",
			SourceType:  knowledgeSourceTypeDocs,
			Title:       "Enterprise overview",
			Content:     "Read the public enterprise overview.",
		},
	}

	context := buildKnowledgeContext(results)
	for _, secret := range []string{"docs:internal-doc", "Secret enterprise playbook", "Unannounced 2027 launch", "https://internal.example/doc"} {
		if strings.Contains(context, secret) {
			t.Fatalf("internal knowledge context exposed %q:\n%s", secret, context)
		}
	}
	if !strings.Contains(context, "VISIBILITY: INTERNAL") || !strings.Contains(context, results[0].Content) {
		t.Fatalf("internal knowledge content missing from context:\n%s", context)
	}

	filtered := publicSourceDocIDs([]string{"docs:internal-doc", "docs:public-doc"}, results)
	if len(filtered) != 1 || filtered[0] != "docs:public-doc" {
		t.Fatalf("publicSourceDocIDs() = %v, want only public source", filtered)
	}

	sources := buildAISources([]string{"docs:internal-doc", "docs:public-doc"}, results)
	if len(sources) != 1 || sources[0].DocID != "docs:public-doc" {
		t.Fatalf("buildAISources() = %+v, want only public source", sources)
	}
}

func TestBuildAISystemPromptProtectsInternalKnowledgeSources(t *testing.T) {
	prompt := buildAISystemPrompt(&model.Agent{Name: "Support Bot"}, "VISIBILITY: INTERNAL\nCONTENT:\nPrivate guidance")

	for _, guidance := range []string{
		"never name, cite, link to, or reveal an internal source",
		"Only include document IDs from PUBLIC knowledge chunks",
	} {
		if !strings.Contains(prompt, guidance) {
			t.Fatalf("prompt missing internal source guidance %q:\n%s", guidance, prompt)
		}
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
}

func TestHasEscalationMessageInHistoryDetectsPriorAIHandoffText(t *testing.T) {
	history := []model.SupportMessage{
		{SenderType: "customer", Content: "I need billing help"},
		{SenderType: "ai", MessageType: "reply", Content: "Let me connect you to a team member who can help with billing."},
	}

	if !hasEscalationMessageInHistory(history) {
		t.Fatal("hasEscalationMessageInHistory() = false, want true for AI handoff text")
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
