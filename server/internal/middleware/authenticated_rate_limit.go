package middleware

import (
	"net/http"
	"path"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/ratelimit"
)

// AuthenticatedRateLimit runs after JWT authentication, once per HTTP request.
// It never wraps the response writer, buffers a stream, or counts stream events.
// Internal callbacks and public widget routes have separate authentication/policy.
func AuthenticatedRateLimit(limiter *ratelimit.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			if !limiter.Allow(r.Context(), "api", userID, false) ||
				(expensiveAPIRequest(r) && !limiter.Allow(r.Context(), "api", userID, true)) {
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded, retry shortly")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func expensiveAPIRequest(r *http.Request) bool {
	p := path.Clean(r.URL.Path)
	// Creating or editing a source queues a crawl; file confirmation queues indexing.
	if (p == "/api/pm/content-sources" || strings.HasPrefix(p, "/api/pm/content-sources/")) &&
		(r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
		return true
	}
	if r.Method != http.MethodPost {
		return false
	}
	switch path.Base(p) {
	case "run-agent", "rewrite-draft", "support-preview", "reindex", "generate", "generate-description", "regenerate", "auto-translate-missing", "dispatch", "sync":
		return true
	}
	if p == "/api/pm/agent-runs" {
		return true
	}
	if strings.HasPrefix(p, "/api/dock/") || strings.HasPrefix(p, "/api/pm/agent-runs/") ||
		strings.HasPrefix(p, "/api/pm/coding-sessions/") || strings.HasPrefix(p, "/api/command-bar/plans/") ||
		strings.HasPrefix(p, "/api/automation/runs/") {
		switch path.Base(p) {
		case "messages", "resume", "continue", "handoff", "retry", "title":
			return true
		}
	}
	return false
}
