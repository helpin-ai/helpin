package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// replyTimeCase mirrors the structure checked into the shared fixture so
// the TS preview helper can assert byte-for-byte identical copy.
type replyTimeCase struct {
	Name          string `json:"name"`
	Preset        string `json:"preset"`
	CustomMinutes int    `json:"custom_minutes"`
	Expected      string `json:"expected"`
}

func replyTimeCases() []replyTimeCase {
	return []replyTimeCase{
		// Built-in presets
		{Name: "few_minutes", Preset: model.SupportReplyTimePresetFewMinutes, CustomMinutes: 0, Expected: "Usually replies in a few minutes"},
		{Name: "few_hours", Preset: model.SupportReplyTimePresetFewHours, CustomMinutes: 0, Expected: "Usually replies in a few hours"},
		{Name: "same_day", Preset: model.SupportReplyTimePresetSameDay, CustomMinutes: 0, Expected: "Usually replies within a day"},

		// Custom: minutes bucket
		{Name: "custom_1min", Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 1, Expected: "Usually replies in about 1 minute"},
		{Name: "custom_15min", Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 15, Expected: "Usually replies in about 15 minutes"},
		{Name: "custom_59min", Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 59, Expected: "Usually replies in about 59 minutes"},

		// Custom: hours bucket (with rounding)
		{Name: "custom_60min_edge", Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 60, Expected: "Usually replies in about 1 hour"},
		{Name: "custom_89min_rounds_down", Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 89, Expected: "Usually replies in about 1 hour"},
		{Name: "custom_90min_rounds_up", Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 90, Expected: "Usually replies in about 2 hours"},
		{Name: "custom_180min_exact", Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 180, Expected: "Usually replies in about 3 hours"},
		{Name: "custom_1439min_max_hours", Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 1439, Expected: "Usually replies in about 24 hours"},

		// Custom: days bucket
		{Name: "custom_1440min_edge", Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 1440, Expected: "Usually replies within 1 day"},
		{Name: "custom_2880min_exact", Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 2880, Expected: "Usually replies within 2 days"},
		{Name: "custom_3600min_rounds_up", Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 3600, Expected: "Usually replies within 3 days"},
		{Name: "custom_10080min_max", Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 10080, Expected: "Usually replies within 7 days"},

		// Custom defensive edges: invalid values fall back to few_minutes
		{Name: "custom_0_min_fallback", Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 0, Expected: "Usually replies in a few minutes"},

		// Unrecognized preset falls back
		{Name: "unknown_preset_fallback", Preset: "not_a_preset", CustomMinutes: 0, Expected: "Usually replies in a few minutes"},
	}
}

func TestFormatReplyTimeCopy(t *testing.T) {
	for _, tc := range replyTimeCases() {
		t.Run(tc.Name, func(t *testing.T) {
			got := FormatReplyTimeCopy(tc.Preset, tc.CustomMinutes)
			if got != tc.Expected {
				t.Fatalf("FormatReplyTimeCopy(%q, %d) = %q, want %q", tc.Preset, tc.CustomMinutes, got, tc.Expected)
			}
		})
	}
}

// TestReplyTimeFixtureIsUpToDate writes the canonical fixture that the
// frontend preview-parity test imports. Regenerate by running:
//
//	GENERATE_REPLY_TIME_FIXTURE=1 go test ./internal/service/ -run TestReplyTimeFixtureIsUpToDate
//
// When GENERATE_REPLY_TIME_FIXTURE is unset, the test asserts the
// checked-in fixture matches the current cases — keeping TS and Go
// byte-for-byte aligned.
func TestReplyTimeFixtureIsUpToDate(t *testing.T) {
	payload, err := json.MarshalIndent(replyTimeCases(), "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	payload = append(payload, '\n')

	path := filepath.Join("..", "..", "..", "packages", "shared", "test-data", "reply-time-cases.json")

	if os.Getenv("GENERATE_REPLY_TIME_FIXTURE") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir fixture dir: %v", err)
		}
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		t.Logf("wrote fixture to %s", path)
		return
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v (run with GENERATE_REPLY_TIME_FIXTURE=1 to create)", path, err)
	}
	if string(existing) != string(payload) {
		t.Fatalf("reply-time fixture drift — regenerate with GENERATE_REPLY_TIME_FIXTURE=1 go test ./internal/service/ -run TestReplyTimeFixtureIsUpToDate")
	}
}

func TestResolveReplyExpectation_WorkspaceDefault(t *testing.T) {
	settings := model.SupportInboxSettings{
		ReplyTimePreset: model.SupportReplyTimePresetFewHours,
	}
	got := ResolveReplyExpectation(settings, nil)
	if got.Preset != model.SupportReplyTimePresetFewHours {
		t.Fatalf("Preset = %q, want few_hours", got.Preset)
	}
	if got.FromMailboxID != nil {
		t.Fatalf("FromMailboxID = %v, want nil", got.FromMailboxID)
	}
	if got.Text != "Usually replies in a few hours" {
		t.Fatalf("Text = %q", got.Text)
	}
}

func TestResolveReplyExpectation_MailboxOverridesWorkspace(t *testing.T) {
	custom := 45
	mailboxPreset := model.SupportReplyTimePresetCustom
	mailbox := &model.SupportMailbox{
		ID:                     "mb-1",
		ReplyTimePreset:        &mailboxPreset,
		ReplyTimeCustomMinutes: &custom,
	}
	settings := model.SupportInboxSettings{
		ReplyTimePreset: model.SupportReplyTimePresetFewMinutes,
	}
	got := ResolveReplyExpectation(settings, mailbox)
	if got.Preset != model.SupportReplyTimePresetCustom {
		t.Fatalf("Preset = %q, want custom", got.Preset)
	}
	if got.CustomMinutes != 45 {
		t.Fatalf("CustomMinutes = %d, want 45", got.CustomMinutes)
	}
	if got.FromMailboxID == nil || *got.FromMailboxID != "mb-1" {
		t.Fatalf("FromMailboxID = %v, want &mb-1", got.FromMailboxID)
	}
	if got.Text != "Usually replies in about 45 minutes" {
		t.Fatalf("Text = %q", got.Text)
	}
}

func TestResolveReplyExpectation_NilMailboxPresetInheritsWorkspace(t *testing.T) {
	mailbox := &model.SupportMailbox{ID: "mb-1"} // no override
	settings := model.SupportInboxSettings{
		ReplyTimePreset: model.SupportReplyTimePresetSameDay,
	}
	got := ResolveReplyExpectation(settings, mailbox)
	if got.Preset != model.SupportReplyTimePresetSameDay {
		t.Fatalf("Preset = %q, want same_day", got.Preset)
	}
	if got.FromMailboxID != nil {
		t.Fatalf("FromMailboxID should be nil when mailbox preset is nil, got %v", got.FromMailboxID)
	}
}

func TestResolveReplyExpectation_InvalidCustomFallsBack(t *testing.T) {
	zero := 0
	bogusPreset := model.SupportReplyTimePresetCustom
	mailbox := &model.SupportMailbox{
		ID:                     "mb-1",
		ReplyTimePreset:        &bogusPreset,
		ReplyTimeCustomMinutes: &zero, // invalid: below min
	}
	settings := model.DefaultSupportInboxSettings()
	got := ResolveReplyExpectation(settings, mailbox)
	if got.Preset != model.SupportReplyTimePresetFewMinutes {
		t.Fatalf("invalid custom should fall back to few_minutes, got %q", got.Preset)
	}
}

func TestResolveReplyExpectation_UnknownPresetFallsBack(t *testing.T) {
	settings := model.SupportInboxSettings{
		ReplyTimePreset: "garbage",
	}
	got := ResolveReplyExpectation(settings, nil)
	if got.Preset != model.SupportReplyTimePresetFewMinutes {
		t.Fatalf("unknown preset should fall back, got %q", got.Preset)
	}
}

func TestOptionalMinutes_OnlyForCustom(t *testing.T) {
	if optionalMinutes(ReplyExpectation{Preset: model.SupportReplyTimePresetFewHours, CustomMinutes: 120}) != nil {
		t.Fatalf("non-custom preset must not surface minutes")
	}
	p := optionalMinutes(ReplyExpectation{Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 45})
	if p == nil || *p != 45 {
		t.Fatalf("custom preset should surface minutes, got %v", p)
	}
	if optionalMinutes(ReplyExpectation{Preset: model.SupportReplyTimePresetCustom, CustomMinutes: 0}) != nil {
		t.Fatalf("zero minutes should not surface")
	}
}

func TestIsValidSupportReplyTimePreset(t *testing.T) {
	valid := []string{"few_minutes", "few_hours", "same_day", "custom"}
	for _, v := range valid {
		if !model.IsValidSupportReplyTimePreset(v) {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []string{"", " ", "FEW_MINUTES", "quick", "seconds", "never"}
	for _, v := range invalid {
		if model.IsValidSupportReplyTimePreset(v) {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestIsValidSupportReplyTimeCustomMinutes(t *testing.T) {
	valid := []int{1, 2, 59, 60, 1439, 1440, 10080}
	for _, n := range valid {
		if !model.IsValidSupportReplyTimeCustomMinutes(n) {
			t.Errorf("expected %d to be valid", n)
		}
	}
	invalid := []int{0, -1, 10081, 99999}
	for _, n := range invalid {
		if model.IsValidSupportReplyTimeCustomMinutes(n) {
			t.Errorf("expected %d to be invalid", n)
		}
	}
}

func TestSupportAvailabilitySnapshot_PopulatesStructuredFields(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	settings.ReplyTimePreset = model.SupportReplyTimePresetCustom
	n := 120
	settings.ReplyTimeCustomMinutes = &n
	notice := "Backlog today."
	settings.SpecialNoticeText = &notice

	snap := resolveSupportAvailability(settings, time.Now())
	avail := snap.WidgetAvailability
	if avail.ReplyTimePreset != model.SupportReplyTimePresetCustom {
		t.Errorf("ReplyTimePreset = %q", avail.ReplyTimePreset)
	}
	if avail.ReplyTimeMinutes == nil || *avail.ReplyTimeMinutes != 120 {
		t.Errorf("ReplyTimeMinutes = %v, want &120", avail.ReplyTimeMinutes)
	}
	if avail.SpecialNoticeText == nil || *avail.SpecialNoticeText != "Backlog today." {
		t.Errorf("SpecialNoticeText = %v", avail.SpecialNoticeText)
	}
	if avail.ReplyTimeText != "Usually replies in about 2 hours" {
		t.Errorf("ReplyTimeText = %q", avail.ReplyTimeText)
	}
	if avail.MailboxID != nil {
		t.Errorf("MailboxID should be nil without override, got %v", avail.MailboxID)
	}
}

func TestSupportAvailabilitySnapshot_MailboxOverrideSetsMailboxID(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	settings.ReplyTimePreset = model.SupportReplyTimePresetFewMinutes
	override := model.SupportReplyTimePresetFewHours
	mailbox := &model.SupportMailbox{
		ID:              "mb-eng",
		ReplyTimePreset: &override,
	}
	snap := resolveSupportAvailabilityForMailbox(settings, mailbox, time.Now())
	if snap.WidgetAvailability.MailboxID == nil || *snap.WidgetAvailability.MailboxID != "mb-eng" {
		t.Fatalf("MailboxID = %v, want &mb-eng", snap.WidgetAvailability.MailboxID)
	}
	if snap.WidgetAvailability.ReplyTimePreset != model.SupportReplyTimePresetFewHours {
		t.Fatalf("preset should be mailbox's few_hours, got %q", snap.WidgetAvailability.ReplyTimePreset)
	}
}

func TestNormalizedSpecialNotice(t *testing.T) {
	if normalizedSpecialNotice(nil) != nil {
		t.Fatalf("nil in → nil out")
	}
	empty := ""
	if normalizedSpecialNotice(&empty) != nil {
		t.Fatalf("empty string in → nil out")
	}
	spaces := "   "
	if normalizedSpecialNotice(&spaces) != nil {
		t.Fatalf("whitespace in → nil out")
	}
	padded := "  Backlog today.  "
	got := normalizedSpecialNotice(&padded)
	if got == nil || *got != "Backlog today." {
		t.Fatalf("padded string should trim, got %v", got)
	}
}
