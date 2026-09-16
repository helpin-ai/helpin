package service

import (
	"context"
	"encoding/json"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"strings"
	"testing"
	"time"
)

func TestAnonymizedConversationSuppressesHumanAndAIReplies(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := repository.NewSupportConversationRepository(db)
	now := time.Now()
	conv := &model.SupportConversation{WorkspaceID: "ws-deleted", Subject: "Kept", AnonymizedAt: &now}
	if err := repo.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	inbox := &SupportInboxService{conversationRepo: repo}
	// No session or CRM dependency should be consulted to re-enrich a deleted customer.
	if visitor, err := inbox.GetVisitorContext(ctx, conv.WorkspaceID, conv.ID); err != nil || visitor == nil {
		t.Fatalf("anonymized visitor context: %+v %v", visitor, err)
	}
	if _, err := inbox.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, model.CreateMessageRequest{Content: "Late reply"}, "user", nil, nil, nil); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("reply not blocked: %v", err)
	}
	commands := &InternalCommandService{supportAIService: &SupportAIService{conversationRepo: repo}}
	result, err := commands.executeSupportSendReply(ctx, model.InternalCommandContext{WorkspaceID: conv.WorkspaceID, TargetType: "support_conversation", TargetID: conv.ID}, json.RawMessage(`{"content":"Hello"}`))
	if err != nil || !strings.Contains(string(result), `"suppressed"`) {
		t.Fatalf("AI reply not suppressed: %s %v", result, err)
	}
}

func TestAnonymizedConversationDropsQueuedEmail(t *testing.T) {
	env := setupEmailFallbackTestEnv(t, model.DefaultSupportInboxSettings())
	ctx := context.Background()
	now := time.Now()
	conv := &model.SupportConversation{WorkspaceID: "11111111-1111-1111-1111-111111111111", Subject: "Kept", AnonymizedAt: &now}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	msg := &model.SupportMessage{WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "user", Content: "Retain pending reply"}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatal(err)
	}
	if err := env.redis.RPush(ctx, env.service.msgListKey(conv.ID), msg.ID).Err(); err != nil {
		t.Fatal(err)
	}
	// The email client is deliberately unavailable. Successful cleanup proves we
	// stop before recipient validation or an external send, including explicit mail.
	env.service.emailClient = nil
	for _, explicit := range []bool{false, true} {
		if err := env.service.fireEmailBatch(ctx, conv.ID, []string{msg.ID}, emailFallbackFireOptions{cleanupRedis: true, explicitEmail: explicit}); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := env.redis.LLen(ctx, env.service.msgListKey(conv.ID)).Result(); err != nil || n != 0 {
		t.Fatalf("queue retained: %d %v", n, err)
	}
	kept, err := env.messageRepo.GetByIDs(ctx, []string{msg.ID})
	if err != nil || len(kept) != 1 || kept[0].Content != msg.Content {
		t.Fatalf("message lost: %+v %v", kept, err)
	}
}
