package service

import (
	"context"
	"net/url"
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

func TestSlackFeedInitialsBadge(t *testing.T) {
	f := feedContext{conv: &model.SupportConversation{Channel: "widget"}}
	name, user := "Maya Patel", "member-42"
	msg := model.SupportMessage{SenderType: "user", SenderDisplayName: &name, SenderUserID: &user}
	header := func() []any {
		return f.feedAuthor(&msg).header("https://api.helpin.example/api")["elements"].([]any)
	}
	elements := header()
	image := elements[0].(map[string]any)
	source, err := url.Parse(image["image_url"].(string))
	if err != nil || len(elements) != 2 || image["type"] != "image" || source.Host != "api.helpin.example" || source.Query().Get("i") != "MP" || source.Query().Get("c") == "" || strings.Contains(source.String(), user) {
		t.Fatalf("initials badge: %#v, %v", elements, err)
	}
	msg.IsInternal = true
	name = "Maya Reed"
	next, err := url.Parse(header()[0].(map[string]any)["image_url"].(string))
	if err != nil || next.Query().Get("c") != source.Query().Get("c") || next.Query().Get("i") != "MR" {
		t.Fatalf("color changed with role or name: %v", next)
	}
	msg.SenderAvatarURL = strPtr("https://images.example/maya.jpg")
	if image := header()[0].(map[string]any); image["image_url"] != *msg.SenderAvatarURL {
		t.Fatalf("profile photo: %v", image)
	}
}

func TestSlackInboxFeedTeamLineBreaks(t *testing.T) {
	s := &SlackService{channels: &SupportChannelService{inbox: &SupportInboxService{}}}
	stored := "Hi there,\\\n\\\nThanks for the **details**.\\\nSee C:\\Docs"
	want := "Hi there,  \n  \nThanks for the **details**.  \nSee C:\\Docs"
	// A backslash in a code block is part of the teammate's example.
	code := "\n\n```sh\nrun \\\n  next\n```"
	for _, channel := range []string{"widget", "email"} {
		f := &feedContext{conv: &model.SupportConversation{Channel: channel}}
		msg := &model.SupportMessage{SenderType: "user", MessageType: "reply", Content: stored + code}
		body, rich, _, err := s.feedContent(context.Background(), f, msg)
		if err != nil {
			t.Fatal(err)
		}
		blocks, _ := feedReplyBody(body, rich)
		if len(blocks) != 1 {
			t.Fatalf("%s: got %d body blocks", channel, len(blocks))
		}
		block, ok := blocks[0].(map[string]string)
		if !ok || block["type"] != "markdown" || block["text"] != want+code {
			t.Errorf("%s: team reply = %#v", channel, blocks[0])
		}
	}
	for _, msg := range []model.SupportMessage{
		{SenderType: "customer", MessageType: "reply", Content: stored},
		{SenderType: "user", MessageType: "reply", Content: stored, ExternalMessageKey: strPtr("incoming")},
		{SenderType: "user", MessageType: "reply", Content: stored, Metadata: `{"via":"slack"}`},
	} {
		f := &feedContext{conv: &model.SupportConversation{Channel: "widget"}}
		if body, rich, _, err := s.feedContent(context.Background(), f, &msg); err != nil || rich || body != stored {
			t.Errorf("literal %s message = %q, rich=%v, err=%v", msg.SenderType, body, rich, err)
		}
	}
}

func TestSlackInboxFeedSpeakerHeader(t *testing.T) {
	for _, tc := range []struct{ name, role, want string }{
		{"Maya Patel", "Team reply", "*Maya Patel* · *Team reply*"},
		{"Sam", "📝 Internal note", "*Sam* · *📝 Internal note*"},
		{"<@U123>", "Visitor", "<@U123> · Visitor"},
		{"", "Visitor", "*Visitor*"},
		{"Visitor", "Visitor", "*Visitor*"},
	} {
		speaker := feedSpeaker{name: tc.name, role: tc.role}
		elements := speaker.header("https://api.example")["elements"].([]any)
		if len(elements) > 2 {
			t.Fatalf("unexpected gap from extra header elements: %v", elements)
		}
		if tc.name == "<@U123>" {
			label := elements[len(elements)-1].(map[string]string)
			if label["type"] != "plain_text" || label["text"] != tc.want {
				t.Fatalf("unsafe display name: %#v", label)
			}
			continue
		}
		last := elements[len(elements)-1].(map[string]any)
		if last["text"] != tc.want {
			t.Errorf("visible header for %q = %q, want %q", tc.name, last["text"], tc.want)
		}
	}
}

func TestSlackInboxFeedHTMLReply(t *testing.T) {
	raw := `<div style="display: none">Tracking preheader</div><p>Please <strong>reset</strong> my account.</p><div class="gmail_quote">Old conversation</div>`
	md := feedHTMLReply(raw)
	if strings.Contains(md, "preheader") || strings.Contains(md, "Old conversation") || !strings.Contains(md, "**reset**") {
		t.Fatalf("visible email = %q", md)
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
