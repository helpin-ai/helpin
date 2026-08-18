package repository

import (
	"context"
	"math"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPMObjectiveRepositoryListPageHugePositivePageReturnsEmpty(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE pm_objectives (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		name TEXT NOT NULL,
		position INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create objective table: %v", err)
	}
	if err := db.Exec(`INSERT INTO pm_objectives (id, workspace_id, name, position, created_at) VALUES (?, ?, ?, 0, CURRENT_TIMESTAMP)`,
		"objective-page-overflow", "workspace-page-overflow", "Overflow guard").Error; err != nil {
		t.Fatalf("seed objective: %v", err)
	}

	objectives, total, err := NewPMObjectiveRepository(db).ListPage(context.Background(), "workspace-page-overflow", model.PMObjectiveListFilters{}, model.PMPagination{
		Page: math.MaxInt, PerPage: 100,
	})
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if total != 1 || len(objectives) != 0 {
		t.Fatalf("huge page returned rows: total=%d objectives=%#v", total, objectives)
	}
}
