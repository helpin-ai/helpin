package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSkipSupportReplySettlesWithoutPublicMessage(t *testing.T) {
	for _, reason := range []string{"spam", "automated_message", "needs_review"} {
		t.Run(reason, func(t *testing.T) {
			chat, db, conv, source, processing, _, _ := setupSupportGreetingTest(t)
			ai := chat.supportAIService
			ai.conversationRepo, ai.messageRepo, ai.processingRepo = chat.conversationRepo, chat.messageRepo, chat.processingRepo
			mustExec(t, db, `CREATE TABLE agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT, external_runtime TEXT, external_runtime_id TEXT)`)
			mustExec(t, db, `INSERT INTO agent_runs VALUES ('host-run', ?, ?, 'runtime-run')`, conv.WorkspaceID, agentRuntimeName)
			mustExec(t, db, `UPDATE support_conversations SET ai_active_run_id='host-run', ai_control_version=1, channel='email', last_customer_message_id=? WHERE id=?`, source.ID, conv.ID)
			commands := &InternalCommandService{supportAIService: ai, supportProcessingRepo: chat.processingRepo, agentRunRepo: repository.NewAgentRunRepository(db)}
			meta := model.InternalCommandContext{WorkspaceID: conv.WorkspaceID, TargetType: "support_conversation", TargetID: conv.ID, RunID: "runtime-run"}
			input, _ := json.Marshal(map[string]string{"reason": reason, "summary": "No customer request; notification only."})
			for attempt := 0; attempt < 2; attempt++ {
				output, err := commands.executeSupportSkipReply(context.Background(), meta, input)
				if err != nil {
					t.Fatal(err)
				}
				var result map[string]any
				_ = json.Unmarshal(output, &result)
				if result["status"] != "suppressed" {
					t.Fatalf("unexpected result: %s", output)
				}
			}
			var row model.AIMessageProcessing
			db.First(&row, "id = ?", processing.ID)
			if row.Status != "completed" || row.ReplyMessageID != nil {
				t.Fatalf("unsettled or replied: %+v", row)
			}
			var messages []model.SupportMessage
			db.Where("conversation_id = ?", conv.ID).Find(&messages)
			if len(messages) != 2 {
				t.Fatalf("wanted source + one audit note; got %d", len(messages))
			}
			for _, msg := range messages {
				if msg.ID != source.ID && (!msg.IsInternal || msg.MessageType != "note") {
					t.Fatalf("unexpected public message: %+v", msg)
				}
			}
			current, err := chat.conversationRepo.GetByID(context.Background(), conv.WorkspaceID, conv.ID, "", model.RoleOwner)
			if err != nil {
				t.Fatal(err)
			}
			if (current.Status == "spam") != (reason == "spam") {
				t.Fatalf("incorrect status: %s", current.Status)
			}
			if (current.HumanTakeover != nil && *current.HumanTakeover) != (reason == "needs_review") {
				t.Fatalf("incorrect takeover: %+v", current.HumanTakeover)
			}
			if derefString(current.AIState) == "escalated" {
				t.Fatal("silent decision became escalation")
			}
			stored, err := chat.messageRepo.GetByID(context.Background(), source.ID)
			if err != nil || !model.SupportEmailSuppressesAI(stored) {
				t.Fatal("source can be replied to on retry")
			}
		})
	}
}

func TestSkipSupportReplyPreservesHumanOwnership(t *testing.T) {
	chat, db, conv, _, processing, _, _ := setupSupportGreetingTest(t)
	ai := chat.supportAIService
	ai.conversationRepo, ai.messageRepo, ai.processingRepo = chat.conversationRepo, chat.messageRepo, chat.processingRepo
	mustExec(t, db, `UPDATE support_conversations SET human_takeover=true WHERE id=?`, conv.ID)
	commands := &InternalCommandService{supportAIService: ai, supportProcessingRepo: chat.processingRepo}
	_, err := commands.executeSupportSkipReply(context.Background(), model.InternalCommandContext{WorkspaceID: conv.WorkspaceID, TargetType: "support_conversation", TargetID: conv.ID}, json.RawMessage(`{"reason":"spam","summary":"Suspicious notification"}`))
	if err != nil {
		t.Fatal(err)
	}
	var current model.SupportConversation
	db.First(&current, "id = ?", conv.ID)
	if current.Status == "spam" {
		t.Fatal("changed human-owned conversation")
	}
	var row model.AIMessageProcessing
	db.First(&row, "id = ?", processing.ID)
	if row.Status != "processing" {
		t.Fatal("changed human-owned turn")
	}
}

func TestSkipSupportReplyDoesNotPartiallyApply(t *testing.T) {
	for _, test := range []struct{ name, sql string }{
		{"newer customer message", `UPDATE support_conversations SET last_customer_message_id='new-question'`},
		{"audit write fails", `CREATE TRIGGER reject_note BEFORE INSERT ON support_messages WHEN NEW.is_internal = 1 BEGIN SELECT RAISE(ABORT, 'audit unavailable'); END`},
	} {
		t.Run(test.name, func(t *testing.T) {
			chat, db, conv, source, processing, _, _ := setupSupportGreetingTest(t)
			ai := chat.supportAIService
			ai.conversationRepo, ai.messageRepo, ai.processingRepo = chat.conversationRepo, chat.messageRepo, chat.processingRepo
			mustExec(t, db, test.sql)
			commands := &InternalCommandService{supportAIService: ai, supportProcessingRepo: chat.processingRepo}
			_, err := commands.executeSupportSkipReply(context.Background(), model.InternalCommandContext{WorkspaceID: conv.WorkspaceID, TargetType: "support_conversation", TargetID: conv.ID}, json.RawMessage(`{"reason":"spam","summary":"Suspicious notification"}`))
			if err == nil {
				t.Fatal("expected error")
			}
			current, err := chat.conversationRepo.GetByID(context.Background(), conv.WorkspaceID, conv.ID, "", model.RoleOwner)
			if err != nil {
				t.Fatal(err)
			}
			if current.Status == "spam" {
				t.Fatal("conversation was incorrectly hidden")
			}
			stored, err := chat.messageRepo.GetByID(context.Background(), source.ID)
			if err != nil || model.SupportEmailSuppressesAI(stored) {
				t.Fatal("partially suppressed source")
			}
			var row model.AIMessageProcessing
			if err := db.First(&row, "id = ?", processing.ID).Error; err != nil {
				t.Fatal(err)
			}
			if row.Status != "processing" {
				t.Fatal("incorrectly settled source")
			}
		})
	}
}
