package service

import (
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// supportSLAClock measures SLA time either on the calendar or within the
// workspace's weekly business hours.
type supportSLAClock struct {
	location *time.Location
	// windows holds each weekday's open ranges in order; nil means calendar time.
	windows map[time.Weekday][]businessHoursWindow
	// week is the counted time in one full week, so long spans skip whole
	// weeks instead of walking every day.
	week time.Duration
}

func newSupportSLAClock(settings model.SupportInboxSettings, businessHours bool) supportSLAClock {
	if !businessHours || !settings.BusinessHoursEnabled {
		return supportSLAClock{location: time.UTC}
	}
	location, err := time.LoadLocation(settings.BusinessHoursTimezone)
	if err != nil {
		return supportSLAClock{location: time.UTC}
	}
	windows := map[time.Weekday][]businessHoursWindow{}
	for day := time.Sunday; day <= time.Saturday; day++ {
		if dayWindows := businessHoursWindows(settings.BusinessHoursSchedule[weekdayKey(day)]); len(dayWindows) > 0 {
			windows[day] = dayWindows
		}
	}
	if len(windows) == 0 {
		// No open hours would make every target unreachable; fall back to
		// calendar time instead.
		return supportSLAClock{location: time.UTC}
	}
	var week time.Duration
	for _, dayWindows := range windows {
		for _, window := range dayWindows {
			week += time.Duration(window.end-window.start) * time.Minute
		}
	}
	return supportSLAClock{location: location, windows: windows, week: week}
}

func (c supportSLAClock) calendar() bool { return c.windows == nil }

// openRange is one business window of a local day, [open, close).
type openRange struct {
	open  time.Time
	close time.Time
}

// ranges returns the business windows of the local day containing t, in order.
func (c supportSLAClock) ranges(t time.Time) []openRange {
	local := t.In(c.location)
	dayWindows := c.windows[local.Weekday()]
	if len(dayWindows) == 0 {
		return nil
	}
	y, m, d := local.Date()
	at := func(minutes int) time.Time {
		if minutes == minutesPerDay {
			return time.Date(y, m, d+1, 0, 0, 0, 0, c.location)
		}
		return time.Date(y, m, d, minutes/60, minutes%60, 0, 0, c.location)
	}
	ranges := make([]openRange, 0, len(dayWindows))
	for _, window := range dayWindows {
		ranges = append(ranges, openRange{open: at(window.start), close: at(window.end)})
	}
	return ranges
}

func (c supportSLAClock) nextDay(t time.Time) time.Time {
	y, m, d := t.In(c.location).Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, c.location)
}

// addWeek moves a local midnight one week later.
func (c supportSLAClock) addWeek(t time.Time) time.Time {
	y, m, d := t.In(c.location).Date()
	return time.Date(y, m, d+7, 0, 0, 0, 0, c.location)
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
	for {
		day := cursor
		for _, r := range c.ranges(day) {
			if !cursor.Before(r.close) {
				continue
			}
			if cursor.Before(r.open) {
				cursor = r.open
			}
			available := r.close.Sub(cursor)
			if remaining <= available {
				return cursor.Add(remaining)
			}
			remaining -= available
			cursor = r.close
		}
		cursor = c.nextDay(day)
		// From a midnight, each full week counts exactly c.week, so at most
		// eight more days are walked.
		for remaining > c.week {
			cursor = c.addWeek(cursor)
			remaining -= c.week
		}
	}
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
	for cursor.Before(to) {
		for _, r := range c.ranges(cursor) {
			open, close := r.open, r.close
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
		for !c.addWeek(cursor).After(to) {
			cursor = c.addWeek(cursor)
			total += c.week
		}
	}
	return total
}

// Sub returns the moment that lies duration of counted time before end.
func (c supportSLAClock) Sub(end time.Time, duration time.Duration) time.Time {
	if duration <= 0 {
		return end
	}
	if c.calendar() {
		return end.Add(-duration)
	}
	local := end.In(c.location)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, c.location)
	upper, remaining := end, duration
	// Ten years of days bounds the walk; targets are at most 90 days.
	for range 3660 {
		ranges := c.ranges(day)
		for i := len(ranges) - 1; i >= 0; i-- {
			open, close := ranges[i].open, ranges[i].close
			if upper.Before(close) {
				close = upper
			}
			if !close.After(open) {
				continue
			}
			available := close.Sub(open)
			if remaining <= available {
				return close.Add(-remaining)
			}
			remaining -= available
		}
		upper = day
		day = time.Date(day.Year(), day.Month(), day.Day()-1, 0, 0, 0, 0, c.location)
	}
	return end.Add(-duration)
}
