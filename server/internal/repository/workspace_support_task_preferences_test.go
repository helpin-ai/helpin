package repository

import (
	"context"
	"encoding/json"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestSupportTaskPreferencesKeepSeparateInboxTeams(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE workspace_members (id TEXT PRIMARY KEY, support_default_team_id TEXT, support_task_team_by_inbox TEXT DEFAULT '{}', updated_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO workspace_members (id) VALUES ('member'), ('other')`).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewWorkspaceRepository(db)
	for _, pair := range [][2]string{{"shared", "engineering"}, {"11111111-1111-4111-8111-111111111111", "marketing"}, {"shared", "support"}} {
		inbox, team := pair[0], pair[1]
		if err := repo.UpdateSupportTaskPreferences(context.Background(), "member", &team, nil, &inbox); err != nil {
			t.Fatal(err)
		}
	}
	var raw string
	if err := db.Raw(`SELECT support_task_team_by_inbox FROM workspace_members WHERE id = 'member'`).Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	var teams map[string]string
	if err := json.Unmarshal([]byte(raw), &teams); err != nil {
		t.Fatal(err)
	}
	if len(teams) != 2 || teams["shared"] != "support" || teams["11111111-1111-4111-8111-111111111111"] != "marketing" {
		t.Fatalf("inbox preferences lost: %v", teams)
	}
	if err := db.Raw(`SELECT support_task_team_by_inbox FROM workspace_members WHERE id = 'other'`).Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	if raw != "{}" {
		t.Fatalf("another member changed: %s", raw)
	}
}
