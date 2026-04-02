package repository

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

func TestPMTaskRepository_StateBoardOrderingAndNormalization(t *testing.T) {
	t.Parallel()

	const (
		workspaceID  = "ws-state-board"
		workflowID   = "wf-state-board"
		todoStateID  = "state-state-todo"
		doingStateID = "state-state-doing"
		doneStateID  = "state-state-done"
		userID       = "user-state-board"
		memberID     = "member-state-board"
	)

	now := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)

	setup := func(t *testing.T) (*PMTaskRepository, *gorm.DB, context.Context) {
		t.Helper()
		db := newPMTaskMemberBoardTestDB(t)
		seedPMTaskMemberBoardUser(t, db, userID, "state-board@test.com", "State Board User")
		seedPMTaskMemberBoardWorkspace(t, db, workspaceID, userID)
		seedPMTaskMemberBoardMember(t, db, memberID, workspaceID, userID, "State Board User")
		seedPMTaskMemberBoardWorkflow(t, db, workflowID, workspaceID, todoStateID, doingStateID, doneStateID)
		return NewPMTaskRepository(db), db, context.Background()
	}

	t.Run("ListByWorkflowState orders done by recency before position", func(t *testing.T) {
		repo, db, ctx := setup(t)
		insertPMTaskMemberBoardTask(t, db, "done-old", workspaceID, workflowID, doneStateID, memberID, 1, 5, now.Add(-4*time.Hour))
		insertPMTaskMemberBoardTask(t, db, "done-new", workspaceID, workflowID, doneStateID, memberID, 2, 0, now.Add(-3*time.Hour))
		if err := db.Exec(
			`UPDATE pm_stories SET completed = ?, completed_at = ?, moved_at = ? WHERE id = ?`,
			true, now.Add(-2*time.Hour), now.Add(-2*time.Hour), "done-old",
		).Error; err != nil {
			t.Fatalf("update done-old: %v", err)
		}
		if err := db.Exec(
			`UPDATE pm_stories SET completed = ?, completed_at = ?, moved_at = ? WHERE id = ?`,
			true, now.Add(-30*time.Minute), now.Add(-30*time.Minute), "done-new",
		).Error; err != nil {
			t.Fatalf("update done-new: %v", err)
		}

		columns, err := repo.ListByWorkflowState(ctx, workflowID, model.PMTaskFilters{}, 0)
		if err != nil {
			t.Fatalf("ListByWorkflowState: %v", err)
		}

		var doneColumn *model.TaskStateColumn
		for i := range columns {
			if columns[i].State.ID == doneStateID {
				doneColumn = &columns[i]
				break
			}
		}
		if doneColumn == nil {
			t.Fatal("expected done column")
		}
		got := []string{doneColumn.Stories[0].ID, doneColumn.Stories[1].ID}
		want := []string{"done-new", "done-old"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("done stories[%d] = %q, want %q (full order %v)", i, got[i], want[i], got)
			}
		}
	})

	t.Run("MoveToState clamps oversized target positions", func(t *testing.T) {
		repo, db, ctx := setup(t)
		insertPMTaskMemberBoardTask(t, db, "move-source", workspaceID, workflowID, todoStateID, memberID, 10, 0, now.Add(10*time.Minute))
		insertPMTaskMemberBoardTask(t, db, "move-target-1", workspaceID, workflowID, doingStateID, memberID, 11, 0, now.Add(11*time.Minute))
		insertPMTaskMemberBoardTask(t, db, "move-target-2", workspaceID, workflowID, doingStateID, memberID, 12, 1, now.Add(12*time.Minute))

		position := 99
		if err := repo.MoveToState(ctx, "move-source", doingStateID, &position, "trace-move-clamp"); err != nil {
			t.Fatalf("MoveToState: %v", err)
		}

		var stories []model.PMTask
		if err := db.WithContext(ctx).
			Where("workflow_state_id = ? AND id IN ?", doingStateID, []string{"move-source", "move-target-1", "move-target-2"}).
			Order("position ASC").
			Find(&stories).Error; err != nil {
			t.Fatalf("query moved stories: %v", err)
		}

		gotIDs := []string{stories[0].ID, stories[1].ID, stories[2].ID}
		wantIDs := []string{"move-target-1", "move-target-2", "move-source"}
		for i := range wantIDs {
			if gotIDs[i] != wantIDs[i] {
				t.Fatalf("moved stories[%d] = %q, want %q (full order %v)", i, gotIDs[i], wantIDs[i], gotIDs)
			}
			if stories[i].Position != i {
				t.Fatalf("moved stories[%d] position = %d, want %d", i, stories[i].Position, i)
			}
		}
	})

	t.Run("NextPosition normalizes duplicate positions before returning the append index", func(t *testing.T) {
		repo, db, ctx := setup(t)
		insertPMTaskMemberBoardTask(t, db, "next-a", workspaceID, workflowID, todoStateID, memberID, 40, 0, now.Add(40*time.Minute))
		insertPMTaskMemberBoardTask(t, db, "next-b", workspaceID, workflowID, todoStateID, memberID, 41, 0, now.Add(39*time.Minute))
		insertPMTaskMemberBoardTask(t, db, "next-c", workspaceID, workflowID, todoStateID, memberID, 42, 3, now.Add(38*time.Minute))

		position, err := repo.NextPosition(ctx, workspaceID, todoStateID)
		if err != nil {
			t.Fatalf("NextPosition: %v", err)
		}
		if position != 3 {
			t.Fatalf("next position = %d, want 3", position)
		}

		var stories []model.PMTask
		if err := db.WithContext(ctx).
			Where("workflow_state_id = ? AND id IN ?", todoStateID, []string{"next-a", "next-b", "next-c"}).
			Order("position ASC, updated_at DESC").
			Find(&stories).Error; err != nil {
			t.Fatalf("query normalized next-position stories: %v", err)
		}

		for i, story := range stories {
			if story.Position != i {
				t.Fatalf("normalized next-position story %q position = %d, want %d", story.ID, story.Position, i)
			}
		}
	})

	t.Run("Reorder clamps oversized positions to the end of the column", func(t *testing.T) {
		repo, db, ctx := setup(t)
		insertPMTaskMemberBoardTask(t, db, "reorder-1", workspaceID, workflowID, todoStateID, memberID, 20, 0, now.Add(20*time.Minute))
		insertPMTaskMemberBoardTask(t, db, "reorder-2", workspaceID, workflowID, todoStateID, memberID, 21, 1, now.Add(21*time.Minute))
		insertPMTaskMemberBoardTask(t, db, "reorder-3", workspaceID, workflowID, todoStateID, memberID, 22, 2, now.Add(22*time.Minute))

		if err := repo.Reorder(ctx, "reorder-1", 99, "trace-reorder-clamp"); err != nil {
			t.Fatalf("Reorder: %v", err)
		}

		var stories []model.PMTask
		if err := db.WithContext(ctx).
			Where("workflow_state_id = ? AND id IN ?", todoStateID, []string{"reorder-1", "reorder-2", "reorder-3"}).
			Order("position ASC").
			Find(&stories).Error; err != nil {
			t.Fatalf("query reordered stories: %v", err)
		}

		gotIDs := []string{stories[0].ID, stories[1].ID, stories[2].ID}
		wantIDs := []string{"reorder-2", "reorder-3", "reorder-1"}
		for i := range wantIDs {
			if gotIDs[i] != wantIDs[i] {
				t.Fatalf("reordered stories[%d] = %q, want %q (full order %v)", i, gotIDs[i], wantIDs[i], gotIDs)
			}
			if stories[i].Position != i {
				t.Fatalf("reordered stories[%d] position = %d, want %d", i, stories[i].Position, i)
			}
		}
	})

	t.Run("Reorder normalizes duplicate positions before applying the requested move", func(t *testing.T) {
		repo, db, ctx := setup(t)
		insertPMTaskMemberBoardTask(t, db, "dup-a", workspaceID, workflowID, todoStateID, memberID, 30, 0, now.Add(30*time.Minute))
		insertPMTaskMemberBoardTask(t, db, "dup-b", workspaceID, workflowID, todoStateID, memberID, 31, 0, now.Add(29*time.Minute))
		insertPMTaskMemberBoardTask(t, db, "dup-c", workspaceID, workflowID, todoStateID, memberID, 32, 0, now.Add(28*time.Minute))
		insertPMTaskMemberBoardTask(t, db, "dup-d", workspaceID, workflowID, todoStateID, memberID, 33, 1, now.Add(27*time.Minute))

		if err := repo.Reorder(ctx, "dup-d", 1, "trace-reorder-normalize-duplicates"); err != nil {
			t.Fatalf("Reorder: %v", err)
		}

		var stories []model.PMTask
		if err := db.WithContext(ctx).
			Where("workflow_state_id = ? AND id IN ?", todoStateID, []string{"dup-a", "dup-b", "dup-c", "dup-d"}).
			Order("position ASC, updated_at DESC").
			Find(&stories).Error; err != nil {
			t.Fatalf("query normalized reorder stories: %v", err)
		}

		gotIDs := []string{stories[0].ID, stories[1].ID, stories[2].ID, stories[3].ID}
		wantIDs := []string{"dup-a", "dup-d", "dup-b", "dup-c"}
		for i := range wantIDs {
			if gotIDs[i] != wantIDs[i] {
				t.Fatalf("normalized reordered stories[%d] = %q, want %q (full order %v)", i, gotIDs[i], wantIDs[i], gotIDs)
			}
			if stories[i].Position != i {
				t.Fatalf("normalized reordered stories[%d] position = %d, want %d", i, stories[i].Position, i)
			}
		}
	})
}
