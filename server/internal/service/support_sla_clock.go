package service

import (
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// supportSLAClock measures SLA time either on the calendar or within the
// workspace's weekly business hours.
type supportSLAClock struct {
	location *time.Location
	// windows holds each weekday's [start, end) in minutes after midnight;
	// nil means calendar time.
	windows map[time.Weekday][2]int
}

// supportSLAClockLimit bounds the business-day walk; a 90-day target with
// one open hour a week still fits.
const supportSLAClockLimit = 5 * 366

func newSupportSLAClock(settings model.SupportInboxSettings, businessHours bool) supportSLAClock {
	if !businessHours || !settings.BusinessHoursEnabled {
		return supportSLAClock{location: time.UTC}
	}
	location, err := time.LoadLocation(settings.BusinessHoursTimezone)
	if err != nil {
		return supportSLAClock{location: time.UTC}
	}
	windows := map[time.Weekday][2]int{}
	for day := time.Sunday; day <= time.Saturday; day++ {
		hours, ok := settings.BusinessHoursSchedule[weekdayKey(day)]
		if !ok || !hours.Enabled {
			continue
		}
		start, end := parseTimeToMinutes(hours.Start), parseTimeToMinutes(hours.End)
		if end > 24*60 {
			end = 24 * 60
		}
		if start < 0 || end <= start {
			continue
		}
		windows[day] = [2]int{start, end}
	}
	if len(windows) == 0 {
		// No open hours would make every target unreachable; fall back to
		// calendar time instead.
		return supportSLAClock{location: time.UTC}
	}
	return supportSLAClock{location: location, windows: windows}
}

func (c supportSLAClock) calendar() bool { return c.windows == nil }

// window returns the business window of the local day containing t.
func (c supportSLAClock) window(t time.Time) (time.Time, time.Time, bool) {
	local := t.In(c.location)
	minutes, ok := c.windows[local.Weekday()]
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	y, m, d := local.Date()
	start := time.Date(y, m, d, minutes[0]/60, minutes[0]%60, 0, 0, c.location)
	end := time.Date(y, m, d, minutes[1]/60, minutes[1]%60, 0, 0, c.location)
	if minutes[1] == 24*60 {
		end = time.Date(y, m, d+1, 0, 0, 0, 0, c.location)
	}
	return start, end, true
}

func (c supportSLAClock) nextDay(t time.Time) time.Time {
	y, m, d := t.In(c.location).Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, c.location)
}

// Add returns the moment that lies duration of counted time after start.
func (c supportSLAClock) Add(start time.Time, duration time.Duration) time.Time {
	if duration <= 0 {
		return start
	}
	if c.calendar() {
		return start.Add(duration)
	}
	cursor, remaining := start, duration
	for i := 0; i < supportSLAClockLimit; i++ {
		if open, close, ok := c.window(cursor); ok && cursor.Before(close) {
			if cursor.Before(open) {
				cursor = open
			}
			if available := close.Sub(cursor); remaining <= available {
				return cursor.Add(remaining)
			} else {
				remaining -= available
			}
		}
		cursor = c.nextDay(cursor)
	}
	return cursor.Add(remaining)
}

// Between returns the counted time in [from, to).
func (c supportSLAClock) Between(from, to time.Time) time.Duration {
	if !to.After(from) {
		return 0
	}
	if c.calendar() {
		return to.Sub(from)
	}
	var total time.Duration
	cursor := from
	for i := 0; i < supportSLAClockLimit && cursor.Before(to); i++ {
		if open, close, ok := c.window(cursor); ok {
			if cursor.After(open) {
				open = cursor
			}
			if to.Before(close) {
				close = to
			}
			if close.After(open) {
				total += close.Sub(open)
			}
		}
		cursor = c.nextDay(cursor)
	}
	return total
}
