package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenUseAccess  = "access"
	TokenUseRefresh = "refresh"

	AccessTokenTTL            = 15 * time.Minute
	RefreshTokenTTL           = 7 * 24 * time.Hour
	RememberMeRefreshTokenTTL = 30 * 24 * time.Hour
)

// Claims represents the JWT claims embedded in each token.
type Claims struct {
	UserID          string `json:"user_id"`
	Email           string `json:"email"`
	TokenUse        string `json:"tu"`
	RememberMe      bool   `json:"remember_me,omitempty"`
	MFASatisfied    bool   `json:"mfa,omitempty"`
	IsPlatformAdmin bool   `json:"pa,omitempty"`
	jwt.RegisteredClaims
}

// TwoFAClaims represents the claims embedded in a short-lived 2FA challenge token.
type TwoFAClaims struct {
	UserID     string `json:"user_id"`
	Email      string `json:"email"`
	RememberMe bool   `json:"remember_me"`
	Purpose    string `json:"purpose"`
	jwt.RegisteredClaims
}

// JWTManager handles token generation and validation.
type JWTManager struct {
	secret []byte
}

type tokenPairOptions struct {
	mfaSatisfied    bool
	isPlatformAdmin bool
}

type TokenPairOption func(*tokenPairOptions)

func WithMFASatisfied(mfaSatisfied bool) TokenPairOption {
	return func(opts *tokenPairOptions) {
		opts.mfaSatisfied = mfaSatisfied
	}
}

func WithPlatformAdmin(isPlatformAdmin bool) TokenPairOption {
	return func(opts *tokenPairOptions) {
		opts.isPlatformAdmin = isPlatformAdmin
	}
}

// NewJWTManager creates a new JWTManager with the given secret.
func NewJWTManager(secret string) *JWTManager {
	return &JWTManager{secret: []byte(secret)}
}

// GenerateTokenPair creates a new access token and refresh token.
// When rememberMe is true, the refresh token lasts 30 days; otherwise 7 days.
func (m *JWTManager) GenerateTokenPair(userID, email string, rememberMe bool, options ...TokenPairOption) (accessToken, refreshToken string, err error) {
	now := time.Now()
	opts := tokenPairOptions{}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}

	// Access token: 15 minutes
	accessClaims := Claims{
		UserID:          userID,
		Email:           email,
		TokenUse:        TokenUseAccess,
		RememberMe:      rememberMe,
		MFASatisfied:    opts.mfaSatisfied,
		IsPlatformAdmin: opts.isPlatformAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   userID,
		},
	}
	accessTkn := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = accessTkn.SignedString(m.secret)
	if err != nil {
		return "", "", fmt.Errorf("sign access token: %w", err)
	}

	// Refresh token: 30 days if remember me, 7 days otherwise.
	refreshDuration := RefreshTokenTTL
	if rememberMe {
		refreshDuration = RememberMeRefreshTokenTTL
	}
	refreshClaims := Claims{
		UserID:          userID,
		Email:           email,
		TokenUse:        TokenUseRefresh,
		RememberMe:      rememberMe,
		MFASatisfied:    opts.mfaSatisfied,
		IsPlatformAdmin: opts.isPlatformAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   userID,
		},
	}
	refreshTkn := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = refreshTkn.SignedString(m.secret)
	if err != nil {
		return "", "", fmt.Errorf("sign refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// ValidateToken parses and validates a JWT string, returning the claims if valid.
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// Generate2FAToken creates a short-lived token that can only be exchanged for a full auth session.
func (m *JWTManager) Generate2FAToken(userID, email string, rememberMe bool) (string, error) {
	now := time.Now()
	claims := TwoFAClaims{
		UserID:     userID,
		Email:      email,
		RememberMe: rememberMe,
		Purpose:    "signin_2fa",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   "2fa:" + userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign 2fa token: %w", err)
	}
	return signed, nil
}

// Validate2FAToken validates a short-lived 2FA challenge token.
func (m *JWTManager) Validate2FAToken(tokenString string) (*TwoFAClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TwoFAClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid 2fa token: %w", err)
	}

	claims, ok := token.Claims.(*TwoFAClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid 2fa token claims")
	}
	if claims.Purpose != "signin_2fa" {
		return nil, fmt.Errorf("invalid 2fa token purpose")
	}

	return claims, nil
}

// PreviewClaims contains claims for a document preview token.
type PreviewClaims struct {
	DocID       string `json:"doc_id"`
	WorkspaceID string `json:"workspace_id"`
	jwt.RegisteredClaims
}

// GeneratePreviewToken creates a short-lived JWT (15 min) for document preview.
func (m *JWTManager) GeneratePreviewToken(docID, workspaceID string) (string, error) {
	claims := PreviewClaims{
		DocID:       docID,
		WorkspaceID: workspaceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   "preview:" + docID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ValidatePreviewToken validates a preview JWT and returns its claims.
func (m *JWTManager) ValidatePreviewToken(tokenString string) (*PreviewClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &PreviewClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid preview token: %w", err)
	}
	claims, ok := token.Claims.(*PreviewClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid preview token claims")
	}
	return claims, nil
}
