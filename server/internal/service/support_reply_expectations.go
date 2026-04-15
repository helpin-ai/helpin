package service

import (
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ReplyExpectation is the resolved reply-time expectation for a given
// workspace + optional mailbox context. It is a pure value derived from
// settings — no I/O, no context. Reusable across handlers, widget config
// builders, and tests.
type ReplyExpectation struct {
	// Preset is one of the SupportReplyTimePreset* values.
	Preset string
	// CustomMinutes is set only when Preset == "custom". Zero otherwise.
	CustomMinutes int
	// Text is the human-facing copy produced by FormatReplyTimeCopy.
	Text string
	// FromMailboxID is set when the expectation came from a mailbox
	// override rather than the workspace default; it lets the widget
	// payload surface which mailbox's override was applied (debug / UX).
	FromMailboxID *string
}

// ResolveReplyExpectation returns the effective reply-time expectation
// for the given workspace settings and optional mailbox override. A nil
// mailbox (or a mailbox with a nil preset) falls back to the workspace
// default; an unrecognized preset on either layer falls back to
// "few_minutes" so the widget always has a sensible string.
func ResolveReplyExpectation(settings model.SupportInboxSettings, mailbox *model.SupportMailbox) ReplyExpectation {
	preset := settings.ReplyTimePreset
	custom := derefInt(settings.ReplyTimeCustomMinutes)
	var fromMailboxID *string

	if mailbox != nil && mailbox.ReplyTimePreset != nil && strings.TrimSpace(*mailbox.ReplyTimePreset) != "" {
		preset = strings.TrimSpace(*mailbox.ReplyTimePreset)
		custom = derefInt(mailbox.ReplyTimeCustomMinutes)
		id := mailbox.ID
		fromMailboxID = &id
	}

	if !model.IsValidSupportReplyTimePreset(preset) {
		preset = model.SupportReplyTimePresetFewMinutes
		custom = 0
		fromMailboxID = nil
	}
	if preset == model.SupportReplyTimePresetCustom && !model.IsValidSupportReplyTimeCustomMinutes(custom) {
		// Invalid custom value → fall back to few_minutes rather than
		// emitting garbage copy. The handler layer should have rejected
		// this write; this is belt-and-braces.
		preset = model.SupportReplyTimePresetFewMinutes
		custom = 0
	}

	return ReplyExpectation{
		Preset:        preset,
		CustomMinutes: custom,
		Text:          FormatReplyTimeCopy(preset, custom),
		FromMailboxID: fromMailboxID,
	}
}

// FormatReplyTimeCopy returns the widget-facing string for a given
// preset + optional custom minutes. Pure; used by the backend resolver,
// the TS preview helper (via a shared fixture), and tests.
//
// Bucketing for custom:
//   - < 60 min       → "Usually replies in about {N} minutes"
//   - 60..1440 min   → "Usually replies in about {N} hours"
//   - >= 1440 min    → "Usually replies within {N} days"
func FormatReplyTimeCopy(preset string, customMinutes int) string {
	switch preset {
	case model.SupportReplyTimePresetFewMinutes:
		return "Usually replies in a few minutes"
	case model.SupportReplyTimePresetFewHours:
		return "Usually replies in a few hours"
	case model.SupportReplyTimePresetSameDay:
		return "Usually replies within a day"
	case model.SupportReplyTimePresetCustom:
		return formatCustomReplyTimeCopy(customMinutes)
	}
	return "Usually replies in a few minutes"
}

func formatCustomReplyTimeCopy(minutes int) string {
	if minutes < 1 {
		return "Usually replies in a few minutes"
	}
	if minutes < 60 {
		return fmt.Sprintf("Usually replies in about %d %s", minutes, pluralize(minutes, "minute", "minutes"))
	}
	if minutes < 1440 {
		hours := minutes / 60
		// Round half-up when >= 30 minutes into the next hour so "90 min"
		// reads as "about 2 hours" rather than "about 1 hour" — the copy
		// says "about" explicitly, so a little rounding is expected.
		if minutes%60 >= 30 {
			hours += 1
		}
		return fmt.Sprintf("Usually replies in about %d %s", hours, pluralize(hours, "hour", "hours"))
	}
	days := minutes / 1440
	if minutes%1440 >= 720 {
		days += 1
	}
	return fmt.Sprintf("Usually replies within %d %s", days, pluralize(days, "day", "days"))
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
