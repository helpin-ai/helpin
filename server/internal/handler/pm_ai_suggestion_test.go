package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type pmAISuggestionHandlerStore struct {
	calls int
	err   error
}

func (s *pmAISuggestionHandlerStore) List(context.Context, string, string, int) ([]model.PMAISuggestionItem, int64, error) {
	s.calls++
	return nil, 0, s.err
}
func (s *pmAISuggestionHandlerStore) Get(context.Context, string, string, string) (*model.PMAISuggestionDetail, error) {
	s.calls++
	return nil, s.err
}
func (s *pmAISuggestionHandlerStore) Decide(context.Context, string, string, string, string, string) (*model.PMAISuggestionDecisionResult, error) {
	s.calls++
	return &model.PMAISuggestionDecisionResult{Status: "accepted", ExecutionStatus: "manual_required"}, s.err
}

func TestPMAISuggestionHandlerPermissionAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body, role, status, workspace string
		want                                              int
	}{
		{"viewer can read", "GET", "/ai-suggestions", "", "viewer", "active", "ws", 200},
		{"viewer cannot decide", "POST", "/ai-suggestions/70000000-0000-4000-8000-000000000001/accept", `{"revision":"current"}`, "viewer", "active", "ws", 403},
		{"member can review", "POST", "/ai-suggestions/70000000-0000-4000-8000-000000000001/accept", `{"revision":"current"}`, "member", "active", "ws", 200},
		{"missing revision", "POST", "/ai-suggestions/70000000-0000-4000-8000-000000000001/accept", `{}`, "member", "active", "ws", 400},
		{"unsupported decision", "POST", "/ai-suggestions/70000000-0000-4000-8000-000000000001/send", `{"revision":"current"}`, "member", "active", "ws", 400},
		{"invalid id", "GET", "/ai-suggestions/not-uuid", "", "member", "active", "ws", 400},
		{"inactive member", "GET", "/ai-suggestions", "", "member", "inactive", "ws", 403},
		{"foreign workspace", "GET", "/ai-suggestions", "", "member", "active", "other", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &pmAISuggestionHandlerStore{}
			h := NewPMAISuggestionHandler(service.NewPMAISuggestionService(store))
			authz := authorization.NewAuthzService(nil, nil, nil)
			router := chi.NewRouter()
			router.With(authorization.RequirePermission(authz, authorization.PermPMRead)).Get("/ai-suggestions", h.List)
			router.With(authorization.RequirePermission(authz, authorization.PermPMRead)).Get("/ai-suggestions/{id}", h.Get)
			router.With(authorization.RequirePermission(authz, authorization.PermPMEdit)).Post("/ai-suggestions/{id}/{decision}", h.Decide)
			req := httptest.NewRequest(tc.method, tc.path+"?workspace_id=ws", strings.NewReader(tc.body))
			req = req.WithContext(authorization.WithActor(req.Context(), &authorization.Actor{UserID: "user", WorkspaceID: tc.workspace, WorkspaceMemberID: "member", Role: tc.role, Status: tc.status}))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != tc.want {
				t.Fatalf("response %d %s, want %d", response.Code, response.Body.String(), tc.want)
			}
			if tc.want >= 400 && store.calls != 0 {
				t.Fatal("invalid/unauthorized request reached repository")
			}
		})
	}
}
func TestPMAISuggestionHandlerStaleDecision(t *testing.T) {
	store := &pmAISuggestionHandlerStore{err: repository.ErrPMAISuggestionStale}
	h := NewPMAISuggestionHandler(service.NewPMAISuggestionService(store))
	router := chi.NewRouter()
	router.Post("/ai-suggestions/{id}/{decision}", h.Decide)
	req := httptest.NewRequest(http.MethodPost, "/ai-suggestions/70000000-0000-4000-8000-000000000001/accept?workspace_id=ws", strings.NewReader(`{"revision":"stale"}`))
	req = req.WithContext(authorization.WithActor(req.Context(), &authorization.Actor{UserID: "user", WorkspaceID: "ws", WorkspaceMemberID: "member", Role: "member", Status: "active"}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != 409 {
		t.Fatalf("stale status %d", response.Code)
	}
}
