package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// resolveHandoffState maps availability at handoff time to a customer-facing state.
// Presence trumps business hours: an available recipient always yields "live".
func resolveHandoffState(hasAvailableRecipient bool, withinOfficeHours bool) string {
	if hasAvailableRecipient {
		return model.HandoffStateLive
	}
	if withinOfficeHours {
		return model.HandoffStateBusy
	}
	return model.HandoffStateAfterHours
}

const escalationFallbackTime = "as soon as possible"

// renderEscalationMessage substitutes {reply_time} and {next_open}. Empty
// values degrade to "as soon as possible". Unknown tokens are left literal.
func renderEscalationMessage(template, replyTime, nextOpen string) string {
	if strings.TrimSpace(replyTime) == "" {
		replyTime = escalationFallbackTime
	}
	if strings.TrimSpace(nextOpen) == "" {
		nextOpen = escalationFallbackTime
	}
	out := strings.ReplaceAll(template, "{reply_time}", replyTime)
	out = strings.ReplaceAll(out, "{next_open}", nextOpen)
	return out
}

// shortReplyTimePhrase converts the reply-time expectation into a short
// phrase for the {reply_time} token ("a few minutes", "about 45 minutes").
// Empty result degrades to the fallback inside renderEscalationMessage.
func shortReplyTimePhrase(preset string, minutes *int) string {
	switch preset {
	case "few_minutes":
		return "a few minutes"
	case "few_hours":
		return "a few hours"
	case "same_day":
		return "a day"
	case "custom":
		if minutes != nil && *minutes > 0 {
			return fmt.Sprintf("about %d minutes", *minutes)
		}
	}
	return ""
}

// selectEscalationTemplate picks the escalation message template for the
// resolved handoff state, falling back to the built-in default for that state
// (never to the live message) when the workspace has not customized it.
func selectEscalationTemplate(s model.SupportInboxSettings, state string) string {
	defaults := model.DefaultSupportInboxSettings()
	pick := func(custom, def string) string {
		if strings.TrimSpace(custom) != "" {
			return custom
		}
		return def
	}
	switch state {
	case model.HandoffStateBusy:
		return pick(s.EscalationMessageBusy, defaults.EscalationMessageBusy)
	case model.HandoffStateAfterHours:
		return pick(s.EscalationMessageAfterHours, defaults.EscalationMessageAfterHours)
	default: // live
		return pick(s.EscalationMessage, defaults.EscalationMessage)
	}
}

// selectEmailHandoffTemplate picks the email handoff message by business hours
// alone, falling back to the built-in message when the workspace has none.
func selectEmailHandoffTemplate(s model.SupportInboxSettings, withinOfficeHours bool) string {
	defaults := model.DefaultSupportInboxSettings()
	custom, fallback := s.EmailHandoffMessageAfterHours, defaults.EmailHandoffMessageAfterHours
	if withinOfficeHours {
		custom, fallback = s.EmailHandoffMessage, defaults.EmailHandoffMessage
	}
	if strings.TrimSpace(custom) != "" {
		return custom
	}
	return fallback
}

// supportNextOpenForCustomer says when business hours start again. It uses the
// customer's own clock when their widget reported a timezone, and otherwise
// the business-hours timezone with its zone name.
func supportNextOpenForCustomer(ctx context.Context, sessionRepo *repository.SupportInboxSessionRepository, conv *model.SupportConversation, settings model.SupportInboxSettings, now time.Time) string {
	loc, err := time.LoadLocation(settings.BusinessHoursTimezone)
	if err != nil {
		return ""
	}
	nextOpen := nextBusinessHoursStart(settings, now.In(loc))
	if customerLoc := supportCustomerLocation(ctx, sessionRepo, conv); customerLoc != nil {
		return humanizeNextOpenForCustomer(nextOpen, now.In(customerLoc))
	}
	return humanizeNextOpen(nextOpen, now.In(loc))
}

// supportCustomerLocation returns the timezone the customer's widget reported, or nil when it is unknown.
func supportCustomerLocation(ctx context.Context, sessionRepo *repository.SupportInboxSessionRepository, conv *model.SupportConversation) *time.Location {
	if sessionRepo == nil || conv == nil {
		return nil
	}
	session, err := sessionRepo.GetLatestByConversation(ctx, conv.WorkspaceID, conv.ID)
	if err != nil {
		slog.WarnContext(ctx, "load widget session for customer timezone", "error", err, "workspace_id", conv.WorkspaceID, "conversation_id", conv.ID)
		return nil
	}
	if session == nil || session.Timezone == nil || strings.TrimSpace(*session.Timezone) == "" {
		return nil
	}
	loc, err := time.LoadLocation(strings.TrimSpace(*session.Timezone))
	if err != nil {
		return nil
	}
	return loc
}

// humanizeNextOpen renders a next-open time relative to localNow in the
// business-hours timezone: "today at 5:00 PM PST", "tomorrow at 9:00 AM PST",
// or "on Monday at 9:00 AM PST". Returns "" when next is nil.
func humanizeNextOpen(next *time.Time, localNow time.Time) string {
	return humanizeNextOpenClock(next, localNow, true)
}

// humanizeNextOpenForCustomer renders a next-open time on the customer's own
// clock, so no zone name is shown: "tomorrow at 6:30 PM".
func humanizeNextOpenForCustomer(next *time.Time, customerNow time.Time) string {
	return humanizeNextOpenClock(next, customerNow, false)
}

func humanizeNextOpenClock(next *time.Time, localNow time.Time, showZone bool) string {
	if next == nil {
		return ""
	}
	t := next.In(localNow.Location())
	clock := t.Format("3:04 PM")
	if showZone {
		zone, _ := t.Zone()
		clock += " " + zone
	}
	startOfDay := func(x time.Time) time.Time {
		y, m, d := x.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, x.Location())
	}
	switch days := int(startOfDay(t).Sub(startOfDay(localNow)).Hours() / 24); {
	case days <= 0:
		return "today at " + clock
	case days == 1:
		return "tomorrow at " + clock
	default:
		return "on " + t.Format("Monday") + " at " + clock
	}
}
