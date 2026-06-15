package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenUseAgentRunTool = "agent_run_tool"
	AgentRunToolTokenTTL = 2 * time.Hour
)

type AgentRunToolClaims struct {
	RunID       string `json:"run_id"`
	WorkspaceID string `json:"workspace_id"`
	TokenUse    string `json:"tu"`
	jwt.RegisteredClaims
}

func (m *JWTManager) GenerateAgentRunToolToken(runID, workspaceID string, ttl time.Duration) (string, error) {
	runID = strings.TrimSpace(runID)
	workspaceID = strings.TrimSpace(workspaceID)
	if runID == "" {
		return "", fmt.Errorf("run_id is required")
	}
	if workspaceID == "" {
		return "", fmt.Errorf("workspace_id is required")
	}
	if ttl <= 0 {
		ttl = AgentRunToolTokenTTL
	}
	now := time.Now()
	claims := AgentRunToolClaims{
		RunID:       runID,
		WorkspaceID: workspaceID,
		TokenUse:    TokenUseAgentRunTool,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   "agent_run_tool:" + runID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) ValidateAgentRunToolToken(tokenString string) (*AgentRunToolClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AgentRunToolClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid agent run tool token: %w", err)
	}
	claims, ok := token.Claims.(*AgentRunToolClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid agent run tool token claims")
	}
	if claims.TokenUse != TokenUseAgentRunTool {
		return nil, fmt.Errorf("invalid agent run tool token use")
	}
	if strings.TrimSpace(claims.RunID) == "" || strings.TrimSpace(claims.WorkspaceID) == "" {
		return nil, fmt.Errorf("invalid agent run tool token scope")
	}
	return claims, nil
}
