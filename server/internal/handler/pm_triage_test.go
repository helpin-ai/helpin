package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type pmTriageAnalyzerFunc func(context.Context, string, string, string) (*model.PMTriageView, error)

func (f pmTriageAnalyzerFunc) Analyze(ctx context.Context, workspace, kind, id string) (*model.PMTriageView, error) {
	return f(ctx, workspace, kind, id)
}

func TestPMTriageHandlerSanitizesErrors(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"provider", errors.New("secret provider response and SQL"), http.StatusServiceUnavailable},
		{"forbidden", &model.ErrForbidden{Message: "private team details"}, http.StatusForbidden},
		{"missing", service.ErrPMTriageSourceUnavailable, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewPMTriageHandler(pmTriageAnalyzerFunc(func(context.Context, string, string, string) (*model.PMTriageView, error) { return nil, tc.err }))
			router := chi.NewRouter()
			router.Post("/tasks/{id}/triage", h.AnalyzeTask)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/tasks/00000000-0000-4000-8000-000000000001/triage?workspace_id=00000000-0000-4000-8000-000000000002", nil))
			if recorder.Code != tc.status || strings.Contains(recorder.Body.String(), "secret") || strings.Contains(recorder.Body.String(), "private team") {
				t.Fatalf("unsafe response %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
func TestPMTriageHandlerValidatesIdentifiers(t *testing.T) {
	calls := 0
	h := NewPMTriageHandler(pmTriageAnalyzerFunc(func(context.Context, string, string, string) (*model.PMTriageView, error) { calls++; return nil, nil }))
	router := chi.NewRouter()
	router.Post("/tasks/{id}/triage", h.AnalyzeTask)
	for _, path := range []string{"/tasks/invalid/triage?workspace_id=00000000-0000-4000-8000-000000000002", "/tasks/00000000-0000-4000-8000-000000000001/triage?workspace_id=invalid"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid identifier returned %d", recorder.Code)
		}
	}
	if calls != 0 {
		t.Fatal("invalid request reached service")
	}
}
