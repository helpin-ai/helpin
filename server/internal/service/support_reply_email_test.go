package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportReplyEmailShowsNumberTitleAndLabeledHistory(t *testing.T) {
	svc := &NotificationService{appBaseURL: "https://app.helpin.ai"}
	event := model.NotificationEventInput{
		WorkspaceID: "ws", EntityType: "support_conversation", EntityID: "conv", EventType: "support_conversation.customer_reply",
		Category: model.NotifCategorySupportReplies, Body: "This worked — thanks!",
		ActorSnapshot:  model.JSONB{"name": "Cassie"},
		EntitySnapshot: model.JSONB{"title": "Google Analytics connection", "display_id": float64(1234)},
		Metadata:       model.JSONB{"support_email_body": "This worked — thanks!"},
	}
	entries := []supportReplyEmailEntry{
		{Author: "Cassie", Role: "Customer", Content: "Old question"},
		{Author: "Arooj", Role: "Private note", Content: "Check the property ID"},
		{Author: "Waqar", Role: "Team reply", Content: "Reconnect the integration"},
	}
	subject, htmlBody, textBody := svc.renderSupportReplyEmail(context.Background(), event, "Usermaven", "usermaven", entries)
	if subject != "Cassie replied to conversation #1234 [Usermaven]" {
		t.Fatalf("subject = %q", subject)
	}
	for _, want := range []string{"Google Analytics connection", "This worked — thanks!", "Old question", "Check the property ID", "Reconnect the integration", "Private note", "Team reply", "Customer", "https://app.helpin.ai/w/usermaven/support/conv"} {
		if !strings.Contains(htmlBody, want) || !strings.Contains(textBody, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if !(strings.Index(htmlBody, "Old question") < strings.Index(htmlBody, "Check the property ID") && strings.Index(htmlBody, "Check the property ID") < strings.Index(htmlBody, "Reconnect the integration")) {
		t.Fatal("history order changed")
	}
	actualSubject, actualHTML, _ := svc.renderImmediateEmail(context.Background(), event)
	if actualSubject != "Cassie replied to conversation #1234 [Helpin]" || !strings.Contains(actualHTML, "Google Analytics connection") {
		t.Fatalf("delayed email renderer did not use support reply template: %q", actualSubject)
	}
}

func TestSupportReplyHistoryExcludesSystemAndQuotedEmail(t *testing.T) {
	email := "email"
	quoted := true
	date := time.Now()
	messages := []model.SupportMessage{
		{SenderType: "customer", MessageType: "reply", Content: "Earlier email plus quoted thread", ViaChannel: &email, EmailVisibleText: "Fresh email", EmailHasQuotedContent: &quoted, EmailProjectionConfidence: "high", CreatedAt: date},
		{SenderType: "user", MessageType: "system", Content: "AI control left a private note", CreatedAt: date},
		{SenderType: "user", MessageType: "reply", IsInternal: true, Content: "Internal detail", CreatedAt: date},
		{SenderType: "user", MessageType: "reply", Content: "Public reply", CreatedAt: date},
	}
	entries := supportReplyHistoryEntries(messages)
	if len(entries) != 3 {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].Content != "Fresh email" || entries[1].Role != "Private note" || entries[2].Role != "Team reply" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestSupportReplyHistoryIsAnchoredToTriggeringMessage(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	if err := db.Exec(`CREATE TABLE support_messages (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, sender_type TEXT, sender_display_name TEXT, message_type TEXT, content TEXT, is_internal BOOLEAN, via_channel TEXT, created_at DATETIME, deleted_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	for index, row := range []struct {
		id, workspace, conv, role, content string
		internal                           bool
	}{
		{"first", "ws", "conv", "customer", "First question", false},
		{"note", "ws", "conv", "user", "Internal clue", true},
		{"system", "ws", "conv", "user", "System event", false},
		{"trigger", "ws", "conv", "customer", "Latest reply", false},
		{"future", "ws", "conv", "user", "Later reply", false},
		{"other", "other-ws", "conv", "user", "Other workspace secret", false},
	} {
		kind := "reply"
		if row.id == "system" {
			kind = "system"
		}
		if err := db.Exec(`INSERT INTO support_messages (id,workspace_id,conversation_id,sender_type,message_type,content,is_internal,created_at) VALUES (?,?,?,?,?,?,?,?)`, row.id, row.workspace, row.conv, row.role, kind, row.content, row.internal, base.Add(time.Duration(index)*time.Minute)).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := &NotificationService{supportMessageRepo: repository.NewSupportMessageRepository(db)}
	entries := svc.loadSupportReplyEmailHistory(context.Background(), model.NotificationEventInput{WorkspaceID: "ws", EntityID: "conv", Metadata: model.JSONB{"support_message_id": "trigger"}})
	if len(entries) != 2 || entries[0].Content != "First question" || entries[1].Content != "Internal clue" {
		t.Fatalf("history = %+v", entries)
	}
}

func TestSupportReplyHistoryEmailTextUsesSafeExcerpt(t *testing.T) {
	payload := model.PostmarkInboundPayload{TextBody: "Fresh reply\n\nOn Friday someone wrote: old thread", StrippedTextReply: "Fresh reply"}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got := supportReplyHistoryEmailText(model.SupportEmailLog{RawBody: string(raw)}); got != "Fresh reply" {
		t.Fatalf("email excerpt = %q", got)
	}
	if got := supportReplyHistoryEmailText(model.SupportEmailLog{RawBody: `{}`}); got != "" {
		t.Fatalf("ambiguous email excerpt = %q", got)
	}
}
