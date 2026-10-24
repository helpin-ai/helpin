package service

import (
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

func TestBuildSupportHandoffNoteMentionsRequestedCodeCheck(t *testing.T) {
	conv := &model.SupportConversation{ID: "conv", WorkspaceID: "ws"}
	tests := []struct {
		name      string
		requested bool
		want      string
	}{
		{name: "code check requested", requested: true, want: "Reason for handoff\nAI couldn't find a confirmed answer. A code check was requested; its findings will appear here as a separate note."},
		{name: "no code check", requested: false, want: "Reason for handoff\nAI couldn't find a confirmed answer."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note := buildSupportHandoffNote(conv, nil, "cannot_answer", SupportHandoffBrief{Issue: "Approved post stays in Draft", InvestigationRequested: tt.requested}, time.Now())
			if !strings.HasSuffix(note.Content, tt.want) {
				t.Errorf("content = %q, want suffix %q", note.Content, tt.want)
			}
		})
	}
}
