package router

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/handler"
)

func TestIntegrationBrowserCallbacksDoNotRequireBearerHeaders(t *testing.T) {
	router := New(Handlers{ExternalMCP: handler.NewExternalMCPHandler(nil, nil, nil, "https://app.helpin.ai"), GitHubApp: &handler.GitHubAppHandler{}}, auth.NewJWTManager("test-secret"), nil, nil, nil)
	for _, path := range []string{"/api/auth/google/callback", "/api/git/github/callback", "/api/github/app-manifest/callback", "/api/crm/email/oauth/callback", "/api/external-mcp/oauth/callback"} {
		t.Run(path, func(t *testing.T) {
			var target http.Handler
			err := chi.Walk(router, func(method, route string, _ http.Handler, middlewares ...func(http.Handler) http.Handler) error {
				if method != http.MethodGet || strings.TrimRight(route, "/") != path {
					return nil
				}
				target = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
				for i := len(middlewares) - 1; i >= 0; i-- {
					target = middlewares[i](target)
				}
				return nil
			})
			if err != nil || target == nil {
				t.Fatalf("callback route missing: %v", err)
			}
			response := httptest.NewRecorder()
			target.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusNoContent {
				t.Fatalf("browser callback blocked: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestExternalMCPCallbackRelaysWithoutExchangingUnauthenticatedCode(t *testing.T) {
	router := New(Handlers{ExternalMCP: handler.NewExternalMCPHandler(nil, nil, nil, "https://app.helpin.ai")}, auth.NewJWTManager("test-secret"), nil, nil, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/external-mcp/oauth/callback?state=state-value&code=code-value&redirect_uri=https://untrusted.example", nil))
	if response.Code != http.StatusFound {
		t.Fatalf("callback status: %d %s", response.Code, response.Body.String())
	}
	destination, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if destination.Host != "app.helpin.ai" || destination.Path != "/oauth/external-mcp/callback" || destination.Query().Get("state") != "state-value" || destination.Query().Get("code") != "code-value" || destination.Query().Get("redirect_uri") != "" {
		t.Fatalf("invalid relay destination: %s", destination)
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("OAuth response must not be cached or forwarded as a referrer")
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/external-mcp/oauth/callback", strings.NewReader(`{"state":"state-value","code":"code-value"}`)))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("code exchange must still require authentication, got %d", response.Code)
	}
}
