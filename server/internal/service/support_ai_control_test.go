package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"github.com/nats-io/nats.go"
)

type failingControlRunCloser struct{ calls []string }

func (f *failingControlRunCloser) CancelRun(_ context.Context, _, id, _ string) (*model.AgentRun, error) {
	f.calls = append(f.calls, id)
	return nil, errors.New("runtime unavailable")
}

func setupAIControlTest(t *testing.T) (*SupportInboxService, *gorm.DB, *model.SupportConversation, model.SupportInboxSettings) {
	t.Helper()
	db := newTestDB(t)
	repo := repository.NewSupportConversationRepository(db)
	conv := &model.SupportConversation{ID: "conv", WorkspaceID: "ws", Subject: "Cannot connect", Status: "open", Source: "widget", Channel: "widget", AIState: strPtr("pending"), AssignedAgentID: strPtr("agent"), AIActiveRunID: strPtr("old-run")}
	if err := repo.Create(context.Background(), conv); err != nil {
		t.Fatal(err)
	}
	svc := &SupportInboxService{conversationRepo: repo, messageRepo: repository.NewSupportMessageRepository(db)}
	settings := model.DefaultSupportInboxSettings()
	settings.AIEnabled = true
	settings.AIResponseMode = "ai_first"
	settings.AIAgentID = strPtr("agent")
	return svc, db, conv, settings
}

func readControlConversation(t *testing.T, db *gorm.DB) *model.SupportConversation {
	t.Helper()
	var conv model.SupportConversation
	if err := db.First(&conv, "id = ?", "conv").Error; err != nil {
		t.Fatal(err)
	}
	return &conv
}

func TestSupportAIControlPauseAndReturnFence(t *testing.T) {
	ctx := context.Background()
	svc, db, conv, settings := setupAIControlTest(t)
	closer := &failingControlRunCloser{}
	svc.supportAIService = &SupportAIService{runCloser: closer}
	before := time.Now().UTC().Add(-time.Minute)
	msg := &model.SupportMessage{ID: "old-customer", WorkspaceID: "ws", ConversationID: "conv", SenderType: "customer", MessageType: "reply", Content: "I tried reconnecting but it still fails", CreatedAt: before}
	if err := svc.messageRepo.Create(ctx, msg); err != nil {
		t.Fatal(err)
	}
	req := model.SupportAIControlRequest{Action: "pause", ExpectedVersion: 0}
	if err := svc.changeConversationAIControl(ctx, conv, "teammate", &req, settings, nil, "paused_by_teammate"); err != nil {
		t.Fatal(err)
	}
	current := readControlConversation(t, db)
	if !model.SupportAIConversationBlocked(current) || current.AIActiveRunID != nil || current.AIControlVersion != 1 || derefString(current.AIPausedByUserID) != "teammate" {
		t.Fatalf("pause did not persist: %+v", current)
	}
	if len(closer.calls) != 1 || closer.calls[0] != "old-run" {
		t.Fatalf("cancellation not attempted: %v", closer.calls)
	}
	if err := svc.changeConversationAIControl(ctx, conv, "other", &req, settings, nil, "paused_by_teammate"); !errors.Is(err, repository.ErrSupportAIControlConflict) {
		t.Fatalf("stale control: %v", err)
	}
	req = model.SupportAIControlRequest{Action: "return", ExpectedVersion: 1}
	if err := svc.changeConversationAIControl(ctx, current, "teammate", &req, settings, nil, ""); err != nil {
		t.Fatal(err)
	}
	resumed := readControlConversation(t, db)
	if model.SupportAIConversationBlocked(resumed) || resumed.AIResumedAt == nil || resumed.AIControlVersion != 2 || resumed.AIPausedAt != nil {
		t.Fatalf("return did not persist: %+v", resumed)
	}
	if model.SupportAIReplyAllowed(settings, resumed, msg) {
		t.Fatal("queued pre-return message became eligible")
	}
	fresh := *msg
	fresh.CreatedAt = resumed.AIResumedAt.Add(time.Second)
	if !model.SupportAIReplyAllowed(settings, resumed, &fresh) {
		t.Fatal("new customer message rejected")
	}
	if bound, err := svc.conversationRepo.BindAIRun(ctx, conv, "stale-launch"); err != nil || bound {
		t.Fatalf("old launch bound: %v/%v", bound, err)
	}
	if bound, err := svc.conversationRepo.BindAIRun(ctx, resumed, "new-run"); err != nil || !bound {
		t.Fatalf("new launch: %v/%v", bound, err)
	}
	var notes []model.SupportMessage
	if err := db.Where("is_internal = true").Find(&notes).Error; err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 || notes[0].MessageType != "system" || derefString(notes[0].SystemEventType) != "ai_paused" || derefString(notes[1].SystemEventType) != "ai_returned" {
		t.Fatalf("notes = %+v", notes)
	}
	public, err := svc.messageRepo.ListByConversation(ctx, "ws", "conv", false)
	if err != nil || len(public) != 1 {
		t.Fatalf("control leaked into public history: %v/%v", public, err)
	}
}

func TestSupportAIControlReturnConfirmsHumanRequestAndClearsOwners(t *testing.T) {
	svc, db, conv, settings := setupAIControlTest(t)
	ctx := context.Background()
	mustExec(t, db, `UPDATE support_conversations SET human_takeover=true,assigned_user_id='human',opened_by_user_id='human',ai_state='escalated',customer_requested_human_at=CURRENT_TIMESTAMP`)
	req := model.SupportAIControlRequest{Action: "return"}
	if err := svc.changeConversationAIControl(ctx, conv, "human", &req, settings, nil, ""); err == nil {
		t.Fatal("human request overridden without confirmation")
	}
	req.ConfirmHumanRequest = true
	if err := svc.changeConversationAIControl(ctx, conv, "human", &req, settings, nil, ""); err != nil {
		t.Fatal(err)
	}
	current := readControlConversation(t, db)
	if current.AssignedUserID != nil || current.OpenedByUserID != nil || current.CustomerRequestedHumanAt != nil || derefString(current.AssignedAgentID) != "agent" {
		t.Fatalf("inconsistent ownership: %+v", current)
	}
	var notes []model.SupportMessage
	db.Find(&notes)
	if len(notes) != 1 || !strings.Contains(notes[0].Content, "customer requested a human") {
		t.Fatal("human request history lost")
	}
}

func TestSupportAIControlIneligibleReturn(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		disabled  bool
	}{
		{"resolved", `UPDATE support_conversations SET status='resolved'`, false},
		{"spam", `UPDATE support_conversations SET status='spam'`, false},
		{"deleted", `UPDATE support_conversations SET anonymized_at=CURRENT_TIMESTAMP`, false},
		{"email disabled", `UPDATE support_conversations SET channel='email',source='email'`, false},
		{"workspace disabled", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, conv, settings := setupAIControlTest(t)
			mustExec(t, db, `UPDATE support_conversations SET human_takeover=true`)
			if tc.sql != "" {
				mustExec(t, db, tc.sql)
			}
			if tc.disabled {
				settings.AIEnabled = false
			}
			req := model.SupportAIControlRequest{Action: "return"}
			if err := svc.changeConversationAIControl(context.Background(), conv, "human", &req, settings, nil, ""); err == nil {
				t.Fatal("ineligible conversation returned")
			}
			if readControlConversation(t, db).AIControlVersion != 0 {
				t.Fatal("failed action mutated ownership")
			}
		})
	}
}

func TestSupportAIControlConcurrentTeammates(t *testing.T) {
	svc, db, conv, settings := setupAIControlTest(t)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	var wg sync.WaitGroup
	results := make([]error, 2)
	for index, actor := range []string{"one", "two"} {
		wg.Add(1)
		go func(index int, actor string) {
			defer wg.Done()
			req := model.SupportAIControlRequest{Action: "pause"}
			results[index] = svc.changeConversationAIControl(context.Background(), conv, actor, &req, settings, nil, "paused_by_teammate")
		}(index, actor)
	}
	wg.Wait()
	success, conflict := 0, 0
	for _, err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, repository.ErrSupportAIControlConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success/conflict = %d/%d", success, conflict)
	}
	var count int64
	db.Model(&model.SupportMessage{}).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate handoff notes: %d", count)
	}
}

func TestSupportAIControlRejectsLateReplyAfterReturn(t *testing.T) {
	svc, db, conv, settings := setupAIControlTest(t)
	ctx := context.Background()
	mustExec(t, db, `CREATE TABLE ai_message_processing(id TEXT PRIMARY KEY,workspace_id TEXT,conversation_id TEXT,source_message_id TEXT,reply_message_id TEXT,status TEXT,tokens_used INTEGER,created_at DATETIME,updated_at DATETIME)`)
	req := model.SupportAIControlRequest{Action: "pause"}
	if err := svc.changeConversationAIControl(ctx, conv, "human", &req, settings, nil, "pause"); err != nil {
		t.Fatal(err)
	}
	req = model.SupportAIControlRequest{Action: "return", ExpectedVersion: 1}
	if err := svc.changeConversationAIControl(ctx, conv, "human", &req, settings, nil, ""); err != nil {
		t.Fatal(err)
	}
	current := readControlConversation(t, db)
	if ok, err := svc.conversationRepo.BindAIRun(ctx, current, "new-run"); err != nil || !ok {
		t.Fatalf("bind: %v/%v", ok, err)
	}
	msg := &model.SupportMessage{ID: "fresh", WorkspaceID: "ws", ConversationID: "conv", SenderType: "customer", MessageType: "reply", Content: "New question", CreatedAt: current.AIResumedAt.Add(time.Second)}
	if err := svc.messageRepo.Create(ctx, msg); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO ai_message_processing(id,workspace_id,conversation_id,source_message_id,status) VALUES ('turn','ws','conv','fresh','processing')`)
	reply := &model.SupportMessage{ID: "late", WorkspaceID: "ws", ConversationID: "conv", SenderType: "ai", MessageType: "reply", Content: "Old answer"}
	repo := repository.NewAIMessageProcessingRepository(db)
	if created, err := repo.CreateReply(ctx, "turn", reply, "old-run"); err != nil || created {
		t.Fatalf("late publication: %v/%v", created, err)
	}
	if created, err := repo.CreateReply(ctx, "turn", reply, "new-run"); err != nil || !created {
		t.Fatalf("new publication: %v/%v", created, err)
	}
}

func TestSupportAIControlStaleSaveCannotUndoPause(t *testing.T) {
	svc, db, conv, settings := setupAIControlTest(t)
	req := model.SupportAIControlRequest{Action: "pause"}
	if err := svc.changeConversationAIControl(context.Background(), conv, "human", &req, settings, nil, "pause"); err != nil {
		t.Fatal(err)
	}
	conv.Subject = "Unrelated change from an old snapshot"
	if err := svc.conversationRepo.Update(context.Background(), conv); !errors.Is(err, repository.ErrSupportAIControlConflict) {
		t.Fatalf("stale save: %v", err)
	}
	if !model.SupportAIConversationBlocked(readControlConversation(t, db)) {
		t.Fatal("stale snapshot undid pause")
	}
	if err := resolveSupportAIConversation(context.Background(), db, conv, "confirmed", time.Now()); err != nil {
		t.Fatal(err)
	}
	if readControlConversation(t, db).Status != "open" {
		t.Fatal("late AI finalizer resolved a human-owned conversation")
	}

}

func TestSupportAIControlDropsDeferredMessagesBeforeReturn(t *testing.T) {
	svc, db, conv, settings := setupAIControlTest(t)
	ctx := context.Background()
	raw, _ := json.Marshal(settings)
	mustExec(t, db, `INSERT INTO support_widget_installations(id,workspace_id,widget_key,secret_key,settings,active) VALUES ('install','ws','key','secret',?,true)`, string(raw))
	mustExec(t, db, `CREATE TABLE ai_message_processing(id TEXT PRIMARY KEY,workspace_id TEXT,conversation_id TEXT,source_message_id TEXT,reply_message_id TEXT,status TEXT,tokens_used INTEGER,updated_at DATETIME)`)
	before := time.Now().UTC().Add(-time.Minute)
	for _, id := range []string{"old", "fresh"} {
		if err := svc.messageRepo.Create(ctx, &model.SupportMessage{ID: id, WorkspaceID: "ws", ConversationID: "conv", SenderType: "customer", MessageType: "reply", Content: id, CreatedAt: before}); err != nil {
			t.Fatal(err)
		}
	}
	req := model.SupportAIControlRequest{Action: "pause"}
	if err := svc.changeConversationAIControl(ctx, conv, "human", &req, settings, nil, "pause"); err != nil {
		t.Fatal(err)
	}
	req = model.SupportAIControlRequest{Action: "return", ExpectedVersion: 1}
	if err := svc.changeConversationAIControl(ctx, conv, "human", &req, settings, nil, ""); err != nil {
		t.Fatal(err)
	}
	current := readControlConversation(t, db)
	mustExec(t, db, `UPDATE support_messages SET created_at=? WHERE id='fresh'`, current.AIResumedAt.Add(time.Second))
	var rows []model.AIMessageProcessing
	for _, id := range []string{"old", "fresh"} {
		mustExec(t, db, `INSERT INTO ai_message_processing(id,workspace_id,conversation_id,source_message_id,status) VALUES (?,'ws','conv',?,'deferred')`, id, id)
		rows = append(rows, model.AIMessageProcessing{ID: id, WorkspaceID: "ws", ConversationID: "conv", SourceMessageID: id})
	}
	chat := &SupportChatService{conversationRepo: svc.conversationRepo, messageRepo: svc.messageRepo, processingRepo: repository.NewAIMessageProcessingRepository(db), supportAIService: &SupportAIService{installationRepo: repository.NewSupportInboxInstallationRepository(db)}}
	eligible := chat.filterChatDeferredMessages(ctx, rows)
	if len(eligible) != 1 || eligible[0].ID != "fresh" {
		t.Fatalf("eligible = %+v", eligible)
	}
	var old model.AIMessageProcessing
	db.First(&old, "id = ?", "old")
	if old.Status != "completed" {
		t.Fatalf("old deferred row not drained: %s", old.Status)
	}
}

func TestSupportHandoffBriefAttributionAndPrivacy(t *testing.T) {
	conv := &model.SupportConversation{ID: "conv", WorkspaceID: "ws", Subject: "Reconnect error"}
	history := []model.SupportMessage{
		{ID: "customer", SenderType: "customer", MessageType: "reply", Content: "The connection still fails"},
		{ID: "ai", SenderType: "ai", MessageType: "reply", Content: "Try reconnecting"},
		{ID: "private", SenderType: "user", MessageType: "reply", IsInternal: true, Content: "PRIVATE teammate note"},
	}
	note := buildSupportHandoffNote(conv, history, "provider_unavailable", SupportHandoffBrief{}, time.Now())
	if !note.IsInternal || strings.Contains(note.Content, "PRIVATE") || !strings.Contains(note.Content, "AI said: Try reconnecting") || !strings.Contains(note.Content, "Customer said: The connection still fails") {
		t.Fatalf("bad fallback: %s", note.Content)
	}
	if strings.Count(note.Content, "The connection still fails") != 1 {
		t.Fatal("fallback repeats the customer issue")
	}
	rich := buildSupportHandoffNote(conv, history, "cannot_answer", SupportHandoffBrief{Issue: "Connection fails", AttemptedSteps: []string{"AI suggested reconnecting; customer has not confirmed trying it."}, UnresolvedQuestions: []string{"Which error appears?"}}, time.Now())
	if !strings.Contains(rich.Content, "not confirmed") || !strings.Contains(rich.Content, "Which error appears?") {
		t.Fatal(rich.Content)
	}
	if len([]rune(briefText(strings.Repeat("x", 1000)))) > 701 {
		t.Fatal("unbounded summary")
	}
}

func TestSupportHandoffRetryCreatesOneBrief(t *testing.T) {
	svc, db, conv, settings := setupAIControlTest(t)
	ctx := context.Background()
	raw, _ := json.Marshal(settings)
	mustExec(t, db, `INSERT INTO support_widget_installations(id,workspace_id,widget_key,secret_key,settings,active) VALUES ('install','ws','key','secret',?,true)`, string(raw))
	ai := &SupportAIService{conversationRepo: svc.conversationRepo, messageRepo: svc.messageRepo, installationRepo: repository.NewSupportInboxInstallationRepository(db), handoffRepo: repository.NewAgentHandoffRepository(db)}
	for range 2 {
		if err := ai.EscalateToHumanForMessageWithIssue(ctx, "ws", conv.ID, "", "provider_unavailable", "", "Connection failed", SupportHandoffBrief{AttemptedSteps: []string{"No answer could be generated"}, UnresolvedQuestions: []string{"Customer still needs an answer"}}); err != nil {
			t.Fatal(err)
		}
	}
	var notes []model.SupportMessage
	if err := db.Where("is_internal = true AND message_type = 'note'").Find(&notes).Error; err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || !strings.Contains(notes[0].Content, "Customer still needs an answer") {
		t.Fatalf("notes = %+v", notes)
	}
}

func TestSupportAIControlAssignsConfiguredAIOnUnownedConversation(t *testing.T) {
	svc, db, conv, settings := setupAIControlTest(t)
	mustExec(t, db, `UPDATE support_conversations SET assigned_agent_id=NULL,ai_state=NULL,ai_active_run_id=NULL`)
	req := model.SupportAIControlRequest{Action: "return"}
	if err := svc.changeConversationAIControl(context.Background(), conv, "human", &req, settings, nil, ""); err != nil {
		t.Fatal(err)
	}
	current := readControlConversation(t, db)
	if derefString(current.AssignedAgentID) != "agent" || derefString(current.AIState) != "pending" {
		t.Fatal("existing assign-agent path did not select the configured AI")
	}
}

func TestSupportAIControlTogglePreservesExistingBrief(t *testing.T) {
	svc, db, conv, settings := setupAIControlTest(t)
	ctx := context.Background()
	seedUser(t, db, "teammate", "teammate@example.test", "Arooj Bukhari", "unused")
	svc.userRepo = repository.NewUserRepository(db)
	brief := buildSupportHandoffNote(conv, nil, "cannot_answer", SupportHandoffBrief{Issue: "Workspace switching fails", AttemptedSteps: []string{"Checked related engineering tickets"}}, time.Now())
	if err := svc.messageRepo.Create(ctx, brief); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"pause", "return", "pause"} {
		current := readControlConversation(t, db)
		req := model.SupportAIControlRequest{Action: action, ExpectedVersion: current.AIControlVersion}
		if err := svc.changeConversationAIControl(ctx, current, "teammate", &req, settings, nil, "paused_by_teammate"); err != nil {
			t.Fatal(err)
		}
	}
	var notes []model.SupportMessage
	if err := db.Where("message_type = 'note'").Find(&notes).Error; err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].ID != brief.ID || notes[0].Content != brief.Content {
		t.Fatalf("original brief must be preserved without duplicate summaries: %+v", notes)
	}
	var events []model.SupportMessage
	if err := db.Where("message_type = 'system'").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("expected three activity rows, got %d", len(events))
	}
	for _, event := range events {
		if !event.IsInternal || derefString(event.SenderUserID) != "teammate" || derefString(event.SenderDisplayName) != "Arooj Bukhari" || event.WidgetVisible() {
			t.Fatalf("incorrect activity attribution: %+v", event)
		}
	}
}

type handoffEventRecorder struct {
	nats.JetStreamContext
	events []websocket.Event
}

func (r *handoffEventRecorder) Publish(_ string, data []byte, _ ...nats.PubOpt) (*nats.PubAck, error) {
	var event websocket.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	r.events = append(r.events, event)
	return &nats.PubAck{}, nil
}

func TestSupportHandoffEventPrecedesBriefInHistoryAndBroadcast(t *testing.T) {
	for _, reason := range []string{"customer_requested", "provider_unavailable"} {
		t.Run(reason, func(t *testing.T) {
			svc, db, conv, settings := setupAIControlTest(t)
			raw, _ := json.Marshal(settings)
			mustExec(t, db, `INSERT INTO support_widget_installations(id,workspace_id,widget_key,secret_key,settings,active) VALUES ('install','ws','key','secret',?,true)`, string(raw))
			recorder := &handoffEventRecorder{}
			ai := &SupportAIService{conversationRepo: svc.conversationRepo, messageRepo: svc.messageRepo, installationRepo: repository.NewSupportInboxInstallationRepository(db), handoffRepo: repository.NewAgentHandoffRepository(db), wsPublisher: websocket.NewOrderedJetStreamPublisher(recorder)}
			if err := ai.EscalateToHuman(context.Background(), "ws", conv.ID, reason); err != nil {
				t.Fatal(err)
			}
			rows, err := svc.messageRepo.ListByConversation(context.Background(), "ws", conv.ID, true)
			if err != nil {
				t.Fatal(err)
			}
			eventIndex, noteIndex := -1, -1
			for i, row := range rows {
				if row.SystemEventType != nil && *row.SystemEventType == systemEventForEscalationReason(reason) {
					eventIndex = i
				}
				if strings.Contains(row.Metadata, `"ai_handoff_brief":true`) {
					noteIndex = i
				}
			}
			if eventIndex < 0 || noteIndex != eventIndex+1 {
				t.Fatalf("handoff event index %d, brief index %d", eventIndex, noteIndex)
			}
			if rows[noteIndex].CreatedAt.UnixMilli() <= rows[eventIndex].CreatedAt.UnixMilli() {
				t.Fatal("handoff and brief must sort correctly at browser timestamp precision")
			}
			ids := []string{}
			for _, event := range recorder.events {
				if event.Entity == "support_conversation_message" {
					ids = append(ids, event.EntityID)
				}
			}
			if len(ids) != len(rows) {
				t.Fatalf("broadcasts %v do not match %d saved messages", ids, len(rows))
			}
			for i := range rows {
				if ids[i] != rows[i].ID {
					t.Fatalf("broadcast order differs at %d", i)
				}
			}
		})
	}
}
