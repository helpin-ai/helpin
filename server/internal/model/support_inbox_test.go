package model

import (
	"reflect"
	"testing"
)

func TestSupportCompanyContextFieldsAreNullableIndexedUUIDs(t *testing.T) {
	tests := []struct {
		name      string
		modelType reflect.Type
	}{
		{name: "conversation", modelType: reflect.TypeOf(SupportConversation{})},
		{name: "widget session", modelType: reflect.TypeOf(SupportWidgetSession{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := tt.modelType.FieldByName("CRMCompanyID")
			if !ok {
				t.Fatal("expected CRMCompanyID field")
			}
			if field.Type != reflect.TypeOf((*string)(nil)) {
				t.Fatalf("CRMCompanyID type = %v, want *string", field.Type)
			}
			if got := field.Tag.Get("json"); got != "crm_company_id" {
				t.Fatalf("CRMCompanyID json tag = %q, want %q", got, "crm_company_id")
			}
			if got := field.Tag.Get("gorm"); got != "type:uuid;index" {
				t.Fatalf("CRMCompanyID gorm tag = %q, want %q", got, "type:uuid;index")
			}
		})
	}
}

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

func TestDefaultSupportInboxSettingsUsesTenSecondEmailFallbackUndoWindow(t *testing.T) {
	settings := DefaultSupportInboxSettings()

	if settings.EmailFallbackDelaySecs != 10 {
		t.Fatalf("expected default email fallback delay to be 10 seconds, got %d", settings.EmailFallbackDelaySecs)
	}
}
