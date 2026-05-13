package service

import (
	"context"
	"testing"

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
