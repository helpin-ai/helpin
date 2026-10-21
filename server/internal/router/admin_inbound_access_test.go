package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/authorization"
)

func TestAdminInboundRoutesRequirePlatformAdminAndMFA(t *testing.T) {
	jwt := auth.NewJWTManager("test-secret")
	router := New(Handlers{}, jwt, authorization.NewAuthzService(nil, roleMemberRepo{}, nil), nil, nil)
	wantRoutes := map[string]bool{
		"GET /api/admin/inbound-emails":                                 false,
		"GET /api/admin/inbound-emails/{id}":                            false,
		"GET /api/admin/inbound-emails/{id}/conversations":              false,
		"GET /api/admin/inbound-emails/{id}/attachments/{attachmentID}": false,
		"POST /api/admin/inbound-emails/{id}/recover":                   false,
	}
	err := chi.Walk(router, func(method, path string, _ http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		key := method + " " + strings.TrimRight(path, "/")
		if _, exists := wantRoutes[key]; !exists {
			return nil
		}
		wantRoutes[key] = true
		var target http.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
		for i := len(middlewares) - 1; i >= 0; i-- {
			target = middlewares[i](target)
		}
		for _, tc := range []struct {
			name                      string
			authenticated, admin, mfa bool
			want                      int
		}{
			{"anonymous", false, false, false, http.StatusUnauthorized},
			{"workspace admin", true, false, true, http.StatusForbidden},
			{"platform admin without MFA", true, true, false, http.StatusForbidden},
			{"platform admin with MFA", true, true, true, http.StatusNoContent},
		} {
			t.Run(key+"/"+tc.name, func(t *testing.T) {
				req := httptest.NewRequest(method, path, nil)
				if tc.authenticated {
					token, _, err := jwt.GenerateTokenPair("test-admin", "admin@example.com", false, auth.WithPlatformAdmin(tc.admin), auth.WithMFASatisfied(tc.mfa))
					if err != nil {
						t.Fatal(err)
					}
					req.Header.Set("Authorization", "Bearer "+token)
				}
				response := httptest.NewRecorder()
				target.ServeHTTP(response, req)
				if response.Code != tc.want {
					t.Fatalf("status %d, want %d", response.Code, tc.want)
				}
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for route, found := range wantRoutes {
		if !found {
			t.Errorf("missing route %s", route)
		}
	}
}
