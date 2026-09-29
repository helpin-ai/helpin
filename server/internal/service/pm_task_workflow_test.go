package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func seedTaskTeamWorkflow(t *testing.T, env taskTestEnv, teamID string) string {
	t.Helper()
	wfID := "workflow-" + teamID
	defaultID := wfID + "-todo"
	if err := env.db.Create(&model.PMWorkflow{ID: wfID, WorkspaceID: env.wsID, TeamID: &teamID, Name: "Team workflow", DefaultStateID: &defaultID}).Error; err != nil {
		t.Fatal(err)
	}
	for i, state := range []struct{ suffix, name, kind string }{{"todo", "To Do", "unstarted"}, {"started", "In Progress", "started"}, {"done", "Done", "done"}} {
		if err := env.db.Create(&model.PMWorkflowState{ID: wfID + "-" + state.suffix, WorkflowID: wfID, Name: state.name, StateType: state.kind, Position: i, IsDefault: i == 0}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return wfID
}

func TestPMTaskCreateTeamWorkflow(t *testing.T) {
	for _, tc := range []struct {
		name, workflow, state, wantState, wantError string
	}{
		{name: "omitted workflow", wantState: "todo"},
		{name: "explicit team workflow", workflow: "team", wantState: "todo"},
		{name: "state only", state: "team", wantState: "started"},
		{name: "workspace workflow rejected", workflow: "default", state: "default", wantError: "workflow_id does not belong to team_id"},
		{name: "workspace state rejected", state: "default", wantError: "workflow_state_id must belong to workflow_id"},
		{name: "unknown workflow rejected", workflow: "missing", wantError: "workflow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newTaskTestEnv(t)
			wfID := seedTaskTeamWorkflow(t, env, env.teamID)
			workflows := map[string]string{"team": wfID, "default": env.wfID, "missing": "missing-workflow"}
			states := map[string]string{"team": wfID + "-started", "default": env.stTodo}
			result, err := env.svc.Create(context.Background(), model.CreateTaskRequest{WorkspaceID: env.wsID, Name: "Team task", TeamID: &env.teamID, WorkflowID: workflows[tc.workflow], WorkflowStateID: states[tc.state]}, env.userID)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
				var count int64
				if err := env.db.Model(&model.PMTask{}).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("rejected task persisted: count=%d, err=%v", count, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.Task.WorkflowID != wfID || result.Task.WorkflowStateID != wfID+"-"+tc.wantState {
				t.Fatalf("workflow/state = %s/%s, want %s/%s", result.Task.WorkflowID, result.Task.WorkflowStateID, wfID, tc.wantState)
			}
		})
	}
}

func TestPMTaskCreateFallsBackWithoutTeamWorkflow(t *testing.T) {
	env := newTaskTestEnv(t)
	result, err := env.svc.Create(context.Background(), model.CreateTaskRequest{WorkspaceID: env.wsID, Name: "Fallback task", TeamID: &env.teamID}, env.userID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Task.WorkflowID != env.wfID || result.Task.WorkflowStateID != env.stTodo {
		t.Fatalf("unexpected workflow/state: %s/%s", result.Task.WorkflowID, result.Task.WorkflowStateID)
	}
}

func TestPMTaskCreateDoesNotBorrowAnotherTeamsWorkflow(t *testing.T) {
	env := newTaskTestEnv(t)
	otherTeam := "other-team"
	seedTaskTeam(t, env, otherTeam, "Other team")
	mustExec(t, env.db, "UPDATE pm_workflows SET team_id = ? WHERE id = ?", otherTeam, env.wfID)
	result, err := env.svc.Create(context.Background(), model.CreateTaskRequest{WorkspaceID: env.wsID, Name: "No shared workflow yet", TeamID: &env.teamID}, env.userID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Task.WorkflowID == env.wfID {
		t.Fatal("task borrowed another team's workflow")
	}
}

func TestPMTaskUpdateRequiresExplicitStatusWhenNoEquivalentExists(t *testing.T) {
	env := newTaskTestEnv(t)
	ctx := context.Background()
	created, err := env.svc.Create(ctx, model.CreateTaskRequest{WorkspaceID: env.wsID, Name: "Completed task", TeamID: &env.teamID, WorkflowStateID: env.stDone}, env.userID)
	if err != nil {
		t.Fatal(err)
	}
	newTeam := "destination-team"
	seedTaskTeam(t, env, newTeam, "Destination")
	newWF := seedTaskTeamWorkflow(t, env, newTeam)
	mustExec(t, env.db, "DELETE FROM pm_workflow_states WHERE id = ?", newWF+"-done")
	_, err = env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{TeamID: &newTeam}, env.userID)
	if err == nil || !strings.Contains(err.Error(), "no equivalent status") {
		t.Fatalf("error = %v, want explicit status required", err)
	}
	unchanged, err := env.svc.GetByID(ctx, created.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Task.WorkflowID != env.wfID || !unchanged.Task.Completed {
		t.Fatal("rejected team change altered the task")
	}
	chosen := newWF + "-started"
	updated, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{TeamID: &newTeam, WorkflowStateID: &chosen}, env.userID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Task.WorkflowID != newWF || updated.Task.WorkflowStateID != chosen || updated.Task.Completed {
		t.Fatal("explicit destination status was not applied")
	}
}

func TestPMTaskUpdateTeamMapsWorkflowState(t *testing.T) {
	for _, suffix := range []string{"todo", "started", "done"} {
		t.Run(suffix, func(t *testing.T) {
			env := newTaskTestEnv(t)
			ctx := context.Background()
			oldWF := seedTaskTeamWorkflow(t, env, env.teamID)
			newTeam := "destination-team"
			seedTaskTeam(t, env, newTeam, "Destination")
			newWF := seedTaskTeamWorkflow(t, env, newTeam)
			created, err := env.svc.Create(ctx, model.CreateTaskRequest{WorkspaceID: env.wsID, Name: "Move me", TeamID: &env.teamID, WorkflowID: oldWF, WorkflowStateID: oldWF + "-" + suffix}, env.userID)
			if err != nil {
				t.Fatal(err)
			}
			updated, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{TeamID: &newTeam}, env.userID)
			if err != nil {
				t.Fatal(err)
			}
			if updated.Task.WorkflowID != newWF || updated.Task.WorkflowStateID != newWF+"-"+suffix || updated.Task.Completed != created.Task.Completed || updated.Task.Started != created.Task.Started {
				t.Fatalf("team change lost workflow or status: %#v", updated.Task)
			}
			if created.Task.CompletedAt != nil && (updated.Task.CompletedAt == nil || !updated.Task.CompletedAt.Equal(*created.Task.CompletedAt)) {
				t.Fatal("moving a completed task changed its completion date")
			}
		})
	}
}

func TestPMTaskUpdateRejectsUnrelatedWorkflow(t *testing.T) {
	env := newTaskTestEnv(t)
	wfID := seedTaskTeamWorkflow(t, env, env.teamID)
	created, err := env.svc.Create(context.Background(), model.CreateTaskRequest{WorkspaceID: env.wsID, Name: "Stay on team", TeamID: &env.teamID, WorkflowID: wfID, WorkflowStateID: wfID + "-todo"}, env.userID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.Update(context.Background(), created.Task.ID, model.UpdateTaskRequest{WorkflowID: &env.wfID, WorkflowStateID: &env.stTodo}, env.userID)
	if err == nil {
		t.Fatal("accepted workspace workflow despite team workflow")
	}
}
