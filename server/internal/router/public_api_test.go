package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/auth"
)

func TestPublicAPIIsMountedOnlyWhenConfigured(t *testing.T) {
	marker := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	for _, path := range []string{"/public/v1", "/public/v1/tasks", "/public/v1/openapi.json"} {
		mounted := New(Handlers{PublicAPI: marker}, auth.NewJWTManager("test-secret"), nil, nil, nil)
		rec := httptest.NewRecorder()
		mounted.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusTeapot {
			t.Errorf("%s mounted status = %d", path, rec.Code)
		}
	}
	unmounted := New(Handlers{}, auth.NewJWTManager("test-secret"), nil, nil, nil)
	rec := httptest.NewRecorder()
	unmounted.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/public/v1/tasks", nil))
	if rec.Code == http.StatusTeapot || rec.Code == http.StatusOK {
		t.Errorf("unmounted status = %d", rec.Code)
	}
}
