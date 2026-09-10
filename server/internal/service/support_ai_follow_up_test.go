package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func setupFollowUpTest(t *testing.T) (*SupportFollowUpService, *gorm.DB, *model.AgentRun, model.SupportAIFollowUp, supportFollowUpDecision) {
	t.Helper()
	db := newTestDB(t)
	if err := db.AutoMigrate(&model.SupportAIFollowUp{}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `CREATE TABLE agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT, target_type TEXT, target_id TEXT, status TEXT, pause_reason TEXT, created_at DATETIME)`)
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	settings := model.DefaultSupportInboxSettings()
	settings.AIEnabled = true
	settings.AIResponseMode = "ai_first"
	settings.AIAgentID = strPtr("agent")
	settings.AIFollowUpEnabled = true
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO support_widget_installations(id,workspace_id,widget_key,secret_key,settings,active) VALUES ('inst','ws','key','secret',?,true)`, string(raw))
	mustExec(t, db, `INSERT INTO support_conversations(id,workspace_id,display_id,subject,status,flow_state,ai_state,channel,assigned_agent_id,last_public_message_id,last_public_sender_type,last_public_message_at,created_at,updated_at) VALUES ('conv','ws',1,'Gmail','open','ai_handling','pending','widget','agent','source','ai',?,?,?)`, now.Add(-24*time.Hour), now.Add(-48*time.Hour), now)
	mustExec(t, db, `INSERT INTO support_messages(id,workspace_id,conversation_id,sender_type,message_type,content,is_internal,created_at) VALUES ('source','ws','conv','ai','reply','Reconnect Gmail using the steps above',false,?)`, now.Add(-24*time.Hour))
	episode := model.SupportAIFollowUp{ID: "episode", WorkspaceID: "ws", ConversationID: "conv", SourceMessageID: "source", RunID: "run", Status: "assessing", DueAt: now, CloseHours: 48, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&episode).Error; err != nil {
		t.Fatal(err)
	}
	chat := &SupportChatService{messageRepo: repository.NewSupportMessageRepository(db), supportAIService: &SupportAIService{}}
	svc := NewSupportFollowUpService(repository.NewSupportFollowUpRepository(db), chat)
	svc.now = func() time.Time { return now }
	run := &model.AgentRun{Status: "running", ID: "run", WorkspaceID: "ws", TargetID: "conv", AgentID: "agent"}
	decision := supportFollowUpDecision{Action: "follow_up", Reason: "Waiting for confirmation", Question: "Were you able to reconnect Gmail?", ClosureNotice: "Without a reply, this conversation closes in 48 hours. Reply anytime to reopen it.", ObligationsClear: true, SourceMessageIDs: []string{"source"}}
	return svc, db, run, episode, decision
}

func TestSupportFollowUpPublishesOnce(t *testing.T) {
	svc, db, run, e, d := setupFollowUpTest(t)
	for range 2 {
		status, err := svc.Complete(context.Background(), run, e.ID, d)
		if err != nil || status != "waiting" {
			t.Fatalf("Complete = %q, %v", status, err)
		}
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("id != 'source'").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("sent %d follow-ups", count)
	}
	latest, err := svc.repo.Latest(context.Background(), "ws", "conv")
	if err != nil {
		t.Fatal(err)
	}
	if latest.SentAt == nil || latest.CloseAt == nil || latest.CloseAt.Sub(*latest.SentAt) != 48*time.Hour {
		t.Fatalf("wrong deadline: %+v", latest)
	}
}

func TestSupportFollowUpSuppressesStaleAssessment(t *testing.T) {
	cases := []struct{ name, sql string }{
		{"customer replied", `UPDATE support_conversations SET last_public_message_id='customer',last_public_sender_type='customer'`},
		{"human takeover", `UPDATE support_conversations SET human_takeover=true`},
		{"assigned teammate", `UPDATE support_conversations SET assigned_user_id='user'`},
		{"outstanding task", `UPDATE support_conversations SET linked_task_id='task'`},
		{"already resolved", `UPDATE support_conversations SET status='resolved'`},
		{"policy disabled", `UPDATE support_widget_installations SET settings='{}'`},
		{"installation disabled", `UPDATE support_widget_installations SET active=false`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, run, e, d := setupFollowUpTest(t)
			mustExec(t, db, tc.sql)
			if _, err := svc.Complete(context.Background(), run, e.ID, d); err != nil {
				t.Fatal(err)
			}
			var count int64
			db.Model(&model.SupportMessage{}).Where("id != 'source'").Count(&count)
			if count != 0 {
				t.Fatal("stale assessment sent a follow-up")
			}
			latest, err := svc.repo.Latest(context.Background(), "ws", "conv")
			if err != nil {
				t.Fatal(err)
			}
			if latest.Status != "cancelled" {
				t.Fatalf("status = %s", latest.Status)
			}
		})
	}
}

func TestSupportFollowUpRejectsUnsafeDecision(t *testing.T) {
	for _, change := range []struct {
		name  string
		apply func(*supportFollowUpDecision)
	}{
		{"outstanding obligation", func(d *supportFollowUpDecision) { d.ObligationsClear = false }},
		{"no closure notice", func(d *supportFollowUpDecision) { d.ClosureNotice = "" }},
		{"unrelated message", func(d *supportFollowUpDecision) { d.SourceMessageIDs = []string{"foreign"} }},
		{"internal disclosure", func(d *supportFollowUpDecision) { d.Question = "I searched the knowledge base. Did it work?" }},
	} {
		t.Run(change.name, func(t *testing.T) {
			svc, _, run, e, d := setupFollowUpTest(t)
			change.apply(&d)
			if _, err := svc.Complete(context.Background(), run, e.ID, d); err == nil {
				t.Fatal("unsafe decision accepted")
			}
		})
	}
}

func TestSupportFollowUpResolvesOnlyAfterDeadline(t *testing.T) {
	svc, db, run, e, d := setupFollowUpTest(t)
	if _, err := svc.Complete(context.Background(), run, e.ID, d); err != nil {
		t.Fatal(err)
	}
	latest, err := svc.repo.Latest(context.Background(), "ws", "conv")
	if err != nil {
		t.Fatal(err)
	}
	// SQLite fixture does not install PostgreSQL's message projection triggers.
	mustExec(t, db, `UPDATE support_conversations SET last_public_message_id=?`, *latest.SentMessageID)
	if err := svc.process(context.Background(), *latest, latest.CloseAt.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	var conv model.SupportConversation
	if err := db.First(&conv, "id = ?", "conv").Error; err != nil {
		t.Fatal(err)
	}
	if conv.Status != "open" {
		t.Fatal("resolved before deadline")
	}
	if err := svc.process(context.Background(), *latest, *latest.CloseAt); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&conv, "id = ?", "conv").Error; err != nil {
		t.Fatal(err)
	}
	if conv.Status != "resolved" || derefString(conv.FlowState) != "resolved_by_ai" || derefString(conv.AIResolutionType) != "assumed" || conv.AIResolvedAt == nil || conv.ResolvedAt == nil {
		t.Fatalf("incomplete resolution: %+v", conv)
	}
}

func TestSupportFollowUpReplyCancelsClosure(t *testing.T) {
	svc, db, run, e, d := setupFollowUpTest(t)
	if _, err := svc.Complete(context.Background(), run, e.ID, d); err != nil {
		t.Fatal(err)
	}
	latest, err := svc.repo.Latest(context.Background(), "ws", "conv")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE support_conversations SET last_public_message_id='reply',last_public_sender_type='customer'`)
	if err := svc.process(context.Background(), *latest, *latest.CloseAt); err != nil {
		t.Fatal(err)
	}
	var conv model.SupportConversation
	db.First(&conv, "id = ?", "conv")
	if conv.Status != "open" {
		t.Fatal("closed despite customer reply")
	}
}

func TestSupportFollowUpCrossTenantCallbackRejected(t *testing.T) {
	svc, _, run, e, d := setupFollowUpTest(t)
	run.WorkspaceID = "other"
	if _, err := svc.Complete(context.Background(), run, e.ID, d); err == nil {
		t.Fatal("cross-tenant callback accepted")
	}
}

func TestSupportFollowUpDefaultsEnabledAndExplicitOptOut(t *testing.T) {
	settings := parseSettings(`{"ai_enabled":true,"ai_response_mode":"ai_first","ai_agent_id":"agent"}`)
	if !supportFollowUpEnabled(settings) {
		t.Fatal("new default should enable eligible installations")
	}
	if settings.AIFollowUpDelayHours != 24 || settings.AIFollowUpSecondDelayHours != 24 || settings.AIFollowUpCloseHours != 1 {
		t.Fatal("wrong default timings")
	}
	settings.AIFollowUpEnabled = false
	if supportFollowUpEnabled(settings) {
		t.Fatal("explicit opt out ignored")
	}
}

func TestSupportFollowUpLaunchUsesRestrictedAgentRuntime(t *testing.T) {
	db := setupAgentRuntimeSupportRunTestDB(t)
	now := time.Now().UTC()
	seedAgentRuntimeSupportAgent(t, db, model.InvocationModeAutonomous, now)
	seedAgentRuntimeSupportConversation(t, db, now)
	tools := `["get_support_conversation","list_conversation_messages","finish_support_follow_up","send_support_reply"]`
	mustExec(t, db, `UPDATE agents SET allowed_tools=? WHERE id='agent-1'`, []byte(tools))
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	agentService := newDelegatedSupportRunService(t, db, runtimeClient)
	chat := &SupportChatService{agentService: agentService, runRepo: repository.NewAgentRunRepository(db), supportAIService: &SupportAIService{}}
	svc := NewSupportFollowUpService(repository.NewSupportFollowUpRepository(db), chat)
	settings := model.DefaultSupportInboxSettings()
	settings.AIAgentID = strPtr("agent-1")
	episode := model.SupportAIFollowUp{ID: "episode", WorkspaceID: "ws-1", ConversationID: "conv-1", SourceMessageID: "source", RunID: "reserved-run", CloseHours: 48}
	conv := &model.SupportConversation{ID: "conv-1", WorkspaceID: "ws-1"}
	if err := svc.launch(context.Background(), episode, conv, settings, now); err != nil {
		t.Fatal(err)
	}
	start := runtimeClient.startRunCalls[0]
	if start.HostRunID != "reserved-run" || start.Target.ID != "conv-1" || start.Trigger["trigger_type"] != supportFollowUpTriggerType {
		t.Fatalf("wrong launch contract: %+v", start)
	}
	if start.TurnPolicy.Mode != "complete_on_finish" {
		t.Fatalf("assessment remains in chat mode: %+v", start.TurnPolicy)
	}
	if len(start.AllowedTools) != 3 {
		t.Fatalf("unexpected tool scope: %+v", start.AllowedTools)
	}
	for _, tool := range start.AllowedTools {
		if tool == "send_support_reply" || tool == "escalate_to_human" {
			t.Fatal("assessment can bypass outcome gate")
		}
	}
	var agent model.Agent
	if err := db.First(&agent, "id = ?", "agent-1").Error; err != nil {
		t.Fatal(err)
	}
	if string(agent.AllowedTools) != tools {
		t.Fatal("launch changed saved support tools")
	}
}

func TestSupportFollowUpManualCancellation(t *testing.T) {
	svc, db, run, e, d := setupFollowUpTest(t)
	if err := svc.repo.CancelConversation(context.Background(), "ws", "conv"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Complete(context.Background(), run, e.ID, d); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("id != 'source'").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("cancelled assessment published")
	}
}

func TestSupportFollowUpDoesNotHideLiveChat(t *testing.T) {
	db := setupAgentRuntimeSupportRunTestDB(t)
	now := time.Now().UTC()
	seedAgentRuntimeSupportAgent(t, db, model.InvocationModeAutonomous, now)
	seedAgentRuntimeSupportConversation(t, db, now)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := newDelegatedSupportRunService(t, db, runtimeClient)
	live := model.AgentRun{ID: "live-run", WorkspaceID: "ws-1", AgentID: "agent-1", TargetType: "support_conversation", TargetID: "conv-1", Status: "paused", PauseReason: model.AgentRunPauseReasonUserMessage, Input: json.RawMessage(`{"trigger":{"trigger_type":"support_chat"}}`), CreatedAt: now.Add(-time.Minute), UpdatedAt: now}
	assessment := model.AgentRun{ID: "assessment-run", WorkspaceID: "ws-1", AgentID: "agent-1", TargetType: "support_conversation", TargetID: "conv-1", Status: "running", Input: json.RawMessage(`{"trigger":{"trigger_type":"support_inactivity_follow_up"}}`), CreatedAt: now, UpdatedAt: now}
	for _, run := range []*model.AgentRun{&live, &assessment} {
		if err := svc.runRepo.Create(context.Background(), run); err != nil {
			t.Fatal(err)
		}
	}
	agent, err := svc.agentRepo.GetByID(context.Background(), "ws-1", "agent-1")
	if err != nil {
		t.Fatal(err)
	}
	returned, err := svc.createRun(context.Background(), createRunParams{workspaceID: "ws-1", agent: agent, targetType: "support_conversation", targetID: "conv-1"})
	if err != nil {
		t.Fatal(err)
	}
	if returned.ID != live.ID {
		t.Fatalf("ordinary chat reused %s instead of %s", returned.ID, live.ID)
	}
	if len(runtimeClient.startRunCalls) != 0 {
		t.Fatal("started an unnecessary additional chat")
	}
}
