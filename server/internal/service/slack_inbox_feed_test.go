package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSlackInboxFeedExcerpt(t *testing.T) {
	long := strings.Repeat("Export ", 19)
	if got := feedExcerpt("Trouble exporting", long); got != strings.TrimSpace(strings.Repeat("Export ", 14))+"…" {
		t.Fatalf("long excerpt = %q", got)
	}
	if got := feedExcerpt(" Checkout failed ", "Checkout\nfailed"); got != "" {
		t.Fatalf("repeated subject = %q", got)
	}
}

func TestSlackInboxFeedAuthor(t *testing.T) {
	f := feedContext{conv: &model.SupportConversation{Channel: "widget"}}
	name := "Maya Patel"
	for _, tc := range []struct {
		kind, role string
		internal   bool
	}{
		{"customer", "Visitor", false}, {"user", "Team reply", false}, {"ai", "AI reply", false}, {"agent", "AI reply", false}, {"user", "📝 Internal note", true},
	} {
		m := model.SupportMessage{SenderType: tc.kind, SenderDisplayName: &name, IsInternal: tc.internal}
		if got := f.feedAuthor(&m); got.role != tc.role || got.name != name {
			t.Fatalf("%s author = %+v", tc.kind, got)
		}
	}
}

func TestSlackInboxFeedVisibility(t *testing.T) {
	for _, tc := range []struct {
		name       string
		ch         SlackChannel
		visibility string
		want       bool
	}{
		{"public workspace inbox", SlackChannel{}, "workspace", true},
		{"private restricted inbox", SlackChannel{IsPrivate: true}, "members_only", true},
		{"public restricted inbox", SlackChannel{}, "members_only", false},
		{"externally shared", SlackChannel{IsShared: true, IsPrivate: true}, "workspace", false},
		{"pending external share", SlackChannel{PendingShared: true}, "workspace", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := slackFeedAllowsChannel(tc.ch, tc.visibility); got != tc.want {
				t.Fatalf("eligible = %v", got)
			}
		})
	}
}

func TestSlackInboxFeedArchivedReason(t *testing.T) {
	other := "merged-conversation"
	now := time.Now()
	for _, tc := range []struct {
		name string
		conv model.SupportConversation
		want string
	}{
		{"merged", model.SupportConversation{MergedIntoID: &other}, "This conversation was merged into another conversation in Helpin."},
		{"anonymized", model.SupportConversation{AnonymizedAt: &now}, "Personal details were removed from this conversation in Helpin."},
		{"anonymized merge", model.SupportConversation{MergedIntoID: &other, AnonymizedAt: &now}, "Personal details were removed from this conversation in Helpin."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.want + " Updates have stopped in this thread."
			if got, _ := (&SlackService{}).feedStopNotice(context.Background(), &tc.conv, "shared", nil); got != want {
				t.Errorf("notice = %q, want %q", got, want)
			}
		})
	}
}

func TestSlackInboxFeedThreadUpdates(t *testing.T) {
	for _, tc := range []struct {
		m    model.SupportMessage
		want bool
	}{
		{model.SupportMessage{MessageType: "reply", SenderType: "customer"}, true},
		{model.SupportMessage{MessageType: "reply", SenderType: "user"}, true},
		{model.SupportMessage{MessageType: "system", SystemEventType: strForSlackTest("resolved")}, true},
		{model.SupportMessage{MessageType: "note", SenderType: "user", IsInternal: true}, true},
		{model.SupportMessage{MessageType: "note", SenderType: "agent", IsInternal: true, Metadata: `{"ai_handoff_brief":true,"handoff":{"reason":"Needs a teammate"}}`}, true},
		{model.SupportMessage{MessageType: "note", SenderType: "agent", IsInternal: true, Metadata: `{"code_check":{"status":"done"}}`}, true},
		{model.SupportMessage{MessageType: "note", SenderType: "agent", IsInternal: true, Metadata: `{"code_check":{"status":"running"}}`}, false},
		{model.SupportMessage{MessageType: "note", SenderType: "agent", IsInternal: true, Content: "Routine internal note"}, false},
		{model.SupportMessage{MessageType: "note", SenderType: "customer", Metadata: `{"handoff":{"reason":"Customer text"}}`}, false},
		{model.SupportMessage{MessageType: "system", SystemEventType: strForSlackTest("ai_escalated"), Metadata: `{"handoff_note_id":"brief"}`}, false},
		{model.SupportMessage{MessageType: "system", SystemEventType: strForSlackTest("reopened")}, false},
		{model.SupportMessage{MessageType: "system", SystemEventType: strForSlackTest("reopened"), SenderUserID: strForSlackTest("agent")}, true},
		{model.SupportMessage{MessageType: "system", SystemEventType: strForSlackTest("tag_added")}, false},
	} {
		if got := slackFeedThreadUpdate(&tc.m); got != tc.want {
			t.Fatalf("message %+v = %v", tc.m, got)
		}
	}
}
func strForSlackTest(v string) *string { return &v }

func TestSlackInboxFeedLateCodeCheck(t *testing.T) {
	handoff := time.Now()
	resumed := handoff.Add(time.Minute)
	for _, tc := range []struct {
		name string
		conv model.SupportConversation
		want bool
	}{
		{"awaiting teammate", model.SupportConversation{Status: "open", AIState: strPtr("escalated")}, true},
		{"resolved", model.SupportConversation{Status: "resolved", AIState: strPtr("escalated")}, false},
		{"returned to AI", model.SupportConversation{Status: "open", AIState: strPtr("pending")}, false},
		{"later handoff", model.SupportConversation{Status: "open", AIState: strPtr("escalated"), AIResumedAt: &resumed}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := feedStillNeedsCodeCheck(&tc.conv, handoff); got != tc.want {
				t.Errorf("share late result = %v, want %v", got, tc.want)
			}
		})
	}
}
