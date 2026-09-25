package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/handler"
)

func TestCRMOutreachRoutesRegisterWithPublicPreferencesAndProtectedLibrary(t *testing.T) {
	router := New(Handlers{CRMOutreach: handler.NewCRMOutreachHandler(nil)}, auth.NewJWTManager("test-secret"), nil, nil, []string{"https://app.example.com"})
	req := httptest.NewRequest(http.MethodGet, "/api/crm/outreach/unsubscribe/invalid", nil)
	req.Header.Set("Origin", "https://app.example.com")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusNotFound {
		t.Fatalf("public link unexpectedly required auth: %d", response.Code)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Fatal("public preference route lacks API CORS")
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/crm/outreach/templates?workspace_id=ws", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("template library allowed unauthenticated access: %d", response.Code)
	}
}
