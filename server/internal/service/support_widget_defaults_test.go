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

func setupWidgetOriginDefaultsTest(t *testing.T, website string) (*SupportInboxService, *repository.SupportInboxInstallationRepository, *repository.WorkspaceRepository) {
	t.Helper()
	db := newTestDB(t)
	seedWorkspace(t, db, "ws-origin-defaults", "Origin defaults", "origin-defaults", "user-origin-defaults")
	if website != "" {
		mustExec(t, db, `UPDATE workspaces SET website_url = ? WHERE id = ?`, website, "ws-origin-defaults")
	}
	installations := repository.NewSupportInboxInstallationRepository(db)
	workspaces := repository.NewWorkspaceRepository(db)
	svc := (&SupportInboxService{installationRepo: installations}).SetWorkspaceRepo(workspaces)
	return svc, installations, workspaces
}

func TestNewInstallationAllowsWorkspaceWebsite(t *testing.T) {
	for _, seed := range []bool{false, true} {
		t.Run(map[bool]string{false: "lazy", true: "seed"}[seed], func(t *testing.T) {
			svc, _, _ := setupWidgetOriginDefaultsTest(t, "https://www.customer.example/pricing")
			ctx := context.Background()
			if seed {
				if err := svc.SeedWorkspaceDefaults(ctx, "ws-origin-defaults", "user-origin-defaults"); err != nil {
					t.Fatal(err)
				}
			}
			inst, _, err := svc.GetInstallation(ctx, "ws-origin-defaults")
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"https://customer.example", "https://*.customer.example"}
			if len(inst.AllowedOrigins) != len(want) || inst.AllowedOrigins[0] != want[0] || inst.AllowedOrigins[1] != want[1] {
				t.Fatalf("allowed origins = %q, want %q", inst.AllowedOrigins, want)
			}
			for _, origin := range []string{"https://customer.example", "https://www.customer.example", "https://app.customer.example"} {
				if err := svc.AuthorizeWidgetOrigin(ctx, origin, widgetorigin.Reference{WidgetKey: inst.WidgetKey}); err != nil {
					t.Fatalf("%s rejected: %v", origin, err)
				}
			}
			if err := svc.AuthorizeWidgetOrigin(ctx, "https://other.example", widgetorigin.Reference{WidgetKey: inst.WidgetKey}); err == nil {
				t.Fatal("unrelated site admitted")
			}
			if got := svc.SuggestedAllowedOrigins(ctx, inst); got != nil {
				t.Fatalf("suggested origins for a saved list = %q", got)
			}
		})
	}
}

func TestSuggestedAllowedOriginsOnlyForEmptyList(t *testing.T) {
	svc, installations, _ := setupWidgetOriginDefaultsTest(t, "")
	ctx := context.Background()
	inst, _, err := svc.GetInstallation(ctx, "ws-origin-defaults")
	if err != nil {
		t.Fatal(err)
	}
	if got := svc.SuggestedAllowedOrigins(ctx, inst); got != nil {
		t.Fatalf("suggested origins without a website = %q", got)
	}
	if _, err := svc.workspaceRepo.Update(ctx, "ws-origin-defaults", nil, nil, nil, ptr("customer.example"), nil, nil); err != nil {
		t.Fatal(err)
	}
	got := svc.SuggestedAllowedOrigins(ctx, inst)
	if len(got) != 2 || got[0] != "https://customer.example" || got[1] != "https://*.customer.example" {
		t.Fatalf("suggested origins = %q", got)
	}
	saved, err := installations.GetByWorkspace(ctx, "ws-origin-defaults")
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.AllowedOrigins) != 0 {
		t.Fatalf("suggestion was saved: %q", saved.AllowedOrigins)
	}
}

func TestApplyWebsiteOriginDefaults(t *testing.T) {
	for _, tc := range []struct {
		name    string
		website string
		saved   []string
		want    []string
	}{
		{"fills empty list", "https://customer.example", nil, []string{"https://customer.example", "https://*.customer.example"}},
		{"keeps saved origins", "https://customer.example", []string{"https://shop.example"}, []string{"https://shop.example"}},
		{"keeps allow all", "https://customer.example", []string{"*"}, []string{"*"}},
		{"no website", "", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, installations, _ := setupWidgetOriginDefaultsTest(t, "")
			ctx := context.Background()
			inst, _, err := svc.GetInstallation(ctx, "ws-origin-defaults")
			if err != nil {
				t.Fatal(err)
			}
			inst.AllowedOrigins = model.DocsStringArray(tc.saved)
			if err := installations.Update(ctx, inst); err != nil {
				t.Fatal(err)
			}
			if tc.website != "" {
				if _, err := svc.workspaceRepo.Update(ctx, "ws-origin-defaults", nil, nil, nil, ptr(tc.website), nil, nil); err != nil {
					t.Fatal(err)
				}
			}
			if err := svc.ApplyWebsiteOriginDefaults(ctx, "ws-origin-defaults"); err != nil {
				t.Fatal(err)
			}
			got, err := installations.GetByWorkspace(ctx, "ws-origin-defaults")
			if err != nil {
				t.Fatal(err)
			}
			if len(got.AllowedOrigins) != len(tc.want) {
				t.Fatalf("allowed origins = %q, want %q", got.AllowedOrigins, tc.want)
			}
			for i := range tc.want {
				if got.AllowedOrigins[i] != tc.want[i] {
					t.Fatalf("allowed origins = %q, want %q", got.AllowedOrigins, tc.want)
				}
			}
		})
	}
}

func TestNormalizeAllowedOriginsSubdomainPatterns(t *testing.T) {
	got, err := normalizeAllowedOrigins([]string{" https://*.Customer.Example ", "https://customer.example", "*"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"*", "https://*.customer.example", "https://customer.example"}
	if len(got) != len(want) {
		t.Fatalf("normalized = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalized = %q, want %q", got, want)
		}
	}
	for _, raw := range []string{"https://*.co.uk", "http://*.customer.example", "https://*.localhost"} {
		if _, err := normalizeAllowedOrigins([]string{raw}); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

type recordingWebsiteDefaults struct{ calls []string }

func (r *recordingWebsiteDefaults) ApplyWebsiteOriginDefaults(_ context.Context, workspaceID string) error {
	r.calls = append(r.calls, workspaceID)
	return nil
}

func TestWorkspaceUpdateAppliesWebsiteOriginDefaults(t *testing.T) {
	db := newTestDB(t)
	seedWorkspace(t, db, "ws-website-hook", "Website hook", "website-hook", "user-website-hook")
	svc := NewWorkspaceService(repository.NewWorkspaceRepository(db), nil, nil)
	hook := &recordingWebsiteDefaults{}
	svc.SetWebsiteDefaultsApplier(hook)
	ctx := context.Background()
	for _, req := range []model.UpdateWorkspaceRequest{{Name: ptr("Renamed")}, {WebsiteURL: ptr("")}} {
		if _, err := svc.Update(ctx, "ws-website-hook", req, "user-website-hook"); err != nil {
			t.Fatal(err)
		}
	}
	if len(hook.calls) != 0 {
		t.Fatalf("hook ran without a website: %q", hook.calls)
	}
	if _, err := svc.Update(ctx, "ws-website-hook", model.UpdateWorkspaceRequest{WebsiteURL: ptr("customer.example")}, "user-website-hook"); err != nil {
		t.Fatal(err)
	}
	if len(hook.calls) != 1 || hook.calls[0] != "ws-website-hook" {
		t.Fatalf("hook calls = %q", hook.calls)
	}
}
