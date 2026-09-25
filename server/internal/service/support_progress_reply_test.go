package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func setupSupportProgressTest(t *testing.T) (*SupportChatService, *InternalCommandService, *gorm.DB, *model.SupportConversation, *model.AgentRun, model.InternalCommandContext) {
	t.Helper()
	chat, db, conv, _, _, _, _ := setupSupportGreetingTest(t)
	mustExec(t, db, `CREATE TABLE agent_runs (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		target_type TEXT NOT NULL,
		target_id TEXT NOT NULL,
		model_tier TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL,
		external_runtime TEXT,
		external_runtime_id TEXT,
		input BLOB NOT NULL DEFAULT '{}',
		output_summary BLOB NOT NULL DEFAULT '{}'
	)`)
	mustExec(t, db, `CREATE TABLE support_run_evidence (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		run_id TEXT NOT NULL,
		evidence_id TEXT NOT NULL,
		reference_id TEXT,
		source_type TEXT NOT NULL,
		source_id TEXT,
		document_id TEXT,
		title TEXT,
		url TEXT,
		is_internal INTEGER NOT NULL DEFAULT 0,
		content TEXT NOT NULL,
		lexical_score REAL NOT NULL DEFAULT 0,
		vector_score REAL NOT NULL DEFAULT 0,
		combined_score REAL NOT NULL DEFAULT 0,
		created_at DATETIME,
		UNIQUE(run_id, evidence_id)
	)`)
	mustExec(t, db, `CREATE TABLE command_bar_plans (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			actor_id TEXT,
			parent_chat_run_id TEXT,
			dock_chat_id TEXT,
			support_conversation_id TEXT,
			parent_notified_at DATETIME,
			status TEXT NOT NULL DEFAULT 'running',
			prompt TEXT NOT NULL,
			page_context BLOB NOT NULL DEFAULT '{}',
			steps BLOB NOT NULL DEFAULT '[]',
			run_ids_by_step BLOB NOT NULL DEFAULT '{}',
			profile_binding BLOB,
			current_step_index INTEGER NOT NULL DEFAULT 0,
			run_count INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			cancelled_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`)

	mustExec(t, db, `INSERT INTO agent_runs(id,workspace_id,agent_id,target_type,target_id,status,external_runtime,external_runtime_id) VALUES ('run','ws','agent','support_conversation','conv','running','agent-runtime','runtime-run')`)
	mustExec(t, db, `UPDATE agent_runs SET input=?, output_summary=? WHERE id='run'`, []byte(`{}`), []byte(`{}`))
	mustExec(t, db, `INSERT INTO command_bar_plans(id,workspace_id,parent_chat_run_id,support_conversation_id,status,prompt) VALUES ('plan','ws','run','conv','running','Check the current pricing')`)
	mustExec(t, db, `UPDATE support_conversations SET ai_active_run_id='run', ai_control_version=1 WHERE id='conv'`)
	conv.AIActiveRunID = strPtr("run")
	conv.AIControlVersion = 1
	ai := chat.supportAIService
	ai.conversationRepo = chat.conversationRepo
	ai.messageRepo = chat.messageRepo
	ai.processingRepo = chat.processingRepo
	ai.installationRepo = repository.NewSupportInboxInstallationRepository(db)
	chat.planRepo = repository.NewCommandBarPlanRepository(db)
	commands := &InternalCommandService{supportAIService: ai, supportProcessingRepo: chat.processingRepo, agentRunRepo: repository.NewAgentRunRepository(db), supportRunEvidenceRepo: repository.NewSupportRunEvidenceRepository(db), commandBarService: &CommandBarService{planRepo: chat.planRepo}}
	run := &model.AgentRun{ID: "run", WorkspaceID: "ws", TargetType: "support_conversation", TargetID: "conv", Status: model.AgentRunStatusPaused, PauseReason: model.AgentRunPauseReasonUserMessage, Input: json.RawMessage(`{"trigger":{"source":"system","trigger_type":"support_chat"}}`), UpdatedAt: time.Now()}
	return chat, commands, db, conv, run, model.InternalCommandContext{WorkspaceID: "ws", TargetType: "support_conversation", TargetID: "conv", AgentID: "agent", RunID: "runtime-run"}
}

func TestSupportResearchAcknowledgmentKeepsTurnOpenForFinalAnswer(t *testing.T) {
	ctx := context.Background()
	chat, commands, db, conv, run, meta := setupSupportProgressTest(t)
	acknowledgment := json.RawMessage(`{"content":"I'm checking the current plan details and will be right back.","reply_kind":"conversational","confidence":0.6}`)
	send := func(input json.RawMessage, want string) {
		t.Helper()
		output, err := commands.executeSupportSendReply(ctx, meta, input)
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatal(err)
		}
		if result.Status != want {
			t.Fatalf("reply = %s, want %s", output, want)
		}
	}
	send(acknowledgment, "awaiting_result")
	send(acknowledgment, "awaiting_result")
	row, err := chat.processingRepo.LatestProcessingForConversation(ctx, "ws", "conv")
	if err != nil || row == nil || row.Status != "waiting_for_result" || row.ReplyMessageID != nil {
		t.Fatalf("pending turn: %+v, %v", row, err)
	}
	if _, claimed := chat.processingRepo.BeginAttempt(ctx, "ws", "source", "conv"); claimed {
		t.Fatal("redelivery reclaimed a waiting turn")
	}
	chat.OnSupportChatRunPaused(ctx, run)
	var afterPause model.AIMessageProcessing
	if err := db.First(&afterPause, "id = ?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterPause.Attempts != 1 || afterPause.Status != "waiting_for_result" {
		t.Fatalf("pause treated pending work as silence: %+v", afterPause)
	}
	messages, err := chat.messageRepo.ListByConversation(ctx, "ws", "conv", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("duplicate acknowledgment: %d messages", len(messages))
	}
	inbox := &SupportInboxService{conversationRepo: chat.conversationRepo}
	if err := inbox.projectWidgetAIProgress(ctx, conv, messages); err != nil {
		t.Fatal(err)
	}
	var progress AIMessageMetadata
	if err := json.Unmarshal([]byte(messages[1].Metadata), &progress); err != nil {
		t.Fatal(err)
	}
	if progress.AIProgressState != "checking" {
		t.Fatalf("missing reconnect progress: %+v", progress)
	}

	// Completion can race with marking the parent notified. A grounded final
	// answer must still settle the open turn, with exactly one final publication.
	mustExec(t, db, `UPDATE command_bar_plans SET status='completed' WHERE id='plan'`)
	if !chat.waitingForSupportResult(ctx, run, conv.ID) {
		t.Fatal("undelivered completed result is still pending")
	}
	evidence := model.SupportRunEvidence{ID: "evidence", WorkspaceID: "ws", RunID: "run", EvidenceID: "child-result:plan", ReferenceID: "child-result:plan", SourceType: supportChildSourceOfficialWeb, Content: "The Pro plan costs $49 per month and includes 10 seats.", VectorScore: .9, CombinedScore: .9}
	if err := commands.supportRunEvidenceRepo.UpsertBatch(ctx, []model.SupportRunEvidence{evidence}); err != nil {
		t.Fatal(err)
	}
	final := json.RawMessage(`{"content":"The Pro plan costs $49 per month and includes 10 seats.","reply_kind":"answer","confidence":0.95,"source_doc_ids":["child-result:plan"],"claims":[{"text":"The Pro plan costs $49 per month and includes 10 seats.","evidence_ids":["child-result:plan"]}]}`)
	send(final, "sent")
	send(final, "suppressed")
	if err := chat.planRepo.MarkParentNotified(ctx, "ws", "plan"); err != nil {
		t.Fatal(err)
	}
	if chat.waitingForSupportResult(ctx, run, conv.ID) {
		t.Fatal("final answer left child wait active")
	}
	var settled model.AIMessageProcessing
	if err := db.First(&settled, "id = ?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if settled.Status != "completed" || settled.ReplyMessageID == nil {
		t.Fatalf("final answer did not settle: %+v", settled)
	}
	messages, err = chat.messageRepo.ListByConversation(ctx, "ws", "conv", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected customer, acknowledgment, and final answer; got %d messages", len(messages))
	}
	if err := inbox.projectWidgetAIProgress(ctx, conv, messages); err != nil {
		t.Fatal(err)
	}
	for _, message := range messages {
		var metadata AIMessageMetadata
		if err := json.Unmarshal([]byte(message.Metadata), &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata.AIProgressState != "" {
			t.Fatal("completed history resurrected loading")
		}
	}
}

func TestSupportProgressReplyPreservesPublicationFences(t *testing.T) {
	for _, tc := range []struct{ name, change string }{
		{"human takeover", `UPDATE support_conversations SET human_takeover=true WHERE id='conv'`},
		{"new run", `UPDATE support_conversations SET ai_active_run_id='new-run' WHERE id='conv'`},
		{"AI disabled", `UPDATE support_widget_installations SET settings='{"ai_enabled":false}' WHERE workspace_id='ws'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chat, _, db, _, _, _ := setupSupportProgressTest(t)
			ctx := context.Background()
			turn, err := chat.processingRepo.LatestProcessingForConversation(ctx, "ws", "conv")
			if err != nil {
				t.Fatal(err)
			}
			mustExec(t, db, tc.change)
			reply := &model.SupportMessage{WorkspaceID: "ws", ConversationID: "conv", SenderType: "ai", SenderAgentID: strPtr("agent"), Content: "Checking the details", MessageType: "reply", Metadata: `{"ai_progress_state":"checking"}`}
			if created, err := chat.processingRepo.CreateProgressReply(ctx, turn.ID, reply, "run"); err != nil || created {
				t.Fatalf("progress bypassed fence: created=%v err=%v", created, err)
			}
			if created, err := chat.processingRepo.CreateReply(ctx, turn.ID, reply, "run"); err != nil || created {
				t.Fatalf("final bypassed fence: created=%v err=%v", created, err)
			}
		})
	}
}
