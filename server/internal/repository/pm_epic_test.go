package repository

import (
	"context"
	"testing"
	"time"
)

func TestPMEpicRepositoryListTasksKeepsWorkflowStatesContiguous(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-epic-task-order"
		userID      = "user-epic-task-order"
		memberID    = "member-epic-task-order"
		epicID      = "epic-task-order"
	)

	db := newPMTaskMemberBoardTestDB(t)
	seedPMTaskMemberBoardUser(t, db, userID, "epic-order@test.com", "Epic Order User")
	seedPMTaskMemberBoardWorkspace(t, db, workspaceID, userID)
	seedPMTaskMemberBoardMember(t, db, memberID, workspaceID, userID, "Epic Order User")
	seedPMTaskMemberBoardWorkflow(t, db, "wf-a", workspaceID, "state-a-todo", "state-a-doing", "state-a-done")
	seedPMTaskMemberBoardWorkflow(t, db, "wf-b", workspaceID, "state-b-todo", "state-b-doing", "state-b-done")

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	insertPMTaskMemberBoardTask(t, db, "a-pos-0", workspaceID, "wf-a", "state-a-todo", memberID, 68, 0, now)
	insertPMTaskMemberBoardTask(t, db, "a-pos-1", workspaceID, "wf-a", "state-a-todo", memberID, 71, 1, now.Add(time.Minute))
	insertPMTaskMemberBoardTask(t, db, "b-pos-0", workspaceID, "wf-b", "state-b-todo", memberID, 94, 0, now.Add(2*time.Minute))

	if err := db.Exec(`UPDATE pm_tasks SET epic_id = ? WHERE id IN ?`, epicID, []string{"a-pos-0", "a-pos-1", "b-pos-0"}).Error; err != nil {
		t.Fatalf("assign epic: %v", err)
	}

	tasks, err := NewPMEpicRepository(db).ListTasks(context.Background(), epicID)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	got := make([]string, len(tasks))
	for i, task := range tasks {
		got[i] = task.ID
	}
	want := []string{"a-pos-0", "a-pos-1", "b-pos-0"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tasks[%d] = %q, want %q (full order %v)", i, got[i], want[i], got)
		}
	}
}
