package service

import (
	"net/url"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestSignGitHubInstallState(t *testing.T) {
	svc := &GitService{stateSecret: "test-secret"}

	tokenString, err := svc.signGitHubInstallState("ws-123", "user-456")
	if err != nil {
		t.Fatalf("signGitHubInstallState returned error: %v", err)
	}
	if tokenString == "" {
		t.Fatal("expected a token string")
	}

	claims := &gitHubInstallState{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if !token.Valid {
		t.Fatal("expected token to be valid")
	}
	if claims.WorkspaceID != "ws-123" {
		t.Fatalf("expected workspace claim ws-123, got %q", claims.WorkspaceID)
	}
	if claims.ActorID != "user-456" {
		t.Fatalf("expected actor claim user-456, got %q", claims.ActorID)
	}
	if claims.ExpiresAt == nil {
		t.Fatal("expected expiration claim to be set")
	}
}

func TestWithGitHubInstallStatus(t *testing.T) {
	result := withGitHubInstallStatus(
		"http://localhost:5173/w/demo/settings/system?tab=delivery",
		"connected",
		"Connected successfully.",
		map[string]string{"integration_id": "abc123"},
	)

	parsed, err := url.Parse(result)
	if err != nil {
		t.Fatalf("parse result URL: %v", err)
	}
	if parsed.Path != "/w/demo/settings/system" {
		t.Fatalf("unexpected path %q", parsed.Path)
	}
	if parsed.Query().Get("tab") != "delivery" {
		t.Fatalf("expected existing query param to be preserved, got %q", parsed.Query().Get("tab"))
	}
	if parsed.Query().Get("github_app") != "connected" {
		t.Fatalf("expected github_app=connected, got %q", parsed.Query().Get("github_app"))
	}
	if !strings.Contains(parsed.Query().Get("github_message"), "Connected successfully") {
		t.Fatalf("expected github_message to be populated, got %q", parsed.Query().Get("github_message"))
	}
	if parsed.Query().Get("integration_id") != "abc123" {
		t.Fatalf("expected integration_id=abc123, got %q", parsed.Query().Get("integration_id"))
	}
}
