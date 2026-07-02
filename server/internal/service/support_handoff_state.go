package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
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

// humanizeNextOpen renders a next-open time relative to localNow in the
// business-hours timezone: "today at 5:00 PM PST", "tomorrow at 9:00 AM PST",
// or "on Monday at 9:00 AM PST". Returns "" when next is nil.
func humanizeNextOpen(next *time.Time, localNow time.Time) string {
	if next == nil {
		return ""
	}
	t := next.In(localNow.Location())
	zone, _ := t.Zone()
	clock := t.Format("3:04 PM") + " " + zone
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
