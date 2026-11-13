package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAIDisplayNamePatchAndValidation(t *testing.T) {
	current := model.DefaultSupportInboxSettings()
	current.AIDisplayName = "Echo"
	if got := mergeSettingsUpdate(current, model.UpdateInstallationSettingsRequest{}); got.AIDisplayName != "Echo" {
		t.Fatalf("unrelated patch reset name: %q", got.AIDisplayName)
	}
	name := "  Demo assistant  "
	if got := mergeSettingsUpdate(current, model.UpdateInstallationSettingsRequest{AIDisplayName: &name}); got.AIDisplayName != "Demo assistant" {
		t.Fatalf("name not normalized: %q", got.AIDisplayName)
	}
	for _, invalid := range []string{strings.Repeat("a", 81), "first\nsecond", "first\rsecond"} {
		current.AIDisplayName = invalid
		if err := (&SupportInboxService{}).validateSettings(context.Background(), "workspace", current); err == nil {
			t.Errorf("accepted invalid display name %q", invalid)
		}
	}
}

func TestWidgetPublicIdentityIncludesSavedAvatar(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	svc := env.service.supportInboxService
	name, assistant, avatar, logo := "Demo support", "Demo assistant", "https://example.com/avatar.png", "https://example.com/logo.png"
	inst, _, err := svc.UpdateInstallationSettings(ctx, "11111111-1111-1111-1111-111111111111", model.UpdateInstallationSettingsRequest{
		WidgetName: &name, AIDisplayName: &assistant, WidgetAvatarURL: &avatar, LogoURL: &logo,
	})
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
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	var public map[string]any
	if err := json.Unmarshal(raw, &public); err != nil {
		t.Fatal(err)
	}
	branding := public["branding"].(map[string]any)
	if public["workspaceName"] != name || public["aiDisplayName"] != assistant || branding["avatarUrl"] != avatar || branding["logoUrl"] != logo {
		t.Fatalf("public widget identity differs from saved settings: %s", raw)
	}
}
