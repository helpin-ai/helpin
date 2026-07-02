package service

import (
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildWidgetAvailabilityDisabledBusinessHoursReturnsOnline(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	settings.BusinessHoursEnabled = false

	availability := buildWidgetAvailability(settings, time.Date(2026, 3, 24, 15, 0, 0, 0, time.UTC), true)

	if !availability.IsOnline {
		t.Fatal("expected widget availability to be online when business hours are disabled")
	}
	if availability.StatusText != "Online now" {
		t.Fatalf("expected online status text, got %q", availability.StatusText)
	}
	if availability.ReplyTimeText != FormatReplyTimeCopy(model.SupportReplyTimePresetFewMinutes, 0) {
		t.Fatalf("expected default online reply text, got %q", availability.ReplyTimeText)
	}
	if availability.ReplyTimePreset != model.SupportReplyTimePresetFewMinutes {
		t.Fatalf("expected default preset few_minutes, got %q", availability.ReplyTimePreset)
	}
	if availability.NextOnlineAt != nil {
		t.Fatal("expected no next online timestamp while online")
	}
}

func TestBuildWidgetAvailabilityWithinBusinessHoursReturnsOnline(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	settings := model.DefaultSupportInboxSettings()
	settings.BusinessHoursEnabled = true
	settings.BusinessHoursTimezone = "America/New_York"

	availability := buildWidgetAvailability(settings, time.Date(2026, 3, 24, 10, 30, 0, 0, loc), true)

	if !availability.IsOnline {
		t.Fatal("expected widget availability to be online during configured business hours")
	}
	if availability.StatusText != "Online now" {
		t.Fatalf("expected online status text, got %q", availability.StatusText)
	}
}

func TestBuildWidgetAvailabilityOutsideBusinessHoursReturnsOfflineMessageAndNextOpening(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	settings := model.DefaultSupportInboxSettings()
	settings.BusinessHoursEnabled = true
	settings.BusinessHoursTimezone = "America/New_York"
	settings.OutsideHoursMessage = "We are offline right now."

	availability := buildWidgetAvailability(settings, time.Date(2026, 3, 24, 8, 0, 0, 0, loc), false)

	if availability.IsOnline {
		t.Fatal("expected widget availability to be offline before opening hours")
	}
	if availability.OutsideHoursMessage == nil || *availability.OutsideHoursMessage != "We are offline right now." {
		t.Fatalf("expected outside hours message to be preserved, got %#v", availability.OutsideHoursMessage)
	}
	if availability.ReplyTimeText != "We are offline right now." {
		t.Fatalf("expected reply time text to use outside-hours message, got %q", availability.ReplyTimeText)
	}
	if availability.NextOnlineAt == nil {
		t.Fatal("expected next online timestamp while offline before opening")
	}
	if !strings.Contains(availability.StatusText, "Back Tue 9:00 AM") {
		t.Fatalf("expected status text to mention next opening, got %q", availability.StatusText)
	}
}
