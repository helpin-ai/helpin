package service

import (
	"context"
	"encoding/json"
	"strings"
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
	if _, claimed, _ := chat.processingRepo.BeginAttempt(ctx, "ws", "source", "conv"); claimed {
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
	evidence := model.SupportRunEvidence{ID: "evidence", WorkspaceID: "ws", RunID: "run", EvidenceID: "child-result:plan", ReferenceID: "child-result:plan", SourceType: supportChildSourceRepository, IsInternal: true, Content: "The Pro plan costs $49 per month and includes 10 seats.", VectorScore: .9, CombinedScore: .9}
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

func TestSupportProgressWithoutChildContinuesAndSettles(t *testing.T) {
	ctx := context.Background()
	chat, commands, db, _, _, meta := setupSupportProgressTest(t)
	mustExec(t, db, `DELETE FROM command_bar_plans`)
	commands.definitions = map[string]InternalCommandDefinition{}
	mustExec(t, db, `UPDATE agent_runs SET input=? WHERE id='run'`, []byte(`{"trigger":{"source":"system","trigger_type":"support_chat"}}`))
	commands.registerSupportReplyCommands()
	def, ok := commands.Definition("support.send_progress")
	if !ok {
		t.Fatal("missing non-terminal progress tool")
	}
	input := json.RawMessage(`{"content":"I'm checking the details for this issue."}`)
	for range 2 {
		out, err := def.Execute(ctx, meta, input)
		if err != nil || !strings.Contains(string(out), `"status":"continuing"`) {
			t.Fatalf("progress: %s %v", out, err)
		}
	}
	currentConv, err := chat.conversationRepo.GetByID(ctx, "ws", "conv", "", model.RoleOwner)
	if err != nil || currentConv.AITurnCount != 0 {
		t.Fatalf("acknowledgment consumed an answer turn: %+v %v", currentConv, err)
	}
	row, err := chat.processingRepo.LatestProcessingForConversation(ctx, "ws", "conv")
	if err != nil || row == nil || row.ReplyMessageID != nil {
		t.Fatalf("acknowledgment settled turn: %+v %v", row, err)
	}
	messages, err := chat.messageRepo.ListByConversation(ctx, "ws", "conv", false)
	if err != nil || len(messages) != 2 {
		t.Fatalf("expected one acknowledgment: %d %v", len(messages), err)
	}
	out, err := commands.executeSupportSendReply(ctx, meta, json.RawMessage(`{"content":"Which account is affected?","reply_kind":"clarify","confidence":1}`))
	if err != nil || !strings.Contains(string(out), `"status":"sent"`) {
		t.Fatalf("final reply: %s %v", out, err)
	}
	out, err = def.Execute(ctx, meta, input)
	if err != nil || !strings.Contains(string(out), `"status":"suppressed"`) {
		t.Fatalf("progress after outcome: %s %v", out, err)
	}
}

// useEmailSupportConversation turns the progress fixture into an email thread
// with AI replies enabled on both channels.
func useEmailSupportConversation(t *testing.T, db *gorm.DB) {
	t.Helper()
	settings := model.DefaultSupportInboxSettings()
	settings.AIEnabled = true
	settings.AIAgentID = strPtr("agent")
	settings.AIResponseMode = "ai_first"
	settings.AIReplyChannels = "both"
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE support_widget_installations SET settings=? WHERE workspace_id='ws'`, string(raw))
	mustExec(t, db, `UPDATE support_conversations SET channel='email', source='email' WHERE id='conv'`)
}

func TestSupportEmailAcknowledgmentsAreNotSent(t *testing.T) {
	ctx := context.Background()
	chat, commands, db, conv, run, meta := setupSupportProgressTest(t)
	useEmailSupportConversation(t, db)
	commands.definitions = map[string]InternalCommandDefinition{}
	mustExec(t, db, `UPDATE agent_runs SET input=? WHERE id='run'`, []byte(`{"trigger":{"source":"system","trigger_type":"support_chat"}}`))
	commands.registerSupportReplyCommands()
	progress, ok := commands.Definition("support.send_progress")
	if !ok {
		t.Fatal("missing progress tool")
	}
	for _, call := range []struct {
		name, want string
		run        func() (json.RawMessage, error)
	}{
		{"progress tool", `"status":"continuing"`, func() (json.RawMessage, error) {
			return progress.Execute(ctx, meta, json.RawMessage(`{"content":"I'm checking the details for this issue."}`))
		}},
		{"acknowledgment while child work runs", `"status":"awaiting_result"`, func() (json.RawMessage, error) {
			return commands.executeSupportSendReply(ctx, meta, json.RawMessage(`{"content":"I'm checking that for you.","reply_kind":"conversational","confidence":1}`))
		}},
	} {
		out, err := call.run()
		if err != nil || !strings.Contains(string(out), call.want) || !strings.Contains(string(out), `"sent":false`) {
			t.Fatalf("%s: %s %v", call.name, out, err)
		}
	}
	messages, err := chat.messageRepo.ListByConversation(ctx, "ws", "conv", false)
	if err != nil || len(messages) != 1 {
		t.Fatalf("email acknowledgment was published: %d messages, %v", len(messages), err)
	}
	// The child result is still awaited without an acknowledgment.
	if !chat.waitingForSupportResult(ctx, run, conv.ID) {
		t.Fatal("email turn stopped waiting for its child result")
	}
	out, err := commands.executeSupportSendReply(ctx, meta, json.RawMessage(`{"content":"Which account is affected?","reply_kind":"clarify","confidence":1}`))
	if err != nil || !strings.Contains(string(out), `"status":"sent"`) {
		t.Fatalf("final reply: %s %v", out, err)
	}
}

func TestSupportProgressToolPreservesOwnership(t *testing.T) {
	for _, change := range []string{
		`UPDATE support_conversations SET human_takeover=true WHERE id='conv'`,
		`UPDATE support_conversations SET ai_active_run_id='another-run' WHERE id='conv'`,
		`UPDATE support_widget_installations SET settings='{"ai_enabled":false}' WHERE workspace_id='ws'`,
	} {
		chat, commands, db, _, _, meta := setupSupportProgressTest(t)
		commands.definitions = map[string]InternalCommandDefinition{}
		mustExec(t, db, `UPDATE agent_runs SET input=? WHERE id='run'`, []byte(`{"trigger":{"source":"system","trigger_type":"support_chat"}}`))
		commands.registerSupportReplyCommands()
		def, ok := commands.Definition("support.send_progress")
		if !ok {
			t.Fatal("missing progress tool")
		}
		mustExec(t, db, change)
		out, err := def.Execute(context.Background(), meta, json.RawMessage(`{"content":"I'm checking the details."}`))
		if err != nil || !strings.Contains(string(out), `"status":"suppressed"`) {
			t.Fatalf("progress: %s %v", out, err)
		}
		messages, err := chat.messageRepo.ListByConversation(context.Background(), "ws", "conv", false)
		if err != nil || len(messages) != 1 {
			t.Fatalf("progress bypassed ownership: %d %v", len(messages), err)
		}
	}
}

func TestSupportProgressSweepRecoversUnfinishedAcknowledgment(t *testing.T) {
	for _, tc := range []struct {
		name, status   string
		child, blocked bool
		wantHandoff    bool
	}{
		{name: "failed investigation", status: "failed", wantHandoff: true},
		{name: "paused after retry", status: "paused", wantHandoff: true},
		{name: "pending child", status: "paused", child: true},
		{name: "human takeover", status: "failed", blocked: true},
		{name: "still working", status: "running"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			chat, commands, db, _, _, _ := setupSupportProgressTest(t)
			chat.runRepo = commands.agentRunRepo
			chat.supportAIService.handoffRepo = repository.NewAgentHandoffRepository(db)
			mustExec(t, db, `ALTER TABLE agent_runs ADD COLUMN updated_at DATETIME`)
			mustExec(t, db, `ALTER TABLE agent_runs ADD COLUMN created_at DATETIME`)
			mustExec(t, db, `ALTER TABLE agent_runs ADD COLUMN pause_reason TEXT`)
			mustExec(t, db, `UPDATE agent_runs SET status=?, pause_reason=?, input=?, updated_at=? WHERE id='run'`, tc.status, model.AgentRunPauseReasonUserMessage, []byte(`{"trigger":{"source":"system","trigger_type":"support_chat"}}`), time.Now().Add(-time.Minute))
			mustExec(t, db, `UPDATE ai_message_processing SET status='waiting_for_result',attempts=2,updated_at=?`, time.Now().Add(-time.Minute))
			if !tc.child {
				mustExec(t, db, `DELETE FROM command_bar_plans`)
			}
			if tc.blocked {
				mustExec(t, db, `UPDATE support_conversations SET human_takeover=true WHERE id='conv'`)
			}
			if err := chat.SweepSupportChat(ctx, time.Second, 10); err != nil {
				t.Fatal(err)
			}
			conv, err := chat.conversationRepo.GetByID(ctx, "ws", "conv", "", model.RoleOwner)
			if err != nil {
				t.Fatal(err)
			}
			got := conv.AIState != nil && *conv.AIState == "escalated"
			if got != tc.wantHandoff {
				t.Fatalf("handoff=%v want %v", got, tc.wantHandoff)
			}
		})
	}
}

func TestSupportProgressRejectsNonLiveRunsAndInvalidContent(t *testing.T) {
	for _, tc := range []struct {
		name, trigger, content string
		rewrite                bool
	}{
		{name: "follow-up", trigger: "support_inactivity_follow_up", content: "Checking this."},
		{name: "background", trigger: supportInvestigationTriggerType, content: "Checking this."},
		{name: "empty", trigger: supportChatTriggerType},
		{name: "long", trigger: supportChatTriggerType, content: strings.Repeat("x", 301)},
		{name: "internal process", trigger: supportChatTriggerType, content: "Let me search the knowledge base.", rewrite: true},
		{name: "placeholder", trigger: supportChatTriggerType, content: "placeholder", rewrite: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chat, commands, db, _, _, meta := setupSupportProgressTest(t)
			raw, _ := json.Marshal(map[string]any{"trigger": map[string]string{"source": "system", "trigger_type": tc.trigger}})
			mustExec(t, db, `UPDATE agent_runs SET input=? WHERE id='run'`, raw)
			input, _ := json.Marshal(map[string]string{"content": tc.content})
			out, err := commands.executeSupportProgress(context.Background(), meta, input)
			if tc.rewrite {
				if err != nil || !strings.Contains(string(out), "rewrite_required") {
					t.Fatalf("expected rewrite: %s %v", out, err)
				}
			} else if err == nil {
				t.Fatalf("expected rejection: %s", out)
			}
			messages, err := chat.messageRepo.ListByConversation(context.Background(), "ws", "conv", false)
			if err != nil || len(messages) != 1 {
				t.Fatalf("invalid progress published: %d %v", len(messages), err)
			}
		})
	}
}

func TestSupportSendReplyRejectsPlaceholderText(t *testing.T) {
	ctx := context.Background()
	chat, commands, _, _, _, meta := setupSupportProgressTest(t)
	input := json.RawMessage(`{"content":"placeholder","reply_kind":"conversational","confidence":0.415}`)
	out, err := commands.executeSupportSendReply(ctx, meta, input)
	if err != nil || !strings.Contains(string(out), `"reason":"placeholder_text"`) {
		t.Fatalf("expected placeholder rewrite: %s %v", out, err)
	}
	messages, err := chat.messageRepo.ListByConversation(ctx, "ws", "conv", false)
	if err != nil || len(messages) != 1 {
		t.Fatalf("placeholder reply published: %d %v", len(messages), err)
	}
}
