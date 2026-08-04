package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPMChecklistItemServiceUpdateClearsAssigneeWithEmptyString(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repo := repository.NewPMChecklistItemRepository(db)
	service := NewPMChecklistItemService(repo, nil, nil, nil, nil)
	assigneeID := "user-1"
	item := &model.PMChecklistItem{
		ID:         "checklist-1",
		TaskID:     "task-1",
		Text:       "Ship the fix",
		AssigneeID: &assigneeID,
	}
	if err := repo.Create(context.Background(), item); err != nil {
		t.Fatalf("create checklist item: %v", err)
	}

	clearValue := ""
	updated, err := service.Update(context.Background(), item.ID, model.UpdateChecklistItemRequest{
		AssigneeID: &clearValue,
	}, "ws-1", "actor-1")
	if err != nil {
		t.Fatalf("update checklist item: %v", err)
	}
	if updated.AssigneeID != nil {
		t.Fatalf("AssigneeID = %v, want nil", *updated.AssigneeID)
	}
}

func TestPMChecklistItemDueDateCreateUpdateClearAndPreserve(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repo := repository.NewPMChecklistItemRepository(db)
	service := NewPMChecklistItemService(repo, nil, nil, nil, nil)
	ctx := context.Background()
	initialDueDate := time.Date(2026, time.August, 15, 0, 0, 0, 0, time.UTC)

	created, err := service.Create(ctx, "task-due-date", model.CreateChecklistItemRequest{
		Text:    "Ship the fix",
		DueDate: &initialDueDate,
	}, "ws-1", "actor-1")
	if err != nil {
		t.Fatalf("create checklist item: %v", err)
	}
	assertChecklistDueDate(t, created.DueDate, initialDueDate)

	updatedDueDate := time.Date(2026, time.August, 22, 0, 0, 0, 0, time.UTC)
	updated, err := service.Update(ctx, created.ID, model.UpdateChecklistItemRequest{
		DueDate: &updatedDueDate,
	}, "ws-1", "actor-1")
	if err != nil {
		t.Fatalf("update checklist due date: %v", err)
	}
	assertChecklistDueDate(t, updated.DueDate, updatedDueDate)

	newText := "Ship the tested fix"
	preserved, err := service.Update(ctx, created.ID, model.UpdateChecklistItemRequest{
		Text: &newText,
	}, "ws-1", "actor-1")
	if err != nil {
		t.Fatalf("update checklist item without due date: %v", err)
	}
	assertChecklistDueDate(t, preserved.DueDate, updatedDueDate)

	cleared, err := service.Update(ctx, created.ID, model.UpdateChecklistItemRequest{
		DueDateSet: true,
	}, "ws-1", "actor-1")
	if err != nil {
		t.Fatalf("clear checklist due date: %v", err)
	}
	if cleared.DueDate != nil {
		t.Fatalf("cleared due date = %v, want nil", cleared.DueDate)
	}

	reloaded, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("reload cleared checklist item: %v", err)
	}
	if reloaded == nil {
		t.Fatal("reloaded checklist item is nil")
	}
	if reloaded.DueDate != nil {
		t.Fatalf("persisted cleared due date = %v, want nil", reloaded.DueDate)
	}
}

func assertChecklistDueDate(t *testing.T, got *time.Time, want time.Time) {
	t.Helper()
	if got == nil {
		t.Fatalf("due date = nil, want %s", want.Format("2006-01-02"))
	}
	if got.Format("2006-01-02") != want.Format("2006-01-02") {
		t.Fatalf("due date = %s, want %s", got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}
