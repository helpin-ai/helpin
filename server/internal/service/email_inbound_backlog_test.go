package service

import (
	"context"
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
