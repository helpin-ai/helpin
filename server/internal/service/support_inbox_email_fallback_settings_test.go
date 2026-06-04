package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestParseSettingsDefaultsEmailFallbackFreshnessWindow(t *testing.T) {
	settings := parseSettings(`{"email_fallback_enabled":true,"email_fallback_delay_secs":180}`)

	if !settings.EmailFallbackEnabled {
		t.Fatal("expected email fallback enabled to be parsed")
	}
	if settings.EmailFallbackDelaySecs != 180 {
		t.Fatalf("expected delay=180, got %d", settings.EmailFallbackDelaySecs)
	}
	if settings.EmailFallbackMaxDeliveryAgeSecs != model.DefaultSupportInboxSettings().EmailFallbackMaxDeliveryAgeSecs {
		t.Fatalf("expected missing max age to default to %d, got %d", model.DefaultSupportInboxSettings().EmailFallbackMaxDeliveryAgeSecs, settings.EmailFallbackMaxDeliveryAgeSecs)
	}
}

func TestDefaultSupportInboxSettingsEnableEmailFallback(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	if !settings.EmailFallbackEnabled {
		t.Fatal("expected email fallback to be enabled by default")
	}
}

func TestParseSettingsForcesEmailFallbackEnabled(t *testing.T) {
	settings := parseSettings(`{"email_fallback_enabled":false}`)

	if !settings.EmailFallbackEnabled {
		t.Fatal("expected email fallback to be forced enabled")
	}
}

func TestMergeSettingsUpdateCannotDisableEmailFallback(t *testing.T) {
	current := model.DefaultSupportInboxSettings()
	disabled := false

	merged := mergeSettingsUpdate(current, model.UpdateInstallationSettingsRequest{
		EmailFallbackEnabled: &disabled,
	})

	if !merged.EmailFallbackEnabled {
		t.Fatal("expected email fallback to remain enabled")
	}
}

func TestMergeSettingsUpdateAppliesEmailFallbackFreshnessWindow(t *testing.T) {
	current := model.DefaultSupportInboxSettings()
	current.EmailFallbackDelaySecs = 120
	current.EmailFallbackMaxDeliveryAgeSecs = 600
	maxAge := 900

	merged := mergeSettingsUpdate(current, model.UpdateInstallationSettingsRequest{
		EmailFallbackMaxDeliveryAgeSecs: &maxAge,
	})

	if merged.EmailFallbackMaxDeliveryAgeSecs != 900 {
		t.Fatalf("expected max delivery age to be patched to 900, got %d", merged.EmailFallbackMaxDeliveryAgeSecs)
	}
	if merged.EmailFallbackDelaySecs != 120 {
		t.Fatalf("expected unrelated delay setting to remain 120, got %d", merged.EmailFallbackDelaySecs)
	}
}

func TestValidateSettingsEmailFallbackFreshnessWindow(t *testing.T) {
	ctx := context.Background()
	svc := &SupportInboxService{}

	tests := []struct {
		name    string
		mutate  func(*model.SupportInboxSettings)
		wantErr string
	}{
		{
			name: "allows default ten minute window",
			mutate: func(settings *model.SupportInboxSettings) {
				settings.EmailFallbackDelaySecs = 120
				settings.EmailFallbackMaxDeliveryAgeSecs = 600
			},
		},
		{
			name: "allows max age equal to delay",
			mutate: func(settings *model.SupportInboxSettings) {
				settings.EmailFallbackDelaySecs = 300
				settings.EmailFallbackMaxDeliveryAgeSecs = 300
			},
		},
		{
			name: "rejects max age under two minutes",
			mutate: func(settings *model.SupportInboxSettings) {
				settings.EmailFallbackDelaySecs = 30
				settings.EmailFallbackMaxDeliveryAgeSecs = 119
			},
			wantErr: "email_fallback_max_delivery_age_secs must be between 120 and 1800",
		},
		{
			name: "rejects max age over thirty minutes",
			mutate: func(settings *model.SupportInboxSettings) {
				settings.EmailFallbackDelaySecs = 120
				settings.EmailFallbackMaxDeliveryAgeSecs = 1801
			},
			wantErr: "email_fallback_max_delivery_age_secs must be between 120 and 1800",
		},
		{
			name: "rejects max age lower than delivery delay",
			mutate: func(settings *model.SupportInboxSettings) {
				settings.EmailFallbackDelaySecs = 300
				settings.EmailFallbackMaxDeliveryAgeSecs = 240
			},
			wantErr: "email_fallback_max_delivery_age_secs must be greater than or equal to email_fallback_delay_secs",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			settings := model.DefaultSupportInboxSettings()
			tc.mutate(&settings)

			err := svc.validateSettings(ctx, "workspace-1", settings)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateSettings returned error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
