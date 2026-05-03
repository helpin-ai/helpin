package repository

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnsurePMExternalLinksTaskColumnSkipsBlankTaskID(t *testing.T) {
	dbName := fmt.Sprintf("file:pm_import_schema_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE pm_external_links (
		id TEXT PRIMARY KEY,
		task_id TEXT,
		entity_type TEXT,
		entity_id TEXT,
		title TEXT NOT NULL,
		url TEXT NOT NULL,
		created_by_id TEXT NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create pm_external_links: %v", err)
	}
	if err := db.Exec(`INSERT INTO pm_external_links (id, task_id, title, url, created_by_id) VALUES
		('link-empty', '', 'Empty task', 'https://example.com/empty', 'user-1'),
		('link-task', '11111111-1111-1111-1111-111111111111', 'Valid task', 'https://example.com/task', 'user-1')
	`).Error; err != nil {
		t.Fatalf("seed pm_external_links: %v", err)
	}

	if err := EnsurePMExternalLinksTaskColumn(db); err != nil {
		t.Fatalf("ensure external links task column: %v", err)
	}

	var rows []struct {
		ID         string
		EntityType string
		EntityID   *string
	}
	if err := db.Table("pm_external_links").Select("id, entity_type, entity_id").Order("id").Scan(&rows).Error; err != nil {
		t.Fatalf("load external links: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].ID != "link-empty" || rows[0].EntityType != "task" || rows[0].EntityID != nil {
		t.Fatalf("expected blank task row to keep nil entity_id, got %#v", rows[0])
	}
	if rows[1].ID != "link-task" || rows[1].EntityType != "task" || rows[1].EntityID == nil || *rows[1].EntityID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected valid task row to backfill entity_id, got %#v", rows[1])
	}
}
