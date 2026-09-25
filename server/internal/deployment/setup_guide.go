package deployment

import (
	"fmt"
	"strconv"
	"strings"
)

// SetupGuidePolicy resolves SETUP_SUCCESS_ENABLED. An empty value selects the
// edition default (SetupGuideDefault); an explicit boolean always wins.
func SetupGuidePolicy(raw string) (bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SetupGuideDefault, nil
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("SETUP_SUCCESS_ENABLED must be true or false")
	}
	return enabled, nil
}
