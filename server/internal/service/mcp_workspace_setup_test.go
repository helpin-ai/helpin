package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWorkspaceSetupMCPDiscoveryAndGrants(t *testing.T) {
	tool := workspaceSetupMCPToolDefinition()
	if tool.Mutating || tool.Scope != MCPScopeContextRead || tool.Toolset != MCPToolsetContext {
		t.Fatalf("wrong inspection requirements: %#v", tool)
	}
	schema, err := resolveMCPToolSchema(tool.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	if schema.Validate(map[string]any{"workspace_id": "other"}) == nil {
		t.Fatal("workspace override allowed")
	}
	s := &MCPService{authz: authorization.NewAuthzService(nil, nil, nil), config: MCPServiceConfig{SupportEnabled: true, CRMEnabled: true}}
	s.SetWorkspaceSetup(&fakeWorkspaceSetupReader{})
	p := &model.MCPPrincipal{Toolsets: []string{MCPToolsetContext}, Scopes: []string{MCPScopeContextRead}, ReadOnly: true}
	if !s.canUseTool(context.Background(), p, &authorization.Actor{Role: "owner"}, tool) {
		t.Fatal("read-only context inspection denied")
	}
	access := SetupAccess{Permissions: map[string]bool{"support.admin": true, "docs.edit": true, "pm.edit": true, "workspace.update": true}, Modules: map[string]bool{"support": true, "docs": true, "pm": true, "automation": true, "crm": true}}
	filtered := s.workspaceSetupMCPAccess(p, access)
	guide := buildWorkspaceSetupGuide("ws-1", []string{model.SetupGoalCustomerSupport, model.SetupGoalProductDelivery, model.SetupGoalHelpCenterDocs}, SetupEvidence{PublicHelpDocCount: 3}, 0, filtered)
	if workspaceSetupStep(t, guide, "foundation.company_context_ready").Path == "" {
		t.Fatal("context grant lost")
	}
	for _, key := range []string{"support.email_inbox_connected", "support.help_docs_ready", "product.project_planned", "help_center.site_published"} {
		if workspaceSetupStep(t, guide, key).Status != model.SetupTaskBlocked {
			t.Fatalf("module grant leaked %s", key)
		}
	}
	if !access.Modules["docs"] {
		t.Fatal("mutated original access")
	}
	p.Toolsets = append(p.Toolsets, MCPToolsetSupport)
	p.Scopes = append(p.Scopes, MCPScopeSupportRead)
	filtered = s.workspaceSetupMCPAccess(p, access)
	if !filtered.Modules["support"] || filtered.Modules["docs"] {
		t.Fatal("incorrect support-only projection")
	}
	s.config.SupportEnabled = false
	if s.workspaceSetupMCPAccess(p, access).Modules["support"] {
		t.Fatal("disabled module exposed")
	}
	p.Scopes = nil
	if s.canUseTool(context.Background(), p, &authorization.Actor{Role: "owner"}, tool) {
		t.Fatal("missing context scope allowed")
	}
}

func TestDefaultMCPReadAccessCoversSetupModules(t *testing.T) {
	s := &MCPService{config: MCPServiceConfig{SupportEnabled: true, CRMEnabled: true}}
	p := &model.MCPPrincipal{Toolsets: DefaultMCPToolsets(), Scopes: DefaultMCPScopes(), ReadOnly: true}
	modules := map[string]bool{"pm": true, "docs": true, "support": true, "crm": true, "automation": true}
	access := s.workspaceSetupMCPAccess(p, SetupAccess{Modules: modules})
	for module := range modules {
		if !access.Modules[module] {
			t.Errorf("default read access blocks %s setup", module)
		}
	}
	if hasMCPWriteScope(p.Scopes) {
		t.Fatal("default read access unexpectedly grants writes")
	}
	s.config.SupportEnabled = false
	if s.workspaceSetupMCPAccess(p, SetupAccess{Modules: modules}).Modules["support"] {
		t.Fatal("default scopes bypassed disabled support module")
	}
}

type fakeWorkspaceSetupReader struct{}

func (*fakeWorkspaceSetupReader) WorkspaceSetup(_ context.Context, id string, access SetupAccess) (model.WorkspaceSetupGuide, error) {
	return buildWorkspaceSetupGuide(id, []string{model.SetupGoalCustomerSupport}, SetupEvidence{}, 0, access), nil
}

func TestWorkspaceSetupMCPBindsIdentityAndLinks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{`CREATE TABLE workspaces (id TEXT PRIMARY KEY,slug TEXT,name TEXT,deleted_at DATETIME)`, `INSERT INTO workspaces (id,slug,name) VALUES ('ws-1','acme','Acme'),('ws-2','other','Other')`} {
		if err = db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &MCPService{workspaceRepo: repository.NewWorkspaceRepository(db), authz: authorization.NewAuthzService(nil, nil, nil), config: MCPServiceConfig{AppBaseURL: "https://app.example.test/", SupportEnabled: true}}
	s.SetWorkspaceSetup(&fakeWorkspaceSetupReader{})
	p := &model.MCPPrincipal{WorkspaceID: "ws-1", Toolsets: []string{MCPToolsetContext}, Scopes: []string{MCPScopeContextRead}}
	result, err := s.executeSpecialMCPTool(context.Background(), p, &authorization.Actor{Role: "admin", WorkspaceID: "ws-1"}, "get_workspace_setup", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	guide := result.Data.(model.WorkspaceSetupGuide)
	if guide.WorkspaceID != "ws-1" {
		t.Fatalf("wrong workspace: %s", guide.WorkspaceID)
	}
	for _, section := range guide.Sections {
		for _, step := range section.Steps {
			if step.Path != "" && !strings.HasPrefix(step.Path, "https://app.example.test/w/acme/") {
				t.Fatalf("wrong handoff: %s", step.Path)
			}
		}
	}
	if workspaceSetupStep(t, guide, "foundation.invitation_created").Path != "https://app.example.test/w/acme/settings/members" {
		t.Fatal("missing member handoff")
	}
	if workspaceSetupStep(t, guide, "support.email_inbox_connected").Status != model.SetupTaskBlocked {
		t.Fatal("ungranted support evidence disclosed")
	}
}
