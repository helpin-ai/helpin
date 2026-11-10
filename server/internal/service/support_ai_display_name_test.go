package service

import (
	"context"
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
