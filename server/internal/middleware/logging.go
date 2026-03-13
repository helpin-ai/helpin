package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// RequestLogger logs incoming HTTP requests with structured slog output.
// 4xx responses are logged at Warn, 5xx at Error, and all others at Info.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := newResponseWriter(w)
		next.ServeHTTP(rw, r)
		duration := time.Since(start)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", duration.Milliseconds(),
		}

		if userID := GetUserID(r.Context()); userID != "" {
			attrs = append(attrs, "user_id", userID)
		}

		switch {
		case rw.statusCode >= 500:
			slog.ErrorContext(r.Context(), "http request", attrs...)
		case rw.statusCode >= 400:
			slog.WarnContext(r.Context(), "http request", attrs...)
		default:
			slog.InfoContext(r.Context(), "http request", attrs...)
		}
	})
}
