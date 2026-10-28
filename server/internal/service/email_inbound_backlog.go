package service

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/observability"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Incoming email is acknowledged to Postmark as soon as it is stored, so a
// stalled inbound worker loses nothing yet stays invisible to the provider.
// The backlog monitor makes that wait visible through metrics, the admin
// diagnostics page, and an alert.
const (
	inboundBacklogCheckInterval = time.Minute
	inboundBacklogAlertAfter    = 10 * time.Minute
	inboundBacklogRealertEvery  = time.Hour
	inboundBacklogAlertMessage  = "Incoming support emails are waiting to be processed"
)

// inboundBacklogRecorder receives each backlog measurement.
type inboundBacklogRecorder interface {
	InboundEmailBacklog(waiting int64, oldestWaitingSeconds float64)
}

// InboundEmailBacklog reports incoming emails that were accepted from the
// provider but have not been processed into conversations yet.
func (s *EmailFallbackService) InboundEmailBacklog(ctx context.Context, now time.Time) (*model.InboundEmailBacklog, error) {
	backlog := &model.InboundEmailBacklog{AlertAfterSeconds: int64(inboundBacklogAlertAfter / time.Second)}
	if s == nil || s.convRepo == nil {
		return backlog, nil
	}
	summary, err := repository.NewSupportInboundJobRepository(s.convRepo.DB()).WaitingSummary(ctx, "email")
	if err != nil {
		return nil, err
	}
	backlog.Waiting = summary.Waiting
	backlog.Retrying = summary.Retrying
	if summary.OldestSince != nil {
		age := max(now.Sub(*summary.OldestSince), 0)
		backlog.OldestWaitingSince = summary.OldestSince
		backlog.OldestWaitingSeconds = int64(age / time.Second)
		backlog.Delayed = age > inboundBacklogAlertAfter
	}
	return backlog, nil
}

// StartInboundBacklogMonitor checks the incoming email backlog every minute
// until ctx is cancelled.
func (s *EmailFallbackService) StartInboundBacklogMonitor(ctx context.Context, recorder inboundBacklogRecorder) {
	monitor := &inboundBacklogMonitor{service: s, recorder: recorder, alert: alertInboundBacklog}
	ticker := time.NewTicker(inboundBacklogCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			monitor.check(ctx, time.Now().UTC())
		}
	}
}

// inboundBacklogMonitor alerts when the backlog becomes delayed, then at most
// hourly while it stays delayed, so a long outage does not flood Sentry.
type inboundBacklogMonitor struct {
	service     *EmailFallbackService
	recorder    inboundBacklogRecorder
	alert       func(backlog *model.InboundEmailBacklog)
	alerting    bool
	lastAlertAt time.Time
}

func (m *inboundBacklogMonitor) check(ctx context.Context, now time.Time) {
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	backlog, err := m.service.InboundEmailBacklog(checkCtx, now)
	if err != nil {
		m.service.logger.ErrorContext(ctx, "inbound email backlog check failed", "error", err)
		return
	}
	if m.recorder != nil {
		m.recorder.InboundEmailBacklog(backlog.Waiting, float64(backlog.OldestWaitingSeconds))
	}
	if !backlog.Delayed {
		if m.alerting {
			m.service.logger.InfoContext(ctx, "inbound email backlog recovered", "waiting", backlog.Waiting)
		}
		m.alerting = false
		return
	}
	if m.alerting && now.Sub(m.lastAlertAt) < inboundBacklogRealertEvery {
		return
	}
	m.alerting = true
	m.lastAlertAt = now
	m.service.logger.ErrorContext(ctx, "inbound email backlog delayed",
		"waiting", backlog.Waiting,
		"retrying", backlog.Retrying,
		"oldest_waiting_seconds", backlog.OldestWaitingSeconds,
	)
	if m.alert != nil {
		m.alert(backlog)
	}
}

func alertInboundBacklog(backlog *model.InboundEmailBacklog) {
	observability.CaptureAlert(inboundBacklogAlertMessage, map[string]any{
		"waiting":                backlog.Waiting,
		"retrying":               backlog.Retrying,
		"oldest_waiting_minutes": backlog.OldestWaitingSeconds / 60,
	})
}
