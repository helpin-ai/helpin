//go:build integration

package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func setupFollowUpPostgres(t *testing.T) (*SupportFollowUpService, *gorm.DB, *model.AgentRun, model.SupportAIFollowUp, supportFollowUpDecision) {
	t.Helper()
	dsn := os.Getenv("SUPPORT_FOLLOWUP_TEST_DSN")
	if dsn == "" {
		t.Skip("SUPPORT_FOLLOWUP_TEST_DSN required")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	schema := "followup_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := base.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		sqlDB, err := base.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	mustExec(t, db, `CREATE TABLE workspaces(id uuid PRIMARY KEY)`)
	if err := db.AutoMigrate(&model.SupportConversation{}, &model.SupportMessage{}, &model.SupportWidgetInstallation{}, &model.SupportEmailLog{}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `CREATE TABLE agent_runs(id uuid PRIMARY KEY, workspace_id uuid, target_type text, target_id text, status text, pause_reason text, created_at timestamptz)`)
	migration, err := os.ReadFile("../dbmigrate/sql/202609090002_support_ai_follow_ups.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := db.Exec(string(migration)).Error; err != nil {
			t.Fatalf("migration: %v", err)
		}
	}
	sequenceMigration, err := os.ReadFile("../dbmigrate/sql/202609090003_support_follow_up_sequences.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(sequenceMigration)).Error; err != nil {
		t.Fatal(err)
	}
	// Public-message projection participates in the same parent lock as the
	// production projection. Other projections are tested in inbox integration tests.
	mustExec(t, db, `CREATE FUNCTION test_project_public_message() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT NEW.is_internal AND NEW.message_type='reply' THEN UPDATE support_conversations SET last_public_message_id=NEW.id,last_public_message_at=NEW.created_at,last_public_sender_type=NEW.sender_type WHERE id=NEW.conversation_id; END IF; RETURN NEW; END $$;
 CREATE TRIGGER test_project_public_message AFTER INSERT ON support_messages FOR EACH ROW EXECUTE FUNCTION test_project_public_message()`)
	ws, convID, source, agentID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	mustExec(t, db, `INSERT INTO workspaces VALUES (?)`, ws)
	settings := fmt.Sprintf(`{"ai_enabled":true,"ai_response_mode":"ai_first","ai_agent_id":%q,"ai_follow_up_enabled":true}`, agentID)
	inst := model.SupportWidgetInstallation{ID: uuid.NewString(), WorkspaceID: ws, WidgetKey: uuid.NewString(), SecretKey: "test", Settings: settings, Active: true}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	conv := model.SupportConversation{ID: convID, WorkspaceID: ws, DisplayID: 1, Subject: "Gmail", Status: "open", FlowState: strPtr("ai_handling"), AIState: strPtr("pending"), Channel: "widget", AssignedAgentID: &agentID}
	if err := db.Create(&conv).Error; err != nil {
		t.Fatal(err)
	}
	message := model.SupportMessage{ID: source, WorkspaceID: ws, ConversationID: convID, SenderType: "ai", MessageType: "reply", Content: "Reconnect Gmail", CreatedAt: now.Add(-25 * time.Hour)}
	if err := db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	e := model.SupportAIFollowUp{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: convID, SourceMessageID: source, RunID: uuid.NewString(), Status: "assessing", DueAt: now, CloseHours: 48, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&e).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewSupportFollowUpService(repository.NewSupportFollowUpRepository(db), &SupportChatService{messageRepo: repository.NewSupportMessageRepository(db), supportAIService: &SupportAIService{}})
	svc.now = func() time.Time { return now }
	run := &model.AgentRun{Status: "running", ID: e.RunID, WorkspaceID: ws, TargetID: convID, AgentID: agentID}
	d := supportFollowUpDecision{Action: "follow_up", Reason: "Waiting for customer confirmation", Question: "Did reconnecting Gmail work?", ClosureNotice: "This conversation closes in 48 hours without a reply. Reply anytime to reopen it.", ObligationsClear: true, SourceMessageIDs: []string{source}}
	return svc, db, run, e, d
}

func TestSupportFollowUpPostgresConcurrentCompletion(t *testing.T) {
	svc, db, run, e, d := setupFollowUpPostgres(t)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := svc.Complete(context.Background(), run, e.ID, d); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	latest, err := svc.repo.Latest(context.Background(), e.WorkspaceID, e.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Status != "waiting" {
		t.Fatalf("own follow-up cancelled by projection: %+v", latest)
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("id <> ?", e.SourceMessageID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("published %d messages", count)
	}
	if err := svc.process(context.Background(), *latest, *latest.CloseAt); err != nil {
		t.Fatal(err)
	}
	var conv model.SupportConversation
	if err := db.First(&conv, "id = ?", e.ConversationID).Error; err != nil {
		t.Fatal(err)
	}
	if conv.Status != "resolved" || derefString(conv.AIResolutionType) != "assumed" {
		t.Fatalf("not resolved: %+v", conv)
	}
}

func TestSupportFollowUpPostgresCancellation(t *testing.T) {
	for _, scenario := range []string{"reply", "edit", "settings", "takeover", "manual"} {
		t.Run(scenario, func(t *testing.T) {
			svc, db, run, e, d := setupFollowUpPostgres(t)
			if _, err := svc.Complete(context.Background(), run, e.ID, d); err != nil {
				t.Fatal(err)
			}
			latest, err := svc.repo.Latest(context.Background(), e.WorkspaceID, e.ConversationID)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "reply":
				mustExec(t, db, `INSERT INTO support_messages(id,workspace_id,conversation_id,sender_type,message_type,content,created_at) VALUES (?,?,?,'customer','reply','Still broken',?)`, uuid.NewString(), e.WorkspaceID, e.ConversationID, time.Now())
			case "edit":
				mustExec(t, db, `UPDATE support_messages SET content='Changed answer' WHERE id=?`, e.SourceMessageID)
			case "settings":
				mustExec(t, db, `UPDATE support_widget_installations SET settings=jsonb_set(settings,'{ai_follow_up_enabled}','false') WHERE workspace_id=?`, e.WorkspaceID)
			case "takeover":
				mustExec(t, db, `UPDATE support_conversations SET human_takeover=true WHERE id=?`, e.ConversationID)
			case "manual":
				if err := svc.repo.CancelConversation(context.Background(), e.WorkspaceID, e.ConversationID); err != nil {
					t.Fatal(err)
				}
			}
			if err := svc.process(context.Background(), *latest, *latest.CloseAt); err != nil {
				t.Fatal(err)
			}
			latest, err = svc.repo.Latest(context.Background(), e.WorkspaceID, e.ConversationID)
			if err != nil {
				t.Fatal(err)
			}
			if latest.Status != "cancelled" {
				t.Fatalf("status=%s", latest.Status)
			}
			var conv model.SupportConversation
			if err := db.First(&conv, "id = ?", e.ConversationID).Error; err != nil {
				t.Fatal(err)
			}
			if conv.Status == "resolved" {
				t.Fatal("cancelled episode closed conversation")
			}
		})
	}
}

func TestSupportFollowUpPostgresDeliveryFailure(t *testing.T) {
	svc, db, run, e, d := setupFollowUpPostgres(t)
	if _, err := svc.Complete(context.Background(), run, e.ID, d); err != nil {
		t.Fatal(err)
	}
	latest, err := svc.repo.Latest(context.Background(), e.WorkspaceID, e.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	log := model.SupportEmailLog{ID: uuid.NewString(), WorkspaceID: e.WorkspaceID, ConversationID: e.ConversationID, Direction: "outbound", MessageIDs: model.DocsStringArray{*latest.SentMessageID}, Status: "bounced"}
	if err := db.Create(&log).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.process(context.Background(), *latest, *latest.CloseAt); err != nil {
		t.Fatal(err)
	}
	latest, err = svc.repo.Latest(context.Background(), e.WorkspaceID, e.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Status != "failed" || latest.Reason != "follow_up_delivery_failed" {
		t.Fatalf("did not block bounced delivery: %+v", latest)
	}
}

func TestSupportFollowUpPostgresQueueSurvivesRestart(t *testing.T) {
	svc, db, _, e, _ := setupFollowUpPostgres(t)
	mustExec(t, db, `DELETE FROM support_ai_follow_ups`)
	settings := model.DefaultSupportInboxSettings()
	var inst model.SupportWidgetInstallation
	if err := db.First(&inst).Error; err != nil {
		t.Fatal(err)
	}
	settings = parseSettings(inst.Settings)
	preview, err := svc.repo.Preview(context.Background(), e.WorkspaceID, settings, svc.now())
	if err != nil {
		t.Fatal(err)
	}
	if preview.Candidates != 1 || len(preview.Sample) != 1 {
		t.Fatalf("preview=%+v", preview)
	}
	for range 2 {
		if err := svc.repo.Seed(context.Background(), e.WorkspaceID, settings, svc.now()); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := svc.repo.Claim(context.Background(), svc.now())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("seed duplicated: %d", len(rows))
	}
	restarted := repository.NewSupportFollowUpRepository(db)
	claimed, err := restarted.Claim(context.Background(), svc.now())
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 0 {
		t.Fatal("lease did not survive restart")
	}
	claimed, err = restarted.Claim(context.Background(), svc.now().Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].RunID != rows[0].RunID {
		t.Fatal("expired claim did not retain reserved runtime identity")
	}
}

func TestSupportFollowUpPostgresReplyRace(t *testing.T) {
	svc, db, run, e, d := setupFollowUpPostgres(t)
	var wg sync.WaitGroup
	var completionErr, replyErr error
	wg.Add(2)
	go func() { defer wg.Done(); _, completionErr = svc.Complete(context.Background(), run, e.ID, d) }()
	go func() {
		defer wg.Done()
		message := model.SupportMessage{ID: uuid.NewString(), WorkspaceID: e.WorkspaceID, ConversationID: e.ConversationID, SenderType: "customer", MessageType: "reply", Content: "Still broken", CreatedAt: svc.now().Add(time.Second)}
		replyErr = db.Create(&message).Error
	}()
	wg.Wait()
	if completionErr != nil || replyErr != nil {
		t.Fatalf("completion=%v, reply=%v", completionErr, replyErr)
	}
	latest, err := svc.repo.Latest(context.Background(), e.WorkspaceID, e.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Status != "cancelled" {
		t.Fatalf("reply did not fence closure: %+v", latest)
	}
}

func TestSupportFollowUpPostgresDailyLimit(t *testing.T) {
	svc, db, _, e, _ := setupFollowUpPostgres(t)
	mustExec(t, db, `DELETE FROM support_ai_follow_ups`)
	var inst model.SupportWidgetInstallation
	if err := db.First(&inst).Error; err != nil {
		t.Fatal(err)
	}
	settings := parseSettings(inst.Settings)
	for i := 2; i <= 35; i++ {
		conv := model.SupportConversation{ID: uuid.NewString(), WorkspaceID: e.WorkspaceID, DisplayID: i, Subject: "Issue", Status: "open", FlowState: strPtr("ai_handling"), AIState: strPtr("pending"), Channel: "widget", AssignedAgentID: settings.AIAgentID}
		if err := db.Create(&conv).Error; err != nil {
			t.Fatal(err)
		}
		msg := model.SupportMessage{ID: uuid.NewString(), WorkspaceID: e.WorkspaceID, ConversationID: conv.ID, SenderType: "ai", MessageType: "reply", Content: "Answer", CreatedAt: svc.now().Add(-25 * time.Hour)}
		if err := db.Create(&msg).Error; err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	var firstErr, secondErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		firstErr = svc.repo.Seed(context.Background(), e.WorkspaceID, settings, svc.now())
	}()
	go func() {
		defer wg.Done()
		secondErr = svc.repo.Seed(context.Background(), e.WorkspaceID, settings, svc.now())
	}()
	wg.Wait()
	if firstErr != nil || secondErr != nil {
		t.Fatalf("seed: %v %v", firstErr, secondErr)
	}
	var count int64
	if err := db.Model(&model.SupportAIFollowUp{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 25 {
		t.Fatalf("daily limit produced %d assessments", count)
	}
}

func TestSupportFollowUpPostgresSecondMessageAndSettingsSnapshot(t *testing.T) {
	svc, db, run, e, d := setupFollowUpPostgres(t)
	mustExec(t, db, `UPDATE support_ai_follow_ups SET sequence_version=2,second_delay_hours=24,close_hours=1 WHERE id=?`, e.ID)
	d.ClosureNotice = "We'll close this conversation shortly. Reply anytime to reopen it."
	if _, err := svc.Complete(context.Background(), run, e.ID, d); err != nil {
		t.Fatal(err)
	}
	row, _ := svc.repo.Latest(context.Background(), run.WorkspaceID, run.TargetID)
	due := row.DueAt
	mustExec(t, db, `UPDATE support_widget_installations SET settings=settings || '{"ai_follow_up_delay_hours":12,"ai_follow_up_second_delay_hours":12,"ai_follow_up_close_hours":12}'::jsonb WHERE workspace_id=?`, run.WorkspaceID)
	row, _ = svc.repo.Latest(context.Background(), run.WorkspaceID, run.TargetID)
	if row.Status != "waiting" || !row.DueAt.Equal(due) || row.CloseHours != 1 {
		t.Fatal("timing change altered active sequence")
	}
	if err := svc.process(context.Background(), *row, due); err != nil {
		t.Fatal(err)
	}
	row, _ = svc.repo.Latest(context.Background(), run.WorkspaceID, run.TargetID)
	if row.Status != "waiting" || row.SecondMessageID == nil {
		t.Fatalf("second reminder self-cancelled: %+v", row)
	}
	var conv model.SupportConversation
	if err := db.First(&conv, "id=?", run.TargetID).Error; err != nil {
		t.Fatal(err)
	}
	if derefString(conv.LastPublicMessageID) != *row.SecondMessageID {
		t.Fatal("second message projection missing")
	}
	reply := model.SupportMessage{ID: uuid.NewString(), WorkspaceID: run.WorkspaceID, ConversationID: run.TargetID, SenderType: "customer", MessageType: "reply", Content: "Still not working", CreatedAt: due.Add(time.Minute)}
	if err := db.Create(&reply).Error; err != nil {
		t.Fatal(err)
	}
	row, _ = svc.repo.Latest(context.Background(), run.WorkspaceID, run.TargetID)
	if row.Status != "cancelled" {
		t.Fatal("customer reply did not cancel second stage")
	}
}

func TestSupportFollowUpPostgresSecondReminderIsIdempotent(t *testing.T) {
	svc, db, run, e, d := setupFollowUpPostgres(t)
	mustExec(t, db, `UPDATE support_ai_follow_ups SET sequence_version=2,second_delay_hours=24,close_hours=1 WHERE id=?`, e.ID)
	d.ClosureNotice = "We'll close this conversation shortly. Reply anytime to reopen it."
	if _, err := svc.Complete(context.Background(), run, e.ID, d); err != nil {
		t.Fatal(err)
	}
	row, _ := svc.repo.Latest(context.Background(), run.WorkspaceID, run.TargetID)
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- svc.process(context.Background(), *row, row.DueAt) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	db.Model(&model.SupportMessage{}).Where("conversation_id=? AND sender_type='ai'", run.TargetID).Count(&count)
	if count != 3 {
		t.Fatalf("expected original answer + two reminders, got %d", count)
	}
}

func TestSupportFollowUpPostgresRolloutKeepsLegacyWaitingDeadline(t *testing.T) {
	svc, db, run, e, d := setupFollowUpPostgres(t)
	if _, err := svc.Complete(context.Background(), run, e.ID, d); err != nil {
		t.Fatal(err)
	}
	before, _ := svc.repo.Latest(context.Background(), run.WorkspaceID, run.TargetID)
	migration, err := os.ReadFile("../dbmigrate/sql/202609090003_support_follow_up_sequences.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(migration)).Error; err != nil {
		t.Fatal(err)
	}
	after, _ := svc.repo.Latest(context.Background(), run.WorkspaceID, run.TargetID)
	if after.Status != "waiting" || !after.CloseAt.Equal(*before.CloseAt) || after.CloseHours != 48 {
		t.Fatal("rollout changed legacy deadline")
	}
	var inst model.SupportWidgetInstallation
	db.First(&inst, "workspace_id=?", run.WorkspaceID)
	settings := parseSettings(inst.Settings)
	if !settings.AIFollowUpEnabled || settings.AIFollowUpCloseHours != 1 || settings.AIFollowUpSecondDelayHours != 24 {
		t.Fatal("rollout defaults not applied")
	}
}
