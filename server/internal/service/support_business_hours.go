package service

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	minutesPerDay = 24 * 60
	// maxBusinessHoursSlotsPerDay bounds the time ranges one day can hold.
	maxBusinessHoursSlotsPerDay = 12
)

var businessHoursDayLabels = map[string]string{
	"mon": "Monday",
	"tue": "Tuesday",
	"wed": "Wednesday",
	"thu": "Thursday",
	"fri": "Friday",
	"sat": "Saturday",
	"sun": "Sunday",
}

// businessHoursWindow is an open range of a day in minutes after midnight, [start, end).
type businessHoursWindow struct {
	start int
	end   int
}

// businessHoursWindows returns an enabled day's open ranges in order. Ranges
// that cannot be read are skipped, and ranges that overlap or touch are merged.
func businessHoursWindows(day model.BusinessHoursDay) []businessHoursWindow {
	if !day.Enabled {
		return nil
	}
	windows := make([]businessHoursWindow, 0, len(day.OpenSlots()))
	for _, slot := range day.OpenSlots() {
		window, ok := parseBusinessHoursSlot(slot)
		if !ok {
			continue
		}
		windows = append(windows, window)
	}
	sortBusinessHoursWindows(windows)
	merged := windows[:0]
	for _, window := range windows {
		if last := len(merged) - 1; last >= 0 && window.start <= merged[last].end {
			merged[last].end = max(merged[last].end, window.end)
			continue
		}
		merged = append(merged, window)
	}
	return merged
}

// normalizeBusinessHoursSchedule checks the schedule in a settings update. It
// stores each day's ranges in order, mirrors the first range into Start and
// End for older clients, and closes enabled days that have no ranges.
func normalizeBusinessHoursSchedule(schedule map[string]model.BusinessHoursDay) (map[string]model.BusinessHoursDay, error) {
	normalized := make(map[string]model.BusinessHoursDay, len(schedule))
	for key, day := range schedule {
		label, ok := businessHoursDayLabels[key]
		if !ok {
			return nil, fmt.Errorf("business hours have an unknown day %q", key)
		}
		windows, err := businessHoursDayWindows(label, day)
		if err != nil {
			return nil, err
		}
		day.Slots = make([]model.BusinessHoursSlot, 0, len(windows))
		for _, window := range windows {
			day.Slots = append(day.Slots, model.BusinessHoursSlot{Start: formatBusinessHoursClock(window.start), End: formatBusinessHoursClock(window.end)})
		}
		if len(day.Slots) == 0 {
			day.Enabled = false
			day.Slots = nil
		} else {
			day.Start, day.End = day.Slots[0].Start, day.Slots[0].End
		}
		normalized[key] = day
	}
	return normalized, nil
}

// businessHoursDayWindows reads one day of a settings update. An enabled day
// must be valid; a closed day keeps the ranges that can be read for when it reopens.
func businessHoursDayWindows(label string, day model.BusinessHoursDay) ([]businessHoursWindow, error) {
	slots := day.OpenSlots()
	if len(slots) > maxBusinessHoursSlotsPerDay {
		return nil, fmt.Errorf("business hours for %s allow at most %d time ranges", label, maxBusinessHoursSlotsPerDay)
	}
	if !day.Enabled {
		day.Enabled = true
		return businessHoursWindows(day), nil
	}
	windows := make([]businessHoursWindow, 0, len(slots))
	for _, slot := range slots {
		start, startOK := parseBusinessHoursClock(slot.Start, false)
		end, endOK := parseBusinessHoursClock(slot.End, true)
		if !startOK || !endOK {
			return nil, fmt.Errorf("business hours for %s have a time that is not valid", label)
		}
		if end <= start {
			return nil, fmt.Errorf("business hours for %s end before they start", label)
		}
		windows = append(windows, businessHoursWindow{start: start, end: end})
	}
	sortBusinessHoursWindows(windows)
	for i := 1; i < len(windows); i++ {
		if windows[i].start < windows[i-1].end {
			return nil, fmt.Errorf("business hours for %s overlap", label)
		}
	}
	return windows, nil
}

func parseBusinessHoursSlot(slot model.BusinessHoursSlot) (businessHoursWindow, bool) {
	start, startOK := parseBusinessHoursClock(slot.Start, false)
	end, endOK := parseBusinessHoursClock(slot.End, true)
	if !startOK || !endOK || end <= start {
		return businessHoursWindow{}, false
	}
	return businessHoursWindow{start: start, end: end}, true
}

// parseBusinessHoursClock reads "HH:MM" as minutes after midnight. "24:00" is
// accepted only as an end time.
func parseBusinessHoursClock(value string, isEnd bool) (int, bool) {
	hoursText, minutesText, ok := strings.Cut(strings.TrimSpace(value), ":")
	if !ok || len(minutesText) != 2 {
		return 0, false
	}
	hours, err := strconv.Atoi(hoursText)
	if err != nil {
		return 0, false
	}
	minutes, err := strconv.Atoi(minutesText)
	if err != nil || hours < 0 || minutes < 0 || minutes > 59 {
		return 0, false
	}
	total := hours*60 + minutes
	if total > minutesPerDay || (total == minutesPerDay && !isEnd) {
		return 0, false
	}
	return total, true
}

func formatBusinessHoursClock(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

func sortBusinessHoursWindows(windows []businessHoursWindow) {
	sort.Slice(windows, func(i, j int) bool {
		if windows[i].start != windows[j].start {
			return windows[i].start < windows[j].start
		}
		return windows[i].end < windows[j].end
	})
}
