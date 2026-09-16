package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DemoReadOnly rejects every request that is not a read for the shared public
// demo viewer account. Workspace RBAC already limits a viewer inside a
// workspace; this closes the account-level mutations that sit outside it
// (profile, password, 2FA, passkeys, workspace creation, notification state).
func DemoReadOnly(isDemo func(email string) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if isDemo != nil && isDemo(GetUserEmail(r.Context())) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(model.APIError{Error: "demo workspace is read-only", Code: "demo_read_only"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
