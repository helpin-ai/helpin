package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTriageCandidatesScopeBeforeRanking(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:pm_triage_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := db.Exec(`CREATE TABLE pm_tasks (id TEXT PRIMARY KEY, workspace_id TEXT, team_id TEXT, display_id INTEGER, name TEXT, description TEXT, updated_at DATETIME, archived BOOLEAN)`).Error; err != nil {
		t.Fatal(err)
	}
	rows := []struct {
		id, workspace, team, name string
		archived                  bool
	}{
		{"source", "workspace", "mine", "CSV export", false},
		{"match", "workspace", "mine", "CSV export fails", false},
		{"related", "workspace", "mine", "CSV formats", false},
		{"foreign-team", "workspace", "other", "CSV export fails", false},
		{"foreign-workspace", "other", "mine", "CSV export fails", false},
		{"archived", "workspace", "mine", "CSV export fails", true},
		{"unrelated", "workspace", "mine", "Login fails", false},
	}
	for _, row := range rows {
		if err := db.Exec("INSERT INTO pm_tasks (id,workspace_id,team_id,name,description,archived) VALUES (?,?,?,?,?,?)", row.id, row.workspace, row.team, row.name, "", row.archived).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewPMTaskRepository(db)
	cases := []struct {
		name  string
		scope PMTriageScope
		want  []string
	}{
		{"member", PMTriageScope{WorkspaceID: "workspace", TeamIDs: []string{"mine"}}, []string{"match", "related"}},
		{"empty membership", PMTriageScope{WorkspaceID: "workspace"}, []string{}},
		{"administrator", PMTriageScope{WorkspaceID: "workspace", AllTeams: true}, []string{"foreign-team", "match", "related"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tasks, err := repo.FindTriageCandidates(context.Background(), tc.scope, "source", "CSV export")
			if err != nil {
				t.Fatal(err)
			}
			if len(tasks) != len(tc.want) {
				t.Fatalf("got %d candidates, want %d", len(tasks), len(tc.want))
			}
			for i, task := range tasks {
				if task.ID != tc.want[i] {
					t.Errorf("candidate %d: got %s want %s", i, task.ID, tc.want[i])
				}
			}
		})
	}
}
func TestTriageSearchTerms(t *testing.T) {
	cases := []struct {
		name, text string
		want       int
	}{
		{"empty", "the and please", 0},
		{"deduplicate", "Export export CSV", 2},
		{"punctuation", "export% _csv_", 2},
		{"unicode", "Überweisung schlägt fehl", 3},
		{"bounded", "one two three four five six seven eight nine ten eleven twelve thirteen fourteen", 12},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := triageSearchTerms(tc.text); len(got) != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}
