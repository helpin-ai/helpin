package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type playbookPMTestAuthority struct {
	playbookLiveTestAuthority
	teamID     string
	pmDisabled bool
	teamLost   bool
}

func (a *playbookPMTestAuthority) ResolveActor(ctx context.Context, ws, user string) (*authorization.Actor, error) {
	actor, err := a.playbookLiveTestAuthority.ResolveActor(ctx, ws, user)
	if err == nil && !a.teamLost {
		actor.TeamMemberships = []authorization.TeamRole{{TeamID: a.teamID, Role: "member"}}
	}
	return actor, err
}

func (a *playbookPMTestAuthority) CanAccessModule(_ context.Context, _ *authorization.Actor, module model.ModuleID) (bool, error) {
	return !a.revoked && !(module == model.ModulePM && a.pmDisabled), nil
}

type playbookTaskTestEnv struct {
	db        *gorm.DB
	actions   *CRMPlaybookActionService
	store     *repository.CRMPlaybookExecutionRepository
	binding   model.AutomationRunBinding
	authority *playbookPMTestAuthority
	ctx       context.Context
	proposal  model.CRMPlaybookAction
}

func playbookTaskFixture(t *testing.T) playbookTaskTestEnv {
	t.Helper()
	// Reuse the existing PM service's SQLite schema in the CRM fixture rather
	// than substituting a fake task service. No application database is opened.
	t.Setenv("CRM_SITUATION_TEST_POSTGRES_DSN", "")
	definition := playbookDefinition(0)
	definition.Policy.PMTasks = "approval_required"
	db, actions, binding, _, ctx := playbookActionDefinitionFixture(t, 0, definition)
	pmSchema := newTestDB(t)
	pmSQL, err := pmSchema.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pmSQL.Close(); err != nil {
			t.Error(err)
		}
	})
	var statements []string
	if err := pmSchema.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND (name LIKE 'pm_%' OR name IN ('users','workspace_teams')) ORDER BY name").Scan(&statements).Error; err != nil {
		t.Fatal(err)
	}
	for _, statement := range statements {
		f.Exec(t, db, statement)
	}
	f.Exec(t, db, "ALTER TABLE workspace_members ADD COLUMN email text")
	f.Exec(t, db, "ALTER TABLE workspace_members ADD COLUMN invited_by text")
	f.Exec(t, db, "ALTER TABLE workspace_members ADD COLUMN invited_at datetime")
	f.Exec(t, db, "ALTER TABLE workspace_members ADD COLUMN accepted_at datetime")
	f.Exec(t, db, "CREATE TABLE crm_associations (id text PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), workspace_id text, from_object_type text, from_object_id text, to_object_type text, to_object_id text, association_label text, created_at datetime)")
	seedUser(t, db, f.SalesUser, "sales@example.test", "Sales owner", "test-only")
	teamID, workflowID, stateID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	f.Exec(t, db, "INSERT INTO workspace_teams(id,workspace_id,name) VALUES(?,?,?)", teamID, f.Workspace, "Customer success")
	f.Exec(t, db, "INSERT INTO pm_workflows(id,workspace_id,name,default_state_id) VALUES(?,?,?,?)", workflowID, f.Workspace, "Customer commitments", stateID)
	f.Exec(t, db, "INSERT INTO pm_workflow_states(id,workflow_id,name,state_type,is_default) VALUES(?,?,?,?,?)", stateID, workflowID, "To do", "unstarted", true)
	workspaces := repository.NewWorkspaceRepository(db)
	tasks := NewPMTaskService(repository.NewPMTaskRepository(db), workspaces, repository.NewPMWorkflowRepository(db),
		repository.NewPMEpicRepository(db), repository.NewPMSprintRepository(db), repository.NewPMLabelRepository(db),
		repository.NewPMChecklistItemRepository(db), repository.NewPMExternalLinkRepository(db), repository.NewPMAttachmentRepository(db),
		NewPMActivityService(repository.NewPMActivityRepository(db)), nil, nil, nil, nil)
	store := repository.NewCRMPlaybookExecutionRepository(db)
	authority := &playbookPMTestAuthority{teamID: teamID}
	actions.execution.authz, actions.execution.members = authority, authority
	actions.executor = NewCRMPlaybookModuleExecutor(actions.execution, store, nil, tasks, nil, workspaces, repository.NewAgentRepository(db), repository.NewCRMDealRepository(db))
	actor, err := authority.ResolveActor(ctx, f.Workspace, f.SalesUser)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	proposal := model.CRMPlaybookAction{Version: 1, Kind: "task", Title: "Prepare the customer evaluation", Reason: "The customer needs an agreed evaluation plan before the next call.",
		Task: &model.CRMPlaybookTaskAction{TeamID: teamID, OwnerMemberID: f.Sales, Name: "Prepare the evaluation plan", Description: "Confirm the customer's goals and document the agreed evaluation dates.", Deadline: &deadline}}
	return playbookTaskTestEnv{db: db, actions: actions, store: store, binding: binding, authority: authority, ctx: authorization.WithActor(ctx, actor), proposal: proposal}
}

func TestCRMPlaybookTaskApprovalCreatesOneNativeTask(t *testing.T) {
	env := playbookTaskFixture(t)
	proposal := proposeActionForTest(t, env.actions, env.ctx, env.binding, env.proposal)
	assertPlaybookTaskCount(t, env.db, 0)
	// Human edits, not the original model draft, are the executed payload.
	env.proposal.Task.Name = "Confirm the revised evaluation plan"
	env.proposal.Task.Description = "Review the customer-agreed dates with the account owner."
	deadline := env.proposal.Task.Deadline.Add(24 * time.Hour)
	env.proposal.Task.Deadline = &deadline
	result, err := env.actions.AcceptSuggestionRevision(env.ctx, f.Workspace, proposal.ID, proposal.Revision, model.JSONB{"playbook_action": env.proposal})
	if err != nil || result.ExecutionStatus != "succeeded" {
		t.Fatalf("native PM task creation: %#v %v", result, err)
	}
	intent, err := env.store.ActionIntent(env.ctx, f.Workspace, proposal.ID)
	if err != nil || intent.ResultType != "task" || intent.ResultID == nil {
		t.Fatalf("missing task correlation: %#v %v", intent, err)
	}
	task, err := repository.NewPMTaskRepository(env.db).GetByID(env.ctx, *intent.ResultID)
	if err != nil || task == nil {
		t.Fatalf("task not available in PM: %#v %v", task, err)
	}
	if task.Task.Name != env.proposal.Task.Name || task.Task.Description == nil || *task.Task.Description != env.proposal.Task.Description || task.Task.Deadline == nil || !task.Task.Deadline.Equal(*env.proposal.Task.Deadline) || task.Task.Completed || task.Task.AssignedAgentID != nil {
		t.Fatalf("PM did not preserve reviewed work: %#v", task.Task)
	}
	owners, err := repository.NewPMTaskRepository(env.db).ListOwnerUserIDs(env.ctx, task.Task.ID)
	if err != nil || len(owners) != 1 || owners[0] != f.SalesUser {
		t.Fatalf("wrong PM responsibility: %#v %v", owners, err)
	}
	for i := 0; i < 2; i++ {
		if _, err := env.actions.AcceptSuggestionRevision(env.ctx, f.Workspace, proposal.ID, proposal.Revision, nil); err != nil {
			t.Fatal(err)
		}
	}
	assertPlaybookTaskCount(t, env.db, 1)
	links, err := repository.NewCRMAssociationRepository(env.db).ListByObject(env.ctx, f.Workspace, "task", task.Task.ID)
	if err != nil || len(links) != 1 || links[0].FromObjectID != f.Company {
		t.Fatalf("missing native CRM link: %#v %v", links, err)
	}
	item, err := repository.NewCRMSituationRepository(env.db).GetByID(env.ctx, f.Workspace, env.binding.SituationID)
	if err != nil || item.Situation.Lifecycle != "open" {
		t.Fatalf("task creation closed customer work: %#v %v", item, err)
	}
	for _, milestone := range item.Situation.PlaybookMilestones {
		if milestone.Status != "pending" {
			t.Fatal("task creation was counted as a customer milestone")
		}
	}
}

func TestCRMPlaybookTaskApprovalRechecksPMAccess(t *testing.T) {
	for _, restriction := range []string{"module", "team", "inactive owner"} {
		t.Run(restriction, func(t *testing.T) {
			env := playbookTaskFixture(t)
			proposal := proposeActionForTest(t, env.actions, env.ctx, env.binding, env.proposal)
			switch restriction {
			case "module":
				env.authority.pmDisabled = true
			case "team":
				env.authority.teamLost = true
			case "inactive owner":
				f.Exec(t, env.db, "UPDATE workspace_members SET status='inactive' WHERE id=?", f.Sales)
			}
			if _, err := env.actions.AcceptSuggestionRevision(env.ctx, f.Workspace, proposal.ID, proposal.Revision, nil); err == nil {
				t.Fatal("lost authority allowed PM task creation")
			}
			assertPlaybookTaskCount(t, env.db, 0)
			current, err := repository.NewCRMSuggestionRepository(env.db).GetByID(env.ctx, f.Workspace, proposal.ID)
			if err != nil || current.Status != "pending" {
				t.Fatalf("failed authorization consumed approval: %#v %v", current, err)
			}
		})
	}
}

func TestCRMPlaybookTaskRecoveryRepairsLinksWithoutCreatingAgain(t *testing.T) {
	env := playbookTaskFixture(t)
	// PM's existing creation service trims task names. Recovery must use the
	// same normalization, including after a lost post-creation response.
	env.proposal.Task.Name = "  Prepare the evaluation plan  "
	proposal := createPlaybookTaskWithLostLink(t, env)
	for i := 0; i < 2; i++ {
		result, err := env.actions.Reconcile(env.ctx, f.Workspace, proposal.ID)
		if err != nil || result.ExecutionStatus != "succeeded" {
			t.Fatalf("recovery did not confirm existing task: %#v %v", result, err)
		}
	}
	assertPlaybookTaskCount(t, env.db, 1)
	task, err := env.store.ActionTask(env.ctx, f.Workspace, "crm-action:"+proposal.ID)
	if err != nil || task == nil || task.Name != strings.TrimSpace(env.proposal.Task.Name) {
		t.Fatalf("recovered the wrong task: %#v %v", task, err)
	}
	links, err := repository.NewCRMAssociationRepository(env.db).ListByObject(env.ctx, f.Workspace, "task", task.ID)
	if err != nil || len(links) != 1 {
		t.Fatalf("recovery did not repair the CRM link exactly once: %#v %v", links, err)
	}
}

func TestCRMPlaybookTaskRecoveryDoesNotGuess(t *testing.T) {
	for _, condition := range []string{"ambiguous identity", "changed name", "changed team", "PM access revoked"} {
		t.Run(condition, func(t *testing.T) {
			env := playbookTaskFixture(t)
			proposal := createPlaybookTaskWithLostLink(t, env)
			task, err := env.store.ActionTask(env.ctx, f.Workspace, "crm-action:"+proposal.ID)
			if err != nil || task == nil {
				t.Fatalf("missing created task: %#v %v", task, err)
			}
			wantCount := int64(1)
			switch condition {
			case "ambiguous identity":
				duplicate := *task
				duplicate.ID, duplicate.DisplayID = uuid.NewString(), task.DisplayID+1
				if err := env.db.Create(&duplicate).Error; err != nil {
					t.Fatal(err)
				}
				wantCount = 2
			case "changed name":
				f.Exec(t, env.db, "UPDATE pm_tasks SET name='Different customer commitment' WHERE id=?", task.ID)
			case "changed team":
				f.Exec(t, env.db, "UPDATE pm_tasks SET team_id=? WHERE id=?", uuid.NewString(), task.ID)
			case "PM access revoked":
				env.authority.pmDisabled = true
			}
			result, err := env.actions.Reconcile(env.ctx, f.Workspace, proposal.ID)
			if err != nil || result.ExecutionStatus != "in_progress" {
				t.Fatalf("guessed a successful result: %#v %v", result, err)
			}
			assertPlaybookTaskCount(t, env.db, wantCount)
			links, err := repository.NewCRMAssociationRepository(env.db).ListByObject(env.ctx, f.Workspace, "task", task.ID)
			if err != nil || len(links) != 0 {
				t.Fatalf("uncertain task acquired customer links: %#v %v", links, err)
			}
		})
	}
}

func createPlaybookTaskWithLostLink(t *testing.T, env playbookTaskTestEnv) *model.CRMSuggestion {
	t.Helper()
	proposal := proposeActionForTest(t, env.actions, env.ctx, env.binding, env.proposal)
	f.Exec(t, env.db, "CREATE TRIGGER reject_crm_task_link BEFORE INSERT ON crm_associations BEGIN SELECT RAISE(ABORT, 'simulated link failure'); END")
	result, err := env.actions.AcceptSuggestionRevision(env.ctx, f.Workspace, proposal.ID, proposal.Revision, nil)
	if err != nil || result.ExecutionStatus != "in_progress" {
		t.Fatalf("lost link result was not left unresolved: %#v %v", result, err)
	}
	assertPlaybookTaskCount(t, env.db, 1)
	f.Exec(t, env.db, "DROP TRIGGER reject_crm_task_link")
	return proposal
}

func assertPlaybookTaskCount(t *testing.T, db *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&model.PMTask{}).Where("workspace_id=?", f.Workspace).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("created %d tasks, want %d", count, want)
	}
}
