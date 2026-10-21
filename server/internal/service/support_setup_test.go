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

func TestSupportSetupUsesCurrentEvidenceAndKeepsTestingExplicit(t *testing.T) {
	access := SetupAccess{Unrestricted: true}
	guide := buildSupportSetupGuide("workspace-1", SetupEvidence{}, access)
	if len(guide.Steps) != 8 {
		t.Fatalf("steps = %d", len(guide.Steps))
	}
	if guide.Steps[0].Status != "available" || guide.Steps[1].Status != "available" {
		t.Fatal("new channels should be available")
	}
	for _, step := range guide.Steps {
		if step.Status == "completed" {
			t.Fatalf("new step completed: %s", step.Key)
		}
	}
	evidence := SetupEvidence{SupportEmailInboxCount: 1, LiveChatInstallationCount: 1, PublicHelpDocCount: 1, BrandKnowledgeSourceCount: 1, TeamInboxCount: 1, AutomaticRoutingCount: 1, SupportAIAgentActive: true}
	guide = buildSupportSetupGuide("workspace-1", evidence, access)
	for _, step := range guide.Steps[:7] {
		if step.Status != "completed" {
			t.Fatalf("%s status = %s", step.Key, step.Status)
		}
	}
	if guide.Steps[7].Status != "unable_to_verify" {
		t.Fatal("configuration must not imply an end-to-end test passed")
	}
	guide = buildSupportSetupGuide("workspace-1", SetupEvidence{}, access)
	if guide.Steps[0].Status == "completed" {
		t.Fatal("stale achievement must not hide a disconnected channel")
	}
}
func TestSupportSetupHidesUnauthorizedEvidenceAndActions(t *testing.T) {
	guide := buildSupportSetupGuide("workspace-1", SetupEvidence{PublicHelpDocCount: 3}, SetupAccess{Modules: map[string]bool{"support": true}, Permissions: map[string]bool{"support.admin": true}})
	for _, step := range guide.Steps {
		if step.Key == "support.help_docs_ready" && (step.Status != "blocked" || step.Path != "") {
			t.Fatalf("unauthorized docs: %#v", step)
		}
	}
	for _, step := range buildSupportSetupGuide("workspace-1", SetupEvidence{}, SetupAccess{}).Steps {
		if step.Path != "" {
			t.Fatalf("unauthorized action: %#v", step)
		}
	}
}
func TestSupportSetupCatalogRequiresAdminAndRejectsWorkspaceOverride(t *testing.T) {
	var found *MCPToolDefinition
	for _, tool := range PublicToolCatalog() {
		if tool.Name == "get_support_setup" {
			copy := tool
			found = &copy
		}
	}
	if found == nil {
		t.Fatal("get_support_setup missing")
	}
	if found.Mutating || found.Permission != authorization.PermSupportAdmin || found.Scope != MCPScopeSupportRead || found.Module != model.ModuleSupport {
		t.Fatalf("requirements = %#v", found)
	}
	resolved, err := resolveMCPToolSchema(found.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(map[string]any{"workspace_id": "other"}); err == nil {
		t.Fatal("workspace override accepted")
	}
	if err := resolved.Validate(map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(found.Description, "browser") {
		t.Fatal("browser handoff undiscoverable")
	}
}

func TestSupportSetupAuthorization(t *testing.T) {
	tool := supportSetupMCPToolDefinition()
	for _, tc := range []struct {
		name, role                                      string
		scope, toolset, enabled, reader, readOnly, want bool
	}{
		{"admin read-only inspection", "admin", true, true, true, true, true, true},
		{"owner inspection", "owner", true, true, true, true, false, true},
		{"member denied", "member", true, true, true, true, false, false},
		{"missing scope", "admin", false, true, true, true, false, false},
		{"missing toolset", "admin", true, false, true, true, false, false},
		{"disabled support", "admin", true, true, false, true, false, false},
		{"unwired setup", "admin", true, true, true, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &MCPService{authz: authorization.NewAuthzService(nil, nil, nil), config: MCPServiceConfig{SupportEnabled: tc.enabled}}
			if tc.reader {
				s.SetSupportSetup(&fakeSupportSetupReader{})
			}
			p := &model.MCPPrincipal{ReadOnly: tc.readOnly}
			if tc.scope {
				p.Scopes = []string{MCPScopeSupportRead}
			}
			if tc.toolset {
				p.Toolsets = []string{MCPToolsetSupport}
			}
			got := s.canUseTool(context.Background(), p, &authorization.Actor{Role: tc.role}, tool)
			if got != tc.want {
				t.Fatalf("allowed = %v, want %v", got, tc.want)
			}
		})
	}
}

type fakeSupportSetupReader struct{}

func (*fakeSupportSetupReader) SupportSetup(_ context.Context, workspaceID string, access SetupAccess) (model.SupportSetupGuide, error) {
	return buildSupportSetupGuide(workspaceID, SetupEvidence{}, access), nil
}

func TestSupportSetupMCPBindsWorkspaceAndFiltersDocsGrant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, slug TEXT, name TEXT, deleted_at DATETIME)`,
		`INSERT INTO workspaces (id, slug, name) VALUES ('workspace-1','acme','Acme'),('workspace-2','other','Other')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &MCPService{workspaceRepo: repository.NewWorkspaceRepository(db), authz: authorization.NewAuthzService(nil, nil, nil), config: MCPServiceConfig{AppBaseURL: "https://app.example.test/"}}
	s.SetSupportSetup(&fakeSupportSetupReader{})
	principal := &model.MCPPrincipal{WorkspaceID: "workspace-1", Toolsets: []string{MCPToolsetSupport}, Scopes: []string{MCPScopeSupportRead}}
	actor := &authorization.Actor{Role: "admin", WorkspaceID: "workspace-1"}
	result, err := s.executeSpecialMCPTool(context.Background(), principal, actor, "get_support_setup", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	guide := result.Data.(model.SupportSetupGuide)
	if guide.WorkspaceID != "workspace-1" {
		t.Fatalf("wrong workspace: %s", guide.WorkspaceID)
	}
	for _, step := range guide.Steps {
		if step.Key == "support.help_docs_ready" && (step.Status != model.SetupTaskBlocked || step.Path != "") {
			t.Fatalf("Docs scope leaked: %#v", step)
		}
		if step.Path != "" && !strings.HasPrefix(step.Path, "https://app.example.test/w/acme/") {
			t.Fatalf("wrong handoff: %s", step.Path)
		}
	}
	principal.Toolsets = append(principal.Toolsets, MCPToolsetDocs)
	principal.Scopes = append(principal.Scopes, MCPScopeDocsRead)
	result, err = s.getMCPSupportSetup(context.Background(), principal, actor)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Data.(model.SupportSetupGuide).Steps {
		if step.Key == "support.help_docs_ready" && step.Path != "https://app.example.test/w/acme/docs" {
			t.Fatalf("Docs handoff missing: %#v", step)
		}
	}
}
