package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPMTaskServiceSeedCreatesRequestedTasks(t *testing.T) {
	env := newTaskTestEnv(t)
	ctx := context.Background()

	result, err := env.svc.Seed(ctx, model.SeedPMTasksRequest{
		WorkspaceID: env.wsID,
		Count:       500,
	})
	if err != nil {
		t.Fatalf("Seed error = %v", err)
	}
	if result.Created != 500 {
		t.Fatalf("result.Created = %d, want 500", result.Created)
	}

	var total int64
	if err := env.db.Model(&model.PMTask{}).Where("workspace_id = ?", env.wsID).Count(&total).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if total != 500 {
		t.Fatalf("total = %d, want 500", total)
	}

	first, err := repository.NewPMTaskRepository(env.db).GetByDisplayID(ctx, env.wsID, 1)
	if err != nil {
		t.Fatalf("GetByDisplayID(1): %v", err)
	}
	if first == nil {
		t.Fatal("expected seeded task with display_id 1")
	}
	if first.Task.Name != "Seeded feature 1" {
		t.Fatalf("first task name = %q, want %q", first.Task.Name, "Seeded feature 1")
	}

	var doneCount int64
	if err := env.db.Model(&model.PMTask{}).
		Where("workspace_id = ? AND completed = ?", env.wsID, true).
		Count(&doneCount).Error; err != nil {
		t.Fatalf("count done tasks: %v", err)
	}
	if doneCount == 0 {
		t.Fatal("expected some seeded done tasks")
	}
}

func TestPMTaskServiceSeedDefaultsAndAppendsFromMaxDisplayID(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	const (
		workspaceID = "ws-task-seed-default"
		ownerUserID = "user-owner"
		memberID    = "member-owner"
	)

	seedUser(t, db, ownerUserID, "owner@example.com", "Owner User", "hash")
	seedWorkspace(t, db, workspaceID, "Workspace", "workspace", ownerUserID)
	seedWorkspaceMember(t, db, memberID, workspaceID, ownerUserID, "owner@example.com", "Owner User", model.RoleOwner)

	taskRepo := repository.NewPMTaskRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	svc := NewPMTaskService(
		taskRepo,
		workspaceRepo,
		workflowRepo,
		repository.NewPMEpicRepository(db),
		repository.NewPMSprintRepository(db),
		repository.NewPMLabelRepository(db),
		nil,
		nil,
		repository.NewPMAttachmentRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	seededWorkflow, err := workflowRepo.SeedDefaultWorkflow(ctx, workspaceID)
	if err != nil {
		t.Fatalf("SeedDefaultWorkflow: %v", err)
	}

	existingTask := &model.PMTask{
		WorkspaceID:     workspaceID,
		DisplayID:       7,
		Name:            "Existing task",
		TaskType:        model.PMTaskTypeFeature,
		WorkflowID:      seededWorkflow.Workflow.ID,
		WorkflowStateID: seededWorkflow.States[1].ID,
		Priority:        model.PMTaskPriorityNone,
		Severity:        model.PMTaskSeverityNone,
		Position:        0,
	}
	if err := taskRepo.Create(ctx, existingTask); err != nil {
		t.Fatalf("seed existing task: %v", err)
	}

	result, err := svc.Seed(ctx, model.SeedPMTasksRequest{WorkspaceID: workspaceID})
	if err != nil {
		t.Fatalf("Seed error = %v", err)
	}
	if result.Created != 500 {
		t.Fatalf("result.Created = %d, want 500", result.Created)
	}

	task, err := taskRepo.GetByDisplayID(ctx, workspaceID, 8)
	if err != nil {
		t.Fatalf("GetByDisplayID(8): %v", err)
	}
	if task == nil {
		t.Fatal("expected seeded task with display_id 8")
	}
}

func TestPMTaskServiceSeedRejectsOversizedBatch(t *testing.T) {
	env := newTaskTestEnv(t)
	ctx := context.Background()

	if _, err := env.svc.Seed(ctx, model.SeedPMTasksRequest{
		WorkspaceID: env.wsID,
		Count:       2001,
	}); err == nil {
		t.Fatal("expected oversize seed request to fail")
	}
}
