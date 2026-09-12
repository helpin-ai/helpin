package model

import "testing"

func TestSupportAIReplyChannels(t *testing.T) {
	email := "email"
	for _, tc := range []struct {
		selection   string
		chat, email bool
	}{
		{"", true, false}, {"chat", true, false}, {"email", false, true}, {"both", true, true}, {"invalid", false, false},
	} {
		settings := DefaultSupportInboxSettings()
		settings.AIReplyChannels = tc.selection
		conv := &SupportConversation{Channel: "widget"}
		msg := &SupportMessage{SenderType: "customer", Content: "Please help"}
		if got := SupportAIReplyAllowed(settings, conv, msg); got != tc.chat {
			t.Errorf("%q chat=%v", tc.selection, got)
		}
		msg.ViaChannel = &email
		if got := SupportAIReplyAllowed(settings, conv, msg); got != tc.email {
			t.Errorf("%q email continuation=%v", tc.selection, got)
		}
		conv.Channel = "email"
		if got := SupportAIReplyAllowed(settings, conv, msg); got != tc.email {
			t.Errorf("%q email=%v", tc.selection, got)
		}
	}
}

func TestSupportAIRejectsAutomaticEmailOnBothChannels(t *testing.T) {
	settings := DefaultSupportInboxSettings()
	settings.AIReplyChannels = "both"
	email := "email"
	for _, msg := range []SupportMessage{
		{SenderType: "customer", ViaChannel: &email, Content: "I am currently away from the office without access to email, returning Wednesday."},
		{SenderType: "customer", ViaChannel: &email, Content: "Received", Metadata: `{"email_auto_reply":true}`},
		{SenderType: "customer", ViaChannel: &email, Content: "Hello", IsInternal: true},
	} {
		if SupportAIReplyAllowed(settings, &SupportConversation{Channel: "email"}, &msg) {
			t.Fatalf("accepted automatic/internal message: %+v", msg)
		}
	}
}

func TestSupportAISuppressedEmailSignals(t *testing.T) {
	settings := DefaultSupportInboxSettings()
	settings.AIReplyChannels = "both"
	for _, metadata := range []string{
		`{"email_ai_suppression_reason":"delivery_report"}`,
		`{"email_ai_suppression_reason":"mailing_list"}`,
		`{"postmark_spam_status":"Yes, score=7"}`,
	} {
		message := &SupportMessage{SenderType: "customer", Content: "Automated notification", Metadata: metadata}
		if SupportAIReplyAllowed(settings, &SupportConversation{Channel: "email"}, message) {
			t.Errorf("accepted suppressed email %s", metadata)
		}
	}
}

func TestSupportAIConversationBlockedAfterOwnershipOrStatusChanges(t *testing.T) {
	human := "teammate"
	for _, c := range []SupportConversation{
		{AssignedUserID: &human}, {OpenedByUserID: &human}, {Status: "resolved"}, {Status: "spam"},
	} {
		if !SupportAIConversationBlocked(&c) {
			t.Fatalf("should block: %+v", c)
		}
	}
	if SupportAIConversationBlocked(&SupportConversation{Status: "open"}) {
		t.Fatal("open AI conversation blocked")
	}
}

func TestSupportAIResolvedConversationAcceptsFreshCustomerTurn(t *testing.T) {
	state := "resolved"
	if SupportAIConversationBlocked(&SupportConversation{Status: "resolved", AIState: &state}) {
		t.Fatal("fresh customer turn after AI resolution blocked")
	}
}
