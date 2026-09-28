package service

import (
	"context"
	"encoding/json"
	"github.com/helpin-ai/helpin/server/internal/model"
	"strings"
	"testing"
)

func TestSupportPrivacyNoticeSettings(t *testing.T) {
	defaults := parseSettings(`{}`)
	if defaults.PrivacyNoticeEnabled || defaults.PrivacyPolicyURL != "" || defaults.PrivacyNoticeText == "" {
		t.Fatal("notice must default to off with default copy")
	}
	enabled, policyURL, text := true, "https://example.com/privacy", "We handle your chat as described in our"
	settings := mergeSettingsUpdate(defaults, model.UpdateInstallationSettingsRequest{PrivacyNoticeEnabled: &enabled, PrivacyPolicyURL: &policyURL, PrivacyNoticeText: &text})
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	stored := parseSettings(string(raw))
	if !stored.PrivacyNoticeEnabled || stored.PrivacyPolicyURL != policyURL || stored.PrivacyNoticeText != text {
		t.Fatal("privacy settings did not survive persistence")
	}
	disabled := false
	stored = mergeSettingsUpdate(stored, model.UpdateInstallationSettingsRequest{PrivacyNoticeEnabled: &disabled})
	if stored.PrivacyNoticeEnabled || stored.PrivacyPolicyURL != policyURL || stored.PrivacyNoticeText != text {
		t.Fatal("disabling must keep configured content")
	}
}
func TestSupportPrivacyNoticeValidation(t *testing.T) {
	for _, tc := range []struct {
		name, url, text  string
		enabled, invalid bool
	}{
		{"disabled", "", "", false, false},
		{"valid", "https://example.com/privacy", "Our policy", true, false},
		{"http", "http://example.com/privacy", "Our policy", true, false},
		{"missing", "", "Our policy", true, true},
		{"script", "javascript:alert(1)", "Our policy", true, true},
		{"relative", "/privacy", "Our policy", true, true},
		{"credentials", "https://user:pass@example.com/privacy", "Our policy", true, true},
		{"empty text", "https://example.com/privacy", "  ", true, true},
		{"long text", "https://example.com/privacy", strings.Repeat("a", 501), true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := model.DefaultSupportInboxSettings()
			settings.PrivacyNoticeEnabled, settings.PrivacyPolicyURL, settings.PrivacyNoticeText = tc.enabled, tc.url, tc.text
			err := (&SupportInboxService{}).validateSettings(context.Background(), "ws", settings)
			if (err != nil) != tc.invalid {
				t.Fatalf("validation error=%v, want invalid=%v", err, tc.invalid)
			}
		})
	}
}

func TestSupportPrivacyNoticeUpdateAndWidgetConfig(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	svc := env.service.supportInboxService
	enabled, policyURL, text := true, "https://example.com/privacy", "By chatting with us, you agree to our"
	inst, _, err := svc.UpdateInstallationSettings(ctx, "11111111-1111-1111-1111-111111111111", model.UpdateInstallationSettingsRequest{PrivacyNoticeEnabled: &enabled, PrivacyPolicyURL: &policyURL, PrivacyNoticeText: &text})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := env.installRepo.GetByWorkspace(ctx, inst.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	config, err := svc.buildWidgetConfigResponse(ctx, saved)
	if err != nil {
		t.Fatal(err)
	}
	if !config.PrivacyNotice.Enabled || config.PrivacyNotice.PolicyURL != policyURL || config.PrivacyNotice.Text != text {
		t.Fatalf("wrong public privacy settings: %+v", config.PrivacyNotice)
	}
	invalid := "javascript:alert(1)"
	if _, _, err := svc.UpdateInstallationSettings(ctx, inst.WorkspaceID, model.UpdateInstallationSettingsRequest{PrivacyPolicyURL: &invalid}); err == nil {
		t.Fatal("unsafe URL saved")
	}
	saved, err = env.installRepo.GetByWorkspace(ctx, inst.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if parseSettings(saved.Settings).PrivacyPolicyURL != policyURL {
		t.Fatal("invalid update replaced saved policy URL")
	}
}
