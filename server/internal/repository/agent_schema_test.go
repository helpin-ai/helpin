package repository

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAgentSchemaTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:agent_schema_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.Exec(`
		CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			runtime_kind TEXT NOT NULL,
			trigger_mode TEXT NOT NULL,
			approval_mode TEXT NOT NULL DEFAULT 'class_default',
			agent_kind TEXT NOT NULL,
			user_id TEXT,
			tools TEXT NOT NULL DEFAULT '[]',
			target_selector TEXT,
			trigger_events TEXT NOT NULL DEFAULT '[]',
			agent_class TEXT,
			capability_profile TEXT
		)
	`).Error; err != nil {
		t.Fatalf("create agents table: %v", err)
	}

	return db
}

func TestMigrateAgentSchemaDropsLegacyColumns(t *testing.T) {
	db := setupAgentSchemaTestDB(t)

	for _, column := range []string{
		"agent_kind",
		"user_id",
		"tools",
		"target_selector",
		"trigger_events",
		"agent_class",
		"capability_profile",
	} {
		if !db.Migrator().HasColumn("agents", column) {
			t.Fatalf("expected legacy column %q to exist before migration", column)
		}
	}

	if err := MigrateAgentSchema(db); err != nil {
		t.Fatalf("MigrateAgentSchema: %v", err)
	}

	for _, column := range []string{
		"agent_kind",
		"user_id",
		"tools",
		"target_selector",
		"trigger_events",
		"agent_class",
		"capability_profile",
	} {
		if db.Migrator().HasColumn("agents", column) {
			t.Fatalf("expected legacy column %q to be dropped", column)
		}
	}
}
