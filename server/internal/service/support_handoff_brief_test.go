package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportHandoffReasonText(t *testing.T) {
	tests := []struct {
		reason string
		want   string
	}{
		{reason: "cannot_answer", want: "AI couldn't find a confirmed answer."},
		{reason: "customer_requested_human", want: "The customer asked to talk to a person."},
		{reason: "answer_validation_unapproved_source", want: "AI's answer didn't pass its source check, so it wasn't sent."},
		{reason: "llm_provider_unavailable", want: "The AI service was unavailable, so AI didn't reply."},
		{reason: "ai_pipeline_error", want: "AI ran into a problem and couldn't reply."},
		{reason: "token_budget_exhausted", want: "The workspace's AI usage limit was reached, so AI didn't reply."},
		{reason: "agent_no_reply", want: "AI stopped without giving an answer."},
		{reason: "max_followups_reached", want: "AI reached its limit of replies in this conversation."},
		{reason: "new_unmapped_reason", want: "new unmapped reason"},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			if got := supportHandoffReasonText(tt.reason); got != tt.want {
				t.Errorf("supportHandoffReasonText(%q) = %q, want %q", tt.reason, got, tt.want)
			}
		})
	}
}

func handoffCardOf(t *testing.T, note *model.SupportMessage) supportHandoffCard {
	t.Helper()
	var metadata struct {
		Card supportHandoffCard `json:"handoff"`
	}
	if err := json.Unmarshal([]byte(note.Metadata), &metadata); err != nil {
		t.Fatalf("metadata: %v", err)
	}
	return metadata.Card
}

func TestBuildSupportHandoffNoteCard(t *testing.T) {
	conv := &model.SupportConversation{ID: "conv", WorkspaceID: "ws"}
	history := []model.SupportMessage{{ID: "customer", SenderType: "customer", MessageType: "reply", Content: "Is there a real person I can talk to?"}}
	tests := []struct {
		name   string
		reason string
		brief  SupportHandoffBrief
		want   supportHandoffCard
	}{
		{
			name:   "AI never ran leaves the issue and checks out",
			reason: "llm_provider_unavailable",
			want:   supportHandoffCard{Reason: "The AI service was unavailable, so AI didn't reply."},
		},
		{
			name:   "verified empty run says nothing was checked",
			reason: "customer_requested",
			brief:  SupportHandoffBrief{Issue: "Asked for a person.", CheckedKnown: true},
			want:   supportHandoffCard{Issue: "Asked for a person.", Reason: "The customer asked to talk to a person.", NothingChecked: true},
		},
		{
			name:   "reason note and sourced findings",
			reason: "risky_request",
			brief: SupportHandoffBrief{
				Issue: "Wants a refund.", ReasonNote: "Refunds need a teammate's decision.", CheckedKnown: true,
				Checked:         []SupportHandoffCheck{{Source: "Help articles", Finding: "Full refund within 14 days."}},
				UnavailableData: []string{"Payment history"},
			},
			want: supportHandoffCard{
				Issue: "Wants a refund.", Reason: "Refunds need a teammate's decision.",
				Checked:         []SupportHandoffCheck{{Source: "Help articles", Finding: "Full refund within 14 days."}},
				UnavailableData: []string{"Payment history"},
			},
		},
		{
			name:   "held reply and docs gap become next steps",
			reason: "answer_validation_incomplete",
			brief:  SupportHandoffBrief{HeldReply: "Hi Sam,\nStarter doesn't include the API.", HelpArticleGap: "Whether Starter includes the API"},
			want: supportHandoffCard{
				Reason:         "AI's answer didn't pass its source check, so it wasn't sent.",
				HeldReply:      "Hi Sam,\n\nStarter doesn't include the API.",
				HelpArticleGap: "Whether Starter includes the API",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handoffCardOf(t, buildSupportHandoffNote(conv, history, tt.reason, tt.brief, time.Now()))
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tt.want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("card = %s\nwant   %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestBuildSupportHandoffNoteContentSaysEachFactOnce(t *testing.T) {
	conv := &model.SupportConversation{ID: "conv", WorkspaceID: "ws"}
	history := []model.SupportMessage{{ID: "customer", SenderType: "customer", MessageType: "reply", Content: "My invoice shows our old company name."}}
	note := buildSupportHandoffNote(conv, history, "llm_provider_unavailable", SupportHandoffBrief{}, time.Now())
	if strings.Contains(note.Content, "old company name") || strings.Contains(note.Content, "AI checked") {
		t.Fatalf("content repeats the chat or claims checks: %q", note.Content)
	}
	if want := "AI handoff\n\nReason\nThe AI service was unavailable, so AI didn't reply."; note.Content != want {
		t.Fatalf("content = %q, want %q", note.Content, want)
	}
}

func TestBuildSupportHandoffNoteRecordsRequestedCodeCheck(t *testing.T) {
	conv := &model.SupportConversation{ID: "conv", WorkspaceID: "ws"}
	note := buildSupportHandoffNote(conv, nil, "cannot_answer", SupportHandoffBrief{Issue: "Approved post stays in Draft", InvestigationRequested: true}, time.Now())
	card := handoffCardOf(t, note)
	if !card.InvestigationRequested || card.Reason != "AI couldn't find a confirmed answer." {
		t.Fatalf("card = %+v", card)
	}
}

func TestFormatSupportDraftReply(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "lines become paragraphs", in: "Hi Jonas,\nThanks for the details.\nIt will move to Scheduled.", want: "Hi Jonas,\n\nThanks for the details.\n\nIt will move to Scheduled."},
		{name: "list items stay together", in: "To fix it:\n1. Open the post\n2. Click Schedule\nThat's it.", want: "To fix it:\n\n1. Open the post\n2. Click Schedule\n\nThat's it."},
		{name: "wrapping quotes removed", in: "“Hi Sam, Starter doesn't include it.”", want: "Hi Sam, Starter doesn't include it."},
		{name: "extra blank lines collapse", in: "Hi,\n\n\n\nThanks.", want: "Hi,\n\nThanks."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSupportDraftReply(tt.in); got != tt.want {
				t.Errorf("formatSupportDraftReply() = %q, want %q", got, tt.want)
			}
		})
	}
}
