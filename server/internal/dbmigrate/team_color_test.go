package dbmigrate

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSolidTeamColors(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	var sql string
	for _, migration := range migrations {
		if migration.Version == "202610020001" {
			sql = migration.SQL
		}
	}
	if sql == "" {
		t.Fatal("missing solid team color migration")
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := db.Exec(`CREATE TABLE workspace_teams (id text PRIMARY KEY, color text)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO workspace_teams VALUES ('blue', '#9ec1f3'), ('red', '#efa29b'), ('solid', '#4e8fea'), ('custom', '#123456'), ('unset', NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
		var rows []struct {
			ID    string
			Color *string
		}
		if err := db.Raw(`SELECT id, color FROM workspace_teams`).Scan(&rows).Error; err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"blue": "#4e8fea", "red": "#e2564a", "solid": "#4e8fea", "custom": "#123456"}
		for _, row := range rows {
			if row.ID == "unset" {
				if row.Color != nil {
					t.Fatal("reset team colors must remain unset")
				}
				continue
			}
			if row.Color == nil || *row.Color != want[row.ID] {
				t.Fatalf("unexpected color for %s: %v", row.ID, row.Color)
			}
		}
	}
}

func TestTeamColorBackfill(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	var backfill string
	for _, migration := range migrations {
		if migration.Version == "202610010004" {
			if index := strings.Index(migration.SQL, "WITH ranked_teams AS"); index >= 0 {
				backfill = migration.SQL[index:]
			}
		}
	}
	if backfill == "" {
		t.Fatal("team color migration must assign colors to existing teams")
	}
	// Run the actual portable backfill SQL; PostgreSQL owns the column constraint.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.Exec(`CREATE TABLE workspace_teams (id text PRIMARY KEY, workspace_id text, name text, color text)`).Error; err != nil {
		t.Fatal(err)
	}
	for i := range 9 {
		if err := db.Exec(`INSERT INTO workspace_teams VALUES (?, 'ws1', ?, NULL)`, fmt.Sprintf("team-%d", i), fmt.Sprintf("Team %d", i)).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO workspace_teams VALUES ('other', 'ws2', 'Team 0', NULL), ('chosen', 'ws3', 'Chosen', '#123456')`).Error; err != nil {
		t.Fatal(err)
	}
	readColors := func() map[string]string {
		t.Helper()
		var rows []struct{ ID, Color string }
		if err := db.Raw(`SELECT id, color FROM workspace_teams`).Scan(&rows).Error; err != nil {
			t.Fatal(err)
		}
		colors := make(map[string]string, len(rows))
		for _, row := range rows {
			colors[row.ID] = row.Color
		}
		return colors
	}
	if err := db.Exec(backfill).Error; err != nil {
		t.Fatal(err)
	}
	colors := readColors()
	distinct := map[string]bool{}
	for i := range 8 {
		color := colors[fmt.Sprintf("team-%d", i)]
		if color == "" {
			t.Fatal("existing team did not get a color")
		}
		distinct[color] = true
	}
	if len(distinct) != 8 || colors["team-8"] != colors["team-0"] {
		t.Fatalf("expected eight distinct colors before repeating: %v", colors)
	}
	if colors["other"] != colors["team-0"] || colors["chosen"] != "#123456" {
		t.Fatalf("expected workspace-scoped assignment and preserved choice: %v", colors)
	}
	if err := db.Exec(backfill).Error; err != nil {
		t.Fatal(err)
	}
	if after := readColors(); !reflect.DeepEqual(colors, after) {
		t.Fatalf("retry changed saved colors: before=%v after=%v", colors, after)
	}
}
