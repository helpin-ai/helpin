package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/helpin-ai/helpin/server/internal/auth"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// AdminAuditLogger emits one structured audit event for every /api/admin request.
func AdminAuditLogger(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()

			next.ServeHTTP(recorder, r)

			claims := auditClaimsFromRequest(r, jwtManager)
			userID := ""
			userEmail := ""
			if claims != nil {
				userID = claims.UserID
				userEmail = claims.Email
			}

			requestID, _ := r.Context().Value(chimiddleware.RequestIDKey).(string)
			slog.InfoContext(r.Context(), "admin_request",
				"user_id", userID,
				"user_email", userEmail,
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", requestID,
			)
		})
	}
}

func auditClaimsFromRequest(r *http.Request, jwtManager *auth.JWTManager) *auth.Claims {
	if claims := ClaimsFrom(r.Context()); claims != nil {
		return claims
	}
	if jwtManager == nil {
		return nil
	}
	header := r.Header.Get("Authorization")
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil
	}
	claims, err := jwtManager.ValidateToken(parts[1])
	if err != nil {
		return nil
	}
	return claims
}
