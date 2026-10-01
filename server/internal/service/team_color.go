package service

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
)

var teamColorHex = regexp.MustCompile(`^#(?:[0-9a-f]{3}|[0-9a-f]{6})$`)

var teamPresetColors = [...]string{"#4e8fea", "#2da88e", "#45a557", "#c7a53d", "#e58c3a", "#e2564a", "#e54e78", "#8b5cf6"}

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
