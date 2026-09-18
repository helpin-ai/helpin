package authorization

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/deployment"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDeploymentModulesRestrictOwnersAndDirectRoutes(t *testing.T) {
	modules, err := deployment.ParseModules("support,docs,agents")
	if err != nil {
		t.Fatal(err)
	}
	authz := NewAuthzService(nil, nil, nil)
	authz.SetDeploymentModules(modules)
	for _, role := range []string{model.RoleOwner, model.RoleAdmin} {
		got, err := authz.AccessibleModules(context.Background(), &Actor{Role: role})
		if err != nil {
			t.Fatal(err)
		}
		for _, module := range modules {
			if !slices.Contains(got, module) {
				t.Errorf("%s missing %s: %v", role, module, got)
			}
		}
		for _, module := range []model.ModuleID{model.ModuleCRM, model.ModulePM, model.ModuleAutomation} {
			if slices.Contains(got, module) {
				t.Errorf("%s bypassed deployment policy for %s", role, module)
			}
		}
	}
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/api/pm/attachments/image/content", 200}, {"/api/pm/attachments-lookalike", 404}, {"/api/pm/associations", 404},
		{"/api/pm/tasks", 404}, {"/api/pm/tasks/task/run-agent", 404}, {"/api/automation/flows", 404}, {"/api/automation/library/triggers", 404}, {"/api/crm/deals", 404},
		{"/api/support/inbox/conversations", 200}, {"/api/docs/spaces", 200}, {"/api/automation/agents", 200}, {"/api/automation/library/tools", 200}, {"/api/automation/runs", 200}, {"/api/pm/agent-presets", 200}, {"/api/pm/agent-runs/run/messages", 200},
		{"/api/crm/contacts/contact", 200}, {"/api/crm/contacts-lookalike", 404}, {"/api/internal/agent-runtime/events", 200}, {"/api/settings/workspace", 200},
	} {
		t.Run(tc.path, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()
			RequireDeploymentAccess(authz)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })).ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status %d, want %d", w.Code, tc.want)
			}
		})
	}
}

func TestParseDeploymentModules(t *testing.T) {
	for _, raw := range []string{"typo", "support", "support,docs,", "docs,*"} {
		if _, err := deployment.ParseModules(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	got, err := deployment.ParseModules("docs,agents,docs")
	if err != nil || len(got) != 2 {
		t.Fatalf("parse=%v %v", got, err)
	}
}

func TestSharedAttachmentsRequireAnEnabledProduct(t *testing.T) {
	for _, tc := range []struct {
		name                                string
		modules                             []model.ModuleID
		attachmentStatus, associationStatus int
	}{
		{"Docs", []model.ModuleID{model.ModuleDocs}, 200, 404},
		{"PM", []model.ModuleID{model.ModulePM}, 200, 200},
		{"neither", []model.ModuleID{model.ModuleAgents}, 404, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewAuthzService(nil, nil, nil)
			s.SetDeploymentModules(tc.modules)
			for path, want := range map[string]int{"/api/pm/attachments/id/content": tc.attachmentStatus, "/api/pm/associations": tc.associationStatus} {
				w := httptest.NewRecorder()
				RequireDeploymentAccess(s)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
				if w.Code != want {
					t.Fatalf("%s status=%d want=%d", path, w.Code, want)
				}
			}
		})
	}
}
