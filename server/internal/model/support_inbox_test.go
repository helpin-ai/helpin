package model

import "testing"

func TestDefaultSupportInboxSettingsEnablesAutomatedRouting(t *testing.T) {
	settings := DefaultSupportInboxSettings()

	if !settings.TriageEnabled {
		t.Fatal("expected automated routing to be enabled by default")
	}
}

func TestDefaultSupportInboxSettingsUsesAIFirstResponseMode(t *testing.T) {
	settings := DefaultSupportInboxSettings()

	if settings.AIResponseMode != "ai_first" {
		t.Fatalf("expected default AI response mode to be ai_first, got %q", settings.AIResponseMode)
	}
}

func TestDefaultSupportInboxSettingsUsesFiveAIFollowupsBeforeHandoff(t *testing.T) {
	settings := DefaultSupportInboxSettings()

	if settings.AIMaxFollowups != 5 {
		t.Fatalf("expected default AI max followups to be 5, got %d", settings.AIMaxFollowups)
	}
}

func TestDefaultSupportInboxSettingsUsesThirtySecondEmailFallbackUndoWindow(t *testing.T) {
	settings := DefaultSupportInboxSettings()

	if settings.EmailFallbackDelaySecs != 30 {
		t.Fatalf("expected default email fallback delay to be 30 seconds, got %d", settings.EmailFallbackDelaySecs)
	}
}
