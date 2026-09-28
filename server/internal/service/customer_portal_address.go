package service

import (
	"context"
	"log/slog"
	"net/url"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// portalHelpcenterPath is where the portal is mounted on a help center host.
const portalHelpcenterPath = "/requests"

// portalAddress is where a workspace's portal is served. On a help center the
// portal is mounted at /requests with short paths (/requests/{reference});
// in the app it lives at /portal/{slug} (/portal/{slug}/requests/{reference}).
type portalAddress struct {
	base       string
	helpcenter bool
}

// home is the portal's public address: its request list.
func (a portalAddress) home() string { return a.base }

// callback is the sign-in link target for a one-time token.
func (a portalAddress) callback(secret string) string {
	return a.base + "/callback?token=" + url.QueryEscape(secret)
}

// request is the page for one request.
func (a portalAddress) request(reference string) string {
	if a.helpcenter {
		return a.base + "/" + url.PathEscape(reference)
	}
	return a.base + "/requests/" + url.PathEscape(reference)
}

// SetHelpcenterHostedDomain serves portals on workspaces' help centers, whose
// hosted subdomains live under domain (such as helpin.center).
func (s *CustomerPortalService) SetHelpcenterHostedDomain(domain string) {
	s.helpcenterDomain = strings.ToLower(strings.Trim(strings.TrimSpace(domain), "."))
}

// PublicURL is the workspace portal's public address.
func (s *CustomerPortalService) PublicURL(ctx context.Context, ws *PortalWorkspace) string {
	return s.address(ctx, ws.ID, ws.Slug).home()
}

// PortalRequestURL links to one request in the workspace's portal.
func (s *CustomerPortalService) PortalRequestURL(ctx context.Context, workspaceID, slug, reference string) string {
	address := s.address(ctx, workspaceID, slug)
	if reference == "" {
		return address.home()
	}
	return address.request(reference)
}

// HelpcenterPortalSlug returns the slug of the enabled portal served on the
// help center identified by its subdomain or custom domain.
func (s *CustomerPortalService) HelpcenterPortalSlug(ctx context.Context, identifier string) (string, error) {
	if s.helpcenterDomain == "" {
		return "", ErrPortalUnavailable
	}
	slug, err := s.repo.WorkspaceSlugForHelpcenter(ctx, identifier)
	if err != nil {
		return "", err
	}
	if slug == "" {
		return "", ErrPortalUnavailable
	}
	if _, err := s.ResolveWorkspace(ctx, slug); err != nil {
		return "", err
	}
	return slug, nil
}

func (s *CustomerPortalService) address(ctx context.Context, workspaceID, slug string) portalAddress {
	app := portalAddress{base: strings.TrimRight(s.baseURL, "/") + "/portal/" + url.PathEscape(slug)}
	if s.helpcenterDomain == "" {
		return app
	}
	helpcenter, err := s.repo.HelpcenterAddress(ctx, workspaceID)
	if err != nil {
		// Links must still work; the app address always does.
		slog.WarnContext(ctx, "portal help center address lookup failed", "workspace_id", workspaceID, "error", err)
		return app
	}
	base := helpcenterPublicBase(helpcenter, s.helpcenterDomain)
	if base == "" {
		return app
	}
	return portalAddress{base: base + portalHelpcenterPath, helpcenter: true}
}

// helpcenterPublicBase is the help center's public origin and base path, as
// its public URL mode sets it, or "" when it has none.
func helpcenterPublicBase(hc *repository.PortalHelpcenterAddress, hostedDomain string) string {
	if hc == nil {
		return ""
	}
	switch hc.PublicURLMode {
	case model.HelpcenterPublicURLModeCustomDomain:
		if domain := portalHostname(derefString(hc.CustomDomain)); domain != "" {
			return "https://" + domain
		}
	case model.HelpcenterPublicURLModeReverseProxy:
		host := portalHostname(derefString(hc.ReverseProxyHost))
		basePath := strings.TrimRight(strings.TrimSpace(derefString(hc.ReverseProxyBasePath)), "/")
		if host != "" && strings.HasPrefix(basePath, "/") {
			return "https://" + host + basePath
		}
	}
	if subdomain := strings.ToLower(strings.TrimSpace(hc.Subdomain)); subdomain != "" && validAskLabel(subdomain) {
		return "https://" + subdomain + "." + hostedDomain
	}
	return ""
}

// portalHostname reduces a stored host or URL to a bare lowercase hostname.
func portalHostname(value string) string {
	host := strings.ToLower(strings.TrimSpace(value))
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	return strings.TrimSuffix(host, ".")
}
