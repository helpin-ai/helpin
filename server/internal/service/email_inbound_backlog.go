package service

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/observability"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Incoming email is acknowledged to Postmark as soon as it is stored, so a
// stalled inbound worker loses nothing yet stays invisible to the provider.
// The backlog monitor makes that wait visible through metrics, the admin
// diagnostics page, and alerts to Sentry and by email.
const (
	inboundBacklogCheckInterval = time.Minute
	inboundBacklogAlertAfter    = 10 * time.Minute
	inboundBacklogRealertEvery  = time.Hour
	inboundBacklogAlertMessage  = "Incoming support emails are waiting to be processed"
	// Every API replica runs a monitor. The sent key lets one of them email
	// each hourly alert; the open key lets one of them send the all-clear.
	inboundBacklogEmailSentKey = "email_inbound_backlog:alert_email_sent"
	inboundBacklogEmailOpenKey = "email_inbound_backlog:alert_email_open"
	inboundBacklogEmailOpenTTL = 7 * 24 * time.Hour
)

// inboundBacklogRecorder receives each backlog measurement.
type inboundBacklogRecorder interface {
	InboundEmailBacklog(waiting int64, oldestWaitingSeconds float64)
}

// inboundBacklogMailer sends the backlog alert emails.
type inboundBacklogMailer interface {
	SendEmail(to, subject, htmlBody, textBody string) error
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
// until ctx is cancelled. Alerts are also emailed to recipients when mailer
// is set.
func (s *EmailFallbackService) StartInboundBacklogMonitor(ctx context.Context, recorder inboundBacklogRecorder, mailer inboundBacklogMailer, recipients []string) {
	if mailer == nil && len(recipients) > 0 {
		s.logger.WarnContext(ctx, "inbound email backlog alerts will not be emailed: no application email sender is configured")
	}
	monitor := &inboundBacklogMonitor{service: s, recorder: recorder, alert: alertInboundBacklog, mailer: mailer, recipients: recipients}
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
// hourly while it stays delayed, so a long outage does not flood Sentry or
// inboxes. Email recipients also hear once when the delay clears.
type inboundBacklogMonitor struct {
	service     *EmailFallbackService
	recorder    inboundBacklogRecorder
	alert       func(backlog *model.InboundEmailBacklog)
	mailer      inboundBacklogMailer
	recipients  []string
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
			m.emailRecovered(ctx, backlog)
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
	m.emailDelayed(ctx, backlog)
}

// emailDelayed emails the alert unless another replica already did this hour.
// When Redis fails the email is sent anyway: a duplicate beats silence.
func (m *inboundBacklogMonitor) emailDelayed(ctx context.Context, backlog *model.InboundEmailBacklog) {
	if m.mailer == nil || len(m.recipients) == 0 {
		return
	}
	if rdb := m.service.redis; rdb != nil {
		// Expiring a check early lets the same replica's next hourly alert claim it.
		claimed, err := rdb.SetNX(ctx, inboundBacklogEmailSentKey, "1", inboundBacklogRealertEvery-inboundBacklogCheckInterval).Result()
		if err != nil {
			m.service.logger.ErrorContext(ctx, "claim inbound email backlog alert email", "error", err)
		} else if !claimed {
			return
		}
		if err := rdb.Set(ctx, inboundBacklogEmailOpenKey, "1", inboundBacklogEmailOpenTTL).Err(); err != nil {
			m.service.logger.ErrorContext(ctx, "mark inbound email backlog alert open", "error", err)
		}
	}
	subject, textBody := inboundBacklogDelayedEmail(m.service.appBaseURL, backlog)
	m.sendEmail(ctx, subject, textBody)
}

// emailRecovered sends the all-clear once, from the replica that closes the
// alert, and only after an alert email went out.
func (m *inboundBacklogMonitor) emailRecovered(ctx context.Context, backlog *model.InboundEmailBacklog) {
	if m.mailer == nil || len(m.recipients) == 0 {
		return
	}
	if rdb := m.service.redis; rdb != nil {
		closed, err := rdb.Del(ctx, inboundBacklogEmailOpenKey).Result()
		if err != nil {
			m.service.logger.ErrorContext(ctx, "close inbound email backlog alert", "error", err)
			return
		}
		if closed == 0 {
			return
		}
		// A new delay must alert at once, not wait out the hourly limit.
		if err := rdb.Del(ctx, inboundBacklogEmailSentKey).Err(); err != nil {
			m.service.logger.ErrorContext(ctx, "reset inbound email backlog alert email", "error", err)
		}
	}
	subject, textBody := inboundBacklogRecoveredEmail(m.service.appBaseURL, backlog)
	m.sendEmail(ctx, subject, textBody)
}

// sendEmail sends one email per recipient so recipients don't see each other.
func (m *inboundBacklogMonitor) sendEmail(ctx context.Context, subject, textBody string) {
	htmlBody := plainTextEmailHTML(textBody)
	for _, to := range m.recipients {
		if err := m.mailer.SendEmail(to, subject, htmlBody, textBody); err != nil {
			m.service.logger.ErrorContext(ctx, "send inbound email backlog alert", "error", err)
		}
	}
}

func alertInboundBacklog(backlog *model.InboundEmailBacklog) {
	observability.CaptureAlert(inboundBacklogAlertMessage, map[string]any{
		"waiting":                backlog.Waiting,
		"retrying":               backlog.Retrying,
		"oldest_waiting_minutes": backlog.OldestWaitingSeconds / 60,
	})
}

func inboundBacklogDelayedEmail(appBaseURL string, backlog *model.InboundEmailBacklog) (subject, textBody string) {
	site := inboundBacklogSite(appBaseURL)
	wait := formatInboundBacklogWait(backlog.OldestWaitingSeconds)
	subject = fmt.Sprintf("Incoming email stuck on %s: %d waiting, oldest %s", site, backlog.Waiting, wait)
	emails := "emails have"
	if backlog.Waiting == 1 {
		emails = "email has"
	}
	status := fmt.Sprintf("%d incoming support %s not reached a conversation on %s. The oldest has waited %s.", backlog.Waiting, emails, site, wait)
	if backlog.Retrying > 0 {
		status += fmt.Sprintf(" %d failed at least once and will be retried.", backlog.Retrying)
	}
	textBody = strings.Join([]string{
		status,
		"Postmark already accepted these emails, so it won't send them again. They are stored, and they'll be added to their conversations once processing resumes.",
		"What to check:\n- The API server logs for \"inbound processing failed\" or \"inbound worker failed\".\n- Email Diagnostics in the platform admin app, which shows the live count.",
		"While emails stay stuck, this alert repeats at most once an hour. You'll get one more email when they clear.",
	}, "\n\n")
	return subject, textBody
}

func inboundBacklogRecoveredEmail(appBaseURL string, backlog *model.InboundEmailBacklog) (subject, textBody string) {
	site := inboundBacklogSite(appBaseURL)
	subject = "Incoming email recovered on " + site
	waiting := "Every accepted email has reached a conversation."
	if backlog.Waiting > 0 {
		waiting = fmt.Sprintf("%d waiting right now, none for longer than %d minutes.", backlog.Waiting, int(inboundBacklogAlertAfter/time.Minute))
	}
	textBody = strings.Join([]string{
		fmt.Sprintf("Incoming support emails are reaching their conversations again on %s. %s", site, waiting),
		"Email Diagnostics in the platform admin app shows the live count.",
	}, "\n\n")
	return subject, textBody
}

// inboundBacklogSite names the installation in alerts, so staging and
// production alerts can be told apart.
func inboundBacklogSite(appBaseURL string) string {
	if parsed, err := url.Parse(strings.TrimSpace(appBaseURL)); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return "Helpin"
}

// formatInboundBacklogWait matches the admin diagnostics page: "14m", "2h 14m".
func formatInboundBacklogWait(seconds int64) string {
	minutes := max(seconds/60, 1)
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	if minutes%60 == 0 {
		return fmt.Sprintf("%dh", minutes/60)
	}
	return fmt.Sprintf("%dh %dm", minutes/60, minutes%60)
}

// plainTextEmailHTML renders paragraphs separated by blank lines, and lines
// starting with "- " as a list.
func plainTextEmailHTML(textBody string) string {
	var out strings.Builder
	for _, paragraph := range strings.Split(textBody, "\n\n") {
		var items []string
		var lines []string
		for _, line := range strings.Split(paragraph, "\n") {
			if item, ok := strings.CutPrefix(line, "- "); ok {
				items = append(items, item)
			} else {
				lines = append(lines, line)
			}
		}
		if len(lines) > 0 {
			out.WriteString("<p>" + html.EscapeString(strings.Join(lines, " ")) + "</p>")
		}
		if len(items) > 0 {
			out.WriteString("<ul>")
			for _, item := range items {
				out.WriteString("<li>" + html.EscapeString(item) + "</li>")
			}
			out.WriteString("</ul>")
		}
	}
	return out.String()
}
