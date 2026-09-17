package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestMergeSettingsUpdateAppliesPortalSettings(t *testing.T) {
	current := model.DefaultSupportInboxSettings()
	enabled := true

	merged := mergeSettingsUpdate(current, model.UpdateInstallationSettingsRequest{
		PortalEnabled:                &enabled,
		PortalRequestsOnly:           &enabled,
		PortalAnonymousIntakeEnabled: &enabled,
		PortalIntakeEnabled:          &enabled,
	})

	if !merged.PortalEnabled || !merged.PortalRequestsOnly || !merged.PortalAnonymousIntakeEnabled || !merged.PortalIntakeEnabled {
		t.Fatalf("expected all portal settings enabled, got %+v", merged)
	}
}

func TestMergeSettingsUpdatePreservesOmittedPortalSettings(t *testing.T) {
	current := model.DefaultSupportInboxSettings()
	current.PortalEnabled = true
	current.PortalRequestsOnly = true
	current.PortalAnonymousIntakeEnabled = true
	current.PortalIntakeEnabled = true

	merged := mergeSettingsUpdate(current, model.UpdateInstallationSettingsRequest{})

	if !merged.PortalEnabled || !merged.PortalRequestsOnly || !merged.PortalAnonymousIntakeEnabled || !merged.PortalIntakeEnabled {
		t.Fatalf("expected omitted portal settings to be preserved, got %+v", merged)
	}
}
