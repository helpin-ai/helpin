package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type docsRewriteHandlerProvider struct {
	err   error
	calls int
}

func (p *docsRewriteHandlerProvider) ChatCompletion(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	panic("must use governed completion")
}
func (p *docsRewriteHandlerProvider) Complete(context.Context, service.AICompletionRequest) (*llm.ChatResponse, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return &llm.ChatResponse{Content: `{"content":"<p>Shorter.</p>"}`}, nil
}
func TestDocsRewriteHandler(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		err        error
		status     int
	}{
		{"preview", `{"content":"<p>Original.</p>","operation":"shorten"}`, nil, 200},
		{"invalid action", `{"content":"text","operation":"translate"}`, nil, 400},
		{"malformed", "{", nil, 400},
		{"timeout", `{"content":"text","operation":"expand"}`, context.DeadlineExceeded, 504},
		{"failure", `{"content":"text","operation":"expand"}`, errors.New("provider failed"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &docsRewriteHandlerProvider{err: tc.err}
			h := &DocsHandler{}
			h.SetAIRewriteService(service.NewDocsAIRewriteService(p))
			req := httptest.NewRequest(http.MethodPost, "/docs/rewrite-selection", strings.NewReader(tc.body))
			req = req.WithContext(middleware.WithWorkspaceID(req.Context(), "workspace"))
			result := httptest.NewRecorder()
			h.RewriteSelection(result, req)
			if result.Code != tc.status {
				t.Fatalf("status %d: %s", result.Code, result.Body.String())
			}
			if tc.status == 400 && p.calls != 0 {
				t.Fatal("invalid request called AI")
			}
			if tc.status == 200 && !strings.Contains(result.Body.String(), "Shorter.") {
				t.Fatal("candidate missing")
			}
		})
	}
}
