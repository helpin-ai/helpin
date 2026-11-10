package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

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
	mustExec(t, db, `CREATE TABLE ai_message_processing (
 validation_repair_used BOOLEAN NOT NULL DEFAULT 0,id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, source_message_id TEXT, reply_message_id TEXT, status TEXT, attempts INTEGER, tokens_used INTEGER, created_at DATETIME, updated_at DATETIME)`)
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
	mustExec(t, db, `CREATE TABLE ai_message_processing (
 validation_repair_used BOOLEAN NOT NULL DEFAULT 0,id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, source_message_id TEXT, reply_message_id TEXT, status TEXT, attempts INTEGER, tokens_used INTEGER, created_at DATETIME, updated_at DATETIME)`)
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
	mustExec(t, db, `CREATE TABLE ai_message_processing (
 validation_repair_used BOOLEAN NOT NULL DEFAULT 0,id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, source_message_id TEXT, reply_message_id TEXT, status TEXT, updated_at DATETIME)`)
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

func TestSupportReplyEmailChoiceDeliversEmailAndStillDeduplicates(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.AIEnabled = true
	settings.AIReplyChannels = "email"
	settings.AIAgentID = strPtr("agent")
	settings.AIResponseMode = "ai_first"
	env := setupEmailFallbackInboundTestEnv(t, settings)
	db := env.convRepo.DB()
	mustExec(t, db, `CREATE TABLE ai_message_processing (
 validation_repair_used BOOLEAN NOT NULL DEFAULT 0,id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, source_message_id TEXT, reply_message_id TEXT, status TEXT, attempts INTEGER, tokens_used INTEGER, created_at DATETIME, updated_at DATETIME)`)
	conv := &model.SupportConversation{ID: "conv", WorkspaceID: "11111111-1111-1111-1111-111111111111", Channel: "widget", Source: "widget", Status: "open"}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	ai := &SupportAIService{processingRepo: repository.NewAIMessageProcessingRepository(db), conversationRepo: env.convRepo, messageRepo: env.messageRepo, installationRepo: env.service.installRepo, wsPublisher: env.service.wsPublisher}
	svc := &InternalCommandService{supportAIService: ai, supportProcessingRepo: repository.NewAIMessageProcessingRepository(db)}
	meta := model.InternalCommandContext{WorkspaceID: conv.WorkspaceID, TargetType: "support_conversation", TargetID: conv.ID}

	source := &model.SupportMessage{ID: "email", WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "customer", ViaChannel: strPtr("email"), Content: "Hello", MessageType: "reply"}
	if err := env.messageRepo.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO ai_message_processing(id,workspace_id,conversation_id,source_message_id,status,attempts,updated_at) VALUES (?,?,?,?,'processing',1,CURRENT_TIMESTAMP)`, source.ID, conv.WorkspaceID, conv.ID, source.ID)
	for range 2 {
		if _, err := svc.executeSupportSendReply(ctx, meta, json.RawMessage(`{"content":"How can I help?","reply_kind":"conversational","confidence":1}`)); err != nil {
			t.Fatal(err)
		}
	}
	var replies []model.SupportMessage
	if err := db.Where("sender_type = ?", "ai").Find(&replies).Error; err != nil {
		t.Fatal(err)
	}
	if len(replies) != 1 {
		t.Fatalf("email replies=%d", len(replies))
	}
	if replies[0].DeliveryMode() != model.SupportDeliveryEmailOnly || replies[0].WidgetVisible() {
		t.Fatal("email answer was not restricted to email delivery")
	}
}

func TestSupportReplyPendingCustomerAfterAIResolutionReopens(t *testing.T) {
	db := newTestDB(t)
	mustExec(t, db, `INSERT INTO support_conversations(id,workspace_id,display_id,subject,status,channel,ai_state) VALUES ('conv','ws',1,'Test','resolved','widget','resolved')`)
	mustExec(t, db, `CREATE TABLE ai_message_processing (
 validation_repair_used BOOLEAN NOT NULL DEFAULT 0,id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, source_message_id TEXT, reply_message_id TEXT, status TEXT, updated_at DATETIME)`)
	source := &model.SupportMessage{ID: "customer", WorkspaceID: "ws", ConversationID: "conv", SenderType: "customer", MessageType: "reply", Content: "Actually, one more question"}
	if err := repository.NewSupportMessageRepository(db).Create(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO ai_message_processing(id,workspace_id,conversation_id,source_message_id,status) VALUES ('turn','ws','conv','customer','processing')`)
	repo := repository.NewAIMessageProcessingRepository(db)
	created, err := repo.CreateReply(context.Background(), "turn", &model.SupportMessage{ID: "answer", WorkspaceID: "ws", ConversationID: "conv", SenderType: "ai", MessageType: "reply", Content: "Here is how"})
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	var conv model.SupportConversation
	if err := db.First(&conv, "id = ?", "conv").Error; err != nil {
		t.Fatal(err)
	}
	if conv.Status != "open" || derefString(conv.AIState) != "pending" {
		t.Fatalf("status=%s ai_state=%s", conv.Status, derefString(conv.AIState))
	}
}

func TestValidationRepairIsDurableAndScoped(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	mustExec(t, db, `CREATE TABLE ai_message_processing (id TEXT PRIMARY KEY,workspace_id TEXT,conversation_id TEXT,source_message_id TEXT,status TEXT,reply_message_id TEXT,validation_repair_used BOOLEAN NOT NULL DEFAULT 0,updated_at DATETIME)`)
	mustExec(t, db, `INSERT INTO ai_message_processing(id,workspace_id,status) VALUES ('turn','ws','processing')`)
	repo := repository.NewAIMessageProcessingRepository(db)
	if ok, err := repo.ClaimValidationRepair(ctx, "other", "turn"); err != nil || ok {
		t.Fatalf("cross-workspace repair: %v %v", ok, err)
	}
	if ok, err := repo.ClaimValidationRepair(ctx, "ws", "turn"); err != nil || !ok {
		t.Fatalf("first repair: %v %v", ok, err)
	}
	repo = repository.NewAIMessageProcessingRepository(db)
	if ok, err := repo.ClaimValidationRepair(ctx, "ws", "turn"); err != nil || ok {
		t.Fatalf("second repair after restart: %v %v", ok, err)
	}
	mustExec(t, db, `UPDATE ai_message_processing SET validation_repair_used=0,status='completed' WHERE id='turn'`)
	if ok, err := repo.ClaimValidationRepair(ctx, "ws", "turn"); err != nil || ok {
		t.Fatalf("completed turn repair: %v %v", ok, err)
	}
}

func TestEvidenceRepairThroughReplyTool(t *testing.T) {
	for _, tc := range []struct {
		name          string
		corrected     bool
		citationError bool
	}{
		{"numeric corrected", true, false},
		{"numeric unresolved", false, false},
		{"abbreviated citations corrected", true, true},
		{"abbreviated citations unresolved", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			corrected := tc.corrected
			ctx := context.Background()
			settings := model.DefaultSupportInboxSettings()
			settings.AIEnabled = true
			settings.AIAgentID = strPtr("agent")
			settings.AIResponseMode = "ai_first"
			settings.AIConfidenceThreshold = 0.1
			env := setupEmailFallbackInboundTestEnv(t, settings)
			db := env.convRepo.DB()
			mustExec(t, db, `CREATE TABLE ai_message_processing (validation_repair_used BOOLEAN NOT NULL DEFAULT 0,id TEXT PRIMARY KEY,workspace_id TEXT,conversation_id TEXT,source_message_id TEXT,reply_message_id TEXT,status TEXT,attempts INTEGER,tokens_used INTEGER,created_at DATETIME,updated_at DATETIME)`)
			copySupportRunEvidenceSchema(t, db)
			var schema string
			if err := setupSupportKnowledgeTestDB(t).Raw("SELECT sql FROM sqlite_master WHERE name='agent_runs'").Scan(&schema).Error; err != nil {
				t.Fatal(err)
			}
			mustExec(t, db, schema)
			conv := &model.SupportConversation{ID: "conv", WorkspaceID: "11111111-1111-1111-1111-111111111111", Channel: "widget", Source: "widget", Status: "open"}
			if err := env.convRepo.Create(ctx, conv); err != nil {
				t.Fatal(err)
			}
			if err := env.messageRepo.Create(ctx, &model.SupportMessage{ID: "source", WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "customer", MessageType: "reply", Content: "What does Pro cost?"}); err != nil {
				t.Fatal(err)
			}
			mustExec(t, db, `INSERT INTO ai_message_processing(id,workspace_id,conversation_id,source_message_id,status,updated_at) VALUES ('turn',?,'conv','source','processing',CURRENT_TIMESTAMP)`, conv.WorkspaceID)
			mustExec(t, db, `INSERT INTO agent_runs(id,workspace_id,agent_id,target_type,target_id,status,external_runtime,external_runtime_id) VALUES ('run',?,'agent','support_conversation','conv','running','agent-runtime','rt')`, conv.WorkspaceID)
			mustExec(t, db, `UPDATE agent_runs SET input=x'7b7d',output_summary=x'7b7d' WHERE id='run'`)
			evidenceID := "aaaaaaaa-1111-4111-8111-111111111111"
			citationID := evidenceID
			badAnswer := "The Pro plan costs $99 per month."
			goodAnswer := "The Pro plan costs $49 per month."
			if tc.citationError {
				citationID = "aaaaaaaa"
				badAnswer = "**1. In the identity provider**\nCopy the x509 certificate.\n\n**2. In the app**\nEnable SSO.\n\n**3. Test it**\nSign in."
				goodAnswer = badAnswer
			}
			if err := db.Create(&model.SupportRunEvidence{WorkspaceID: conv.WorkspaceID, RunID: "run", EvidenceID: evidenceID, SourceType: "docs", Content: "The Pro plan costs $49 per month. Copy the x509 certificate, enable SSO, and sign in.", VectorScore: 0.9, CombinedScore: 0.9}).Error; err != nil {
				t.Fatal(err)
			}
			ai := &SupportAIService{handoffRepo: repository.NewAgentHandoffRepository(db), processingRepo: repository.NewAIMessageProcessingRepository(db), conversationRepo: env.convRepo, messageRepo: env.messageRepo, installationRepo: env.service.installRepo, wsPublisher: env.service.wsPublisher}
			recorder := &capturePlatformTraceRecorder{traces: make(chan *model.SupportAIRetrievalTrace, 1)}
			ai.SetSupportAIRetrievalTraceRecorder(recorder)
			svc := &InternalCommandService{supportAIService: ai, supportProcessingRepo: repository.NewAIMessageProcessingRepository(db), agentRunRepo: repository.NewAgentRunRepository(db), supportRunEvidenceRepo: repository.NewSupportRunEvidenceRepository(db)}
			meta := model.InternalCommandContext{WorkspaceID: conv.WorkspaceID, RunID: "rt", TargetType: "support_conversation", TargetID: conv.ID}
			send := func(content, kind string) string {
				raw, _ := json.Marshal(map[string]any{"content": content, "reply_kind": kind, "confidence": 0.99, "can_answer": true, "source_doc_ids": []string{citationID}, "claims": []AIResponseClaim{{Text: content, EvidenceIDs: []string{citationID}}}})
				output, err := svc.executeSupportSendReply(ctx, meta, raw)
				if err != nil {
					t.Fatal(err)
				}
				var result struct {
					Status               string   `json:"status"`
					AvailableEvidenceIDs []string `json:"available_evidence_ids"`
				}
				if err := json.Unmarshal(output, &result); err != nil {
					t.Fatal(err)
				}
				if result.Status == "rewrite_required" || result.Status == "escalated" {
					select {
					case trace := <-recorder.traces:
						var metadata map[string]any
						if err := json.Unmarshal(trace.Metadata, &metadata); err != nil {
							t.Fatal(err)
						}
						want := "offered"
						if result.Status == "escalated" {
							want = "exhausted"
						}
						if metadata["repair_decision"] != want {
							t.Fatalf("repair decision=%v want %s", metadata["repair_decision"], want)
						}
					case <-time.After(time.Second):
						t.Fatal("missing repair trace")
					}
				}
				if result.Status == "rewrite_required" && (len(result.AvailableEvidenceIDs) != 1 || result.AvailableEvidenceIDs[0] != evidenceID) {
					t.Fatalf("missing complete correction evidence IDs: %v", result.AvailableEvidenceIDs)
				}
				return result.Status
			}
			if got := send(badAnswer, "answer"); got != "rewrite_required" {
				t.Fatalf("first rejection=%s", got)
			}
			var processing model.AIMessageProcessing
			if err := db.First(&processing, "id = ?", "turn").Error; err != nil {
				t.Fatal(err)
			}
			if processing.Status != "processing" || !processing.ValidationRepairUsed || processing.ReplyMessageID != nil {
				t.Fatalf("repair settled or lost turn: %+v", processing)
			}
			var run model.AgentRun
			if err := db.First(&run, "id = ?", "run").Error; err != nil {
				t.Fatal(err)
			}
			if run.Status != "running" {
				t.Fatalf("repair stopped run: %s", run.Status)
			}
			var count int64
			if err := db.Model(&model.SupportMessage{}).Where("sender_type='ai'").Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("rejected answer leaked: %d %v", count, err)
			}
			if !corrected {
				if got := send(badAnswer, "conversational"); got != "escalated" {
					t.Fatalf("second invalid answer=%s", got)
				}
				if got := send(badAnswer, "answer"); got != "suppressed" {
					t.Fatalf("after handoff=%s", got)
				}
				if err := db.Model(&model.SupportMessage{}).Where("sender_type='ai' AND is_internal=false AND content = ?", badAnswer).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("invalid price sent: %d %v", count, err)
				}
				return
			}
			citationID = evidenceID
			if got := send(goodAnswer, "conversational"); got != "sent" {
				t.Fatalf("corrected answer=%s", got)
			}
			if got := send(goodAnswer, "answer"); got != "suppressed" {
				t.Fatalf("duplicate=%s", got)
			}
		})
	}
}
