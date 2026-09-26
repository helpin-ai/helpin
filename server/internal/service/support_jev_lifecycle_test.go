package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/decision"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type lifecycleJev struct {
	choice      string
	probability float64
	calls       int
	states      []string
	err         error
	during      func()
}

func (f *lifecycleJev) DecideMany(_ context.Context, state string, questions map[string]decision.Question) (*decision.Result, error) {
	f.calls++
	f.states = append(f.states, state)
	if f.during != nil {
		f.during()
	}
	if f.err != nil {
		return nil, f.err
	}
	result := &decision.Result{Model: decision.Model, InputTokens: 20, OutputTokens: 2, Answers: map[string]decision.Answer{}}
	for key, q := range questions {
		ps := map[string]float64{}
		for choice := range q.Choices {
			ps[choice] = (1 - f.probability) / float64(len(q.Choices)-1)
		}
		ps[f.choice] = f.probability
		result.Answers[key] = decision.Answer{Choice: f.choice, Probabilities: ps, ProviderConfidence: .01}
	}
	return result, nil
}
func attachLifecycleJev(t *testing.T, db *gorm.DB, chat *SupportChatService, p *lifecycleJev) *SupportJevService {
	t.Helper()
	mustExec(t, db, `INSERT INTO workspaces(id,name,slug,owner_id) VALUES ('ws','Test','test','owner')`)
	if err := db.AutoMigrate(&model.AIExecutionUsage{}); err != nil {
		t.Fatal(err)
	}
	tags := NewSupportTagService(repository.NewSupportTagRepository(db), repository.NewSupportConversationRepository(db), nil)
	svc, err := NewSupportJevService(SupportJevConfig{RoutingMode: "off", TagsMode: "off", RoutingThreshold: .9, TagThreshold: .95, HandoffThreshold: .95, FollowUpThreshold: .95, DailyLimit: 100}, p, repository.NewSupportJevRepository(db), repository.NewAIExecutionUsageRepository(db), tags)
	if err != nil {
		t.Fatal(err)
	}
	chat.SetJevService(svc)
	return svc
}

func TestJevFollowUpActions(t *testing.T) {
	for _, tc := range []struct {
		choice, status   string
		handoff, handled bool
	}{
		{"team_owes_work", "handoff", true, true}, {"needs_human", "handoff", true, true},
		{"confirmed_resolved", "skipped", false, true}, {"no_follow_up", "skipped", false, true},
		{"waiting_customer", "assessing", false, false},
	} {
		t.Run(tc.choice, func(t *testing.T) {
			svc, db, _, row, _ := setupFollowUpTest(t)
			p := &lifecycleJev{choice: tc.choice, probability: .99}
			attachLifecycleJev(t, db, svc.chat, p)
			conv := readControlConversation(t, db)
			handled, classification, err := svc.assessJevFollowUp(context.Background(), row, conv, svc.now())
			if err != nil || handled != tc.handled || classification != tc.choice {
				t.Fatalf("assessment=%v/%s/%v", handled, classification, err)
			}
			latest, err := svc.repo.Latest(context.Background(), "ws", "conv")
			if err != nil || latest.Status != tc.status {
				t.Fatalf("episode=%+v err=%v", latest, err)
			}
			current := readControlConversation(t, db)
			if supportConversationHumanOwned(current) != tc.handoff {
				t.Fatalf("wrong ownership: %+v", current)
			}
			if current.Status == "resolved" {
				t.Fatal("classification silently closed conversation")
			}
			var public int64
			if err := db.Model(&model.SupportMessage{}).Where("id <> 'source' AND is_internal = false").Count(&public).Error; err != nil {
				t.Fatal(err)
			}
			if public != 0 {
				t.Fatal("classifier sent a generated customer message")
			}
			var usage int64
			if err := db.Model(&model.AIExecutionUsage{}).Count(&usage).Error; err != nil || usage != 1 {
				t.Fatalf("usage=%d/%v", usage, err)
			}
		})
	}
}

func TestJevLifecycleFallback(t *testing.T) {
	for _, tc := range []struct {
		name, mode, choice string
		p                  float64
		err                error
		calls              int
	}{
		{name: "off", mode: "off", choice: "team_owes_work", p: .99, calls: 0},
		{name: "shadow", mode: "shadow", choice: "team_owes_work", p: .99, calls: 1},
		{name: "low probability", choice: "team_owes_work", p: .6, calls: 1},
		{name: "uncertain", choice: "uncertain", p: .99, calls: 1},
		{name: "provider failure", err: errors.New("offline"), calls: 1},
		{name: "malformed choice", choice: "delete", p: .99, calls: 1},
		{name: "not a probability", choice: "team_owes_work", p: math.NaN(), calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, _, row, _ := setupFollowUpTest(t)
			p := &lifecycleJev{choice: tc.choice, probability: tc.p, err: tc.err}
			jev := attachLifecycleJev(t, db, svc.chat, p)
			if tc.mode != "" {
				jev.config.FollowUpMode = tc.mode
			}
			handled, classification, err := svc.assessJevFollowUp(context.Background(), row, readControlConversation(t, db), svc.now())
			if err != nil || handled || classification != "" || p.calls != tc.calls {
				t.Fatalf("fallback=%v/%s/%v calls=%d", handled, classification, err, p.calls)
			}
			latest, err := svc.repo.Latest(context.Background(), "ws", "conv")
			if err != nil || latest.Status != "assessing" {
				t.Fatalf("fallback mutated episode: %+v/%v", latest, err)
			}
		})
	}
}

func TestJevFollowUpRechecksAfterProvider(t *testing.T) {
	for _, tc := range []struct{ name, sql string }{
		{"customer reply", `UPDATE support_conversations SET last_public_message_id='new',last_public_sender_type='customer'`},
		{"manual takeover", `UPDATE support_conversations SET human_takeover=true`},
		{"human assignment", `UPDATE support_conversations SET assigned_user_id='human'`},
		{"control changed", `UPDATE support_conversations SET ai_control_version=ai_control_version+1`},
		{"message edited", `UPDATE support_messages SET content='Actually the customer replied'`},
		{"installation disabled", `UPDATE support_widget_installations SET active=false`},
		{"busy", `INSERT INTO agent_runs(id,workspace_id,target_type,target_id,status) VALUES ('busy','ws','support_conversation','conv','running')`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, _, row, _ := setupFollowUpTest(t)
			p := &lifecycleJev{choice: "team_owes_work", probability: .99, during: func() { mustExec(t, db, tc.sql) }}
			attachLifecycleJev(t, db, svc.chat, p)
			handled, _, err := svc.assessJevFollowUp(context.Background(), row, readControlConversation(t, db), svc.now())
			if err != nil || !handled {
				t.Fatalf("stale result=%v/%v", handled, err)
			}
			latest, err := svc.repo.Latest(context.Background(), "ws", "conv")
			if err != nil || latest.Status != "cancelled" {
				t.Fatalf("stale episode=%+v/%v", latest, err)
			}
			var count int64
			if err := db.Model(&model.SupportMessage{}).Where("id <> 'source'").Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("stale result wrote messages=%d/%v", count, err)
			}
		})
	}
}

func TestJevLifecycleContext(t *testing.T) {
	now := time.Now()
	msg := model.SupportMessage{ID: "source", WorkspaceID: "ws", ConversationID: "conv", SenderType: "customer", MessageType: "reply", Content: "Still broken", CreatedAt: now, SenderDisplayName: strPtr("PRIVATE NAME"), Metadata: `{"secret":"PRIVATE METADATA"}`}
	history := []model.SupportMessage{
		{ID: "old", WorkspaceID: "ws", ConversationID: "conv", SenderType: "ai", MessageType: "reply", Content: "I will investigate", CreatedAt: now.Add(-time.Hour)},
		{ID: "note", WorkspaceID: "ws", ConversationID: "conv", SenderType: "user", MessageType: "reply", IsInternal: true, Content: "PRIVATE NOTE", CreatedAt: now}, msg,
	}
	state, valid := supportJevLifecycleState("ws", "conv", "source", history)
	if !valid || strings.Contains(state, "PRIVATE") || !strings.Contains(state, "I will investigate") {
		t.Fatalf("bad context: %s", state)
	}
	for _, tc := range []struct {
		name   string
		change func([]model.SupportMessage) []model.SupportMessage
	}{
		{"oversized earlier obligation", func(h []model.SupportMessage) []model.SupportMessage {
			h[0].Content = strings.Repeat("x", 16000)
			return h
		}},
		{"empty latest message", func(h []model.SupportMessage) []model.SupportMessage { h[2].Content = ""; return h }},
		{"newer reply", func(h []model.SupportMessage) []model.SupportMessage {
			n := msg
			n.ID = "new"
			n.CreatedAt = now.Add(time.Second)
			return append(h, n)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, valid := supportJevLifecycleState("ws", "conv", "source", tc.change(append([]model.SupportMessage(nil), history...))); valid {
				t.Fatal("incomplete context accepted")
			}
		})
	}
}

func TestJevFollowUpProcessDoesNotLaunchAgentForHandoff(t *testing.T) {
	svc, db, _, row, _ := setupFollowUpTest(t)
	p := &lifecycleJev{choice: "team_owes_work", probability: .99}
	attachLifecycleJev(t, db, svc.chat, p)
	// agentService is intentionally nil: the due-episode path must finish directly.
	for range 2 {
		if err := svc.process(context.Background(), row, svc.now()); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := svc.repo.Latest(context.Background(), "ws", "conv")
	if err != nil || latest.Status != "handoff" || p.calls != 1 {
		t.Fatalf("process=%+v/%v calls=%d", latest, err, p.calls)
	}
	var notes int64
	if err := db.Model(&model.SupportMessage{}).Where("message_type = 'note'").Count(&notes).Error; err != nil || notes != 1 {
		t.Fatalf("notes=%d/%v", notes, err)
	}
}

func setupJevChat(t *testing.T, p *lifecycleJev) (*SupportChatService, *gorm.DB, *model.SupportMessage) {
	t.Helper()
	inbox, db, conv, settings := setupAIControlTest(t)
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO support_widget_installations(id,workspace_id,widget_key,secret_key,settings,active) VALUES ('inst','ws','key','secret',?,true)`, string(raw))
	mustExec(t, db, `CREATE TABLE ai_message_processing(id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),workspace_id TEXT,conversation_id TEXT,source_message_id TEXT,reply_message_id TEXT,status TEXT,tokens_used INTEGER,attempts INTEGER,created_at DATETIME,updated_at DATETIME)`)
	ai := &SupportAIService{conversationRepo: inbox.conversationRepo, messageRepo: inbox.messageRepo, installationRepo: repository.NewSupportInboxInstallationRepository(db), handoffRepo: repository.NewAgentHandoffRepository(db)}
	chat := &SupportChatService{conversationRepo: inbox.conversationRepo, messageRepo: inbox.messageRepo, supportAIService: ai, processingRepo: repository.NewAIMessageProcessingRepository(db)}
	msg := &model.SupportMessage{ID: "source", WorkspaceID: "ws", ConversationID: conv.ID, SenderType: "customer", MessageType: "reply", Content: "I followed those steps twice and nothing changed", CreatedAt: time.Now()}
	if err := inbox.messageRepo.Create(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE support_conversations SET last_public_message_id='source',last_public_sender_type='customer'`)
	attachLifecycleJev(t, db, chat, p)
	return chat, db, msg
}

func TestJevHandoffVisitorEntryPoint(t *testing.T) {
	for _, reason := range []string{"stuck", "customer_requested_human", "action_unavailable"} {
		t.Run(reason, func(t *testing.T) {
			p := &lifecycleJev{choice: reason, probability: .99}
			chat, db, msg := setupJevChat(t, p)
			for range 2 {
				if err := chat.HandleVisitorMessage(context.Background(), "ws", "conv", msg); err != nil {
					t.Fatal(err)
				}
			}
			conv := readControlConversation(t, db)
			if !supportConversationHumanOwned(conv) || derefString(conv.AIState) != "escalated" || p.calls != 1 {
				t.Fatalf("handoff=%+v calls=%d", conv, p.calls)
			}
			var rows []model.AgentHandoff
			if err := db.Find(&rows).Error; err != nil || len(rows) != 1 || rows[0].Reason != reason {
				t.Fatalf("handoffs=%+v/%v", rows, err)
			}
			var turn model.AIMessageProcessing
			if err := db.First(&turn, "source_message_id = ?", msg.ID).Error; err != nil || turn.Status != "completed" {
				t.Fatalf("processing=%+v/%v", turn, err)
			}
		})
	}
}

func TestJevHandoffPreservesTakeoverDuringAssessment(t *testing.T) {
	p := &lifecycleJev{choice: "stuck", probability: .99}
	chat, db, msg := setupJevChat(t, p)
	p.during = func() {
		mustExec(t, db, `UPDATE support_conversations SET human_takeover=true,assigned_user_id='human',ai_control_version=1`)
	}
	if err := chat.HandleVisitorMessage(context.Background(), "ws", "conv", msg); err != nil {
		t.Fatal(err)
	}
	conv := readControlConversation(t, db)
	if derefString(conv.AssignedUserID) != "human" || conv.AIControlVersion != 1 {
		t.Fatalf("takeover overwritten: %+v", conv)
	}
	var count int64
	if err := db.Model(&model.AgentHandoff{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("stale handoff=%d/%v", count, err)
	}
}

func TestJevLifecycleConfiguration(t *testing.T) {
	svc, db, _, _, _ := setupFollowUpTest(t)
	p := &lifecycleJev{choice: "continue", probability: .99}
	jev := attachLifecycleJev(t, db, svc.chat, p)
	for _, tc := range []struct {
		name   string
		change func(*SupportJevConfig)
	}{
		{"invalid handoff mode", func(c *SupportJevConfig) { c.HandoffMode = "bad" }},
		{"invalid followup mode", func(c *SupportJevConfig) { c.FollowUpMode = "bad" }},
		{"zero threshold", func(c *SupportJevConfig) { c.HandoffThreshold = 0 }},
		{"negative threshold", func(c *SupportJevConfig) { c.FollowUpThreshold = -1 }},
		{"nan threshold", func(c *SupportJevConfig) { c.HandoffThreshold = math.NaN() }},
		{"threshold above one", func(c *SupportJevConfig) { c.FollowUpThreshold = 1.1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := jev.config
			tc.change(&cfg)
			if _, err := NewSupportJevService(cfg, p, jev.events, jev.usage, jev.tags); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	if jev.config.HandoffMode != "primary" || jev.config.FollowUpMode != "primary" {
		t.Fatal("lifecycle default is not primary")
	}
}

func TestJevLifecycleAdmissionAndPersistenceFallback(t *testing.T) {
	for _, scenario := range []string{"workspace excluded", "daily cap", "duplicate", "usage failure", "no provider"} {
		t.Run(scenario, func(t *testing.T) {
			svc, db, _, row, _ := setupFollowUpTest(t)
			p := &lifecycleJev{choice: "team_owes_work", probability: .99}
			jev := attachLifecycleJev(t, db, svc.chat, p)
			history, err := svc.chat.messageRepo.ListByConversation(context.Background(), "ws", "conv", false)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "workspace excluded":
				jev.workspaces = map[string]bool{"other": true}
			case "daily cap":
				jev.config.DailyLimit = 1
				if _, err := jev.evaluate(context.Background(), "ws", "conv", "test", "other", map[string]decision.Question{"lifecycle": supportJevFollowUpQuestion()}); err != nil {
					t.Fatal(err)
				}
			case "duplicate":
				if _, err := jev.classifyFollowUp(context.Background(), row, history); err != nil {
					t.Fatal(err)
				}
			case "usage failure":
				mustExec(t, db, `DROP TABLE ai_execution_usage`)
			case "no provider":
				svc.chat.SetJevService(nil)
			}
			handled, classification, err := svc.assessJevFollowUp(context.Background(), row, readControlConversation(t, db), svc.now())
			if err != nil || handled || classification != "" {
				t.Fatalf("fallback=%v/%s/%v", handled, classification, err)
			}
		})
	}
}

func TestJevHandoffModesAndGuards(t *testing.T) {
	for _, scenario := range []string{"off", "shadow", "uncertain", "continue", "provider failure", "human owner", "newer message", "edited message", "policy changed"} {
		t.Run(scenario, func(t *testing.T) {
			p := &lifecycleJev{choice: "stuck", probability: .99}
			chat, db, msg := setupJevChat(t, p)
			conv := readControlConversation(t, db)
			switch scenario {
			case "off", "shadow":
				chat.jev.config.HandoffMode = scenario
			case "uncertain":
				p.probability = .7
			case "continue":
				p.choice = "continue"
			case "provider failure":
				p.err = errors.New("offline")
			case "human owner":
				conv.AssignedUserID = strPtr("human")
			case "newer message":
				p.during = func() { mustExec(t, db, `UPDATE support_conversations SET last_public_message_id='new'`) }
			case "edited message":
				p.during = func() { mustExec(t, db, `UPDATE support_messages SET content='It is fixed now'`) }
			case "policy changed":
				p.during = func() { mustExec(t, db, `UPDATE support_widget_installations SET settings='{}'`) }
			}
			history, err := chat.messageRepo.ListByConversation(context.Background(), "ws", "conv", false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := chat.assessJevHandoff(context.Background(), conv, msg, history); err != nil {
				t.Fatal(err)
			}
			var count int64
			if err := db.Model(&model.AgentHandoff{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("unexpected handoff=%d/%v", count, err)
			}
			if (scenario == "off" || scenario == "human owner") && p.calls != 0 {
				t.Fatal("guard did not suppress provider call")
			}
		})
	}
}

func TestJevHandoffAtomicSourceFence(t *testing.T) {
	p := &lifecycleJev{choice: "stuck", probability: .99}
	chat, db, msg := setupJevChat(t, p)
	version := int64(0)
	mustExec(t, db, `UPDATE support_conversations SET last_public_message_id='newer'`)
	err := chat.supportAIService.EscalateToHumanForMessageWithIssue(context.Background(), "ws", "conv", msg.ID, "stuck", "", "", SupportHandoffBrief{ExpectedMessageID: msg.ID, ExpectedControlVersion: &version})
	if err != nil {
		t.Fatal(err)
	}
	if supportConversationHumanOwned(readControlConversation(t, db)) {
		t.Fatal("stale source bypassed atomic fence")
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("id <> 'source'").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("stale handoff published=%d/%v", count, err)
	}
}

func TestJevWaitingCustomerPreservesReminderSequence(t *testing.T) {
	svc, db, run, row, d := setupFollowUpTest(t)
	p := &lifecycleJev{choice: "waiting_customer", probability: .99}
	attachLifecycleJev(t, db, svc.chat, p)
	mustExec(t, db, `UPDATE support_ai_follow_ups SET sequence_version=2,second_delay_hours=24,close_hours=1`)
	handled, classification, err := svc.assessJevFollowUp(context.Background(), row, readControlConversation(t, db), svc.now())
	if err != nil || handled || classification != "waiting_customer" {
		t.Fatalf("assessment=%v/%s/%v", handled, classification, err)
	}
	d.ClosureNotice = "We'll close this conversation shortly. Reply anytime to reopen it."
	if _, err := svc.Complete(context.Background(), run, row.ID, d); err != nil {
		t.Fatal(err)
	}
	first, err := svc.repo.Latest(context.Background(), "ws", "conv")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE support_conversations SET last_public_message_id=?`, *first.SentMessageID)
	if err := svc.process(context.Background(), *first, first.DueAt); err != nil {
		t.Fatal(err)
	}
	second, err := svc.repo.Latest(context.Background(), "ws", "conv")
	if err != nil {
		t.Fatal(err)
	}
	if second.SecondMessageID == nil || second.CloseAt == nil {
		t.Fatalf("second reminder missing: %+v", second)
	}
	mustExec(t, db, `UPDATE support_conversations SET last_public_message_id=?`, *second.SecondMessageID)
	if err := svc.process(context.Background(), *second, *second.CloseAt); err != nil {
		t.Fatal(err)
	}
	conv := readControlConversation(t, db)
	if conv.Status != "resolved" || derefString(conv.AIResolutionType) != "assumed" || p.calls != 1 {
		t.Fatalf("sequence=%+v Jev calls=%d", conv, p.calls)
	}
}

func TestJevHardHumanRequestRetainsPrecedence(t *testing.T) {
	p := &lifecycleJev{choice: "continue", probability: .99}
	chat, db, msg := setupJevChat(t, p)
	msg.Content = "I want to speak to a real person"
	mustExec(t, db, `UPDATE support_messages SET content=? WHERE id=?`, msg.Content, msg.ID)
	if err := chat.HandleVisitorMessage(context.Background(), "ws", "conv", msg); err != nil {
		t.Fatal(err)
	}
	if p.calls != 0 || !supportConversationHumanOwned(readControlConversation(t, db)) {
		t.Fatal("explicit human request lost precedence")
	}
}
