package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func addPortalHelpcenter(t *testing.T, db *gorm.DB, workspaceID, subdomain, mode, customDomain, proxyHost, proxyPath string) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS docs_helpcenter_configs (id TEXT PRIMARY KEY, workspace_id TEXT UNIQUE, subdomain TEXT, custom_domain TEXT, public_url_mode TEXT, reverse_proxy_host TEXT, reverse_proxy_base_path TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`DELETE FROM docs_helpcenter_configs WHERE workspace_id = ?`, workspaceID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO docs_helpcenter_configs (id, workspace_id, subdomain, custom_domain, public_url_mode, reverse_proxy_host, reverse_proxy_base_path) VALUES (?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''))`,
		uuid.NewString(), workspaceID, subdomain, customDomain, mode, proxyHost, proxyPath).Error; err != nil {
		t.Fatal(err)
	}
}

func TestPortalIsServedOnTheHelpCenterAtRequests(t *testing.T) {
	ctx := context.Background()
	svc, db, _, ws := setupCustomerPortalService(t)
	svc.baseURL = "https://app.helpin.ai"
	svc.SetHelpcenterHostedDomain("helpin.center")

	if got := svc.PublicURL(ctx, ws); got != "https://app.helpin.ai/portal/acme" {
		t.Fatalf("without a help center = %q, want the app path", got)
	}
	for _, tc := range []struct {
		name, mode, custom, proxyHost, proxyPath, want string
	}{
		{"hosted subdomain", model.HelpcenterPublicURLModeHostedSubdomain, "", "", "", "https://acme-help.helpin.center/requests"},
		{"custom domain", model.HelpcenterPublicURLModeCustomDomain, "Help.Acme.com", "", "", "https://help.acme.com/requests"},
		{"reverse proxy subpath", model.HelpcenterPublicURLModeReverseProxy, "", "acme.com", "/docs/", "https://acme.com/docs/requests"},
		{"custom mode without a domain", model.HelpcenterPublicURLModeCustomDomain, "", "", "", "https://acme-help.helpin.center/requests"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addPortalHelpcenter(t, db, ws.ID, "acme-help", tc.mode, tc.custom, tc.proxyHost, tc.proxyPath)
			if got := svc.PublicURL(ctx, ws); got != tc.want {
				t.Fatalf("PublicURL = %q, want %q", got, tc.want)
			}
		})
	}

	addPortalHelpcenter(t, db, ws.ID, "acme-help", model.HelpcenterPublicURLModeHostedSubdomain, "", "", "")
	if got := svc.PortalRequestURL(ctx, ws.ID, ws.Slug, "req_1"); got != "https://acme-help.helpin.center/requests/req_1" {
		t.Fatalf("request link = %q", got)
	}
	if got := svc.PortalRequestURL(ctx, ws.ID, ws.Slug, ""); got != "https://acme-help.helpin.center/requests" {
		t.Fatalf("list link = %q", got)
	}
	if got := svc.Configuration(ctx, ws).PublicURL; got != "https://acme-help.helpin.center/requests" {
		t.Fatalf("configuration public_url = %q", got)
	}

	// Without a hosted help center domain (Community), portals stay in the app.
	svc.SetHelpcenterHostedDomain("")
	if got := svc.PortalRequestURL(ctx, ws.ID, ws.Slug, "req_1"); got != "https://app.helpin.ai/portal/acme/requests/req_1" {
		t.Fatalf("app request link = %q", got)
	}
}

func TestPortalSignInLinksOpenOnTheHelpCenter(t *testing.T) {
	ctx := context.Background()
	svc, db, sender, ws := setupCustomerPortalService(t)
	svc.SetHelpcenterHostedDomain("helpin.center")
	addPortalHelpcenter(t, db, ws.ID, "acme-help", model.HelpcenterPublicURLModeHostedSubdomain, "", "", "")
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	svc.RequestLink(ctx, ws, "customer@example.com")
	if len(sender.sent) != 1 || !strings.Contains(sender.sent[0].text, "https://acme-help.helpin.center/requests/callback?token=") {
		t.Fatalf("sign-in email = %+v", sender.sent)
	}
}

func TestHelpcenterHostsFindTheirPortal(t *testing.T) {
	ctx := context.Background()
	svc, db, _, ws := setupCustomerPortalService(t)
	addPortalHelpcenter(t, db, ws.ID, "acme-help", model.HelpcenterPublicURLModeCustomDomain, "help.acme.com", "", "")
	if _, err := svc.HelpcenterPortalSlug(ctx, "acme-help"); !errors.Is(err, ErrPortalUnavailable) {
		t.Fatalf("without a hosted help center domain = %v", err)
	}
	svc.SetHelpcenterHostedDomain("helpin.center")
	for _, identifier := range []string{"acme-help", "ACME-HELP", "help.acme.com"} {
		if slug, err := svc.HelpcenterPortalSlug(ctx, identifier); err != nil || slug != "acme" {
			t.Errorf("HelpcenterPortalSlug(%q) = %q, %v", identifier, slug, err)
		}
	}
	if _, err := svc.HelpcenterPortalSlug(ctx, "unknown"); !errors.Is(err, ErrPortalUnavailable) {
		t.Fatalf("unknown help center = %v", err)
	}
	if err := db.Exec(`UPDATE support_widget_installations SET settings = '{"portal_enabled":false}' WHERE workspace_id = ?`, ws.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.HelpcenterPortalSlug(ctx, "acme-help"); !errors.Is(err, ErrPortalUnavailable) {
		t.Fatalf("disabled portal = %v", err)
	}
}
