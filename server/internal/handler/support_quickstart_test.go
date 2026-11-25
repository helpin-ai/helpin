package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestSupportQuickstartPreviewRefusesABadAddressAsJSON(t *testing.T) {
	h := NewSupportQuickstartHandler(service.NewSupportQuickstartService(service.SupportQuickstartDeps{}))
	rec := httptest.NewRecorder()
	h.Preview(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"website_url":"localhost"}`)))

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("status = %d, type = %q, want a JSON 400", rec.Code, rec.Header().Get("Content-Type"))
	}
}

// The request logger wraps the response; events must still flush through it
// as they are written, or the visitor sees nothing until the end.
func TestRequestLoggerLetsAStreamFlush(t *testing.T) {
	rec := httptest.NewRecorder()
	middleware.RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("data: {}\n\n")); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Fatalf("Flush() error = %v, want the stream to flush", err)
		}
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))

	if !rec.Flushed {
		t.Error("response was not flushed")
	}
}
