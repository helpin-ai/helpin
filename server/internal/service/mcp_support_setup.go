package service

import (
	"context"
	"net/url"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type mcpSupportSetupReader interface {
	SupportSetup(context.Context, string, SetupAccess) (model.SupportSetupGuide, error)
}

// SetSupportSetup connects the shared read-only onboarding evidence service.
func (s *MCPService) SetSupportSetup(reader mcpSupportSetupReader) { s.supportSetup = reader }

func supportSetupMCPToolDefinition() MCPToolDefinition {
	return MCPToolDefinition{
		Name: "get_support_setup", Title: "Inspect support setup",
		Description: "Start or resume customer support onboarding. Read current configuration checks, permission-aware browser handoff links, and instructions for email, website chat, knowledge, inboxes, routing, AI, and end-to-end testing. No settings are changed. Requires support administration permission. Use the returned links with your own browser tools or hand them to the user, then call again to verify progress.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
		Toolset:     MCPToolsetSupport, Scope: MCPScopeSupportRead, Permission: authorization.PermSupportAdmin, Module: model.ModuleSupport,
	}
}

func (s *MCPService) getMCPSupportSetup(ctx context.Context, principal *model.MCPPrincipal, actor *authorization.Actor) (*MCPToolResult, error) {
	if s.supportSetup == nil {
		return nil, ErrMCPForbidden
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, principal.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if workspace == nil {
		return nil, ErrMCPNotFound
	}
	access := SetupAccess{Permissions: map[string]bool{}, Modules: map[string]bool{}}
	for _, permission := range s.authz.PermissionsForActor(actor) {
		access.Permissions[string(permission)] = true
	}
	modules, err := s.authz.AccessibleModules(ctx, actor)
	if err != nil {
		return nil, err
	}
	for _, module := range modules {
		access.Modules[string(module)] = true
	}
	// Do not disclose Docs readiness outside the connection's approved grants.
	hasDocsGrant := containsMCPValue(principal.Toolsets, MCPToolsetDocs) && containsMCPValue(principal.Scopes, MCPScopeDocsRead)
	if !hasDocsGrant {
		access.Modules["docs"] = false
	}
	guide, err := s.supportSetup.SupportSetup(ctx, principal.WorkspaceID, access)
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(s.config.AppBaseURL, "/") + "/w/" + url.PathEscape(workspace.Slug)
	for i := range guide.Steps {
		if !hasDocsGrant && guide.Steps[i].Key == "support.help_docs_ready" {
			guide.Steps[i].BlockedReason = "This connection needs the Docs toolset and Docs read access to inspect help articles."
		}
		if guide.Steps[i].Path != "" {
			guide.Steps[i].Path = base + guide.Steps[i].Path
		}
	}
	return &MCPToolResult{Summary: "Current support setup checks. Configuration does not prove an end-to-end test passed.", Data: guide, Links: map[string]string{"workspace": base, "mcp_access": base + "/settings/mcp"}}, nil
}
