package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type fakePublicShareService struct{}

func (fakePublicShareService) Create(context.Context, string, string, string, string) (*model.PublicShareLink, error) {
	return &model.PublicShareLink{Token: "token", URL: "https://helpin.ai/shared/token"}, nil
}
func (fakePublicShareService) GetLink(context.Context, string, string, string, string) (*model.PublicShareLink, error) {
	return &model.PublicShareLink{Token: "token", URL: "https://helpin.ai/shared/token"}, nil
}
func (fakePublicShareService) Revoke(context.Context, string, string, string, string) error {
	return nil
}
func (fakePublicShareService) GetPublic(context.Context, string) (*model.PublicSharedResource, error) {
	return &model.PublicSharedResource{ResourceType: model.PublicShareResourceDockChat, DockChat: &model.PublicSharedDockChat{Title: "Shared chat"}}, nil
}

func TestPublicShareHandlerSetsPrivateNoIndexHeaders(t *testing.T) {
	handler := NewPublicShareHandler(fakePublicShareService{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/public/shares/token", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("token", "token")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))

	handler.GetPublic(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("X-Robots-Tag"); got != "noindex, nofollow, noarchive" {
		t.Fatalf("X-Robots-Tag = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}
