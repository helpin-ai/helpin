package service

import (
	"context"
	"encoding/json"
	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/redis/go-redis/v9"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestDelayedTeamReply(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		age                  time.Duration
		status, sender, kind string
		internal             bool
		want                 bool
	}{
		{name: "before deadline", age: 4 * time.Minute, status: "open"},
		{name: "at deadline", age: 5 * time.Minute, status: "open", want: true},
		{name: "human reply", age: 6 * time.Minute, status: "open", sender: "user", kind: "reply"},
		{name: "agent reply", age: 6 * time.Minute, status: "open", sender: "agent", kind: "reply"},
		{name: "internal note", age: 6 * time.Minute, status: "open", sender: "user", kind: "note", internal: true, want: true},
		{name: "customer followup", age: 6 * time.Minute, status: "open", sender: "customer", kind: "reply", want: true},
		{name: "resolved", age: 6 * time.Minute, status: "resolved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t)
			ctx := context.Background()
			now := time.Now().UTC()
			at := now.Add(-tc.age)
			conv := &model.SupportConversation{WorkspaceID: "delay-ws", Subject: "Waiting", Status: tc.status, Channel: "widget", AIEscalatedAt: &at, AIState: strPtr("escalated"), HumanTakeover: boolPtr(true)}
			cr := repository.NewSupportConversationRepository(db)
			mr := repository.NewSupportMessageRepository(db)
			if err := cr.Create(ctx, conv); err != nil {
				t.Fatal(err)
			}
			if tc.sender != "" {
				if err := mr.Create(ctx, &model.SupportMessage{WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: tc.sender, MessageType: tc.kind, Content: "hello", IsInternal: tc.internal, CreatedAt: at.Add(time.Second)}); err != nil {
					t.Fatal(err)
				}
			}
			s := &SupportInboxService{conversationRepo: cr, messageRepo: mr}
			got, err := s.sendDelayedTeamReply(ctx, conv.ID, model.DefaultSupportInboxSettings(), now)
			if err != nil {
				t.Fatal(err)
			}
			if (got != nil) != tc.want {
				t.Fatalf("message=%v want sent=%v", got, tc.want)
			}
			if tc.want {
				if got.MessageType != "system" {
					t.Fatal("notice must not count as a reply")
				}
				if got.Content != "Our team hasn’t been able to reply yet. Your conversation is still waiting for a teammate. You can return to this chat to check for a reply." {
					t.Fatal("promised email when unconfigured")
				}
				again, err := s.sendDelayedTeamReply(ctx, conv.ID, model.DefaultSupportInboxSettings(), now.Add(time.Minute))
				if err != nil || again != nil {
					t.Fatalf("duplicate: %v %v", again, err)
				}
			}
		})
	}
}

func TestDelayedTeamReplyEmailAndRestart(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "known"}[known], func(t *testing.T) {
			db := newTestDB(t)
			ctx := context.Background()
			now := time.Now().UTC()
			at := now.Add(-7 * time.Minute)
			cr := repository.NewSupportConversationRepository(db)
			mr := repository.NewSupportMessageRepository(db)
			conv := &model.SupportConversation{WorkspaceID: "delay-ws", Subject: "Waiting", Status: "open", Channel: "widget", AIEscalatedAt: &at, AIState: strPtr("escalated"), AssignedUserID: strPtr("teammate")}
			if known {
				conv.CustomerEmail = strPtr("visitor@example.com")
			}
			if err := cr.Create(ctx, conv); err != nil {
				t.Fatal(err)
			}
			settings := model.DefaultSupportInboxSettings()
			settings.DelayedTeamReplyMinutes = 8
			settings.DelayedTeamReplyMessage = "Known custom message"
			settings.DelayedTeamReplyMessageNoEmail = "Missing custom message"
			s := &SupportInboxService{conversationRepo: cr, messageRepo: mr, emailFallbackService: &EmailFallbackService{emailClient: &email.Client{}, redis: redis.NewClient(&redis.Options{})}}
			early, err := s.sendDelayedTeamReply(ctx, conv.ID, settings, now)
			if err != nil || early != nil {
				t.Fatalf("custom deadline: %v %v", early, err)
			}
			msg, err := s.sendDelayedTeamReply(ctx, conv.ID, settings, now.Add(time.Minute))
			if err != nil || msg == nil {
				t.Fatalf("send: %v %v", msg, err)
			}
			want := "Missing custom message"
			if known {
				want = "Known custom message"
			}
			if msg.Content != want {
				t.Fatalf("content=%s", msg.Content)
			}
			var metadata map[string]bool
			if err := json.Unmarshal([]byte(msg.Metadata), &metadata); err != nil {
				t.Fatal(err)
			}
			if metadata["capture_email"] == known {
				t.Fatal("wrong email capture state")
			}
			// A new worker instance must observe the database marker, not an in-memory flag.
			restarted := &SupportInboxService{conversationRepo: cr, messageRepo: mr}
			if err := restarted.ProcessDelayedTeamReplies(ctx, now.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			var count int64
			db.Model(&model.SupportMessage{}).Where("conversation_id = ?", conv.ID).Count(&count)
			if count != 1 {
				t.Fatalf("notice count=%d", count)
			}
			// A new handoff gets its own deadline and one notice.
			next := now.Add(2 * time.Hour)
			if err := db.Model(conv).Update("ai_escalated_at", next).Error; err != nil {
				t.Fatal(err)
			}
			msg, err = restarted.sendDelayedTeamReply(ctx, conv.ID, settings, next.Add(8*time.Minute))
			if err != nil || msg == nil {
				t.Fatalf("new handoff: %v %v", msg, err)
			}
		})
	}
}

func TestDelayedTeamReplyBusinessHours(t *testing.T) {
	// Wednesday; the team works 9:00–17:00 UTC on weekdays.
	open := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	closed := time.Date(2026, 10, 28, 20, 0, 0, 0, time.UTC)
	settings := model.DefaultSupportInboxSettings()
	settings.BusinessHoursEnabled = true
	settings.BusinessHoursTimezone = "UTC"
	for day, hours := range settings.BusinessHoursSchedule {
		hours.Enabled = day != "sat" && day != "sun"
		hours.Start, hours.End = "09:00", "17:00"
		settings.BusinessHoursSchedule[day] = hours
	}
	for _, tc := range []struct {
		name         string
		handoffState string
		now          time.Time
		want         string
	}{
		{name: "after-hours handoff stays quiet", handoffState: model.HandoffStateAfterHours, now: closed},
		{name: "handoff before closing says when the team is back", handoffState: model.HandoffStateLive, now: closed, want: "Our team is away right now and back tomorrow at 9:00 AM UTC. We’ll reply in this chat and email you if you miss it."},
		{name: "during business hours the workspace message is sent", handoffState: model.HandoffStateBusy, now: open, want: settings.DelayedTeamReplyMessage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t)
			ctx := context.Background()
			at := tc.now.Add(-6 * time.Minute)
			cr := repository.NewSupportConversationRepository(db)
			mr := repository.NewSupportMessageRepository(db)
			conv := &model.SupportConversation{WorkspaceID: "delay-ws", Subject: "Waiting", Status: "open", Channel: "widget", AIEscalatedAt: &at, AIState: strPtr("escalated"), HandoffState: strPtr(tc.handoffState), CustomerEmail: strPtr("visitor@example.com")}
			if err := cr.Create(ctx, conv); err != nil {
				t.Fatal(err)
			}
			s := &SupportInboxService{conversationRepo: cr, messageRepo: mr, emailFallbackService: &EmailFallbackService{emailClient: &email.Client{}, redis: redis.NewClient(&redis.Options{})}}
			msg, err := s.sendDelayedTeamReply(ctx, conv.ID, settings, tc.now)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if msg != nil {
					t.Fatalf("sent %q, want no notice", msg.Content)
				}
				// The handoff is settled, so the team opening later sends nothing either.
				if again, err := s.sendDelayedTeamReply(ctx, conv.ID, settings, open.AddDate(0, 0, 1)); err != nil || again != nil {
					t.Fatalf("later notice: %v %v", again, err)
				}
				return
			}
			if msg == nil || msg.Content != tc.want {
				t.Fatalf("notice = %v, want %q", msg, tc.want)
			}
		})
	}
}

func TestDelayedTeamReplyAwayContent(t *testing.T) {
	for _, tc := range []struct {
		name              string
		nextOpen          string
		canEmail, capture bool
		want              string
	}{
		{name: "email known", nextOpen: "tomorrow at 9:00 AM", canEmail: true, want: "Our team is away right now and back tomorrow at 9:00 AM. We’ll reply in this chat and email you if you miss it."},
		{name: "email missing", nextOpen: "on Monday at 9:00 AM", canEmail: true, capture: true, want: "Our team is away right now and back on Monday at 9:00 AM. Leave your email and we’ll let you know when someone replies, so you don’t have to wait here."},
		{name: "no email sending", nextOpen: "tomorrow at 9:00 AM", want: "Our team is away right now and back tomorrow at 9:00 AM. You can return to this chat to check for a reply."},
		{name: "no opening known", canEmail: true, want: "Our team is away right now. We’ll reply in this chat and email you if you miss it."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := delayedTeamReplyAwayContent(tc.nextOpen, tc.canEmail, tc.capture); got != tc.want {
				t.Fatalf("content = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDelayedTeamReplyResolutionCancelsAfterReopen(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	at := now.Add(-time.Minute)
	cr := repository.NewSupportConversationRepository(db)
	mr := repository.NewSupportMessageRepository(db)
	conv := &model.SupportConversation{WorkspaceID: "delay-ws", Subject: "Waiting", Status: "open", Channel: "widget", AIEscalatedAt: &at, AIState: strPtr("escalated")}
	if err := cr.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	if err := cr.UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{"status": "resolved"}); err != nil {
		t.Fatal(err)
	}
	if err := cr.UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{"status": "open"}); err != nil {
		t.Fatal(err)
	}
	s := &SupportInboxService{conversationRepo: cr, messageRepo: mr}
	if err := s.ProcessDelayedTeamReplies(ctx, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&model.SupportMessage{}).Where("conversation_id = ?", conv.ID).Count(&count)
	if count != 0 {
		t.Fatalf("revived timer: %d messages", count)
	}
}

func TestDelayedTeamReplySavedDefault(t *testing.T) {
	const updated = "Looks like our team needs a little more time. We’ll reply in this chat and email you if you miss it. Thanks for your patience."
	for _, tc := range []struct{ name, raw, want string }{
		{"new settings", `{}`, updated},
		{"saved original default", `{"delayed_team_reply_message":"Our team hasn’t been able to reply yet. You don’t need to keep this chat open. We’ll email you when someone responds."}`, updated},
		{"custom message", `{"delayed_team_reply_message":"Thanks, we will be in touch."}`, "Thanks, we will be in touch."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseSettings(tc.raw).DelayedTeamReplyMessage; got != tc.want {
				t.Fatalf("message = %q, want %q", got, tc.want)
			}
		})
	}
}
