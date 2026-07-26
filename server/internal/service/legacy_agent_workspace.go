package service

import (
	"os"
	"path/filepath"
	"strings"
)

// legacyAgentWorkspacePath preserves read access to diffs produced by the
// in-process executor but never persisted as artifacts. Keep this fallback
// through the historical coding-session deprecation window.
func legacyAgentWorkspacePath(runID string) string {
	value := strings.ToLower(strings.TrimSpace(runID))
	if value == "" {
		value = "run"
	} else {
		var builder strings.Builder
		for _, r := range value {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
				builder.WriteRune(r)
			default:
				builder.WriteByte('-')
			}
		}
		value = strings.Trim(builder.String(), "-")
		if value == "" {
			value = "run"
		}
	}
	return filepath.Join(os.TempDir(), "helpin-agent-workspaces", value, "repo")
}
