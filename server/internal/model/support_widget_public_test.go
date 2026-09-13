package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPublicWidgetMessagePreservesVisitorFeaturesOnly(t *testing.T) {
	name, channel, task := "Alex", "email", "PRIVATE_TASK"
	quoted := true
	metadata := `{"ai_agent_id":"PRIVATE_AGENT","ai_model":"PRIVATE_MODEL","ai_tokens_used":42,"ai_confidence":0.8,"ai_validation_reasons":["PRIVATE_REASON"],"future_internal_field":"PRIVATE_FUTURE","capture_email":true,"delayed_team_reply":true,"ai_reply_kind":"answer","ai_sources":[{"docId":"public-doc","title":"Public help","url":"https://example.com/help","blockId":"PRIVATE_BLOCK","confidence":0.9,"future":"PRIVATE_NESTED"}],"link_previews":[{"url":"https://example.com","title":"Example","internal":"PRIVATE_PREVIEW"}],"link_security":[{"status":"safe"}]}`
	m := SupportMessage{ID: "m", ConversationID: "c", SenderType: "agent", MessageType: "reply", Content: "Hello", SenderDisplayName: &name, ViaChannel: &channel, Metadata: metadata, EmailBCC: DocsStringArray{"PRIVATE_BCC"}, EmailDeliveryError: "PRIVATE_DELIVERY", SenderAgentID: &task, EmailVisibleText: "Visible email", EmailQuotedText: "Public quoted reply", EmailHasQuotedContent: &quoted, CreatedAt: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC), Attachments: []SupportAttachmentPayload{{ID: "a", FileKey: "PRIVATE_KEY", FileName: "photo.png", FileType: "image/png", URL: "https://example.com/photo.png", FileSize: 123}}}
	public := PublicWidgetMessage(&m)
	if public == nil || public.SenderType != "ai" || public.SenderName == nil || *public.SenderName != "Alex" {
		t.Fatalf("lost public identity: %+v", public)
	}
	b, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "PRIVATE_") || strings.Contains(string(b), "ai_confidence") || strings.Contains(string(b), "ai_tokens_used") {
		t.Fatalf("internal data in visitor response: %s", b)
	}
	for _, want := range []string{"Public help", "https://example.com/help", "capture_email", "delayed_team_reply", "link_previews", "Visible email", "Public quoted reply", "photo.png"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("lost public feature %s", want)
		}
	}
	if m.Metadata != metadata || m.Attachments[0].FileKey != "PRIVATE_KEY" {
		t.Fatal("staff message was mutated")
	}
}

func TestPublicWidgetMessagesRejectPrivateAndRoutingMessages(t *testing.T) {
	assigned, unknown, delayed := SystemEventAssigned, "future_internal_event", SystemEventDelayedTeamReply
	input := []SupportMessage{
		{ID: "public", Content: "Hello"}, {ID: "note", IsInternal: true, Content: "Private note"},
		{ID: "email", Metadata: `{"delivery_mode":"email_only"}`},
		{ID: "routing", SystemEventType: &assigned}, {ID: "unknown", SystemEventType: &unknown},
		{ID: "delayed", SystemEventType: &delayed, MessageType: "system"},
	}
	got := PublicWidgetMessages(input)
	if len(got) != 2 || got[0].ID != "public" || got[1].ID != "delayed" {
		t.Fatalf("incorrect public history: %+v", got)
	}
	if PublicWidgetMessage(nil) != nil {
		t.Fatal("nil message must stay absent")
	}
	b, _ := json.Marshal(PublicWidgetMessages(nil))
	if string(b) != "[]" {
		t.Fatalf("empty history = %s", b)
	}
}

func TestPublicWidgetConversationsExcludeStaffState(t *testing.T) {
	secret, preview, name := "PRIVATE_REFERENCE", "Public preview", "Alex"
	input := []SupportConversation{{ID: "c", Subject: "Question", CRMContactID: &secret, LinkedTaskID: &secret, AIActiveRunID: &secret, Priority: "PRIVATE_PRIORITY", ListLastMessagePreview: &secret, LastMessage: &preview, OpenedByDisplayName: &name, UnreadCount: 2}}
	b, err := json.Marshal(PublicWidgetConversations(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "PRIVATE_") {
		t.Fatalf("staff data leaked: %s", b)
	}
	for _, want := range []string{"Public preview", "Alex", `"unread_count":2`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("lost public summary: %s", want)
		}
	}
}

func TestPublicWidgetMetadataFailsClosed(t *testing.T) {
	for _, raw := range []string{"", `{`, `null`, `[]`, `{"ai_sources":"invalid","capture_email":true}`, `{"unknown":"private"}`} {
		if got := PublicWidgetMetadata(raw); got != "{}" {
			t.Errorf("metadata %q = %s", raw, got)
		}
	}
}
