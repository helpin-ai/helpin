package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/deployment"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/widgetorigin"
)

func TestWidgetCreationPolicyAndSavedPolicy(t *testing.T) {
	for _, seed := range []bool{false, true} {
		t.Run(map[bool]string{false: "lazy", true: "seed"}[seed], func(t *testing.T) {
			db := newTestDB(t)
			seedWorkspace(t, db, "ws-widget-policy", "Widget policy", "widget-policy", "user-widget-policy")
			repo := repository.NewSupportInboxInstallationRepository(db)
			svc := &SupportInboxService{installationRepo: repo}
			ctx := context.Background()
			if seed {
				if err := svc.SeedWorkspaceDefaults(ctx, "ws-widget-policy", "user-widget-policy"); err != nil {
					t.Fatal(err)
				}
			}
			inst, _, err := svc.GetInstallation(ctx, "ws-widget-policy")
			if err != nil {
				t.Fatal(err)
			}
			if inst.IdentityVerificationMode != deployment.WidgetIdentityMode {
				t.Fatalf("identity mode = %q", inst.IdentityVerificationMode)
			}
			if len(inst.AllowedOrigins) != 0 {
				t.Fatal("fresh installation must require site setup")
			}
			if err := svc.AuthorizeWidgetOrigin(ctx, "https://site.example", widgetorigin.Reference{WidgetKey: inst.WidgetKey}); err == nil {
				t.Fatal("unconfigured widget admitted")
			}
			inst.IdentityVerificationMode = model.IdentityVerificationModeEnforced
			if err := repo.Update(ctx, inst); err != nil {
				t.Fatal(err)
			}
			if seed {
				if err := svc.SeedWorkspaceDefaults(ctx, "ws-widget-policy", "user-widget-policy"); err != nil {
					t.Fatal(err)
				}
			}
			got, _, err := svc.GetInstallation(ctx, "ws-widget-policy")
			if err != nil {
				t.Fatal(err)
			}
			if got.IdentityVerificationMode != model.IdentityVerificationModeEnforced {
				t.Fatal("saved policy downgraded")
			}
		})
	}
}
