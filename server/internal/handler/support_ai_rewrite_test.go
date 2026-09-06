package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type timedOutRewriteProvider struct{}

func (timedOutRewriteProvider) ChatCompletion(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, &llm.ProviderError{Err: context.DeadlineExceeded}
}

func TestSupportDraftRewriteTimeoutPreservesDraftWithActionableError(t *testing.T) {
	svc := service.NewSupportAIService(timedOutRewriteProvider{}, nil, "", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "")
	h := &SupportAIHandler{aiService: svc}
	for _, tt := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"existing conversation", h.RewriteSupportDraft},
		{"new conversation", h.RewriteNewSupportDraft},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/rewrite-draft", strings.NewReader(`{"content":"Original draft","operation":"fix_grammar"}`))
			ctx := middleware.WithWorkspaceID(req.Context(), "ws-1")
			routeCtx := chi.NewRouteContext()
			routeCtx.URLParams.Add("id", "conv-1")
			req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, routeCtx))
			recorder := httptest.NewRecorder()
			tt.handler(recorder, req)
			if recorder.Code != http.StatusGatewayTimeout {
				t.Errorf("status = %d, want 504", recorder.Code)
			}
			if !strings.Contains(recorder.Body.String(), "Your draft is unchanged") {
				t.Errorf("response = %s, want actionable timeout error", recorder.Body.String())
			}
		})
	}
}
