package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTriageCandidatesScopeBeforeRanking(t *testing.T) {
	db := setupTriageCandidatesDB(t)
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
		{"bounded", "one two three four five six seven eight nine ten eleven twelve thirteen fourteen", 14},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := triageSearchTerms(tc.text); len(got) != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func setupTriageCandidatesDB(t *testing.T) *gorm.DB {
	t.Helper()
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
	return db
}

func TestTriageCandidateRelevance(t *testing.T) {
	db := setupTriageCandidatesDB(t)
	insert := func(id, title, description string, age int) {
		t.Helper()
		if err := db.Exec("INSERT INTO pm_tasks(id,workspace_id,team_id,name,description,archived,updated_at) VALUES(?,?,?,?,?,?,?)", id, "workspace", "mine", title, description, false, time.Now().Add(-time.Duration(age)*time.Hour)).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 20; i++ {
		insert(fmt.Sprintf("generic-%02d", i), "Page error", "", 0)
	}
	insert("specific", "OAuth callback", "", 100)
	insert("substring", "Rapid capital improvement", "", 0)
	insert("markup", "Unrelated record", `<p class="oauth callback">Other content</p>`, 0)
	repo := NewPMTaskRepository(db)
	scope := PMTriageScope{WorkspaceID: "workspace", TeamIDs: []string{"mine"}}
	tasks, err := repo.FindTriageCandidates(context.Background(), scope, "", "Page error OAuth callback")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) == 0 || tasks[0].ID != "specific" {
		t.Fatalf("distinctive older match should lead: %+v", tasks)
	}
	for _, task := range tasks {
		if task.ID == "markup" {
			t.Fatal("HTML attributes became matching evidence")
		}
	}
	tasks, err = repo.FindTriageCandidates(context.Background(), scope, "", "API")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("substring produced false matches: %+v", tasks)
	}
	insert("late", "Zebra diagnosis", "", 0)
	terms := []string{}
	for i := 0; i < 80; i++ {
		terms = append(terms, fmt.Sprintf("context%d", i))
	}
	tasks, err = repo.FindTriageCandidates(context.Background(), scope, "", strings.Join(terms, " ")+" zebra")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].ID != "late" {
		t.Fatalf("lost diagnostic at end of description: %+v", tasks)
	}
	insert("plural", "Invoice exports", "", 0)
	tasks, err = repo.FindTriageCandidates(context.Background(), scope, "", "Invoice export")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].ID != "plural" {
		t.Fatalf("lost plural match: %+v", tasks)
	}
}

func TestTriageCandidatePoolAndTermsAreBounded(t *testing.T) {
	db := setupTriageCandidatesDB(t)
	for i := 0; i < 60; i++ {
		if err := db.Exec("INSERT INTO pm_tasks(id,workspace_id,team_id,name,description,archived) VALUES(?,?,?,?,?,?)", fmt.Sprint(i), "workspace", "mine", "Export invoices", "", false).Error; err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := NewPMTaskRepository(db).FindTriageCandidates(context.Background(), PMTriageScope{WorkspaceID: "workspace", AllTeams: true}, "", "Export invoice")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 50 {
		t.Fatalf("candidate pool = %d", len(tasks))
	}
	words := []string{}
	for i := 0; i < 100; i++ {
		words = append(words, fmt.Sprintf("word%d", i))
	}
	terms := triageSearchTerms(strings.Join(words, " "))
	if len(terms) != 32 || terms[len(terms)-1] != "word99" {
		t.Fatalf("unbounded or missing tail: %v", terms)
	}
}
