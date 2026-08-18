package automationcron

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron"
)

func ValidateExpression(expression string) error {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return fmt.Errorf("cron expression is required")
	}
	if _, err := cron.ParseStandard(expression); err != nil {
		return fmt.Errorf("cron expression must use standard 5-field syntax")
	}
	return nil
}

// Next returns the first scheduled tick strictly after the supplied time.
// Automation rule schedules are persisted in UTC, so normalize both the input
// and result to keep API consumers independent of the server's local timezone.
func Next(expression string, after time.Time) (time.Time, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return time.Time{}, fmt.Errorf("cron expression is required")
	}
	schedule, err := cron.ParseStandard(expression)
	if err != nil {
		return time.Time{}, fmt.Errorf("cron expression must use standard 5-field syntax")
	}
	return schedule.Next(after.UTC()).UTC(), nil
}
