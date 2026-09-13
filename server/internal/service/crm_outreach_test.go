package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type outreachTestAuth struct{ denied bool }

func (a *outreachTestAuth) ResolveActor(_ context.Context, ws, user string) (*authorization.Actor, error) {
	return &authorization.Actor{UserID: user, WorkspaceID: ws, WorkspaceMemberID: "member-owner", Role: "owner", Status: "active"}, nil
}
func (a *outreachTestAuth) Can(*authorization.Actor, authorization.Permission) bool { return !a.denied }
func (a *outreachTestAuth) CanAccessModule(context.Context, *authorization.Actor, model.ModuleID) (bool, error) {
	return !a.denied, nil
}

type outreachTestMail struct {
	sends, checks int
	fail, found   bool
}

func (m *outreachTestMail) ValidateActionSender(_ context.Context, ws, account, user string) error {
	if ws != "ws" || account != "mail" || user != "owner" {
		return errors.New("mailbox ownership")
	}
	return nil
}
func (m *outreachTestMail) SendActionEmail(context.Context, string, string, string, model.CRMPlaybookEmailAction) (*model.CRMEmailMessage, error) {
	m.sends++
	if m.fail {
		return nil, errors.New("connection interrupted")
	}
	return &model.CRMEmailMessage{ID: "sent"}, nil
}
func (m *outreachTestMail) ReconcileActionEmail(context.Context, string, string, string, model.CRMPlaybookEmailAction) (bool, *string, error) {
	m.checks++
	id := "sent"
	return m.found, &id, nil
}
func (m *outreachTestMail) LinkThreadDeal(context.Context, string, string, *string) error { return nil }
func outreachFixture(t *testing.T) (*gorm.DB, *CRMOutreachService, *outreachTestMail) {
	t.Helper()
	db := setupCRMEmailLifecycleTestDB(t)
	if err := db.AutoMigrate(&model.CRMEmailTemplate{}, &model.CRMEmailSequence{}, &model.CRMSequenceEnrollment{}, &model.CRMSequenceDelivery{}, &model.CRMEmailSuppression{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_email_accounts(id,workspace_id,member_id,email_address,last_synced_at) VALUES('mail','ws','owner','owner@example.com',?)`, now)
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_contacts(id,workspace_id,display_id,first_name,email) VALUES('contact','ws','C1','Amna','amna@example.com')`)
	mail := &outreachTestMail{}
	svc := &CRMOutreachService{repo: repository.NewCRMOutreachRepository(db), accounts: repository.NewCRMEmailRepository(db), auth: &outreachTestAuth{}, mail: mail, appURL: "https://example.com", now: func() time.Time { return now }}
	return db, svc, mail
}
func outreachSequence(t *testing.T, s *CRMOutreachService, mode string) *model.CRMEmailSequence {
	t.Helper()
	seq, err := s.SaveSequence(context.Background(), "ws", "owner", "", model.CRMEmailSequence{Name: "Demo follow up", Status: "active", Timezone: "UTC", StartHour: 9, EndHour: 17, Weekdays: true, Steps: []model.CRMSequenceStep{{Kind: "email", Mode: mode, Subject: "Hi {{first_name}}", BodyHTML: "<p>Hello {{first_name}}</p>"}}})
	if err != nil {
		t.Fatal(err)
	}
	return seq
}
func outreachEnroll(t *testing.T, s *CRMOutreachService, seq *model.CRMEmailSequence) *model.CRMSequenceEnrollment {
	t.Helper()
	rows, err := s.Enroll(context.Background(), "ws", "owner", seq.ID, model.CRMSequenceEnrollRequest{AccountID: "mail", ContactIDs: []string{"contact"}, Version: seq.Version})
	if err != nil {
		t.Fatal(err)
	}
	return &rows[0]
}
func TestCRMOutreachPersonalization(t *testing.T) {
	got, err := RenderCRMEmail("<p>{{first_name}} at {{company|your team}}</p>", map[string]string{"first_name": "<Amna & Ali>"}, true)
	if err != nil || got != "<p>&lt;Amna &amp; Ali&gt; at your team</p>" {
		t.Fatalf("%q %v", got, err)
	}
	for _, input := range []string{"{{deal_name}}", "{{unknown|x}}", "{{broken"} {
		if _, err := RenderCRMEmail(input, map[string]string{}, false); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}
func TestCRMOutreachDeliveryWindowDST(t *testing.T) {
	row := model.CRMSequenceEnrollment{Timezone: "America/New_York", StartHour: 9, EndHour: 17, Weekdays: true}
	friday := time.Date(2026, 3, 6, 23, 0, 0, 0, time.UTC)
	want := time.Date(2026, 3, 9, 13, 0, 0, 0, time.UTC)
	if got := NextCRMSequenceWindow(friday, row); !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
	if got := NextCRMSequenceWindow(want, row); !got.Equal(want) {
		t.Fatal("moved valid time")
	}
}
func TestCRMOutreachEnrollmentSnapshotAndDuplicate(t *testing.T) {
	_, s, _ := outreachFixture(t)
	seq := outreachSequence(t, s, "automatic")
	row := outreachEnroll(t, s, seq)
	if row.Steps[0].Subject != "Hi Amna" || !strings.Contains(row.Steps[0].BodyHTML, row.UnsubscribeToken) {
		t.Fatal("missing personalization/unsubscribe")
	}
	seq.Steps[0].Subject = "Changed"
	if _, err := s.SaveSequence(context.Background(), "ws", "owner", seq.ID, *seq); err != nil {
		t.Fatal(err)
	}
	stored, err := s.repo.Enrollment(context.Background(), "ws", row.ID)
	if err != nil || stored.Steps[0].Subject != "Hi Amna" {
		t.Fatal("snapshot changed")
	}
	if _, err := s.Enroll(context.Background(), "ws", "owner", seq.ID, model.CRMSequenceEnrollRequest{AccountID: "mail", ContactIDs: []string{"contact"}, Version: seq.Version}); err == nil {
		t.Fatal("stale enrollment accepted")
	}
	reason, err := s.repo.DuplicateOrSuppressed(context.Background(), "ws", seq.ID, row.Email)
	if err != nil || !strings.Contains(reason, "Already enrolled") {
		t.Fatal(reason, err)
	}
	if _, err := s.repo.Enrollment(context.Background(), "other", row.ID); err == nil {
		t.Fatal("cross-workspace read")
	}
}
func TestCRMOutreachInterruptedDeliveryNeverResends(t *testing.T) {
	_, s, m := outreachFixture(t)
	seq := outreachSequence(t, s, "automatic")
	row := outreachEnroll(t, s, seq)
	m.fail = true
	if err := s.DispatchDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	stored, _ := s.repo.Enrollment(context.Background(), "ws", row.ID)
	if stored.Status != "uncertain" || m.sends != 1 {
		t.Fatalf("%s sends=%d", stored.Status, m.sends)
	}
	if err := s.Control(context.Background(), "ws", "owner", row.ID, "resume"); err == nil {
		t.Fatal("allowed blind retry")
	}
	if err := s.Control(context.Background(), "ws", "owner", row.ID, "check_delivery"); err == nil {
		t.Fatal("absence treated as sent")
	}
	m.found = true
	if err := s.Control(context.Background(), "ws", "owner", row.ID, "check_delivery"); err != nil {
		t.Fatal(err)
	}
	stored, _ = s.repo.Enrollment(context.Background(), "ws", row.ID)
	if stored.Status != "completed" || m.sends != 1 {
		t.Fatalf("%s sends=%d", stored.Status, m.sends)
	}
}
func TestCRMOutreachReviewApprovalPreservesEdits(t *testing.T) {
	_, s, m := outreachFixture(t)
	seq := outreachSequence(t, s, "review")
	row := outreachEnroll(t, s, seq)
	if err := s.DispatchDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	stored, _ := s.repo.Enrollment(context.Background(), "ws", row.ID)
	if stored.Status != "needs_review" || m.sends != 0 {
		t.Fatal("review sent early", stored.Status)
	}
	if err := s.Control(context.Background(), "ws", "other", row.ID, "approve", "Approved", "<p>Edited</p>"); err == nil {
		t.Fatal("wrong owner approved")
	}
	if err := s.Control(context.Background(), "ws", "owner", row.ID, "approve", "Approved", "<p>Edited</p>"); err != nil {
		t.Fatal(err)
	}
	stored, _ = s.repo.Enrollment(context.Background(), "ws", row.ID)
	if stored.Steps[0].Subject != "Approved" || !strings.Contains(stored.Steps[0].BodyHTML, "Edited") || !strings.Contains(stored.Steps[0].BodyHTML, stored.UnsubscribeToken) {
		t.Fatal("lost edit or unsubscribe")
	}
	if err := s.DispatchDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m.sends != 1 {
		t.Fatal("approved email not sent")
	}
}
func TestCRMOutreachUnsubscribeStopsAllAndRejectsEnrollment(t *testing.T) {
	_, s, m := outreachFixture(t)
	seq := outreachSequence(t, s, "automatic")
	row := outreachEnroll(t, s, seq)
	if err := s.Unsubscribe(context.Background(), row.UnsubscribeToken, false); err != nil {
		t.Fatal(err)
	}
	stored, _ := s.repo.Enrollment(context.Background(), "ws", row.ID)
	if stored.Status != "active" {
		t.Fatal("GET unsubscribed")
	}
	if err := s.Unsubscribe(context.Background(), row.UnsubscribeToken, true); err != nil {
		t.Fatal(err)
	}
	if err := s.DispatchDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m.sends != 0 {
		t.Fatal("sent after unsubscribe")
	}
	seq2 := outreachSequence(t, s, "automatic")
	previews, err := s.Preview(context.Background(), "ws", "owner", seq2.ID, model.CRMSequenceEnrollRequest{AccountID: "mail", ContactIDs: []string{"contact"}, Version: seq2.Version})
	if err != nil || !strings.Contains(previews[0].Error, "unsubscribed") {
		t.Fatal(previews, err)
	}
}
func TestCRMOutreachMailboxRateLimit(t *testing.T) {
	db, s, _ := outreachFixture(t)
	seq := outreachSequence(t, s, "automatic")
	row := outreachEnroll(t, s, seq)
	existing := model.CRMSequenceDelivery{ID: uuid.NewString(), EnrollmentID: uuid.NewString(), WorkspaceID: "ws", AccountID: "mail", Kind: "email", CreatedAt: s.now()}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	claimed, err := s.repo.Claim(context.Background(), s.now())
	if err != nil {
		t.Fatal(err)
	}
	d := &model.CRMSequenceDelivery{ID: uuid.NewString(), EnrollmentID: row.ID, AccountID: "mail", Kind: "email"}
	if _, err := s.repo.PrepareDelivery(context.Background(), claimed, d, s.now()); !errors.Is(err, repository.ErrOutreachRateLimit) {
		t.Fatalf("limit bypassed: %v", err)
	}
}
func TestCRMOutreachStopsOnReplyAndRevokedAccess(t *testing.T) {
	for _, scenario := range []string{"reply", "access", "bounce", "stale_sync"} {
		t.Run(scenario, func(t *testing.T) {
			db, s, m := outreachFixture(t)
			seq := outreachSequence(t, s, "automatic")
			row := outreachEnroll(t, s, seq)
			switch scenario {
			case "reply":
				mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_email_messages(id,workspace_id,email_account_id,message_external_id,from_address,to_addresses,cc_addresses,subject,direction,sent_at) VALUES('reply','ws','mail','reply','amna@example.com',CAST('[]' AS BLOB),CAST('[]' AS BLOB),'Re: Hi','inbound',?)`, s.now().Add(time.Second))
			case "access":
				s.auth = &outreachTestAuth{denied: true}
			case "bounce":
				mustExecCRMEmailLifecycle(t, db, `UPDATE crm_contacts SET email_status='invalid' WHERE id='contact'`)
			case "stale_sync":
				mustExecCRMEmailLifecycle(t, db, `UPDATE crm_email_accounts SET last_synced_at=NULL`)
			}
			if err := s.DispatchDue(context.Background()); err != nil {
				t.Fatal(err)
			}
			stored, _ := s.repo.Enrollment(context.Background(), "ws", row.ID)
			if m.sends != 0 {
				t.Fatal("sent despite stop criterion")
			}
			want := map[string]string{"reply": "replied", "access": "failed", "bounce": "bounced", "stale_sync": "active"}[scenario]
			if stored.Status != want {
				t.Fatalf("got %s want %s", stored.Status, want)
			}
		})
	}
}

type outreachTestTasks struct {
	db    *gorm.DB
	calls int
}

func (t *outreachTestTasks) Create(_ context.Context, req model.CreateTaskRequest, _ string) (*model.TaskDetail, error) {
	if len(req.OwnerMemberIDs) != 1 || req.OwnerMemberIDs[0] != "member-owner" {
		return nil, errors.New("wrong task owner reference")
	}
	t.calls++
	err := t.db.Exec(`INSERT INTO pm_tasks(id,workspace_id,external_id,workflow_state_id) VALUES('task','ws',?,'todo')`, *req.ExternalID).Error
	return &model.TaskDetail{Task: model.PMTask{ID: "task"}}, err
}
func outreachTaskTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, sql := range []string{
		`CREATE TABLE pm_tasks(id TEXT PRIMARY KEY,workspace_id TEXT,external_id TEXT,workflow_state_id TEXT)`,
		`CREATE TABLE pm_workflow_states(id TEXT PRIMARY KEY,state_type TEXT)`,
		`INSERT INTO pm_workflow_states VALUES('todo','started'),('done','done')`,
		`CREATE TABLE crm_associations(id TEXT PRIMARY KEY,workspace_id TEXT,from_object_type TEXT,from_object_id TEXT,to_object_type TEXT,to_object_id TEXT,association_label TEXT,created_at DATETIME)`,
	} {
		mustExecCRMEmailLifecycle(t, db, sql)
	}
}
func TestCRMOutreachTaskWaitsWithoutDuplicateCreation(t *testing.T) {
	db, s, _ := outreachFixture(t)
	outreachTaskTables(t, db)
	tasks := &outreachTestTasks{db: db}
	s.tasks = tasks
	seq := outreachSequence(t, s, "automatic")
	seq.Steps = []model.CRMSequenceStep{{Kind: "task", TaskName: "Call {{full_name}}", TeamID: "team"}}
	seq, err := s.SaveSequence(context.Background(), "ws", "owner", seq.ID, *seq)
	if err != nil {
		t.Fatal(err)
	}
	row := outreachEnroll(t, s, seq)
	for range 2 {
		if err := s.DispatchDue(context.Background()); err != nil {
			t.Fatal(err)
		}
		now := s.now().Add(6 * time.Minute)
		s.now = func() time.Time { return now }
	}
	stored, _ := s.repo.Enrollment(context.Background(), "ws", row.ID)
	if stored.Status != "waiting_task" || tasks.calls != 1 {
		t.Fatalf("status=%s tasks=%d error=%s", stored.Status, tasks.calls, stored.Error)
	}
	mustExecCRMEmailLifecycle(t, db, `UPDATE pm_tasks SET workflow_state_id='done'`)
	if err := s.DispatchDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	stored, _ = s.repo.Enrollment(context.Background(), "ws", row.ID)
	if stored.Status != "completed" || tasks.calls != 1 {
		t.Fatal("task completion failed", stored.Status)
	}
}
func TestCRMOutreachAutomaticStageEnrollmentUsesPrimaryContactOnce(t *testing.T) {
	db, s, _ := outreachFixture(t)
	outreachTaskTables(t, db)
	for _, sql := range []string{
		`CREATE TABLE crm_pipelines(id TEXT PRIMARY KEY,workspace_id TEXT)`,
		`CREATE TABLE crm_pipeline_stages(id TEXT PRIMARY KEY,pipeline_id TEXT,stage_type TEXT)`,
		`CREATE TABLE crm_deals(id TEXT PRIMARY KEY,workspace_id TEXT,name TEXT,stage_id TEXT)`,
		`CREATE TABLE pm_activity_log(id TEXT PRIMARY KEY,workspace_id TEXT,entity_type TEXT,entity_id TEXT,event_type TEXT,action TEXT,new_value TEXT,created_at DATETIME)`,
		`INSERT INTO crm_pipelines VALUES('pipeline','ws')`,
		`INSERT INTO crm_pipeline_stages VALUES('stage','pipeline','open')`,
		`INSERT INTO crm_deals VALUES('deal','ws','Renewal','stage')`,
		`INSERT INTO crm_associations(id,workspace_id,from_object_type,from_object_id,to_object_type,to_object_id,association_label) VALUES('a','ws','deal','deal','contact','contact','deal_primary_contact')`,
	} {
		mustExecCRMEmailLifecycle(t, db, sql)
	}
	seq := outreachSequence(t, s, "automatic")
	seq.EntryStageID = "stage"
	seq.EntryAccountID = "mail"
	seq, err := s.SaveSequence(context.Background(), "ws", "owner", seq.ID, *seq)
	if err != nil {
		t.Fatal(err)
	}
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO pm_activity_log VALUES('old','ws','deal','deal','deal.stage_changed','updated','stage',?)`, s.now().Add(-time.Hour))
	if err := s.processEntries(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.repo.Enrollments(context.Background(), "ws", seq.ID, "", "")
	if len(rows) != 0 {
		t.Fatal("enrolled past event")
	}
	for _, id := range []string{"a", "b"} {
		mustExecCRMEmailLifecycle(t, db, `INSERT INTO pm_activity_log VALUES(?,'ws','deal','deal','deal.stage_changed','updated','stage',?)`, id, s.now().Add(time.Second))
	}
	for range 2 {
		if err := s.processEntries(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	rows, _ = s.repo.Enrollments(context.Background(), "ws", seq.ID, "", "")
	if len(rows) != 1 || rows[0].ContactID != "contact" || rows[0].DealID != "deal" {
		t.Fatalf("bad enrollments: %#v", rows)
	}
}
func TestCRMOutreachTemplateVisibilityAndOptimisticEdits(t *testing.T) {
	_, s, _ := outreachFixture(t)
	ctx := context.Background()
	row, err := s.SaveTemplate(ctx, "ws", "owner", "", model.CRMEmailTemplate{Name: "Private", Subject: "Hello", BodyHTML: "<p>Hello</p>", Shared: false})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.Templates(ctx, "ws", "other")
	if err != nil || len(rows) != 0 {
		t.Fatal("leaked private template")
	}
	stale := *row
	row.Shared = true
	updated, err := s.SaveTemplate(ctx, "ws", "owner", row.ID, *row)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveTemplate(ctx, "ws", "owner", row.ID, stale); err == nil {
		t.Fatal("stale edit accepted")
	}
	if _, err := s.SaveTemplate(ctx, "ws", "other", row.ID, *updated); err == nil {
		t.Fatal("another owner edited")
	}
	rows, err = s.Templates(ctx, "ws", "other")
	if err != nil || len(rows) != 1 {
		t.Fatal("shared template unavailable")
	}
}
func TestCRMOutreachLeaseFencesPausedWorker(t *testing.T) {
	_, s, _ := outreachFixture(t)
	seq := outreachSequence(t, s, "automatic")
	outreachEnroll(t, s, seq)
	ctx := context.Background()
	claim, err := s.repo.Claim(ctx, s.now())
	if err != nil || claim == nil {
		t.Fatal(err)
	}
	if err := s.Control(ctx, "ws", "owner", claim.ID, "pause"); err != nil {
		t.Fatal(err)
	}
	if err := s.repo.Finish(ctx, claim, "completed", "", s.now(), 1); !errors.Is(err, repository.ErrOutreachConflict) {
		t.Fatal("stale worker overwrote pause", err)
	}
	if _, err := s.repo.PrepareDelivery(ctx, claim, &model.CRMSequenceDelivery{ID: uuid.NewString(), Kind: "email", EnrollmentID: claim.ID}, s.now()); err == nil {
		t.Fatal("paused worker can send")
	}
}
func TestCRMOutreachTemplateRenderUsesCompanyAndGuardsOwnership(t *testing.T) {
	db, s, _ := outreachFixture(t)
	outreachTaskTables(t, db)
	ctx := context.Background()
	mustExecCRMEmailLifecycle(t, db, `CREATE TABLE crm_companies(id TEXT PRIMARY KEY,workspace_id TEXT,name TEXT)`)
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_companies VALUES('company','ws','Content & Studio')`)
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_associations(id,workspace_id,from_object_type,from_object_id,to_object_type,to_object_id) VALUES('a','ws','contact','contact','company','company')`)
	template, err := s.SaveTemplate(ctx, "ws", "owner", "", model.CRMEmailTemplate{Name: "Demo", Subject: "Hello {{first_name}}", BodyHTML: "<p>Thanks {{company}}</p>"})
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := s.RenderTemplate(ctx, "ws", "owner", template.ID, "amna@example.com", "mail", "")
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Subject != "Hello Amna" || rendered.BodyHTML != "<p>Thanks Content &amp; Studio</p>" {
		t.Fatal("incorrect personalization", rendered)
	}
	if _, err := s.RenderTemplate(ctx, "ws", "other", template.ID, "amna@example.com", "mail", ""); err == nil {
		t.Fatal("other owner rendered private template")
	}
}
func TestCRMOutreachActivityPaginationAndWorkspaceFilter(t *testing.T) {
	db, s, _ := outreachFixture(t)
	ctx := context.Background()
	seq := outreachSequence(t, s, "automatic")
	for i := 0; i < 55; i++ {
		row := model.CRMSequenceEnrollment{ID: uuid.NewString(), WorkspaceID: "ws", SequenceID: seq.ID, Email: uuid.NewString() + "@example.com", ContactName: "Amna", Status: "active", StepCount: 1, Steps: seq.Steps, UnsubscribeToken: uuid.NewString(), CreatedAt: s.now().Add(time.Duration(i) * time.Second)}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.Enrollments(ctx, "ws", "", "", "", model.CRMSequenceEnrollmentFilter{Page: 1, Search: "amna", Status: "active"})
	if err != nil || len(first) != 50 {
		t.Fatal("incorrect first page", len(first), err)
	}
	if first[0].Steps != nil || first[0].StepCount != 1 {
		t.Fatal("list includes email content or lacks count")
	}
	second, err := s.Enrollments(ctx, "ws", "", "", "", model.CRMSequenceEnrollmentFilter{Page: 2})
	if err != nil || len(second) != 5 {
		t.Fatal("incorrect second page", len(second), err)
	}
	other, err := s.Enrollments(ctx, "other", "", "", "", model.CRMSequenceEnrollmentFilter{Search: "amna"})
	if err != nil || len(other) != 0 {
		t.Fatal("search leaked another workspace")
	}
}
