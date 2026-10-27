package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestResolveHandoffState(t *testing.T) {
	cases := []struct {
		name         string
		hasRecipient bool
		withinHours  bool
		want         string
	}{
		{"online within hours -> live", true, true, model.HandoffStateLive},
		{"online outside hours -> live (presence trumps hours)", true, false, model.HandoffStateLive},
		{"nobody, within hours -> busy", false, true, model.HandoffStateBusy},
		{"nobody, outside hours -> after_hours", false, false, model.HandoffStateAfterHours},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveHandoffState(c.hasRecipient, c.withinHours); got != c.want {
				t.Fatalf("resolveHandoffState(%v,%v) = %q, want %q", c.hasRecipient, c.withinHours, got, c.want)
			}
		})
	}
}

func TestRenderEscalationMessage(t *testing.T) {
	got := renderEscalationMessage("reply in {reply_time}, back {next_open}", "a few minutes", "tomorrow at 9:00 AM PST")
	want := "reply in a few minutes, back tomorrow at 9:00 AM PST"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// Missing values degrade gracefully.
	got = renderEscalationMessage("back {next_open}", "", "")
	if got != "back as soon as possible" {
		t.Fatalf("degraded got %q", got)
	}
	// Unknown token rendered literally.
	got = renderEscalationMessage("hi {unknown}", "x", "y")
	if got != "hi {unknown}" {
		t.Fatalf("unknown token got %q", got)
	}
}

func TestHumanizeNextOpen(t *testing.T) {
	now := time.Date(2026, 7, 4, 20, 0, 0, 0, time.UTC) // Saturday evening
	if humanizeNextOpen(nil, now) != "" {
		t.Fatal("nil should be empty")
	}
	monday := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	if got := humanizeNextOpen(&monday, now); got != "on Monday at 9:00 AM UTC" {
		t.Fatalf("monday got %q", got)
	}
	sunday := time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC)
	if got := humanizeNextOpen(&sunday, now); got != "tomorrow at 9:00 AM UTC" {
		t.Fatalf("tomorrow got %q", got)
	}
}

func TestSelectEscalationTemplate(t *testing.T) {
	s := model.SupportInboxSettings{
		EscalationMessage:           "live {reply_time}",
		EscalationMessageBusy:       "busy {reply_time}",
		EscalationMessageAfterHours: "away {next_open}",
	}
	if got := selectEscalationTemplate(s, model.HandoffStateLive); got != "live {reply_time}" {
		t.Fatalf("live got %q", got)
	}
	if got := selectEscalationTemplate(s, model.HandoffStateBusy); got != "busy {reply_time}" {
		t.Fatalf("busy got %q", got)
	}
	if got := selectEscalationTemplate(s, model.HandoffStateAfterHours); got != "away {next_open}" {
		t.Fatalf("after_hours got %q", got)
	}
	// Empty busy field falls back to default, NOT to the live message.
	s2 := model.SupportInboxSettings{EscalationMessage: "custom live"}
	if got := selectEscalationTemplate(s2, model.HandoffStateBusy); got == "custom live" || got == "" {
		t.Fatalf("busy fallback must be the built-in default, got %q", got)
	}
}

func TestShortReplyTimePhrase(t *testing.T) {
	if got := shortReplyTimePhrase("few_minutes", nil); got != "a few minutes" {
		t.Fatalf("few_minutes got %q", got)
	}
	if got := shortReplyTimePhrase("same_day", nil); got != "a day" {
		t.Fatalf("same_day got %q", got)
	}
	m := 45
	if got := shortReplyTimePhrase("custom", &m); got != "about 45 minutes" {
		t.Fatalf("custom got %q", got)
	}
	if got := shortReplyTimePhrase("", nil); got != "" {
		t.Fatalf("unknown preset must degrade to empty, got %q", got)
	}
}

func TestHumanizeNextOpenForCustomer(t *testing.T) {
	karachi, err := time.LoadLocation("Asia/Karachi")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	// Monday 9:00 AM in New York is Monday 6:00 PM in Karachi.
	nextOpen := time.Date(2026, 10, 26, 13, 0, 0, 0, time.UTC)
	customerNow := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC).In(karachi)

	if got := humanizeNextOpenForCustomer(&nextOpen, customerNow); got != "on Monday at 6:00 PM" {
		t.Fatalf("got %q, want %q", got, "on Monday at 6:00 PM")
	}
	if humanizeNextOpenForCustomer(nil, customerNow) != "" {
		t.Fatal("nil should be empty")
	}
}

func TestSupportAIServiceNextOpenForCustomer(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-next-open"
	settings := model.DefaultSupportInboxSettings()
	settings.BusinessHoursEnabled = true
	settings.BusinessHoursTimezone = "America/New_York"
	// Saturday 10:00 AM in New York; the team is back Monday at 9:00 AM.
	now := time.Date(2026, 10, 24, 14, 0, 0, 0, time.UTC)
	svc := &SupportAIService{sessionRepo: repository.NewSupportInboxSessionRepository(db)}

	mustExec(t, db, `INSERT INTO support_widget_sessions (id, workspace_id, conversation_id, session_token, anonymous_id, timezone, expires_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"session-karachi", workspaceID, "conv-widget", "token-karachi", "anon-1", "Asia/Karachi", now.Add(time.Hour), now, now)

	tests := []struct {
		name           string
		conversationID string
		want           string
	}{
		{name: "widget reported the customer's timezone", conversationID: "conv-widget", want: "on Monday at 6:00 PM"},
		{name: "customer timezone unknown", conversationID: "conv-email", want: "on Monday at 9:00 AM EDT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv := &model.SupportConversation{ID: tt.conversationID, WorkspaceID: workspaceID}
			if got := svc.nextOpenForCustomer(ctx, conv, settings, now); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
