package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func splitDay(slots ...model.BusinessHoursSlot) model.BusinessHoursDay {
	return model.BusinessHoursDay{Enabled: true, Slots: slots}
}

func TestBusinessHoursWindows(t *testing.T) {
	tests := []struct {
		name string
		day  model.BusinessHoursDay
		want []businessHoursWindow
	}{
		{
			name: "settings saved before slots use start and end",
			day:  model.BusinessHoursDay{Start: "09:00", End: "17:00", Enabled: true},
			want: []businessHoursWindow{{start: 540, end: 1020}},
		},
		{
			name: "slots come back in order",
			day:  splitDay(model.BusinessHoursSlot{Start: "14:00", End: "18:00"}, model.BusinessHoursSlot{Start: "09:00", End: "12:00"}),
			want: []businessHoursWindow{{start: 540, end: 720}, {start: 840, end: 1080}},
		},
		{
			name: "overlapping and touching slots merge",
			day: splitDay(
				model.BusinessHoursSlot{Start: "09:00", End: "12:00"},
				model.BusinessHoursSlot{Start: "11:00", End: "13:00"},
				model.BusinessHoursSlot{Start: "13:00", End: "14:00"},
			),
			want: []businessHoursWindow{{start: 540, end: 840}},
		},
		{
			name: "a slot can end at midnight",
			day:  splitDay(model.BusinessHoursSlot{Start: "22:00", End: "24:00"}),
			want: []businessHoursWindow{{start: 1320, end: 1440}},
		},
		{
			name: "slots that cannot be read are skipped",
			day: splitDay(
				model.BusinessHoursSlot{Start: "17:00", End: "09:00"},
				model.BusinessHoursSlot{Start: "24:00", End: "24:00"},
				model.BusinessHoursSlot{Start: "9am", End: "5pm"},
				model.BusinessHoursSlot{Start: "10:00", End: "11:00"},
			),
			want: []businessHoursWindow{{start: 600, end: 660}},
		},
		{
			name: "a closed day has no windows",
			day:  model.BusinessHoursDay{Start: "09:00", End: "17:00"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := businessHoursWindows(tt.day)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("businessHoursWindows() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeBusinessHoursSchedule(t *testing.T) {
	got, err := normalizeBusinessHoursSchedule(map[string]model.BusinessHoursDay{
		"mon": splitDay(model.BusinessHoursSlot{Start: "14:00", End: "18:00"}, model.BusinessHoursSlot{Start: "09:00", End: "12:00"}),
		"tue": {Start: "09:00", End: "17:00", Enabled: true},
		"wed": {Enabled: true},
		"sat": {Start: "17:00", End: "09:00"},
	})
	if err != nil {
		t.Fatalf("normalizeBusinessHoursSchedule: %v", err)
	}

	monday := got["mon"]
	wantMonday := []model.BusinessHoursSlot{{Start: "09:00", End: "12:00"}, {Start: "14:00", End: "18:00"}}
	if !reflect.DeepEqual(monday.Slots, wantMonday) {
		t.Errorf("monday slots = %v, want %v", monday.Slots, wantMonday)
	}
	if monday.Start != "09:00" || monday.End != "12:00" {
		t.Errorf("monday start/end = %s-%s, want the first slot 09:00-12:00", monday.Start, monday.End)
	}
	if tuesday := got["tue"]; !reflect.DeepEqual(tuesday.Slots, []model.BusinessHoursSlot{{Start: "09:00", End: "17:00"}}) {
		t.Errorf("tuesday slots = %v, want its start and end as one slot", tuesday.Slots)
	}
	if got["wed"].Enabled {
		t.Error("an enabled day without times should be closed")
	}
	if saturday := got["sat"]; saturday.Enabled || len(saturday.Slots) != 0 {
		t.Errorf("a closed day keeps only readable times, got %+v", saturday)
	}
}

func TestNormalizeBusinessHoursSchedule_RejectsInvalidOpenDays(t *testing.T) {
	tooMany := make([]model.BusinessHoursSlot, maxBusinessHoursSlotsPerDay+1)
	for i := range tooMany {
		tooMany[i] = model.BusinessHoursSlot{Start: formatBusinessHoursClock(i * 60), End: formatBusinessHoursClock(i*60 + 30)}
	}
	tests := []struct {
		name string
		day  model.BusinessHoursDay
		key  string
		want string
	}{
		{name: "end before start", key: "mon", day: splitDay(model.BusinessHoursSlot{Start: "17:00", End: "09:00"}), want: "business hours for Monday end before they start"},
		{name: "overlap", key: "tue", day: splitDay(model.BusinessHoursSlot{Start: "09:00", End: "12:00"}, model.BusinessHoursSlot{Start: "11:00", End: "13:00"}), want: "business hours for Tuesday overlap"},
		{name: "invalid time", key: "wed", day: splitDay(model.BusinessHoursSlot{Start: "9am", End: "17:00"}), want: "business hours for Wednesday have a time that is not valid"},
		{name: "midnight start", key: "thu", day: splitDay(model.BusinessHoursSlot{Start: "24:00", End: "24:00"}), want: "business hours for Thursday have a time that is not valid"},
		{name: "too many ranges", key: "fri", day: splitDay(tooMany...), want: "business hours for Friday allow at most 12 time ranges"},
		{name: "unknown day", key: "funday", day: splitDay(model.BusinessHoursSlot{Start: "09:00", End: "17:00"}), want: `business hours have an unknown day "funday"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeBusinessHoursSchedule(map[string]model.BusinessHoursDay{tt.key: tt.day})
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestBusinessHoursWithSplitDay(t *testing.T) {
	settings := model.SupportInboxSettings{
		BusinessHoursEnabled:  true,
		BusinessHoursTimezone: "UTC",
		BusinessHoursSchedule: map[string]model.BusinessHoursDay{
			// Monday, October 26 2026: morning and afternoon with a lunch break.
			"mon": splitDay(model.BusinessHoursSlot{Start: "09:00", End: "12:00"}, model.BusinessHoursSlot{Start: "13:00", End: "17:00"}),
			"tue": splitDay(model.BusinessHoursSlot{Start: "10:00", End: "12:00"}),
		},
	}
	monday := func(hour, minute int) time.Time { return time.Date(2026, 10, 26, hour, minute, 0, 0, time.UTC) }

	tests := []struct {
		name     string
		now      time.Time
		wantOpen bool
		wantNext time.Time
	}{
		{name: "morning", now: monday(10, 0), wantOpen: true},
		{name: "lunch break", now: monday(12, 30), wantNext: monday(13, 0)},
		{name: "afternoon", now: monday(16, 59), wantOpen: true},
		{name: "evening", now: monday(17, 0), wantNext: time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)},
		{name: "before opening", now: monday(8, 0), wantNext: monday(9, 0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWithinBusinessHours(settings, tt.now); got != tt.wantOpen {
				t.Fatalf("isWithinBusinessHours() = %v, want %v", got, tt.wantOpen)
			}
			if tt.wantOpen {
				return
			}
			next := nextBusinessHoursStart(settings, tt.now)
			if next == nil || !next.Equal(tt.wantNext) {
				t.Fatalf("nextBusinessHoursStart() = %v, want %v", next, tt.wantNext)
			}
		})
	}
}

func TestSupportSLAClock_SplitDay(t *testing.T) {
	clock := newSupportSLAClock(model.SupportInboxSettings{
		BusinessHoursEnabled:  true,
		BusinessHoursTimezone: "UTC",
		BusinessHoursSchedule: map[string]model.BusinessHoursDay{
			"mon": splitDay(model.BusinessHoursSlot{Start: "09:00", End: "12:00"}, model.BusinessHoursSlot{Start: "13:00", End: "17:00"}),
			"tue": splitDay(model.BusinessHoursSlot{Start: "09:00", End: "12:00"}, model.BusinessHoursSlot{Start: "13:00", End: "17:00"}),
		},
	}, true)
	monday := func(hour, minute int) time.Time { return time.Date(2026, 10, 26, hour, minute, 0, 0, time.UTC) }

	if got := clock.Between(monday(11, 0), monday(14, 0)); got != 2*time.Hour {
		t.Errorf("Between across lunch = %v, want 2h", got)
	}
	if got := clock.Add(monday(11, 0), 2*time.Hour); !got.Equal(monday(14, 0)) {
		t.Errorf("Add across lunch = %v, want %v", got, monday(14, 0))
	}
	if got := clock.Add(monday(12, 30), 30*time.Minute); !got.Equal(monday(13, 30)) {
		t.Errorf("Add from lunch = %v, want %v", got, monday(13, 30))
	}
	// Monday has 7 open hours; 8 hours lands one hour into Tuesday.
	if got := clock.Add(monday(9, 0), 8*time.Hour); !got.Equal(time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("Add into the next day = %v, want Tuesday 10:00", got)
	}
	// Two weeks of open time (14 hours each) end at the second Tuesday's close.
	if got := clock.Add(monday(9, 0), 28*time.Hour); !got.Equal(time.Date(2026, 11, 3, 17, 0, 0, 0, time.UTC)) {
		t.Errorf("Add across weeks = %v, want Tuesday November 3 17:00", got)
	}
}

func TestSupportSLAClock_MidnightEnd(t *testing.T) {
	clock := newSupportSLAClock(model.SupportInboxSettings{
		BusinessHoursEnabled:  true,
		BusinessHoursTimezone: "UTC",
		BusinessHoursSchedule: map[string]model.BusinessHoursDay{
			"mon": splitDay(model.BusinessHoursSlot{Start: "08:00", End: "10:00"}, model.BusinessHoursSlot{Start: "22:00", End: "24:00"}),
			"tue": splitDay(model.BusinessHoursSlot{Start: "08:00", End: "10:00"}),
		},
	}, true)

	start := time.Date(2026, 10, 26, 23, 0, 0, 0, time.UTC)
	if got := clock.Add(start, 2*time.Hour); !got.Equal(time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("Add past midnight = %v, want Tuesday 09:00", got)
	}
}

func TestUpdateInstallationSettings_SavesBusinessHoursSlots(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	svc := env.service.supportInboxService
	workspaceID := "11111111-1111-1111-1111-111111111111"

	schedule := map[string]model.BusinessHoursDay{
		"mon": splitDay(model.BusinessHoursSlot{Start: "13:00", End: "17:00"}, model.BusinessHoursSlot{Start: "09:00", End: "12:00"}),
	}
	_, settings, err := svc.UpdateInstallationSettings(ctx, workspaceID, model.UpdateInstallationSettingsRequest{BusinessHoursSchedule: schedule})
	if err != nil {
		t.Fatalf("UpdateInstallationSettings: %v", err)
	}
	want := []model.BusinessHoursSlot{{Start: "09:00", End: "12:00"}, {Start: "13:00", End: "17:00"}}
	if got := settings.BusinessHoursSchedule["mon"].Slots; !reflect.DeepEqual(got, want) {
		t.Fatalf("saved slots = %v, want %v", got, want)
	}

	overlapping := map[string]model.BusinessHoursDay{
		"mon": splitDay(model.BusinessHoursSlot{Start: "09:00", End: "12:00"}, model.BusinessHoursSlot{Start: "11:00", End: "13:00"}),
	}
	if _, _, err := svc.UpdateInstallationSettings(ctx, workspaceID, model.UpdateInstallationSettingsRequest{BusinessHoursSchedule: overlapping}); err == nil {
		t.Fatal("overlapping business hours should be rejected")
	}
}
