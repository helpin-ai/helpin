package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
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
