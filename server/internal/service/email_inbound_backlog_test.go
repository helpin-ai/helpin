package service

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type recordedInboundBacklog struct {
	waiting int64
	oldest  float64
}

type fakeInboundBacklogRecorder struct{ calls []recordedInboundBacklog }

func (r *fakeInboundBacklogRecorder) InboundEmailBacklog(waiting int64, oldestWaitingSeconds float64) {
	r.calls = append(r.calls, recordedInboundBacklog{waiting: waiting, oldest: oldestWaitingSeconds})
}

func createInboundBacklogJob(t *testing.T, env *emailFallbackTestEnv, kind, status string, attempts int, createdAt time.Time, recoveryRequestedAt *time.Time) string {
	t.Helper()
	job := &model.SupportInboundJob{
		ID: uuid.NewString(), Kind: kind, Payload: "{}", Status: status, Attempts: attempts,
		AvailableAt: createdAt, RecoveryRequestedAt: recoveryRequestedAt, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	if err := env.convRepo.DB().Create(job).Error; err != nil {
		t.Fatalf("create inbound job: %v", err)
	}
	return job.ID
}

func TestInboundEmailBacklogCountsOnlyUnfinishedEmails(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

	createInboundBacklogJob(t, env, "email", "pending", 2, now.Add(-30*time.Minute), nil)
	createInboundBacklogJob(t, env, "email", "processing", 0, now.Add(-5*time.Minute), nil)
	createInboundBacklogJob(t, env, "email", "completed", 1, now.Add(-2*time.Hour), nil)
	createInboundBacklogJob(t, env, "email", "failed", 10, now.Add(-3*time.Hour), nil)
	createInboundBacklogJob(t, env, "attachment", "pending", 0, now.Add(-4*time.Hour), nil)
	// A recovered email waits from the recovery request, not its original receipt.
	recovered := now.Add(-time.Minute)
	createInboundBacklogJob(t, env, "email", "pending", 3, now.Add(-5*time.Hour), &recovered)

	backlog, err := env.service.InboundEmailBacklog(ctx, now)
	if err != nil {
		t.Fatalf("inbound backlog: %v", err)
	}
	if backlog.Waiting != 3 || backlog.Retrying != 2 {
		t.Fatalf("waiting/retrying = %d/%d, want 3/2", backlog.Waiting, backlog.Retrying)
	}
	if backlog.OldestWaitingSeconds != int64((30*time.Minute)/time.Second) {
		t.Fatalf("oldest waiting = %ds, want 1800s", backlog.OldestWaitingSeconds)
	}
	if !backlog.Delayed || backlog.AlertAfterSeconds != 600 {
		t.Fatalf("delayed=%v alert_after=%d, want true/600", backlog.Delayed, backlog.AlertAfterSeconds)
	}
}

func TestInboundEmailBacklogEmptyIsNotDelayed(t *testing.T) {
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	backlog, err := env.service.InboundEmailBacklog(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatalf("inbound backlog: %v", err)
	}
	if backlog.Waiting != 0 || backlog.OldestWaitingSince != nil || backlog.Delayed {
		t.Fatalf("unexpected empty backlog: %+v", backlog)
	}
}

func TestInboundBacklogMonitorAlertsOnceThenHourlyAndResetsAfterRecovery(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	base := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	jobID := createInboundBacklogJob(t, env, "email", "pending", 0, base, nil)

	recorder := &fakeInboundBacklogRecorder{}
	var alerts []model.InboundEmailBacklog
	monitor := &inboundBacklogMonitor{service: env.service, recorder: recorder, alert: func(backlog *model.InboundEmailBacklog) {
		alerts = append(alerts, *backlog)
	}}
	steps := []struct {
		at         time.Duration
		wantAlerts int
	}{
		{at: 5 * time.Minute, wantAlerts: 0},
		{at: 11 * time.Minute, wantAlerts: 1},
		{at: 12 * time.Minute, wantAlerts: 1},
		{at: 70 * time.Minute, wantAlerts: 1},
		{at: 72 * time.Minute, wantAlerts: 2},
	}
	for _, step := range steps {
		monitor.check(ctx, base.Add(step.at))
		if len(alerts) != step.wantAlerts {
			t.Fatalf("after %s: %d alerts, want %d", step.at, len(alerts), step.wantAlerts)
		}
	}
	if got := recorder.calls[0]; got.waiting != 1 || got.oldest != 300 {
		t.Fatalf("first recorded backlog = %+v, want 1 waiting for 300s", got)
	}
	if alerts[0].Waiting != 1 || alerts[0].OldestWaitingSeconds != 660 {
		t.Fatalf("first alert = %+v", alerts[0])
	}

	if err := env.convRepo.DB().Model(&model.SupportInboundJob{}).Where("id = ?", jobID).Update("status", "completed").Error; err != nil {
		t.Fatal(err)
	}
	monitor.check(ctx, base.Add(73*time.Minute))
	if monitor.alerting {
		t.Fatal("monitor still alerting after the backlog cleared")
	}
	if got := recorder.calls[len(recorder.calls)-1]; got.waiting != 0 || got.oldest != 0 {
		t.Fatalf("cleared backlog recorded as %+v", got)
	}

	// A new delay alerts straight away rather than waiting for the hourly reminder.
	createInboundBacklogJob(t, env, "email", "pending", 0, base.Add(74*time.Minute), nil)
	monitor.check(ctx, base.Add(90*time.Minute))
	if len(alerts) != 3 {
		t.Fatalf("new delay after recovery: %d alerts, want 3", len(alerts))
	}
}

type sentBacklogEmail struct{ to, subject, htmlBody, textBody string }

type fakeInboundBacklogMailer struct{ sent []sentBacklogEmail }

func (m *fakeInboundBacklogMailer) SendEmail(to, subject, htmlBody, textBody string) error {
	m.sent = append(m.sent, sentBacklogEmail{to: to, subject: subject, htmlBody: htmlBody, textBody: textBody})
	return nil
}

func (m *fakeInboundBacklogMailer) subjects() []string {
	var subjects []string
	for _, email := range m.sent {
		subjects = append(subjects, email.to+": "+email.subject)
	}
	return subjects
}

func newEmailingBacklogMonitor(env *emailFallbackTestEnv, mailer *fakeInboundBacklogMailer) *inboundBacklogMonitor {
	return &inboundBacklogMonitor{service: env.service, mailer: mailer, recipients: []string{"ops@example.com", "lead@example.com"}}
}

func TestInboundBacklogMonitorEmailsAlertHourlyThenAllClear(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackTestEnv(t, model.DefaultSupportInboxSettings())
	env.service.appBaseURL = "https://app.example.com"
	base := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	jobID := createInboundBacklogJob(t, env, "email", "pending", 2, base, nil)
	mailer := &fakeInboundBacklogMailer{}
	monitor := newEmailingBacklogMonitor(env, mailer)
	// Redis expiry follows its own clock, so it moves with the checks.
	var elapsed time.Duration
	checkAt := func(at time.Duration) {
		env.redisServer.FastForward(at - elapsed)
		elapsed = at
		monitor.check(ctx, base.Add(at))
	}

	checkAt(5 * time.Minute)
	if len(mailer.sent) != 0 {
		t.Fatalf("emailed before the delay: %v", mailer.subjects())
	}
	checkAt(11 * time.Minute)
	want := []string{
		"ops@example.com: Incoming email stuck on app.example.com: 1 waiting, oldest 11m",
		"lead@example.com: Incoming email stuck on app.example.com: 1 waiting, oldest 11m",
	}
	if got := mailer.subjects(); !slices.Equal(got, want) {
		t.Fatalf("first alert emails = %q, want %q", got, want)
	}
	alert := mailer.sent[0]
	for _, part := range []string{"1 incoming support email has not reached a conversation on app.example.com", "1 failed at least once", "won't send them again", "at most once an hour"} {
		if !strings.Contains(alert.textBody, part) {
			t.Errorf("alert text missing %q:\n%s", part, alert.textBody)
		}
	}
	if !strings.Contains(alert.htmlBody, "<li>Email Diagnostics in the platform admin app, which shows the live count.</li>") {
		t.Errorf("alert html lacks the checklist: %s", alert.htmlBody)
	}

	checkAt(12 * time.Minute)
	if len(mailer.sent) != 2 {
		t.Fatalf("emailed again within the hour: %v", mailer.subjects())
	}
	checkAt(72 * time.Minute)
	if len(mailer.sent) != 4 || !strings.HasSuffix(mailer.sent[3].subject, "oldest 1h 12m") {
		t.Fatalf("hourly reminder = %v", mailer.subjects())
	}

	if err := env.convRepo.DB().Model(&model.SupportInboundJob{}).Where("id = ?", jobID).Update("status", "completed").Error; err != nil {
		t.Fatal(err)
	}
	checkAt(73 * time.Minute)
	if len(mailer.sent) != 6 || mailer.sent[5].subject != "Incoming email recovered on app.example.com" {
		t.Fatalf("all-clear = %v", mailer.subjects())
	}
	if !strings.Contains(mailer.sent[5].textBody, "Every accepted email has reached a conversation.") {
		t.Errorf("all-clear text = %q", mailer.sent[5].textBody)
	}
	checkAt(74 * time.Minute)
	if len(mailer.sent) != 6 {
		t.Fatalf("all-clear repeated: %v", mailer.subjects())
	}

	// A new delay soon after alerts at once instead of waiting out the hour.
	createInboundBacklogJob(t, env, "email", "pending", 0, base.Add(75*time.Minute), nil)
	checkAt(86 * time.Minute)
	if len(mailer.sent) != 8 {
		t.Fatalf("new delay after recovery: %v", mailer.subjects())
	}
}

func TestInboundBacklogMonitorEmailsOnceAcrossReplicas(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackTestEnv(t, model.DefaultSupportInboxSettings())
	base := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	jobID := createInboundBacklogJob(t, env, "email", "pending", 0, base, nil)
	mailer := &fakeInboundBacklogMailer{}
	first := newEmailingBacklogMonitor(env, mailer)
	second := newEmailingBacklogMonitor(env, mailer)

	first.check(ctx, base.Add(11*time.Minute))
	second.check(ctx, base.Add(11*time.Minute+20*time.Second))
	if len(mailer.sent) != 2 {
		t.Fatalf("two replicas sent %v, want one alert per recipient", mailer.subjects())
	}
	// A replica that restarts mid-outage does not resend the alert.
	restarted := newEmailingBacklogMonitor(env, mailer)
	restarted.check(ctx, base.Add(30*time.Minute))
	if len(mailer.sent) != 2 {
		t.Fatalf("restarted replica resent the alert: %v", mailer.subjects())
	}

	if err := env.convRepo.DB().Model(&model.SupportInboundJob{}).Where("id = ?", jobID).Update("status", "completed").Error; err != nil {
		t.Fatal(err)
	}
	first.check(ctx, base.Add(31*time.Minute))
	second.check(ctx, base.Add(31*time.Minute))
	restarted.check(ctx, base.Add(31*time.Minute))
	if len(mailer.sent) != 4 || !strings.HasPrefix(mailer.sent[2].subject, "Incoming email recovered") {
		t.Fatalf("all-clear across replicas = %v, want one per recipient", mailer.subjects())
	}
}

func TestInboundBacklogMonitorEmailsWithoutRedis(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	base := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	jobID := createInboundBacklogJob(t, env, "email", "pending", 0, base, nil)
	mailer := &fakeInboundBacklogMailer{}
	monitor := newEmailingBacklogMonitor(env, mailer)

	monitor.check(ctx, base.Add(11*time.Minute))
	if err := env.convRepo.DB().Model(&model.SupportInboundJob{}).Where("id = ?", jobID).Update("status", "completed").Error; err != nil {
		t.Fatal(err)
	}
	monitor.check(ctx, base.Add(12*time.Minute))
	if len(mailer.sent) != 4 || !strings.HasPrefix(mailer.sent[3].subject, "Incoming email recovered on ") {
		t.Fatalf("emails without Redis = %v", mailer.subjects())
	}
}

func TestInboundBacklogMonitorWithoutRecipientsSendsNothing(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackTestEnv(t, model.DefaultSupportInboxSettings())
	base := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	createInboundBacklogJob(t, env, "email", "pending", 0, base, nil)
	mailer := &fakeInboundBacklogMailer{}
	monitor := &inboundBacklogMonitor{service: env.service, mailer: mailer}
	monitor.check(ctx, base.Add(11*time.Minute))
	noMailer := &inboundBacklogMonitor{service: env.service, recipients: []string{"ops@example.com"}}
	noMailer.check(ctx, base.Add(11*time.Minute))
	if len(mailer.sent) != 0 || !monitor.alerting || !noMailer.alerting {
		t.Fatalf("sent=%v alerting=%v/%v", mailer.subjects(), monitor.alerting, noMailer.alerting)
	}
	if env.redisServer.Exists(inboundBacklogEmailSentKey) {
		t.Fatal("claimed an alert email that was never sent")
	}
}

func TestInboundBacklogRecoveredEmailCountsRemainingEmails(t *testing.T) {
	_, text := inboundBacklogRecoveredEmail("", &model.InboundEmailBacklog{Waiting: 3})
	if !strings.Contains(text, "3 waiting right now, none for longer than 10 minutes.") {
		t.Fatalf("recovered text = %q", text)
	}
}

func TestFormatInboundBacklogWait(t *testing.T) {
	tests := []struct {
		seconds int64
		want    string
	}{
		{seconds: 0, want: "1m"},
		{seconds: 660, want: "11m"},
		{seconds: 3600, want: "1h"},
		{seconds: 8040, want: "2h 14m"},
		{seconds: 2 * 24 * 3600, want: "48h"},
	}
	for _, tt := range tests {
		if got := formatInboundBacklogWait(tt.seconds); got != tt.want {
			t.Errorf("formatInboundBacklogWait(%d) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

func TestPlainTextEmailHTMLEscapesAndLists(t *testing.T) {
	got := plainTextEmailHTML("A <b> & \"c\".\n\nCheck:\n- one\n- <two>")
	want := `<p>A &lt;b&gt; &amp; &#34;c&#34;.</p><p>Check:</p><ul><li>one</li><li>&lt;two&gt;</li></ul>`
	if got != want {
		t.Fatalf("plainTextEmailHTML = %s, want %s", got, want)
	}
}
