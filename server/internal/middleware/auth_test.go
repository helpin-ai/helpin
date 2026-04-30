package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestRequireAuth_ValidToken(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")
	accessToken, _, err := jwtMgr.GenerateTokenPair("user-abc", "alice@example.com", false)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error: %v", err)
	}

	var capturedUserID, capturedEmail string
	var capturedClaims *auth.Claims
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = GetUserID(r.Context())
		capturedEmail = GetUserEmail(r.Context())
		capturedClaims = ClaimsFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireAuth(jwtMgr)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if capturedUserID != "user-abc" {
		t.Errorf("UserID = %q, want %q", capturedUserID, "user-abc")
	}
	if capturedEmail != "alice@example.com" {
		t.Errorf("Email = %q, want %q", capturedEmail, "alice@example.com")
	}
	if capturedClaims == nil || capturedClaims.TokenUse != auth.TokenUseAccess {
		t.Fatalf("expected access claims in context, got %+v", capturedClaims)
	}
}

func TestRequireAuth_RejectsRefreshToken(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")
	_, refreshToken, err := jwtMgr.GenerateTokenPair("user-abc", "alice@example.com", false)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error: %v", err)
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called")
	})
	handler := RequireAuth(jwtMgr)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+refreshToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_MissingAuthorizationHeader(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called")
	})

	handler := RequireAuth(jwtMgr)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var apiErr model.APIError
	if err := json.NewDecoder(rec.Body).Decode(&apiErr); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if apiErr.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestRequireAuth_InvalidHeaderFormat(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called")
	})

	handler := RequireAuth(jwtMgr)(inner)

	tests := []struct {
		name   string
		header string
	}{
		{"no Bearer prefix", "Token some-token"},
		{"just the token", "some-token-without-prefix"},
		{"empty Bearer value", "Bearer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", tt.header)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}

			var apiErr model.APIError
			if err := json.NewDecoder(rec.Body).Decode(&apiErr); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if apiErr.Error == "" {
				t.Error("expected non-empty error message")
			}
		})
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	secret := "test-secret"
	jwtMgr := auth.NewJWTManager(secret)

	// Create a token that expired 1 hour ago
	claims := &auth.Claims{
		UserID: "user-expired",
		Email:  "expired@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Subject:   "user-expired",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called")
	})

	handler := RequireAuth(jwtMgr)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var apiErr model.APIError
	if err := json.NewDecoder(rec.Body).Decode(&apiErr); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if apiErr.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestRequireAuth_WrongSecret(t *testing.T) {
	// Generate token with one secret
	otherMgr := auth.NewJWTManager("other-secret")
	accessToken, _, err := otherMgr.GenerateTokenPair("user-xyz", "xyz@example.com", false)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error: %v", err)
	}

	// Validate with a different secret
	jwtMgr := auth.NewJWTManager("test-secret")
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called")
	})

	handler := RequireAuth(jwtMgr)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var apiErr model.APIError
	if err := json.NewDecoder(rec.Body).Decode(&apiErr); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if apiErr.Error == "" {
		t.Error("expected non-empty error message")
	}
}
