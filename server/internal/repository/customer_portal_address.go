package repository

import (
	"context"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PortalHelpcenterAddress is where a workspace's help center is served. The
// customer portal is mounted on it at /requests.
type PortalHelpcenterAddress struct {
	Subdomain    string
	CustomDomain *string
	// CustomDomainLive is true once the custom domain is verified.
	CustomDomainLive     bool
	PublicURLMode        string
	ReverseProxyHost     *string
	ReverseProxyBasePath *string
}

// HelpcenterAddress returns the workspace's help center address, or nil when
// it has no help center.
func (r *CustomerPortalRepository) HelpcenterAddress(ctx context.Context, workspaceID string) (*PortalHelpcenterAddress, error) {
	var configs []model.DocsHelpcenterConfig
	if err := r.db.WithContext(ctx).
		Select("subdomain", "custom_domain", "custom_domain_status", "public_url_mode", "reverse_proxy_host", "reverse_proxy_base_path").
		Where("workspace_id = ?", workspaceID).Limit(1).Find(&configs).Error; err != nil {
		return nil, err
	}
	if len(configs) == 0 {
		return nil, nil
	}
	cfg := configs[0]
	return &PortalHelpcenterAddress{
		Subdomain: cfg.Subdomain, CustomDomain: cfg.CustomDomain, CustomDomainLive: cfg.CustomDomainLive(), PublicURLMode: cfg.PublicURLMode,
		ReverseProxyHost: cfg.ReverseProxyHost, ReverseProxyBasePath: cfg.ReverseProxyBasePath,
	}, nil
}

// WorkspaceSlugForHelpcenter returns the slug of the workspace whose help
// center has identifier as its subdomain or custom domain, or "".
func (r *CustomerPortalRepository) WorkspaceSlugForHelpcenter(ctx context.Context, identifier string) (string, error) {
	identifier = strings.ToLower(strings.TrimSpace(identifier))
	if identifier == "" {
		return "", nil
	}
	// Subdomains win over custom domains, as in help center resolution.
	for _, column := range []string{"hc.subdomain", "hc.custom_domain"} {
		var slugs []string
		query := r.db.WithContext(ctx).Table("docs_helpcenter_configs AS hc").
			Joins("JOIN workspaces AS ws ON ws.id = hc.workspace_id").
			Where("lower("+column+") = ?", identifier)
		if column == "hc.custom_domain" {
			// Only a verified custom domain serves the portal.
			query = query.Where("hc.custom_domain_status IN ?", liveHelpcenterDomainStatuses)
		}
		if err := query.Limit(1).Pluck("ws.slug", &slugs).Error; err != nil {
			return "", err
		}
		if len(slugs) > 0 {
			return slugs[0], nil
		}
	}
	return "", nil
}

// WorkspaceSlug returns the workspace's slug.
func (r *CustomerPortalRepository) WorkspaceSlug(ctx context.Context, workspaceID string) (string, error) {
	var ws model.Workspace
	if err := r.db.WithContext(ctx).Select("slug").Where("id = ?", workspaceID).First(&ws).Error; err != nil {
		return "", err
	}
	return ws.Slug, nil
}
