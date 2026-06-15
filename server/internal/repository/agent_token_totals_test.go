package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAgentRepositoryHydratesTokenTotals(t *testing.T) {
	dbName := fmt.Sprintf("file:agent_token_totals_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			trigger_mode TEXT NOT NULL,
			approval_mode TEXT NOT NULL,
			max_concurrent_runs INTEGER NOT NULL DEFAULT 1,
			default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create agents table: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			tokens_used INTEGER NOT NULL DEFAULT 0
		)
	`).Error; err != nil {
		t.Fatalf("create agent_runs table: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE agent_team_access (
			agent_id TEXT NOT NULL,
			team_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (agent_id, team_id)
		)
	`).Error; err != nil {
		t.Fatalf("create agent_team_access table: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO agents (
			id, workspace_id, is_system, name, status, runtime_kind, trigger_mode, approval_mode, created_at, updated_at
		) VALUES (
			'agent-1', 'workspace-1', 1, 'Mira', 'idle', 'native_sdk', 'manual', 'never', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		t.Fatalf("insert agent: %v", err)
	}
	for _, stmt := range []string{
		`INSERT INTO agent_runs (id, workspace_id, agent_id, tokens_used) VALUES ('run-1', 'workspace-1', 'agent-1', 120)`,
		`INSERT INTO agent_runs (id, workspace_id, agent_id, tokens_used) VALUES ('run-2', 'workspace-1', 'agent-1', 80)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("insert run: %v", err)
		}
	}

	repo := NewAgentRepository(db)
	listed, err := repo.List(context.Background(), "workspace-1")
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(listed))
	}
	if listed[0].TokensUsedTotal != 200 {
		t.Fatalf("expected listed token total 200, got %d", listed[0].TokensUsedTotal)
	}

	got, err := repo.GetByID(context.Background(), "workspace-1", "agent-1")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if got == nil {
		t.Fatal("expected agent")
	}
	if got.TokensUsedTotal != 200 {
		t.Fatalf("expected fetched token total 200, got %d", got.TokensUsedTotal)
	}
}
