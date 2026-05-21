package automationcron

import (
	"fmt"
	"strings"

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
