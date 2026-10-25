package router

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/handler"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type iconMemberRepo struct{ askMemberRepo }

func (r iconMemberRepo) GetMembership(ctx context.Context, workspace, user string) (*authorization.MemberInfo, error) {
	member, err := r.askMemberRepo.GetMembership(ctx, workspace, user)
	if member != nil && user == "admin" {
		member.Role = model.RoleAdmin
	}
	return member, err
}
func TestIconSuggestionPermissions(t *testing.T) {
	jwt := auth.NewJWTManager("test-secret")
	authz := authorization.NewAuthzService(nil, iconMemberRepo{}, nil)
	router := New(Handlers{IconSuggestion: handler.NewIconSuggestionHandler(nil)}, jwt, authz, nil, nil)
	for _, tc := range []struct {
		path, user string
		want       int
	}{
		{"/api/docs/icon-suggestion", "member", 204},
		{"/api/docs/icon-suggestion", "outsider", 403},
		{"/api/docs/icon-suggestion", "", 401},
		{"/api/support/inbox/icon-suggestion", "member", 403},
		{"/api/support/inbox/icon-suggestion", "admin", 204},
		{"/api/support/inbox/icon-suggestion", "outsider", 403},
		{"/api/support/inbox/icon-suggestion", "", 401},
	} {
		t.Run(tc.path+tc.user, func(t *testing.T) {
			var target http.Handler
			err := chi.Walk(router, func(method, route string, _ http.Handler, middlewares ...func(http.Handler) http.Handler) error {
				if method != http.MethodPost || strings.TrimRight(route, "/") != tc.path {
					return nil
				}
				target = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
				for i := len(middlewares) - 1; i >= 0; i-- {
					target = middlewares[i](target)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if target == nil {
				t.Fatal("route missing")
			}
			req := httptest.NewRequest(http.MethodPost, tc.path+"?workspace_id=workspace", strings.NewReader(`{"name":"Billing"}`))
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
				t.Fatalf("status=%d want=%d body=%s", response.Code, tc.want, response.Body.String())
			}
		})
	}
}
