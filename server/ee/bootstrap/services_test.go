//go:build ee

package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/ee/aiconnections"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestEERequiresManagedConnectionEncryption(t *testing.T) {
	_, err := New(Options{Config: &config.Config{}})
	if err == nil {
		t.Fatal("missing encryption key accepted")
	}
}

func TestEEInstallsCommercialPoliciesAndRejectsUnconfiguredBYOK(t *testing.T) {
	services, err := New(Options{Config: &config.Config{AIConnectionEncryptionKey: strings.Repeat("ab", 32)}})
	if err != nil {
		t.Fatal(err)
	}
	if services.Usage == nil || services.Entitlements == nil || services.WorkspaceLifecycle == nil || services.Seats == nil || services.ConnectionPolicy == nil || services.RunWorkers == nil || !services.RequireConfiguredProviders {
		t.Fatal("commercial dependencies missing")
	}
	_, err = services.ConnectionPolicy.ResolveConnectionPolicy(context.Background(), "workspace", &model.AIConnection{WorkspaceID: "workspace", Scope: "workspace", Funding: "customer", Provider: "openai"})
	if !errors.Is(err, aiconnections.ErrBYOKUnavailable) {
		t.Fatalf("unconfigured BYOK: %v", err)
	}
	accepted, err := services.ConnectionPolicy.ResolveConnectionPolicy(context.Background(), "workspace", &model.AIConnection{WorkspaceID: "workspace", Scope: "workspace", Funding: "managed", Provider: "openai"})
	if err != nil || accepted.FundingMode != aiusage.FundingHelpinHosted {
		t.Fatalf("managed funding: %+v, %v", accepted, err)
	}
}
