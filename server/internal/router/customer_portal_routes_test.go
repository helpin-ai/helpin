package router

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/handler"
)

func TestCustomerPortalConfigRoutes(t *testing.T) {
	routes := New(Handlers{CustomerPortal: &handler.CustomerPortalHandler{}}, auth.NewJWTManager("test-secret"), nil, nil, nil)
	for _, path := range []string{"/api/public/portal/acme", "/api/public/portal/acme/"} {
		if !routes.Match(chi.NewRouteContext(), http.MethodGet, path) {
			t.Errorf("GET %s is not registered", path)
		}
	}
}
