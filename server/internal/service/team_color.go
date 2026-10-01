package service

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
)

var teamColorHex = regexp.MustCompile(`^#(?:[0-9a-f]{3}|[0-9a-f]{6})$`)

// Match the saved swatches in the shared Epic and team color picker.
var teamPresetColors = [...]string{"#a6ade6", "#9ec1f3", "#94d2e7", "#8ccfc1", "#99cea3", "#b8ce97", "#e0ce94", "#f1c093", "#efa29b", "#f19eb5", "#e79dcb", "#d69ee1", "#bfa5fa", "#9bcae8", "#cbb9a8", "#c1c9d3"}

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
