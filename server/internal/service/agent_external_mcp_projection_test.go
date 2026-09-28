package service

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestProjectEnabledExternalMCPToolsKeepsSavedAssignments(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE external_mcp_servers (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, status TEXT NOT NULL, enabled BOOLEAN NOT NULL)`,
		`CREATE TABLE external_mcp_tools (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, server_id TEXT NOT NULL, runtime_alias TEXT NOT NULL, enabled BOOLEAN NOT NULL)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, server := range []struct {
		id, status string
		enabled    bool
	}{
		{"active", model.ExternalMCPStatusConnected, true},
		{"paused", model.ExternalMCPStatusConnected, false},
		{"auth", model.ExternalMCPStatusPendingOAuth, true},
	} {
		if err := db.Exec(`INSERT INTO external_mcp_servers (id, workspace_id, status, enabled) VALUES (?, 'workspace-1', ?, ?)`, server.id, server.status, server.enabled).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, tool := range []struct {
		id, serverID, alias string
		enabled             bool
	}{
		{"active-read", "active", "mcp__active__search", true},
		{"active-write", "active", "mcp__active__write", false},
		{"paused-read", "paused", "mcp__paused__search", true},
		{"auth-read", "auth", "mcp__auth__search", true},
	} {
		if err := db.Exec(`INSERT INTO external_mcp_tools (id, workspace_id, server_id, runtime_alias, enabled) VALUES (?, 'workspace-1', ?, ?, ?)`, tool.id, tool.serverID, tool.alias, tool.enabled).Error; err != nil {
			t.Fatal(err)
		}
	}

	service := &AgentService{externalMCPService: &ExternalMCPService{
		repo: repository.NewExternalMCPRepository(db),
		cfg:  ExternalMCPServiceConfig{Enabled: true},
	}}
	saved := []string{"list_tasks", "mcp__active__search", "mcp__active__write", "mcp__paused__search", "mcp__auth__search", "mcp__helpin__internal"}
	agent := &model.Agent{AllowedTools: json.RawMessage(`["list_tasks","mcp__active__search","mcp__active__write","mcp__paused__search","mcp__auth__search","mcp__helpin__internal"]`)}
	projected, err := service.projectEnabledExternalMCPTools(context.Background(), "workspace-1", runtimeAgentFromHelpinAgent(agent, "helpin"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"list_tasks", "mcp__active__search", "mcp__auth__search", "mcp__helpin__internal"}
	if !slices.Equal(projected.AllowedTools, want) {
		t.Fatalf("run tools = %v, want %v", projected.AllowedTools, want)
	}
	if !slices.Equal(parseJSONStringSlice(agent.AllowedTools), saved) {
		t.Fatalf("saved assignments changed: %s", agent.AllowedTools)
	}
}
