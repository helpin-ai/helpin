package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPMTaskServiceCreateRollsBackTaskAndRelationsWhenChecklistInsertFails(t *testing.T) {
	env := newTaskTestEnv(t)
	for _, task := range []model.PMTask{
		{ID: "existing-position-a", WorkspaceID: env.wsID, DisplayID: 20, Name: "Existing A", TaskType: model.PMTaskTypeFeature, WorkflowID: env.wfID, WorkflowStateID: env.stTodo, Position: 5},
		{ID: "existing-position-b", WorkspaceID: env.wsID, DisplayID: 21, Name: "Existing B", TaskType: model.PMTaskTypeFeature, WorkflowID: env.wfID, WorkflowStateID: env.stTodo, Position: 9},
	} {
		if dbErr := env.db.Create(&task).Error; dbErr != nil {
			t.Fatalf("seed positioned task: %v", dbErr)
		}
	}
	mustExec(t, env.db, `CREATE TRIGGER fail_task_checklist_insert BEFORE INSERT ON pm_checklist_items
		WHEN NEW.text = 'force rollback' BEGIN SELECT RAISE(ABORT, 'forced checklist failure'); END`)

	_, err := env.svc.Create(context.Background(), model.CreateTaskRequest{
		WorkspaceID: env.wsID, Name: "must roll back", WorkflowID: env.wfID,
		WorkflowStateID: env.stTodo, OwnerMemberIDs: []string{"member-story-001"},
		ChecklistItems: []model.CreateChecklistItemRequest{{Text: "created first"}, {Text: "force rollback"}},
	}, env.userID)
	if err == nil {
		t.Fatal("expected checklist insertion failure")
	}
	for table, want := range map[string]int64{"pm_tasks": 2, "pm_task_owners": 0, "pm_task_followers": 0, "pm_checklist_items": 0} {
		var got int64
		if dbErr := env.db.Table(table).Count(&got).Error; dbErr != nil {
			t.Fatalf("count %s: %v", table, dbErr)
		}
		if got != want {
			t.Fatalf("%s count = %d, want %d after rollback", table, got, want)
		}
	}
	var positioned []model.PMTask
	if dbErr := env.db.Where("id IN ?", []string{"existing-position-a", "existing-position-b"}).Order("id ASC").Find(&positioned).Error; dbErr != nil {
		t.Fatalf("reload positioned tasks: %v", dbErr)
	}
	if len(positioned) != 2 || positioned[0].Position != 5 || positioned[1].Position != 9 {
		t.Fatalf("existing positions changed despite rollback: %#v", positioned)
	}
}

func TestPMTaskServiceUpdateRollsBackCoreAndOwnersWhenLabelInsertFails(t *testing.T) {
	env := newTaskTestEnv(t)
	ctx := context.Background()
	task, err := env.svc.Create(ctx, model.CreateTaskRequest{
		WorkspaceID: env.wsID, Name: "original", WorkflowID: env.wfID,
		WorkflowStateID: env.stTodo, OwnerMemberIDs: []string{"member-story-001"},
	}, env.userID)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	seedUser(t, env.db, "user-atomic-2", "atomic2@test.com", "Atomic Two", "hash")
	seedWorkspaceMember(t, env.db, "member-atomic-2", env.wsID, "user-atomic-2", "atomic2@test.com", "Atomic Two", model.RoleMember)
	mustExec(t, env.db, `INSERT INTO pm_labels (id, workspace_id, name, color, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"label-atomic-fail", env.wsID, "Failing label", "#123456", false, time.Now(), time.Now())
	mustExec(t, env.db, `CREATE TRIGGER fail_task_label_insert BEFORE INSERT ON pm_task_labels
		WHEN NEW.label_id = 'label-atomic-fail' BEGIN SELECT RAISE(ABORT, 'forced label failure'); END`)

	name := "changed but rolled back"
	_, err = env.svc.Update(ctx, task.Task.ID, model.UpdateTaskRequest{
		Name: &name, OwnerMemberIDs: []string{"member-atomic-2"}, LabelIDs: []string{"label-atomic-fail"},
	}, env.userID)
	if err == nil {
		t.Fatal("expected label insertion failure")
	}
	var persisted model.PMTask
	if dbErr := env.db.First(&persisted, "id = ?", task.Task.ID).Error; dbErr != nil {
		t.Fatalf("reload task: %v", dbErr)
	}
	if persisted.Name != "original" {
		t.Fatalf("task name = %q, want original after rollback", persisted.Name)
	}
	var owners []model.PMTaskOwner
	if dbErr := env.db.Where("task_id = ?", task.Task.ID).Find(&owners).Error; dbErr != nil {
		t.Fatalf("reload owners: %v", dbErr)
	}
	if len(owners) != 1 || owners[0].UserID != env.userID {
		t.Fatalf("owners = %#v, want original owner after rollback", owners)
	}
}
