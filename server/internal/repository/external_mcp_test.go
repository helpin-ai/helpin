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

func setupExternalMCPCatalogTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:external_mcp_catalog_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE external_mcp_servers (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			status TEXT NOT NULL,
			enabled BOOLEAN NOT NULL
		)
	`).Error; err != nil {
		t.Fatalf("create external_mcp_servers table: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE external_mcp_tools (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			server_id TEXT NOT NULL,
			remote_name TEXT NOT NULL,
			runtime_alias TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			input_schema BLOB NOT NULL,
			access TEXT NOT NULL DEFAULT 'read',
			enabled BOOLEAN NOT NULL,
			schema_hash TEXT NOT NULL DEFAULT '',
			last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		t.Fatalf("create external_mcp_tools table: %v", err)
	}
	return db
}

func TestExternalMCPRepositoryListCatalogToolsOnlyReturnsConnectedServerTools(t *testing.T) {
	db := setupExternalMCPCatalogTestDB(t)
	servers := []struct {
		id          string
		workspaceID string
		status      string
		enabled     bool
		toolEnabled bool
	}{
		{id: "connected", workspaceID: "workspace-1", status: model.ExternalMCPStatusConnected, enabled: true, toolEnabled: true},
		{id: "pending", workspaceID: "workspace-1", status: model.ExternalMCPStatusPendingOAuth, enabled: true, toolEnabled: true},
		{id: "disconnected", workspaceID: "workspace-1", status: model.ExternalMCPStatusDisconnected, enabled: true, toolEnabled: true},
		{id: "errored", workspaceID: "workspace-1", status: model.ExternalMCPStatusError, enabled: true, toolEnabled: true},
		{id: "server-disabled", workspaceID: "workspace-1", status: model.ExternalMCPStatusConnected, enabled: false, toolEnabled: true},
		{id: "tool-disabled", workspaceID: "workspace-1", status: model.ExternalMCPStatusConnected, enabled: true, toolEnabled: false},
		{id: "other-workspace", workspaceID: "workspace-2", status: model.ExternalMCPStatusConnected, enabled: true, toolEnabled: true},
	}
	for _, server := range servers {
		if err := db.Exec(
			"INSERT INTO external_mcp_servers (id, workspace_id, status, enabled) VALUES (?, ?, ?, ?)",
			server.id, server.workspaceID, server.status, server.enabled,
		).Error; err != nil {
			t.Fatalf("insert server %q: %v", server.id, err)
		}
		if err := db.Exec(
			`INSERT INTO external_mcp_tools
				(id, workspace_id, server_id, remote_name, runtime_alias, input_schema, enabled)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			"tool-"+server.id,
			server.workspaceID,
			server.id,
			"remote_"+server.id,
			"mcp__"+server.id+"__tool",
			[]byte("{}"),
			server.toolEnabled,
		).Error; err != nil {
			t.Fatalf("insert tool for server %q: %v", server.id, err)
		}
	}

	repo := NewExternalMCPRepository(db)
	tools, err := repo.ListCatalogTools(context.Background(), "workspace-1")
	if err != nil {
		t.Fatalf("list external MCP catalog tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 connected external MCP tool, got %d: %#v", len(tools), tools)
	}
	if tools[0].RuntimeAlias != "mcp__connected__tool" {
		t.Fatalf("expected connected server tool, got %q", tools[0].RuntimeAlias)
	}
}
