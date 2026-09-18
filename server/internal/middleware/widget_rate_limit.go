package middleware

// Rate limiting for the public /widget surface. Visitor traffic is
// unauthenticated, and since visitor messages can start AI runs, the write
// endpoints need an abuse ceiling. Fixed-window counters in Redis keyed by
// the widget session token (with a wider per-IP ceiling as backstop against
// token rotation); read endpoints stay unlimited. Fails open when Redis is
// unavailable — availability beats strictness on a support channel.

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// widgetSessionLimitPerMinute bounds writes from one widget session.
	widgetSessionLimitPerMinute = 30
	// widgetIPLimitPerMinute bounds writes from one client IP across
	// sessions (a rotating-token attacker collapses onto this ceiling).
	widgetIPLimitPerMinute = 120
)

// WidgetRateLimit returns middleware limiting mutating widget requests.
// Safe (GET/HEAD/OPTIONS) requests pass through untouched.
func WidgetRateLimit(redisClient *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if redisClient == nil {
				next.ServeHTTP(w, r)
				return
			}
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			window := time.Now().Unix() / 60
			prefix := "widget:rl"
			// Telemetry must never exhaust the customer message/upload allowance.
			if strings.HasSuffix(r.URL.Path, "/widget/telemetry") {
				prefix = "widget:telemetry:rl"
			}
			checks := []struct {
				key   string
				limit int
			}{}
			if token := strings.TrimSpace(r.Header.Get("X-Session-Token")); token != "" {
				checks = append(checks, struct {
					key   string
					limit int
				}{fmt.Sprintf("%s:s:%s:%d", prefix, token, window), widgetSessionLimitPerMinute})
			}
			if ip := clientIPForRateLimit(r); ip != "" {
				checks = append(checks, struct {
					key   string
					limit int
				}{fmt.Sprintf("%s:ip:%s:%d", prefix, ip, window), widgetIPLimitPerMinute})
			}

			for _, check := range checks {
				count, err := redisClient.Incr(r.Context(), check.key).Result()
				if err != nil {
					// Redis down: fail open, but say so once per request.
					slog.WarnContext(r.Context(), "widget rate limit check failed", "error", err)
					break
				}
				if count == 1 {
					redisClient.Expire(r.Context(), check.key, 2*time.Minute)
				}
				if count > int64(check.limit) {
					w.Header().Set("Retry-After", "60")
					http.Error(w, `{"error":"rate limit exceeded, retry shortly"}`, http.StatusTooManyRequests)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIPForRateLimit extracts the client IP (RealIP middleware has already
// resolved X-Forwarded-For into RemoteAddr when trusted).
func clientIPForRateLimit(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return strings.TrimSpace(host)
}
