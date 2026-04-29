package authorization

import (
	"encoding/json"
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// RequirePlatformAdmin allows only platform admins whose current session has satisfied MFA.
func RequirePlatformAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.ClaimsFrom(r.Context())
		if claims == nil || claims.TokenUse != auth.TokenUseAccess {
			writePlatformAdminError(w, http.StatusUnauthorized, "authentication required", "auth_required")
			return
		}
		if !claims.IsPlatformAdmin || !claims.MFASatisfied {
			writePlatformAdminError(w, http.StatusForbidden, "platform admin access required", "platform_admin_required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writePlatformAdminError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(model.APIError{Error: message, Code: code})
}
