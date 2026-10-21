package service

import (
	"context"
	"maps"
	"net/url"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type mcpWorkspaceSetupReader interface {
	WorkspaceSetup(context.Context, string, SetupAccess) (model.WorkspaceSetupGuide, error)
}

func (s *MCPService) SetWorkspaceSetup(reader mcpWorkspaceSetupReader) { s.workspaceSetup = reader }

func workspaceSetupMCPToolDefinition() MCPToolDefinition {
	return MCPToolDefinition{
		Name: "get_workspace_setup", Title: "Inspect workspace setup",
		Description: "Start or resume Helpin onboarding. Inspect company context, teams, invitations and membership, plus the workspace's selected goals (support/help desk, help center, internal docs, projects, CRM or automation). Returns current checks, permission-aware browser links and detailed setup instructions. Does not change settings. Requires Context read access; module checks also require that module's read grant. Use your client's browser tools or give the links to the user, then inspect again after saved changes.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
		Toolset:     MCPToolsetContext, Scope: MCPScopeContextRead, Permission: authorization.PermWorkspaceRead,
	}
}

func (s *MCPService) workspaceSetupMCPAccess(principal *model.MCPPrincipal, access SetupAccess) SetupAccess {
	access.Modules = maps.Clone(access.Modules)
	if access.Modules == nil {
		access.Modules = map[string]bool{}
	}
	for _, grant := range []struct {
		module, toolset, scope string
		enabled                bool
	}{
		{"pm", MCPToolsetPM, MCPScopePMRead, true},
		{"docs", MCPToolsetDocs, MCPScopeDocsRead, true},
		{"support", MCPToolsetSupport, MCPScopeSupportRead, s.config.SupportEnabled},
		{"crm", MCPToolsetCRM, MCPScopeCRMRead, s.config.CRMEnabled},
		{"automation", MCPToolsetAgents, MCPScopeAgentsRead, true},
	} {
		access.Modules[grant.module] = access.Modules[grant.module] && grant.enabled && containsMCPValue(principal.Toolsets, grant.toolset) && containsMCPValue(principal.Scopes, grant.scope)
	}
	return access
}

func (s *MCPService) getMCPWorkspaceSetup(ctx context.Context, principal *model.MCPPrincipal, actor *authorization.Actor) (*MCPToolResult, error) {
	if s.workspaceSetup == nil {
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
	guide, err := s.workspaceSetup.WorkspaceSetup(ctx, principal.WorkspaceID, s.workspaceSetupMCPAccess(principal, access))
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(s.config.AppBaseURL, "/") + "/w/" + url.PathEscape(workspace.Slug)
	for i := range guide.Sections {
		for j := range guide.Sections[i].Steps {
			step := &guide.Sections[i].Steps[j]
			if step.Path != "" {
				step.Path = base + step.Path
			}
		}
	}
	return &MCPToolResult{Summary: "Current workspace setup checks and browser handoffs. No configuration changed; completed checks do not prove delivery or real-world tests.", Data: guide, Links: map[string]string{"workspace": base, "setup": base + "/setup", "mcp_access": base + "/settings/mcp", "ai_models": base + "/settings/ai"}}, nil
}
