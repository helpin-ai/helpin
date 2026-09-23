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
)

type roleMemberRepo struct{}

// The user ID doubles as the workspace role so each case can pick one.
func (roleMemberRepo) GetMembership(_ context.Context, workspace, user string) (*authorization.MemberInfo, error) {
	if workspace != "workspace" {
		return nil, nil
	}
	return &authorization.MemberInfo{ID: user, Role: user, Status: "active"}, nil
}

func (roleMemberRepo) GetTeamMemberships(context.Context, string) ([]authorization.TeamRole, error) {
	return nil, nil
}

// Revealing or rotating the widget identity signing secret must require the
// support admin permission through the real registered middleware chain.
func TestWidgetSigningSecretRoutesRequireSupportAdmin(t *testing.T) {
	jwt := auth.NewJWTManager("test-secret")
	authz := authorization.NewAuthzService(nil, roleMemberRepo{}, nil)
	router := New(Handlers{}, jwt, authz, nil, nil)
	for _, path := range []string{
		"/api/support/inbox/installations/reveal-secret",
		"/api/support/inbox/installations/rotate-secret",
	} {
		var target http.Handler
		if err := chi.Walk(router, func(method, route string, _ http.Handler, middlewares ...func(http.Handler) http.Handler) error {
			if method != http.MethodPost || strings.TrimRight(route, "/") != path {
				return nil
			}
			target = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
			for i := len(middlewares) - 1; i >= 0; i-- {
				target = middlewares[i](target)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if target == nil {
			t.Fatalf("route missing: POST %s", path)
		}
		for _, tc := range []struct {
			user string
			want int
		}{
			{"viewer", http.StatusForbidden},
			{"member", http.StatusForbidden},
			{"admin", http.StatusNoContent},
			{"owner", http.StatusNoContent},
			{"", http.StatusUnauthorized},
		} {
			t.Run(path+"/"+tc.user, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, path+"?workspace_id=workspace", nil)
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
}
