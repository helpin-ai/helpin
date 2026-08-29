package repository

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEventProjectRepositoryResolveProjectSet(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:event-projects?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	createEventProjectAliasTestTable(t, db)
	workspaceID := "00000000-0000-0000-0000-000000000001"
	aliases := []model.WorkspaceEventProjectAlias{
		{ID: "alias-widget", WorkspaceID: workspaceID, ProjectID: "legacy-widget", Source: "legacy_widget_key", CreatedAt: time.Now()},
		{ID: "alias-secret", WorkspaceID: workspaceID, ProjectID: "legacy-secret", Source: "legacy_server_secret", CreatedAt: time.Now().Add(time.Second)},
	}
	if err := db.Create(&aliases).Error; err != nil {
		t.Fatalf("create aliases: %v", err)
	}

	projects, err := NewEventProjectRepository(db).ResolveProjectSet(context.Background(), workspaceID)
	if err != nil {
		t.Fatalf("resolve project set: %v", err)
	}
	want := []string{workspaceID, "legacy-widget", "legacy-secret"}
	if len(projects) != len(want) {
		t.Fatalf("projects = %#v", projects)
	}
	for i := range want {
		if projects[i] != want[i] {
			t.Fatalf("projects[%d] = %q, want %q", i, projects[i], want[i])
		}
	}
}

func TestEventProjectAliasCannotCrossWorkspaces(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:event-project-ownership?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	createEventProjectAliasTestTable(t, db)
	first := model.WorkspaceEventProjectAlias{ID: "first", WorkspaceID: "ws-1", ProjectID: "shared", Source: "legacy_widget_key"}
	second := model.WorkspaceEventProjectAlias{ID: "second", WorkspaceID: "ws-2", ProjectID: "shared", Source: "legacy_widget_key"}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first alias: %v", err)
	}
	if err := db.Create(&second).Error; err == nil {
		t.Fatal("expected cross-workspace alias collision to fail")
	}
}

func TestListAllActiveEventWorkspaceIDsDoesNotRequireLiveRollout(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:event-workspaces-shadow?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE support_widget_installations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, active BOOLEAN NOT NULL)`,
		`INSERT INTO support_widget_installations VALUES ('one', 'workspace-shadow', 1)`,
		`INSERT INTO support_widget_installations VALUES ('two', 'workspace-live', 1)`,
		`INSERT INTO support_widget_installations VALUES ('three', 'workspace-disabled', 0)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed event workspaces: %v", err)
		}
	}
	workspaceIDs, err := NewEventProjectRepository(db).ListAllActiveEventWorkspaceIDs(context.Background())
	if err != nil {
		t.Fatalf("list event workspaces: %v", err)
	}
	want := []string{"workspace-live", "workspace-shadow"}
	if len(workspaceIDs) != len(want) {
		t.Fatalf("workspace IDs=%v, want %v", workspaceIDs, want)
	}
	for index := range want {
		if workspaceIDs[index] != want[index] {
			t.Fatalf("workspace IDs=%v, want %v", workspaceIDs, want)
		}
	}
}

func createEventProjectAliasTestTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE workspace_event_project_aliases (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, project_id TEXT NOT NULL UNIQUE,
		source TEXT NOT NULL, valid_from DATETIME, valid_to DATETIME, created_at DATETIME,
		UNIQUE (workspace_id, project_id)
	)`).Error; err != nil {
		t.Fatalf("create alias table: %v", err)
	}
}
