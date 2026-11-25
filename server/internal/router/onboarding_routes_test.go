package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/handler"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// The onboarding preview runs before an account exists, so it must answer
// without a token, and each route must pass through its rate limit.
func TestOnboardingPreviewRoutesArePublicAndRateLimited(t *testing.T) {
	limited := map[string]string{}
	limiter := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				limited[r.URL.Path] = name
				next.ServeHTTP(w, r)
			})
		}
	}
	routes := New(Handlers{
		SupportQuickstart:         handler.NewSupportQuickstartHandler(service.NewSupportQuickstartService(service.SupportQuickstartDeps{})),
		OnboardingReadRateLimit:   limiter("read"),
		OnboardingAnswerRateLimit: limiter("answer"),
	}, auth.NewJWTManager("test-secret"), nil, nil, nil)

	tests := []struct {
		path, body, limiter string
	}{
		{path: "/api/onboarding/support/preview", body: `{"website_url":"localhost"}`, limiter: "read"},
		{path: "/api/onboarding/support/preview-answer", body: `{"website_url":"localhost","question":"Hi?"}`, limiter: "answer"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			routes.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body)))
			// An invalid address is refused by the handler, not by auth.
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body %s, want 400", rec.Code, rec.Body.String())
			}
			if limited[tt.path] != tt.limiter {
				t.Errorf("limiter = %q, want %q", limited[tt.path], tt.limiter)
			}
		})
	}
}
