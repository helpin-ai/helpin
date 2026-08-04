package middleware

// Rate limiting for the public help center AI endpoints. Answers cost LLM
// tokens on an unauthenticated surface, so the ceilings are strict: per-IP
// minute and day windows (fixed-window Redis counters, failing open when
// Redis is unavailable). Cache hits still pass through the limiter — the
// budget being protected is request volume, and cached answers are cheap
// enough that the generous minute window covers legitimate use.

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	helpcenterAnswerPerMinutePerIP = 6
	helpcenterAnswerPerDayPerIP    = 60
)

// HelpcenterAnswerRateLimit limits POSTs to the public answer endpoints.
func HelpcenterAnswerRateLimit(redisClient *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if redisClient == nil {
				next.ServeHTTP(w, r)
				return
			}
			ip := clientIPForRateLimit(r)
			if ip == "" {
				next.ServeHTTP(w, r)
				return
			}
			now := time.Now().UTC()
			checks := []struct {
				key   string
				limit int
				ttl   time.Duration
			}{
				{fmt.Sprintf("hc:rl:m:%s:%d", ip, now.Unix()/60), helpcenterAnswerPerMinutePerIP, 2 * time.Minute},
				{fmt.Sprintf("hc:rl:d:%s:%s", ip, now.Format("20060102")), helpcenterAnswerPerDayPerIP, 48 * time.Hour},
			}
			for _, check := range checks {
				count, err := redisClient.Incr(r.Context(), check.key).Result()
				if err != nil {
					slog.WarnContext(r.Context(), "helpcenter rate limit check failed", "error", err)
					break
				}
				if count == 1 {
					redisClient.Expire(r.Context(), check.key, check.ttl)
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
