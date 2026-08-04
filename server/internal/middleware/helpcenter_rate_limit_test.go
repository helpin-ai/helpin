package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestHelpcenterAnswerRateLimitBlocksAfterMinuteWindow(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	handler := HelpcenterAnswerRateLimit(client)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < helpcenterAnswerPerMinutePerIP; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/hc/x/answer", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d = %d, want 200", i, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/hc/x/answer", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit request = %d, want 429", rec.Code)
	}
}

func TestHelpcenterAnswerRateLimitFailsOpenWithoutRedis(t *testing.T) {
	handler := HelpcenterAnswerRateLimit(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/hc/x/answer", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("nil-redis request = %d, want 200", rec.Code)
	}
}
