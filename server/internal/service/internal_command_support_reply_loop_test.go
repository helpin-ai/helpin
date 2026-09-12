package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportReplyRequiresUnansweredCustomerTurn(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.AIEnabled = true
	settings.AIAgentID = strPtr("agent")
	settings.AIResponseMode = "ai_first"
	env := setupEmailFallbackInboundTestEnv(t, settings)
	db := env.convRepo.DB()
	mustExec(t, db, `CREATE TABLE ai_message_processing (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, source_message_id TEXT, reply_message_id TEXT, status TEXT, attempts INTEGER, tokens_used INTEGER, created_at DATETIME, updated_at DATETIME)`)
	conv := &model.SupportConversation{ID: "conv", WorkspaceID: "11111111-1111-1111-1111-111111111111", Channel: "widget", Source: "widget", Status: "open"}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	ai := &SupportAIService{processingRepo: repository.NewAIMessageProcessingRepository(db), conversationRepo: env.convRepo, messageRepo: env.messageRepo, installationRepo: env.service.installRepo, wsPublisher: env.service.wsPublisher}
	svc := &InternalCommandService{supportAIService: ai, supportProcessingRepo: repository.NewAIMessageProcessingRepository(db)}
	meta := model.InternalCommandContext{WorkspaceID: conv.WorkspaceID, TargetType: "support_conversation", TargetID: conv.ID}
	input := json.RawMessage(`{"content":"Hello! How can I help?","reply_kind":"conversational","confidence":1}`)
	send := func(want string) {
		t.Helper()
		raw, err := svc.executeSupportSendReply(ctx, meta, input)
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		if result.Status != want {
			t.Fatalf("status=%q, want %q", result.Status, want)
		}
	}
	send("suppressed")
	for _, source := range []string{"first", "second"} {
		if err := env.messageRepo.Create(ctx, &model.SupportMessage{ID: source, WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "customer", Content: "Hello", MessageType: "reply"}); err != nil {
			t.Fatal(err)
		}
		mustExec(t, db, `INSERT INTO ai_message_processing(id,workspace_id,conversation_id,source_message_id,status,attempts,updated_at) VALUES (?,?,?,?, 'processing',1,CURRENT_TIMESTAMP)`, source, conv.WorkspaceID, conv.ID, source)
		send("sent")
		for range 3 {
			send("suppressed")
		}
	}

	for _, emailConversation := range []bool{false, true} {
		sourceID := "email-source"
		via := "email"
		if emailConversation {
			sourceID = "old-email-thread"
			via = "widget"
			mustExec(t, db, `UPDATE support_conversations SET channel='email',source='email' WHERE id='conv'`)
		}
		if err := env.messageRepo.Create(ctx, &model.SupportMessage{ID: sourceID, WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "customer", ViaChannel: &via, Content: "I am away", MessageType: "reply"}); err != nil {
			t.Fatal(err)
		}
		mustExec(t, db, `INSERT INTO ai_message_processing(id,workspace_id,conversation_id,source_message_id,status,attempts,updated_at) VALUES (?,?,?,?,'processing',1,CURRENT_TIMESTAMP)`, sourceID, conv.WorkspaceID, conv.ID, sourceID)
		send("suppressed")
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("sender_type='ai'").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("AI messages=%d, want 2", count)
	}
}

func TestSupportReplyAtomicPersistence(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	mustExec(t, db, `INSERT INTO support_conversations(id,workspace_id,display_id,subject,status,channel) VALUES ('conv','ws',1,'Test','open','widget')`)
	mustExec(t, db, `CREATE TABLE ai_message_processing (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, source_message_id TEXT, reply_message_id TEXT, status TEXT, attempts INTEGER, tokens_used INTEGER, created_at DATETIME, updated_at DATETIME)`)
	mustExec(t, db, `INSERT INTO ai_message_processing(id,workspace_id,conversation_id,status) VALUES ('turn','ws','conv','processing')`)
	repo := repository.NewAIMessageProcessingRepository(db)
	reply := func(id string) *model.SupportMessage {
		return &model.SupportMessage{ID: id, WorkspaceID: "ws", ConversationID: "conv", SenderType: "ai", MessageType: "reply", Content: "Hello"}
	}
	bad := reply("bad")
	bad.MessageType = "system"
	if created, err := repo.CreateReply(ctx, "turn", bad); err == nil || created {
		t.Fatalf("invalid reply = %v/%v", created, err)
	}
	if created, err := repo.CreateReply(ctx, "turn", reply("first")); err != nil || !created {
		t.Fatalf("valid retry = %v/%v", created, err)
	}
	for _, transition := range []func() error{
		func() error { return repo.MarkProcessing(ctx, "turn") },
		func() error { return repo.MarkDeferred(ctx, "turn") },
		func() error { return repo.MarkFailed(ctx, "turn") },
		func() error { return repo.MarkCompleted(ctx, "turn", nil, 0) },
	} {
		if err := transition(); err != nil {
			t.Fatal(err)
		}
	}
	if created, err := repo.CreateReply(ctx, "turn", reply("duplicate")); err != nil || created {
		t.Fatalf("duplicate = %v/%v", created, err)
	}
	var row model.AIMessageProcessing
	if err := db.First(&row, "id = ?", "turn").Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != "completed" || derefString(row.ReplyMessageID) != "first" {
		t.Fatalf("lost completed reply: %+v", row)
	}
	var count int64
	db.Model(&model.SupportMessage{}).Count(&count)
	if count != 1 {
		t.Fatalf("created %d replies", count)
	}
}

func TestSupportReplyConcurrentPersistence(t *testing.T) {
	db := newTestDB(t)
	mustExec(t, db, `INSERT INTO support_conversations(id,workspace_id,display_id,subject,status,channel) VALUES ('conv','ws',1,'Test','open','widget')`)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	mustExec(t, db, `CREATE TABLE ai_message_processing (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, source_message_id TEXT, reply_message_id TEXT, status TEXT, updated_at DATETIME)`)
	mustExec(t, db, `INSERT INTO ai_message_processing(id,workspace_id,conversation_id,status) VALUES ('turn','ws','conv','processing')`)
	repo := repository.NewAIMessageProcessingRepository(db)
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for _, id := range []string{"a", "b"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			created, err := repo.CreateReply(context.Background(), "turn", &model.SupportMessage{ID: id, WorkspaceID: "ws", ConversationID: "conv", SenderType: "ai", MessageType: "reply", Content: "Hello"})
			if err != nil {
				t.Error(err)
			}
			results <- created
		}(id)
	}
	wg.Wait()
	close(results)
	count := 0
	for created := range results {
		if created {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("concurrent replies=%d, want 1", count)
	}
}
