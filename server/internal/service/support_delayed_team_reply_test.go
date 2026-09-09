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
