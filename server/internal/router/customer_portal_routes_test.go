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
	for _, path := range []string{"/api/public/portal/acme", "/api/public/portal/acme/", "/api/public/portal/acme/suggestions"} {
		if !routes.Match(chi.NewRouteContext(), http.MethodGet, path) {
			t.Errorf("GET %s is not registered", path)
		}
	}
	if !routes.Match(chi.NewRouteContext(), http.MethodPost, "/api/public/portal/acme/suggestions/events") {
		t.Error("POST /api/public/portal/acme/suggestions/events is not registered")
	}
	for _, path := range []string{"/api/portal/auth/request-link", "/api/portal/auth/exchange", "/api/portal/auth/logout"} {
		if routes.Match(chi.NewRouteContext(), http.MethodPost, path) {
			t.Errorf("legacy bearer portal route POST %s is still registered", path)
		}
	}
	if routes.Match(chi.NewRouteContext(), http.MethodGet, "/api/portal/auth/session") {
		t.Error("legacy bearer portal route GET /api/portal/auth/session is still registered")
	}
}

func TestCustomerPortalAdminRoutes(t *testing.T) {
	routes := New(Handlers{CustomerPortalAdmin: &handler.CustomerPortalAdminHandler{}}, auth.NewJWTManager("test-secret"), nil, nil, nil)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/support/inbox/portal/access"},
		{http.MethodGet, "/api/support/inbox/portal/contacts/c1/access"},
		{http.MethodPut, "/api/support/inbox/portal/contacts/c1/access"},
		{http.MethodPost, "/api/support/inbox/conversations/conv1/portal-confirmation"},
		{http.MethodPut, "/api/support/inbox/portal/contacts/c1/role"},
		{http.MethodGet, "/api/support/inbox/portal/companies/co1"},
		{http.MethodPut, "/api/support/inbox/portal/companies/co1"},
		{http.MethodPost, "/api/support/inbox/portal/companies/co1/allow-contacts"},
		{http.MethodGet, "/api/support/inbox/conversations/conv1/portal-context"},
	} {
		if !routes.Match(chi.NewRouteContext(), route.method, route.path) {
			t.Errorf("%s %s is not registered", route.method, route.path)
		}
	}
}

func TestSupportAttachmentPolicyRoute(t *testing.T) {
	routes := New(Handlers{SupportAttachment: &handler.SupportAttachmentHandler{}}, auth.NewJWTManager("test-secret"), nil, nil, nil)
	if !routes.Match(chi.NewRouteContext(), http.MethodGet, "/api/support/inbox/attachments/policy") {
		t.Error("GET /api/support/inbox/attachments/policy is not registered")
	}
}
