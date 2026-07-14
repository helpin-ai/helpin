package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestMCPAuthorizationCodeIsSingleUse(t *testing.T) {
	repo := setupMCPRepositoryTest(t)
	ctx := context.Background()
	now := time.Now()
	code := &model.MCPOAuthAuthorizationCode{
		ID: "code-1", CodeHash: "hash-1", ConnectionID: "connection-1", ClientID: "client-1",
		RedirectURI: "https://client.example/callback", CodeChallenge: "challenge",
		CodeChallengeMethod: "S256", ExpiresAt: now.Add(time.Minute),
	}
	if err := repo.CreateAuthorizationCode(ctx, code); err != nil {
		t.Fatalf("CreateAuthorizationCode() error = %v", err)
	}
	consumed, err := repo.ConsumeAuthorizationCode(ctx, code.CodeHash, now)
	if err != nil || consumed == nil {
		t.Fatalf("ConsumeAuthorizationCode() = %#v, %v", consumed, err)
	}
	replayed, err := repo.ConsumeAuthorizationCode(ctx, code.CodeHash, now)
	if err != nil {
		t.Fatalf("ConsumeAuthorizationCode() replay error = %v", err)
	}
	if replayed != nil {
		t.Fatalf("ConsumeAuthorizationCode() replay = %#v, want nil", replayed)
	}
}

func TestMCPRefreshRotationIsAtomic(t *testing.T) {
	repo := setupMCPRepositoryTest(t)
	ctx := context.Background()
	now := time.Now()
	old := &model.MCPRefreshToken{
		ID: "refresh-1", TokenHash: "hash-1", FamilyID: "family-1",
		ConnectionID: "connection-1", ExpiresAt: now.Add(time.Hour),
	}
	if err := repo.CreateRefreshToken(ctx, old); err != nil {
		t.Fatalf("CreateRefreshToken() error = %v", err)
	}
	replacement := &model.MCPRefreshToken{
		ID: "refresh-2", TokenHash: "hash-2", FamilyID: old.FamilyID,
		ConnectionID: old.ConnectionID, ExpiresAt: now.Add(time.Hour),
	}
	if err := repo.RotateRefreshToken(ctx, old.ID, replacement, now); err != nil {
		t.Fatalf("RotateRefreshToken() error = %v", err)
	}
	if err := repo.RotateRefreshToken(ctx, old.ID, &model.MCPRefreshToken{
		ID: "refresh-3", TokenHash: "hash-3", FamilyID: old.FamilyID,
		ConnectionID: old.ConnectionID, ExpiresAt: now.Add(time.Hour),
	}, now); err == nil {
		t.Fatal("RotateRefreshToken() allowed a consumed token to rotate again")
	}
}

func TestMCPAgentRunSafetyCounts(t *testing.T) {
	repo := setupMCPRepositoryTest(t)
	ctx := context.Background()
	db := repo.db
	now := time.Now()
	if err := db.Exec(`INSERT INTO mcp_audit_events (id, workspace_id, user_id, event_type, tool_name, outcome, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"event-1", "workspace-1", "user-1", "tool.call", "start_agent_run", "success", now).Error; err != nil {
		t.Fatalf("insert audit event: %v", err)
	}
	count, err := repo.CountRecentToolCalls(ctx, "workspace-1", "user-1", "start_agent_run", now.Add(-time.Minute))
	if err != nil || count != 1 {
		t.Fatalf("CountRecentToolCalls() = %d, %v; want 1", count, err)
	}
	statements := []string{
		`INSERT INTO mcp_connections (id, user_id) VALUES ('connection-1', 'user-1')`,
		`INSERT INTO agent_runs (id, status) VALUES ('run-1', 'running')`,
		`INSERT INTO mcp_agent_run_attributions (run_id, workspace_id, connection_id) VALUES ('run-1', 'workspace-1', 'connection-1')`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed active run count: %v", err)
		}
	}
	userCount, workspaceCount, err := repo.CountActiveMCPAgentRuns(ctx, "workspace-1", "user-1")
	if err != nil || userCount != 1 || workspaceCount != 1 {
		t.Fatalf("CountActiveMCPAgentRuns() = %d, %d, %v; want 1, 1", userCount, workspaceCount, err)
	}
}

func setupMCPRepositoryTest(t *testing.T) *MCPRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:mcp_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE mcp_oauth_authorization_codes (id text primary key, code_hash text unique not null, connection_id text not null, client_id text not null, redirect_uri text not null, code_challenge text not null, code_challenge_method text not null, expires_at datetime not null, consumed_at datetime, created_at datetime)`,
		`CREATE TABLE mcp_refresh_tokens (id text primary key, token_hash text unique not null, family_id text not null, connection_id text not null, expires_at datetime not null, consumed_at datetime, revoked_at datetime, replaced_by_id text, created_at datetime)`,
		`CREATE TABLE mcp_audit_events (id text primary key, workspace_id text not null, user_id text, event_type text not null, tool_name text, outcome text not null, created_at datetime not null)`,
		`CREATE TABLE mcp_connections (id text primary key, user_id text not null)`,
		`CREATE TABLE mcp_service_principals (id text primary key, actor_user_id text not null)`,
		`CREATE TABLE agent_runs (id text primary key, status text not null)`,
		`CREATE TABLE mcp_agent_run_attributions (run_id text primary key, workspace_id text not null, connection_id text, service_principal_id text)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create MCP test table: %v", err)
		}
	}
	return NewMCPRepository(db)
}
