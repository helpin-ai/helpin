package auth

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestMCPAccessTokenAudienceAndUse(t *testing.T) {
	manager := NewJWTManager("01234567890123456789012345678901")
	principal := model.MCPPrincipal{
		ConnectionID: "connection-1",
		WorkspaceID:  "workspace-1",
		UserID:       "user-1",
		ClientID:     "client-1",
		Scopes:       []string{"helpin.context.read"},
		Toolsets:     []string{"context"},
		ReadOnly:     true,
		TokenVersion: 3,
	}
	token, _, err := manager.GenerateMCPAccessToken(principal, "https://mcp.helpin.ai", "https://mcp.helpin.ai/mcp")
	if err != nil {
		t.Fatalf("GenerateMCPAccessToken() error = %v", err)
	}
	claims, err := manager.ValidateMCPAccessToken(token, "https://mcp.helpin.ai", "https://mcp.helpin.ai/mcp")
	if err != nil {
		t.Fatalf("ValidateMCPAccessToken() error = %v", err)
	}
	if claims.ConnectionID != principal.ConnectionID || claims.TokenVersion != 3 || !claims.ReadOnly {
		t.Fatalf("claims = %#v, want connection/version/read-only from principal", claims)
	}
	if _, err := manager.ValidateMCPAccessToken(token, "https://mcp.helpin.ai", "https://other.example/mcp"); err == nil {
		t.Fatal("ValidateMCPAccessToken() accepted the wrong audience")
	}
}
