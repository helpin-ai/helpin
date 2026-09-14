package service

import "time"

// NextAIUsageBoundary returns the next monthly renewal-anniversary boundary.
func NextAIUsageBoundary(anchor, after time.Time) time.Time {
	location := anchor.Location()
	after = after.In(location)
	year, month := after.Year(), after.Month()
	candidate := anniversaryInMonth(anchor, year, month)
	if !candidate.After(after) {
		month++
		if month > time.December {
			month = time.January
			year++
		}
		candidate = anniversaryInMonth(anchor, year, month)
	}
	return candidate
}

func anniversaryInMonth(anchor time.Time, year int, month time.Month) time.Time {
	firstNextMonth := time.Date(year, month+1, 1, anchor.Hour(), anchor.Minute(), anchor.Second(), anchor.Nanosecond(), anchor.Location())
	lastDay := firstNextMonth.AddDate(0, 0, -1).Day()
	day := anchor.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, anchor.Hour(), anchor.Minute(), anchor.Second(), anchor.Nanosecond(), anchor.Location())
}
