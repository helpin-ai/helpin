package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestAuthHandler_DemoSignin_DisabledReturnsNotFound(t *testing.T) {
	h, _ := newAuthHandlerTestFixture(t)

	rec := httptest.NewRecorder()
	h.DemoSignin(rec, newJSONRequest(t, http.MethodPost, "/api/auth/demo", model.DemoSigninRequest{}))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestAuthHandler_DemoSignin_IssuesViewerSession(t *testing.T) {
	h, userID := newAuthHandlerTestFixture(t)
	h.authService.ConfigureDemo(service.DemoConfig{ViewerEmail: "Alice@Example.com"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/demo", nil)
	h.DemoSignin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp model.SigninResponse
	decodeJSONResponse(t, rec, &resp)
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}
	if resp.User == nil || resp.User.ID != userID {
		t.Fatalf("expected demo user %s, got %+v", userID, resp.User)
	}
	if !h.authService.IsDemoUser("alice@example.com") {
		t.Fatal("expected fixture user to be recognised as the demo user")
	}
	var sawAccessCookie bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == "helpin_access_token" && c.Value != "" {
			sawAccessCookie = true
		}
	}
	if !sawAccessCookie {
		t.Fatal("expected access token cookie to be set")
	}
}

func TestAuthHandler_DemoSignin_RequiresEmailWhenConfigured(t *testing.T) {
	h, _ := newAuthHandlerTestFixture(t)
	h.authService.ConfigureDemo(service.DemoConfig{ViewerEmail: "alice@example.com", RequireEmail: true})

	rec := httptest.NewRecorder()
	h.DemoSignin(rec, newJSONRequest(t, http.MethodPost, "/api/auth/demo", model.DemoSigninRequest{}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing email status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.DemoSignin(rec, newJSONRequest(t, http.MethodPost, "/api/auth/demo", model.DemoSigninRequest{Email: "not-an-email"}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid email status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.DemoSignin(rec, newJSONRequest(t, http.MethodPost, "/api/auth/demo", model.DemoSigninRequest{Email: "visitor@example.org"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("valid email status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestAuthHandler_DemoSignin_PostsLeadToWebhook(t *testing.T) {
	var (
		mu      sync.Mutex
		payload map[string]string
		got     = make(chan struct{}, 1)
	)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.WriteHeader(http.StatusNoContent)
		got <- struct{}{}
	}))
	defer webhook.Close()

	h, _ := newAuthHandlerTestFixture(t)
	h.authService.ConfigureDemo(service.DemoConfig{ViewerEmail: "alice@example.com", LeadWebhookURL: webhook.URL})

	rec := httptest.NewRecorder()
	h.DemoSignin(rec, newJSONRequest(t, http.MethodPost, "/api/auth/demo", model.DemoSigninRequest{Email: "Visitor@Example.org"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	select {
	case <-got:
	case <-time.After(3 * time.Second):
		t.Fatal("webhook was not called")
	}
	mu.Lock()
	defer mu.Unlock()
	if payload["email"] != "visitor@example.org" || payload["source"] != "demo" {
		t.Fatalf("unexpected webhook payload: %+v", payload)
	}
}

func TestAuthHandler_GetConfig_ReportsDemoFlags(t *testing.T) {
	h, _ := newAuthHandlerTestFixture(t)
	h.authService.ConfigureDemo(service.DemoConfig{ViewerEmail: "alice@example.com", RequireEmail: true})

	rec := httptest.NewRecorder()
	h.GetConfig(rec, httptest.NewRequest(http.MethodGet, "/api/auth/config", nil))
	var cfg map[string]any
	decodeJSONResponse(t, rec, &cfg)
	if cfg["demo_enabled"] != true || cfg["demo_requires_email"] != true {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
