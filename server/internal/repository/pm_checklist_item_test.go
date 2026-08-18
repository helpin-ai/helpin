package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPMChecklistItemRepositoryDueDateRoundTripAndClear(t *testing.T) {
	t.Parallel()

	dbName := fmt.Sprintf("file:pm-checklist-due-date-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE pm_checklist_items (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		text TEXT NOT NULL,
		completed BOOLEAN NOT NULL DEFAULT 0,
		position INTEGER NOT NULL DEFAULT 0,
		assignee_id TEXT,
		due_date DATE,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create checklist table: %v", err)
	}

	repo := NewPMChecklistItemRepository(db)
	dueDate := time.Date(2026, time.September, 3, 0, 0, 0, 0, time.UTC)
	item := &model.PMChecklistItem{
		ID:      "checklist-due-date",
		TaskID:  "task-1",
		Text:    "Verify persistence",
		DueDate: &dueDate,
	}
	ctx := context.Background()
	if err := repo.Create(ctx, item); err != nil {
		t.Fatalf("create checklist item: %v", err)
	}

	stored, err := repo.GetByID(ctx, item.ID)
	if err != nil {
		t.Fatalf("get checklist item: %v", err)
	}
	if stored == nil || stored.DueDate == nil {
		t.Fatalf("stored due date = %v, want %s", stored, dueDate.Format("2006-01-02"))
	}
	if stored.DueDate.Format("2006-01-02") != dueDate.Format("2006-01-02") {
		t.Fatalf("stored due date = %s, want %s", stored.DueDate.Format("2006-01-02"), dueDate.Format("2006-01-02"))
	}

	stored.DueDate = nil
	if err := repo.Update(ctx, stored); err != nil {
		t.Fatalf("clear checklist due date: %v", err)
	}

	cleared, err := repo.GetByID(ctx, item.ID)
	if err != nil {
		t.Fatalf("reload checklist item: %v", err)
	}
	if cleared == nil {
		t.Fatal("reloaded checklist item is nil")
	}
	if cleared.DueDate != nil {
		t.Fatalf("cleared due date = %v, want nil", cleared.DueDate)
	}
}
