package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	// TokenUseMCPAccess identifies public MCP access tokens.
	TokenUseMCPAccess = "mcp_access"
	// MCPAccessTokenTTL is the lifetime of a public MCP access token.
	MCPAccessTokenTTL = 15 * time.Minute
)

// MCPAccessClaims are audience-bound claims for one workspace connection.
type MCPAccessClaims struct {
	ConnectionID string   `json:"connection_id"`
	WorkspaceID  string   `json:"workspace_id"`
	UserID       string   `json:"user_id"`
	ClientID     string   `json:"client_id"`
	Scopes       []string `json:"scopes"`
	Toolsets     []string `json:"toolsets"`
	ReadOnly     bool     `json:"read_only"`
	TokenVersion int      `json:"token_version"`
	TokenUse     string   `json:"tu"`
	jwt.RegisteredClaims
}

// GenerateMCPAccessToken issues a short-lived access token for an authorized connection.
func (m *JWTManager) GenerateMCPAccessToken(
	principal model.MCPPrincipal,
	issuer string,
	audience string,
) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(MCPAccessTokenTTL)
	claims := MCPAccessClaims{
		ConnectionID: principal.ConnectionID,
		WorkspaceID:  principal.WorkspaceID,
		UserID:       principal.UserID,
		ClientID:     principal.ClientID,
		Scopes:       principal.Scopes,
		Toolsets:     principal.Toolsets,
		ReadOnly:     principal.ReadOnly,
		TokenVersion: principal.TokenVersion,
		TokenUse:     TokenUseMCPAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    issuer,
			NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)),
			Subject:   principal.ConnectionID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign MCP access token: %w", err)
	}
	return signed, expiresAt, nil
}

// ValidateMCPAccessToken validates signature, issuer, audience, expiry, and token use.
func (m *JWTManager) ValidateMCPAccessToken(tokenString, issuer, audience string) (*MCPAccessClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&MCPAccessClaims{},
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return m.secret, nil
		},
		jwt.WithAudience(audience),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid MCP access token: %w", err)
	}
	claims, ok := token.Claims.(*MCPAccessClaims)
	if !ok || !token.Valid || claims.TokenUse != TokenUseMCPAccess {
		return nil, fmt.Errorf("invalid MCP access token claims")
	}
	if claims.ConnectionID == "" || claims.WorkspaceID == "" || claims.UserID == "" {
		return nil, fmt.Errorf("incomplete MCP access token claims")
	}
	return claims, nil
}
