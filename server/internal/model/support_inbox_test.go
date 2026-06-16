package model

import "testing"

func TestDefaultSupportInboxSettingsEnablesAutomatedRouting(t *testing.T) {
	settings := DefaultSupportInboxSettings()

	if !settings.TriageEnabled {
		t.Fatal("expected automated routing to be enabled by default")
	}
}
