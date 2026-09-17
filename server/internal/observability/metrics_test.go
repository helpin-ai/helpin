package observability

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHTTPMetricsUseTemplatesAndTrackFailures(t *testing.T) {
	m := NewMetrics()
	router := chi.NewRouter()
	router.Use(m.HTTP)
	router.Get("/objects/{id}", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "failed", 503) })
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/objects/customer-secret?token=secret", nil))
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	if strings.Contains(w.Body.String(), "secret") {
		t.Fatal("raw path/query leaked")
	}
	if !strings.Contains(w.Body.String(), `helpin_http_requests_total{method="GET",route="/objects/{id}",status="503"} 1`) {
		t.Fatal("missing templated server error")
	}
}

func TestBackgroundCountersRemainBounded(t *testing.T) {
	m := NewMetrics()
	h := m.BackgroundHandler(slog.NewJSONHandler(io.Discard, nil)).WithAttrs([]slog.Attr{slog.String("workspace_id", "secret")})
	logger := slog.New(h)
	logger.ErrorContext(context.Background(), "AI usage period worker failed")
	logger.Error("unrelated secret error")
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	if strings.Contains(w.Body.String(), "secret") {
		t.Fatal("unbounded error labels")
	}
	if !strings.Contains(w.Body.String(), `helpin_background_errors_total{worker="ai_usage"} 1`) {
		t.Fatal("missing worker counter")
	}
}
