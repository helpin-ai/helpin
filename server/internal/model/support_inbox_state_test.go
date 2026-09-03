package model

import (
	"testing"
	"time"
)

func TestSupportConversationApplyMessageProjection(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)

	t.Run("customer reply creates human workload for a human-owned conversation", func(t *testing.T) {
		conversation := SupportConversation{Status: SupportConversationStatusOpen}
		message := SupportMessage{ID: "message-1", SenderType: "customer", MessageType: "reply", Content: "Need help", CreatedAt: now}

		conversation.ApplyMessageProjection(message)

		if !conversation.CustomerAwaitingResponse || !conversation.NeedsHumanReply {
			t.Fatalf("customer reply state = awaiting:%v human:%v, want both true", conversation.CustomerAwaitingResponse, conversation.NeedsHumanReply)
		}
		if conversation.UnansweredCustomerMessageCount != 1 {
			t.Fatalf("unanswered count = %d, want 1", conversation.UnansweredCustomerMessageCount)
		}
		if conversation.LastCustomerMessageID == nil || *conversation.LastCustomerMessageID != message.ID {
			t.Fatalf("last customer message = %v, want %s", conversation.LastCustomerMessageID, message.ID)
		}
	})

	t.Run("customer reply does not create human workload while AI owns conversation", func(t *testing.T) {
		flow := SupportConversationFlowStateAIHandling
		conversation := SupportConversation{Status: SupportConversationStatusOpen, FlowState: &flow}

		conversation.ApplyMessageProjection(SupportMessage{ID: "message-2", SenderType: "customer", MessageType: "reply", Content: "Hello", CreatedAt: now})

		if !conversation.CustomerAwaitingResponse {
			t.Fatal("customer reply must keep the shared blue-dot state")
		}
		if conversation.NeedsHumanReply {
			t.Fatal("AI-owned reply must not enter human workload")
		}
	})

	t.Run("public teammate reply clears response state", func(t *testing.T) {
		conversation := SupportConversation{Status: SupportConversationStatusOpen, CustomerAwaitingResponse: true, NeedsHumanReply: true, UnansweredCustomerMessageCount: 3}

		conversation.ApplyMessageProjection(SupportMessage{ID: "message-3", SenderType: "user", MessageType: "reply", Content: "Done", CreatedAt: now})

		if conversation.CustomerAwaitingResponse || conversation.NeedsHumanReply || conversation.UnansweredCustomerMessageCount != 0 {
			t.Fatalf("teammate response state = awaiting:%v human:%v count:%d", conversation.CustomerAwaitingResponse, conversation.NeedsHumanReply, conversation.UnansweredCustomerMessageCount)
		}
	})

	t.Run("internal note does not clear customer response state", func(t *testing.T) {
		conversation := SupportConversation{Status: SupportConversationStatusOpen, CustomerAwaitingResponse: true, NeedsHumanReply: true, UnansweredCustomerMessageCount: 1}

		conversation.ApplyMessageProjection(SupportMessage{ID: "message-4", SenderType: "user", MessageType: "reply", IsInternal: true, Content: "@Sam please look", CreatedAt: now})

		if !conversation.CustomerAwaitingResponse || !conversation.NeedsHumanReply || conversation.UnansweredCustomerMessageCount != 1 {
			t.Fatal("internal note changed shared response state")
		}
	})
}

func TestSupportConversationRecomputeHumanAttention(t *testing.T) {
	conversation := SupportConversation{Status: SupportConversationStatusOpen, CustomerAwaitingResponse: true}
	conversation.RecomputeHumanAttention()
	if !conversation.NeedsHumanReply {
		t.Fatal("open human-owned conversation should need a human reply")
	}

	conversation.Status = SupportConversationStatusResolved
	conversation.RecomputeHumanAttention()
	if conversation.CustomerAwaitingResponse || conversation.NeedsHumanReply {
		t.Fatal("resolved conversation must clear response state")
	}
}

func TestSupportConversationUserStateReadAndManualUnread(t *testing.T) {
	t0 := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Minute)
	state := SupportConversationUserState{RelevanceMask: SupportRelevanceAssignee, ManuallyUnread: true}

	state.ApplyCustomerReply("customer-1", t1)
	if state.UnreadCustomerMessageCount != 1 || state.EffectiveUnreadCount() != 1 {
		t.Fatalf("unread counts = message:%d effective:%d, want 1", state.UnreadCustomerMessageCount, state.EffectiveUnreadCount())
	}

	if advanced := state.MarkReadThrough("customer-1", t1, 0); !advanced {
		t.Fatal("new cursor should advance")
	}
	if state.EffectiveUnreadCount() != 0 || state.ManuallyUnread {
		t.Fatal("read-through should clear message and manual unread")
	}

	if advanced := state.MarkReadThrough("older", t0, 2); advanced {
		t.Fatal("older cursor must not move personal read state backwards")
	}
}

func TestManualOnlyUserDoesNotSubscribeToFutureReplies(t *testing.T) {
	state := SupportConversationUserState{ManuallyUnread: true}
	state.ApplyCustomerReply("customer-1", time.Now())
	if state.UnreadCustomerMessageCount != 0 {
		t.Fatal("manual-only user must not receive future customer-reply increments")
	}
	if state.EffectiveUnreadCount() != 1 {
		t.Fatal("manual reminder should remain visible")
	}
}
