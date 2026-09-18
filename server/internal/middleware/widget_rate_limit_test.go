package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupWidgetRateLimitTest(t *testing.T) http.Handler {
	t.Helper()
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return WidgetRateLimit(client)(next)
}

func TestWidgetRateLimitAllowsReadsUnbounded(t *testing.T) {
	handler := setupWidgetRateLimitTest(t)
	for i := 0; i < widgetIPLimitPerMinute+10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/widget/messages", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET request %d = %d, want 200", i, rec.Code)
		}
	}
}

func TestWidgetRateLimitBlocksSessionOverLimit(t *testing.T) {
	handler := setupWidgetRateLimitTest(t)
	for i := 0; i < widgetSessionLimitPerMinute; i++ {
		req := httptest.NewRequest(http.MethodPost, "/widget/messages", nil)
		req.Header.Set("X-Session-Token", "session-1")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST %d = %d, want 200", i, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/widget/messages", nil)
	req.Header.Set("X-Session-Token", "session-1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit POST = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q, want 60", rec.Header().Get("Retry-After"))
	}

	// A different session from the same IP is still allowed (IP ceiling is wider).
	req = httptest.NewRequest(http.MethodPost, "/widget/messages", nil)
	req.Header.Set("X-Session-Token", "session-2")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("fresh session POST = %d, want 200", rec.Code)
	}
}

func TestWidgetRateLimitBlocksIPOverLimit(t *testing.T) {
	handler := setupWidgetRateLimitTest(t)
	for i := 0; i < widgetIPLimitPerMinute; i++ {
		req := httptest.NewRequest(http.MethodPost, "/widget/messages", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST %d = %d, want 200", i, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/widget/messages", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit POST = %d, want 429", rec.Code)
	}
}

func TestWidgetRateLimitFailsOpenWithoutRedis(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := WidgetRateLimit(nil)(next)
	req := httptest.NewRequest(http.MethodPost, "/widget/messages", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("nil-redis POST = %d, want 200", rec.Code)
	}
}

func TestTelemetryDoesNotConsumeCustomerWriteAllowance(t *testing.T) {
	handler := setupWidgetRateLimitTest(t)
	for i := 0; i < widgetIPLimitPerMinute+1; i++ {
		r := httptest.NewRequest(http.MethodPost, "/widget/telemetry", nil)
		handler.ServeHTTP(httptest.NewRecorder(), r)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/widget/messages", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("telemetry exhausted message budget: %d", w.Code)
	}
}
