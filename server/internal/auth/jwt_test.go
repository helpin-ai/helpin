package auth

import (
	"crypto/rsa"
	"crypto/rand"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testSecret  = "test-secret-key-for-jwt-testing"
	testUserID  = "usr_abc123"
	testEmail   = "alice@example.com"
)

func TestGenerateAndValidateAccessToken(t *testing.T) {
	mgr := NewJWTManager(testSecret)

	accessToken, _, err := mgr.GenerateTokenPair(testUserID, testEmail, false)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	claims, err := mgr.ValidateToken(accessToken)
	if err != nil {
		t.Fatalf("ValidateToken(accessToken) error = %v", err)
	}

	if claims.UserID != testUserID {
		t.Errorf("UserID = %q, want %q", claims.UserID, testUserID)
	}
	if claims.Email != testEmail {
		t.Errorf("Email = %q, want %q", claims.Email, testEmail)
	}
	if claims.Subject != testUserID {
		t.Errorf("Subject = %q, want %q", claims.Subject, testUserID)
	}
	if claims.ExpiresAt == nil {
		t.Fatal("ExpiresAt is nil")
	}
	// Access token expires in 15 minutes; verify it's within a reasonable window.
	expiresIn := time.Until(claims.ExpiresAt.Time)
	if expiresIn < 14*time.Minute || expiresIn > 16*time.Minute {
		t.Errorf("access token expiry = %v from now, want ~15m", expiresIn)
	}
}

func TestGenerateAndValidateRefreshToken(t *testing.T) {
	mgr := NewJWTManager(testSecret)

	_, refreshToken, err := mgr.GenerateTokenPair(testUserID, testEmail, false)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	claims, err := mgr.ValidateToken(refreshToken)
	if err != nil {
		t.Fatalf("ValidateToken(refreshToken) error = %v", err)
	}

	if claims.UserID != testUserID {
		t.Errorf("UserID = %q, want %q", claims.UserID, testUserID)
	}
	if claims.Email != testEmail {
		t.Errorf("Email = %q, want %q", claims.Email, testEmail)
	}
	if claims.Subject != testUserID {
		t.Errorf("Subject = %q, want %q", claims.Subject, testUserID)
	}
	if claims.ExpiresAt == nil {
		t.Fatal("ExpiresAt is nil")
	}
	// Refresh token (no remember me) expires in 24 hours.
	expiresIn := time.Until(claims.ExpiresAt.Time)
	if expiresIn < 23*time.Hour+59*time.Minute || expiresIn > 24*time.Hour+1*time.Minute {
		t.Errorf("refresh token expiry = %v from now, want ~24h", expiresIn)
	}
}

func TestGenerateAndValidateTwoFAToken(t *testing.T) {
	mgr := NewJWTManager(testSecret)

	token, err := mgr.Generate2FAToken(testUserID, testEmail, true)
	if err != nil {
		t.Fatalf("Generate2FAToken() error = %v", err)
	}

	claims, err := mgr.Validate2FAToken(token)
	if err != nil {
		t.Fatalf("Validate2FAToken() error = %v", err)
	}

	if claims.UserID != testUserID {
		t.Errorf("UserID = %q, want %q", claims.UserID, testUserID)
	}
	if claims.Email != testEmail {
		t.Errorf("Email = %q, want %q", claims.Email, testEmail)
	}
	if !claims.RememberMe {
		t.Error("expected remember_me to round-trip")
	}
	if claims.Purpose != "signin_2fa" {
		t.Errorf("Purpose = %q, want signin_2fa", claims.Purpose)
	}
}

func TestGenerateTokenPair_RememberMe(t *testing.T) {
	mgr := NewJWTManager(testSecret)

	_, refreshToken, err := mgr.GenerateTokenPair(testUserID, testEmail, true)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	claims, err := mgr.ValidateToken(refreshToken)
	if err != nil {
		t.Fatalf("ValidateToken(refreshToken) error = %v", err)
	}

	// Refresh token with remember me expires in 30 days.
	expiresIn := time.Until(claims.ExpiresAt.Time)
	if expiresIn < 29*24*time.Hour+23*time.Hour || expiresIn > 30*24*time.Hour+1*time.Minute {
		t.Errorf("refresh token expiry = %v from now, want ~30d", expiresIn)
	}
}

func TestValidateToken_Expired(t *testing.T) {
	mgr := NewJWTManager(testSecret)

	// Directly construct a token that expired 1 hour ago.
	claims := Claims{
		UserID: testUserID,
		Email:  testEmail,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Subject:   testUserID,
		},
	}
	tkn := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := tkn.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	_, err = mgr.ValidateToken(tokenString)
	if err == nil {
		t.Error("ValidateToken() expected error for expired token, got nil")
	}
}

func TestValidateToken_Tampered(t *testing.T) {
	mgr := NewJWTManager(testSecret)

	accessToken, _, err := mgr.GenerateTokenPair(testUserID, testEmail, false)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Tamper with the token by flipping a character in the signature (last segment).
	tampered := []byte(accessToken)
	lastIdx := len(tampered) - 1
	if tampered[lastIdx] == 'A' {
		tampered[lastIdx] = 'B'
	} else {
		tampered[lastIdx] = 'A'
	}

	_, err = mgr.ValidateToken(string(tampered))
	if err == nil {
		t.Error("ValidateToken() expected error for tampered token, got nil")
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	generator := NewJWTManager("secret-one")
	validator := NewJWTManager("secret-two")

	accessToken, _, err := generator.GenerateTokenPair(testUserID, testEmail, false)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	_, err = validator.ValidateToken(accessToken)
	if err == nil {
		t.Error("ValidateToken() expected error when validating with wrong secret, got nil")
	}
}

func TestValidateToken_EmptyString(t *testing.T) {
	mgr := NewJWTManager(testSecret)

	_, err := mgr.ValidateToken("")
	if err == nil {
		t.Error("ValidateToken() expected error for empty token string, got nil")
	}
}

func TestValidateToken_MalformedString(t *testing.T) {
	mgr := NewJWTManager(testSecret)

	tests := []struct {
		name  string
		token string
	}{
		{"plain text", "this-is-not-a-jwt"},
		{"random base64", "aGVsbG8gd29ybGQ.dGhpcyBpcyBub3Q.YSBqd3Q"},
		{"only dots", "..."},
		{"single segment", "eyJhbGciOiJIUzI1NiJ9"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mgr.ValidateToken(tt.token)
			if err == nil {
				t.Errorf("ValidateToken(%q) expected error for malformed token, got nil", tt.token)
			}
		})
	}
}

func TestValidateToken_WrongSigningMethod(t *testing.T) {
	mgr := NewJWTManager(testSecret)

	// Generate an RSA key pair for signing.
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	// Create a token signed with RS256 instead of HS256.
	claims := Claims{
		UserID: testUserID,
		Email:  testEmail,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   testUserID,
		},
	}
	tkn := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := tkn.SignedString(rsaKey)
	if err != nil {
		t.Fatalf("failed to sign RSA token: %v", err)
	}

	_, err = mgr.ValidateToken(tokenString)
	if err == nil {
		t.Error("ValidateToken() expected error for RSA-signed token validated with HMAC secret, got nil")
	}
}
