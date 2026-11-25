package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

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
func TestSlackInboxFeedThreadUpdates(t *testing.T) {
	for _, tc := range []struct {
		m    model.SupportMessage
		want bool
	}{
		{model.SupportMessage{MessageType: "reply", SenderType: "customer"}, true},
		{model.SupportMessage{MessageType: "reply", SenderType: "user"}, true},
		{model.SupportMessage{MessageType: "system", SystemEventType: strForSlackTest("resolved")}, true},
		{model.SupportMessage{MessageType: "note", SenderType: "user", IsInternal: true}, true},
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
