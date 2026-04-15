package model

// SupportReplyTimePreset enumerates the valid values for the
// reply_time_preset column on support_inbox_settings and
// support_mailboxes. Phase 1 ships preset-based expectations; a future
// "dynamic" mode (computed median from real conversations) will extend
// this list.
//
// Source of truth for the widget, admin settings UI, and the preview
// helper on the frontend — all consumers import the matching shared TS
// union to stay in lockstep.
type SupportReplyTimePreset = string

const (
	// SupportReplyTimePresetFewMinutes renders "Usually replies in a few minutes".
	SupportReplyTimePresetFewMinutes SupportReplyTimePreset = "few_minutes"
	// SupportReplyTimePresetFewHours renders "Usually replies in a few hours".
	SupportReplyTimePresetFewHours SupportReplyTimePreset = "few_hours"
	// SupportReplyTimePresetSameDay renders "Usually replies within a day".
	SupportReplyTimePresetSameDay SupportReplyTimePreset = "same_day"
	// SupportReplyTimePresetCustom requires ReplyTimeCustomMinutes to be set
	// and renders a formatted copy based on the magnitude.
	SupportReplyTimePresetCustom SupportReplyTimePreset = "custom"
)

// SupportReplyTimeCustomMinutesMin / Max bound the allowed range for the
// custom-minutes value. Upper bound is 7 days — beyond that, "usually
// replies" stops being a meaningful expectation.
const (
	SupportReplyTimeCustomMinutesMin = 1
	SupportReplyTimeCustomMinutesMax = 10080
)

// SupportSpecialNoticeMaxLength caps the length of the outage / notice
// banner. Picked to fit a two-line banner without scrolling on mobile.
const SupportSpecialNoticeMaxLength = 500

// supportReplyTimePresets is the authoritative set of valid preset values.
var supportReplyTimePresets = map[SupportReplyTimePreset]struct{}{
	SupportReplyTimePresetFewMinutes: {},
	SupportReplyTimePresetFewHours:   {},
	SupportReplyTimePresetSameDay:    {},
	SupportReplyTimePresetCustom:     {},
}

// IsValidSupportReplyTimePreset reports whether s is a recognized preset.
func IsValidSupportReplyTimePreset(s string) bool {
	_, ok := supportReplyTimePresets[s]
	return ok
}

// IsValidSupportReplyTimeCustomMinutes reports whether n is in the
// accepted range.
func IsValidSupportReplyTimeCustomMinutes(n int) bool {
	return n >= SupportReplyTimeCustomMinutesMin && n <= SupportReplyTimeCustomMinutesMax
}
