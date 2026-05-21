package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const accessTokenCookieName = "helpin_access_token"

// RequireAuth is middleware that validates the Authorization header or access
// token cookie JWT and
// injects the authenticated user's ID and email into the request context.
func RequireAuth(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := bearerToken(r.Header.Get("Authorization"))
			if tokenString == "" {
				if cookie, err := r.Cookie(accessTokenCookieName); err == nil {
					tokenString = strings.TrimSpace(cookie.Value)
				}
			}
			if tokenString == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization token")
				return
			}

			if header := r.Header.Get("Authorization"); header != "" && bearerToken(header) == "" {
				writeError(w, http.StatusUnauthorized, "invalid authorization header format")
				return
			}

			claims, err := jwtManager.ValidateToken(tokenString)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			if claims.TokenUse != auth.TokenUseAccess {
				writeError(w, http.StatusUnauthorized, "access token required")
				return
			}

			ctx := WithUserID(r.Context(), claims.UserID)
			ctx = WithUserEmail(ctx, claims.Email)
			ctx = WithClaims(ctx, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(header string) string {
	if strings.TrimSpace(header) == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// RequireInternalAPISecret validates the Authorization bearer token against
// INTERNAL_API_SECRET env var for service-to-service calls.
func RequireInternalAPISecret(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv("INTERNAL_API_SECRET")
		if secret == "" {
			writeError(w, http.StatusServiceUnavailable, "internal API not configured")
			return
		}

		header := r.Header.Get("Authorization")
		if header == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			writeError(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(secret)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid internal API secret")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(model.APIError{Error: message})
}
