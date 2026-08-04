package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func TestPMChecklistItemServiceRejectsCrossWorkspaceAndTeamAccessForEveryMethod(t *testing.T) {
	db := newTestDB(t)
	taskRepo := repository.NewPMTaskRepository(db)
	checklistRepo := repository.NewPMChecklistItemRepository(db)
	service := NewPMChecklistItemService(checklistRepo, taskRepo, nil, nil, nil)
	now := time.Now().UTC()
	teamA := "team-a"
	teamB := "team-b"
	for _, task := range []model.PMTask{
		{ID: "task-a", WorkspaceID: "ws-a", DisplayID: 1, Name: "Allowed", TaskType: model.PMTaskTypeFeature, WorkflowID: "wf-a", WorkflowStateID: "state-a", TeamID: &teamA, CreatedAt: now, UpdatedAt: now},
		{ID: "task-cross-workspace", WorkspaceID: "ws-b", DisplayID: 1, Name: "Other workspace", TaskType: model.PMTaskTypeFeature, WorkflowID: "wf-b", WorkflowStateID: "state-b", TeamID: &teamB, CreatedAt: now, UpdatedAt: now},
		{ID: "task-cross-team", WorkspaceID: "ws-a", DisplayID: 2, Name: "Other team", TaskType: model.PMTaskTypeFeature, WorkflowID: "wf-a", WorkflowStateID: "state-a", TeamID: &teamB, CreatedAt: now, UpdatedAt: now},
	} {
		if err := db.Create(&task).Error; err != nil {
			t.Fatalf("seed task %s: %v", task.ID, err)
		}
	}
	for _, item := range []model.PMChecklistItem{
		{ID: "item-cross-workspace", TaskID: "task-cross-workspace", Text: "Other workspace"},
		{ID: "item-cross-team", TaskID: "task-cross-team", Text: "Other team"},
	} {
		if err := checklistRepo.Create(context.Background(), &item); err != nil {
			t.Fatalf("seed checklist %s: %v", item.ID, err)
		}
	}
	ctx := authorization.WithActor(context.Background(), &authorization.Actor{
		UserID: "actor-a", WorkspaceID: "ws-a", Role: model.RoleMember,
		TeamMemberships: []authorization.TeamRole{{TeamID: teamA, Role: "member"}},
	})

	for _, tc := range []struct {
		name   string
		taskID string
		itemID string
	}{
		{name: "cross workspace", taskID: "task-cross-workspace", itemID: "item-cross-workspace"},
		{name: "cross team", taskID: "task-cross-team", itemID: "item-cross-team"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := service.List(ctx, tc.taskID, "ws-a"); err == nil {
				t.Fatal("List unexpectedly allowed access")
			}
			if _, err := service.Create(ctx, tc.taskID, model.CreateChecklistItemRequest{Text: "Must not create"}, "ws-a", "actor-a"); err == nil {
				t.Fatal("Create unexpectedly allowed access")
			}
			text := "Must not update"
			if _, err := service.Update(ctx, tc.itemID, model.UpdateChecklistItemRequest{Text: &text}, "ws-a", "actor-a"); err == nil {
				t.Fatal("Update unexpectedly allowed access")
			}
			if err := service.Delete(ctx, tc.itemID, "ws-a", "actor-a"); err == nil {
				t.Fatal("Delete unexpectedly allowed access")
			}
			item, err := checklistRepo.GetByID(context.Background(), tc.itemID)
			if err != nil || item == nil || item.Text == text {
				t.Fatalf("forbidden item mutated/deleted: item=%#v err=%v", item, err)
			}
		})
	}
}

func TestPMChecklistItemServiceUpdateClearsAssigneeWithEmptyString(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repo := repository.NewPMChecklistItemRepository(db)
	taskRepo := repository.NewPMTaskRepository(db)
	seedChecklistServiceTask(t, db, "task-1", "ws-1", nil)
	service := NewPMChecklistItemService(repo, taskRepo, nil, nil, nil)
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
	taskRepo := repository.NewPMTaskRepository(db)
	seedChecklistServiceTask(t, db, "task-due-date", "ws-1", nil)
	service := NewPMChecklistItemService(repo, taskRepo, nil, nil, nil)
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

func seedChecklistServiceTask(t *testing.T, db *gorm.DB, id, workspaceID string, teamID *string) {
	t.Helper()
	now := time.Now().UTC()
	task := model.PMTask{ID: id, WorkspaceID: workspaceID, DisplayID: 1, Name: id, TaskType: model.PMTaskTypeFeature, WorkflowID: "wf", WorkflowStateID: "state", TeamID: teamID, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("seed task %s: %v", id, err)
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
