package service

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
)

var teamColorHex = regexp.MustCompile(`^#(?:[0-9a-f]{3}|[0-9a-f]{6})$`)

// Match the saved swatches in the team color picker.
var teamPresetColors = [...]string{"#5e6ad2", "#4e8fea", "#3daed4", "#2da88e", "#45a557", "#7da642", "#c7a53d", "#e58c3a", "#e2564a", "#e54e78", "#d44ca0", "#b44ec9", "#8b5cf6", "#4a9ed6", "#a08060", "#788596"}

func randomTeamColor() string {
	return teamPresetColors[rand.IntN(len(teamPresetColors))]
}

// An omitted color preserves the saved choice; an empty string resets it.
func normalizeTeamColor(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	color := strings.ToLower(strings.TrimSpace(*value))
	if color == "" {
		return &color, nil
	}
	if !teamColorHex.MatchString(color) {
		return nil, fmt.Errorf("color must be a hex color such as #5e6ad2")
	}
	if len(color) == 4 {
		color = "#" + strings.Repeat(string(color[1]), 2) + strings.Repeat(string(color[2]), 2) + strings.Repeat(string(color[3]), 2)
	}
	return &color, nil
}
