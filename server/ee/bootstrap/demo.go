//go:build ee

package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/ee/demo"
	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/edition"
)

// Environment variables that turn on demo provisioning. Which workspace may
// ask for demos and which account owns them is deployment configuration, not
// source.
const (
	// envDemoHomeWorkspaces lists, comma-separated, the IDs of the workspaces
	// allowed to build demos. Empty leaves the feature off.
	envDemoHomeWorkspaces = "DEMO_PROVISIONING_WORKSPACE_IDS"
	// envDemoOwnerEmail is the account that owns the demo organization and
	// every demo workspace.
	envDemoOwnerEmail = "DEMO_PROVISIONING_OWNER_EMAIL"
	// envDemoOrganizationName names the organization that holds the demo
	// workspaces. Optional; the default is "Demos".
	envDemoOrganizationName = "DEMO_PROVISIONING_ORGANIZATION_NAME"
)

// demoConfigFromEnv reads the demo provisioning settings.
func demoConfigFromEnv(hostedDomain string) demo.Config {
	var workspaces []string
	for _, id := range strings.Split(os.Getenv(envDemoHomeWorkspaces), ",") {
		if id = strings.TrimSpace(id); id != "" {
			workspaces = append(workspaces, id)
		}
	}
	return demo.Config{
		HomeWorkspaceIDs:       workspaces,
		OwnerEmail:             os.Getenv(envDemoOwnerEmail),
		OrganizationName:       os.Getenv(envDemoOrganizationName),
		HelpcenterHostedDomain: hostedDomain,
	}
}

// demoProvisioning holds the demo service once the product services it
// drives exist. The API builds those after the edition is constructed, so
// the service arrives through extend and the worker starts only if it did.
type demoProvisioning struct {
	service *demo.Service
}

// extend builds the demo service on the product's services and adds its
// tools to the public MCP server. With the feature off it does nothing.
func (d *demoProvisioning) extend(ext edition.Extension) error {
	cfg := demoConfigFromEnv(ext.HelpcenterHostedDomain)
	if !cfg.Enabled() {
		return nil
	}
	if ext.DB == nil || ext.MCP == nil || ext.Workspaces == nil || ext.Organizations == nil || ext.Users == nil ||
		ext.Helpcenter == nil || ext.DocsImport == nil || ext.DocsSpaces == nil || ext.DocsCollections == nil ||
		ext.SupportInbox == nil || ext.CRMCompanies == nil || ext.CRMActivities == nil {
		return fmt.Errorf("demo provisioning is configured but a product service it needs is missing")
	}
	service := demo.NewService(cfg, demo.Deps{
		Store:         eerepository.NewDemoRepository(ext.DB),
		Users:         ext.Users,
		Organizations: ext.Organizations,
		Workspaces:    ext.Workspaces,
		Helpcenter:    ext.Helpcenter,
		Imports:       ext.DocsImport,
		Spaces:        ext.DocsSpaces,
		Collections:   ext.DocsCollections,
		Support:       ext.SupportInbox,
		Companies:     ext.CRMCompanies,
		Activities:    ext.CRMActivities,
	})
	if err := ext.MCP.RegisterToolExtensions(service.MCPTools()...); err != nil {
		return fmt.Errorf("register demo tools: %w", err)
	}
	d.service = service
	slog.Info("demo provisioning enabled", "home_workspaces", len(cfg.HomeWorkspaceIDs))
	return nil
}

// run builds queued demos until ctx ends. It returns at once when the
// feature is off.
func (d *demoProvisioning) run(ctx context.Context) {
	if d.service == nil {
		return
	}
	d.service.Run(ctx, 3*time.Second)
}
