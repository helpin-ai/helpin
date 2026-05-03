package authorization

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/middleware"
)

func TestRequirePlatformAdmin_AllowsAdminMFASession(t *testing.T) {
	claims := &auth.Claims{
		UserID:          "user-1",
		Email:           "admin@example.com",
		TokenUse:        auth.TokenUseAccess,
		MFASatisfied:    true,
		IsPlatformAdmin: true,
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/email-queue", nil)
	req = req.WithContext(middleware.WithClaims(req.Context(), claims))

	rec := httptest.NewRecorder()
	RequirePlatformAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRequirePlatformAdmin_RejectsNonAdminOrNoMFA(t *testing.T) {
	tests := []struct {
		name   string
		claims *auth.Claims
		status int
	}{
		{"missing claims", nil, http.StatusUnauthorized},
		{"refresh token", &auth.Claims{TokenUse: auth.TokenUseRefresh, IsPlatformAdmin: true, MFASatisfied: true}, http.StatusUnauthorized},
		{"non admin", &auth.Claims{TokenUse: auth.TokenUseAccess, IsPlatformAdmin: false, MFASatisfied: true}, http.StatusForbidden},
		{"no mfa", &auth.Claims{TokenUse: auth.TokenUseAccess, IsPlatformAdmin: true, MFASatisfied: false}, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/email-queue", nil)
			if tt.claims != nil {
				req = req.WithContext(middleware.WithClaims(req.Context(), tt.claims))
			}
			rec := httptest.NewRecorder()

			RequirePlatformAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("inner handler should not be called")
			})).ServeHTTP(rec, req)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
		})
	}
}
