package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type contextTestFetcher struct{ text string }

func (f contextTestFetcher) FetchText(context.Context, string) (string, error) { return f.text, nil }

type contextTestLLM struct{ err error }

func (l contextTestLLM) ChatCompletion(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	if l.err != nil {
		return nil, l.err
	}
	return &llm.ChatResponse{Content: "- Acme answers support questions.", FinishReason: "stop"}, nil
}

func TestGenerateCompanyProductDescriptionErrorContract(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		llm        llm.Provider
		pageText   string
		wantStatus int
		wantCode   string
		wantError  string
	}{
		{name: "success", body: `{"workspace_name":"Acme","website_url":"acme.com"}`, llm: contextTestLLM{}, pageText: "Acme",
			wantStatus: http.StatusOK},
		{name: "no AI", body: `{"workspace_name":"Acme","website_url":"acme.com"}`, pageText: "Acme",
			wantStatus: http.StatusUnprocessableEntity, wantCode: "ai_unavailable",
			wantError: "AI isn't connected yet. Connect an AI provider to generate this, or write it yourself."},
		{name: "unreadable website", body: `{"workspace_name":"Acme","website_url":"acme.com"}`, llm: contextTestLLM{},
			wantStatus: http.StatusUnprocessableEntity, wantCode: "website_unreadable",
			wantError: "We couldn't read your website. It may need JavaScript or block automated visitors. Write a short description instead."},
		{name: "model failure hides detail", body: `{"workspace_name":"Acme","website_url":"acme.com"}`, pageText: "Acme",
			llm:        contextTestLLM{err: &llm.ProviderError{Provider: "openrouter", Message: "internal upstream detail sk-secret"}},
			wantStatus: http.StatusUnprocessableEntity, wantCode: "generation_failed",
			wantError: "We couldn't generate a description. Write a short description instead."},
		{name: "invalid URL", body: `{"workspace_name":"Acme","website_url":"ftp://acme.com"}`, llm: contextTestLLM{},
			wantStatus: http.StatusBadRequest},
		{name: "private URL", body: `{"workspace_name":"Acme","website_url":"http://169.254.169.254"}`, llm: contextTestLLM{},
			wantStatus: http.StatusBadRequest},
		{name: "workspace without permission", body: `{"workspace_name":"Acme","website_url":"acme.com","workspace_id":"ws"}`,
			llm: contextTestLLM{}, pageText: "Acme", wantStatus: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := service.NewWorkspaceService(nil, nil, nil).
				SetContextGeneratorDependencies(tt.llm, contextTestFetcher{text: tt.pageText})
			h := NewWorkspaceHandler(svc, nil)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/api/workspaces/context/generate-description", strings.NewReader(tt.body))
			h.GenerateCompanyProductDescription(w, r)
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", w.Code, tt.wantStatus, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "sk-secret") || strings.Contains(w.Body.String(), "upstream") {
				t.Fatalf("response leaked internal detail: %s", w.Body.String())
			}
			if tt.wantCode == "" {
				return
			}
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body) != 2 || body["code"] != tt.wantCode || body["error"] != tt.wantError {
				t.Fatalf("body = %v, want error %q code %q", body, tt.wantError, tt.wantCode)
			}
		})
	}
}
