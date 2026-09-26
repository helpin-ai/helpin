package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type askMemberRepo struct{}

func (askMemberRepo) GetMembership(_ context.Context, workspace, user string) (*authorization.MemberInfo, error) {
	if workspace != "workspace" || user == "outsider" {
		return nil, nil
	}
	status := "active"
	if user == "inactive" {
		status = "inactive"
	}
	return &authorization.MemberInfo{ID: user, Role: model.RoleMember, Status: status}, nil
}
func (askMemberRepo) GetTeamMemberships(context.Context, string) ([]authorization.TeamRole, error) {
	return nil, nil
}

// Exercise the real registered middleware chain, replacing only the final
// handler. Default selection itself is covered by the service tests.
func TestAskAgentDefaultsDoesNotRequireAutomationAccess(t *testing.T) {
	jwt := auth.NewJWTManager("test-secret")
	authz := authorization.NewAuthzService(nil, askMemberRepo{}, nil)
	router := New(Handlers{}, jwt, authz, nil, nil)
	for _, tc := range []struct {
		path, user string
		want       int
	}{
		{"/api/dock/ai-defaults", "member", http.StatusNoContent},
		{"/api/dock/ai-defaults", "outsider", http.StatusForbidden},
		{"/api/dock/ai-defaults", "inactive", http.StatusForbidden},
		{"/api/dock/ai-defaults", "", http.StatusUnauthorized},
		{"/api/automation/agents", "member", http.StatusForbidden},
	} {
		t.Run(tc.path+"/"+tc.user, func(t *testing.T) {
			var target http.Handler
			err := chi.Walk(router, func(method, route string, _ http.Handler, middlewares ...func(http.Handler) http.Handler) error {
				if method != http.MethodGet || strings.TrimRight(route, "/") != tc.path {
					return nil
				}
				target = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
				for i := len(middlewares) - 1; i >= 0; i-- {
					target = middlewares[i](target)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if target == nil {
				t.Fatalf("route missing: %s", tc.path)
			}
			req := httptest.NewRequest(http.MethodGet, tc.path+"?workspace_id=workspace", nil)
			if tc.user != "" {
				token, _, err := jwt.GenerateTokenPair(tc.user, "test@example.com", false)
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+token)
			}
			response := httptest.NewRecorder()
			middleware.RequireAuth(jwt)(target).ServeHTTP(response, req)
			if response.Code != tc.want {
				t.Fatalf("status %d, want %d: %s", response.Code, tc.want, response.Body.String())
			}
		})
	}
}
