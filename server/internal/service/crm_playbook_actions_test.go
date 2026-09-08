package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

type playbookActionTestExecutor struct {
	calls  int
	result crmPlaybookActionResult
}

func (e *playbookActionTestExecutor) Validate(context.Context, model.CRMPlaybookExecutionSource, model.CRMPlaybookActionFacts, model.CRMPlaybookAction, *authorization.Actor) error {
	return nil
}
func (e *playbookActionTestExecutor) Execute(context.Context, model.CRMPlaybookActionIntent, model.CRMPlaybookAction, *authorization.Actor) crmPlaybookActionResult {
	e.calls++
	return e.result
}

func playbookActionFixture(t *testing.T, index int) (*gorm.DB, *CRMPlaybookActionService, model.AutomationRunBinding, *playbookActionTestExecutor, context.Context) {
	t.Helper()
	return playbookActionDefinitionFixture(t, index, playbookDefinition(index))
}

func playbookActionDefinitionFixture(t *testing.T, index int, definition model.CRMPlaybookDefinition) (*gorm.DB, *CRMPlaybookActionService, model.AutomationRunBinding, *playbookActionTestExecutor, context.Context) {
	t.Helper()
	db, host, binding, authority := boundHostDefinitionFixture(t, index, definition)
	f.PlaybookActionStorage(t, db)
	execution := host.playbookExecution
	crm := NewCRMSituationService(repository.NewCRMSituationRepository(db), authority)
	execution.playbooks = NewCRMPlaybookService(repository.NewCRMPlaybookRepository(db), authority, crm)
	executor := &playbookActionTestExecutor{result: crmPlaybookActionResult{status: "succeeded", resultType: "test_result", resultID: f.Ptr(f.Company)}}
	actions := NewCRMPlaybookActionService(repository.NewCRMPlaybookExecutionRepository(db), repository.NewCRMSuggestionRepository(db), execution, executor)
	return db, actions, binding, executor, authorization.WithActor(context.Background(), f.Actor("admin"))
}

func proposalForJourney(index int) model.CRMPlaybookAction {
	keys := []string{"intent_confirmed", "scope_confirmed", "risk_resolved"}
	return model.CRMPlaybookAction{Version: 1, Kind: "milestone", Title: "Confirm customer progress", Reason: "Review the linked customer evidence", Milestone: &model.CRMPlaybookMilestoneAction{Key: keys[index], Evidence: "The customer confirmed the objective in the linked evidence."}}
}

func TestCRMPlaybookUnknownResultCanBeInspectedWithoutReexecution(t *testing.T) {
	for _, outcome := range []string{"completed", "not_completed"} {
		t.Run(outcome, func(t *testing.T) {
			db, svc, binding, executor, ctx := playbookActionFixture(t, 0)
			executor.result = playbookActionUnknown("The provider result is unknown.")
			proposal := proposeActionForTest(t, svc, ctx, binding, proposalForJourney(0))
			accepted, err := svc.AcceptSuggestionRevision(ctx, f.Workspace, proposal.ID, proposal.Revision, nil)
			if err != nil {
				t.Fatal(err)
			}
			req := model.CRMPlaybookActionInspection{Revision: accepted.Revision, Outcome: outcome, Evidence: "I inspected the actual customer record and confirmed this result.", Confirmed: true}
			if _, err := svc.Inspect(ctx, f.Workspace, proposal.ID, req); err == nil {
				t.Fatal("inspection raced a running action")
			}
			svc.now = func() time.Time { return time.Now().Add(10 * time.Minute) }
			result, err := svc.Inspect(ctx, f.Workspace, proposal.ID, req)
			if err != nil {
				t.Fatal(err)
			}
			want := "succeeded"
			if outcome == "not_completed" {
				want = "failed"
			}
			if result.ExecutionStatus != want || executor.calls != 1 {
				t.Fatalf("inspection reran operation: %#v calls=%d", result, executor.calls)
			}
			if result.Context["playbook_action_inspection"] == nil {
				t.Fatal("missing human attribution")
			}
			intent, _ := repository.NewCRMPlaybookExecutionRepository(db).ActionIntent(ctx, f.Workspace, proposal.ID)
			if intent.ResultType != "human_verified" {
				t.Fatal("human finding presented as provider verification")
			}
			if _, err := svc.Inspect(ctx, f.Workspace, proposal.ID, req); err == nil {
				t.Fatal("stale inspection overwrote result")
			}
		})
	}
}

func proposeActionForTest(t *testing.T, svc *CRMPlaybookActionService, ctx context.Context, b model.AutomationRunBinding, a model.CRMPlaybookAction) *model.CRMSuggestion {
	t.Helper()
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.Propose(ctx, f.Workspace, b.RunID, raw)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCRMPlaybookActionsUseOneCanonicalProposalAndExactApproval(t *testing.T) {
	for index, name := range []string{"buying_intent", "sales_handoff", "renewal_recovery"} {
		t.Run(name, func(t *testing.T) {
			db, svc, binding, executor, ctx := playbookActionFixture(t, index)
			_, source, err := svc.execution.AuthorizeRun(ctx, f.Workspace, binding.RunID)
			if err != nil {
				t.Fatal(err)
			}
			action := proposalForJourney(index)
			action.Milestone.Key = source.Policy.Definition.Milestones[0].Key
			first := proposeActionForTest(t, svc, ctx, binding, action)
			action.Title = "A slightly different title is not another action"
			second := proposeActionForTest(t, svc, ctx, binding, action)
			if first.ID != second.ID || first.Revision != second.Revision || executor.calls != 0 {
				t.Fatal("interpretation created duplicates or executed before approval")
			}
			item, err := repository.NewCRMSituationRepository(db).GetByID(ctx, f.Workspace, binding.SituationID)
			if err != nil || len(item.Actions) != 1 {
				t.Fatalf("not linked to canonical Signal: %v", err)
			}
			accepted, err := svc.AcceptSuggestionRevision(ctx, f.Workspace, first.ID, first.Revision, nil)
			if err != nil || accepted.ExecutionStatus != "succeeded" || executor.calls != 1 {
				t.Fatalf("approval: %v %#v", err, accepted)
			}
			f.Exec(t, db, "UPDATE crm_situations SET lifecycle='paused',revision=revision+1 WHERE id=?", binding.SituationID)
			replay, err := svc.AcceptSuggestionRevision(ctx, f.Workspace, first.ID, first.Revision, nil)
			if err != nil || replay.ExecutionStatus != "succeeded" || executor.calls != 1 {
				t.Fatalf("approval replay executed again: %v", err)
			}
		})
	}
}

func TestCRMPlaybookActionsRejectStaleOrUnauthorizedApprovals(t *testing.T) {
	for _, condition := range []string{"revision", "expiry", "pause", "other approver", "dismissed", "changed facts", "no revision"} {
		t.Run(condition, func(t *testing.T) {
			db, svc, binding, executor, ctx := playbookActionFixture(t, 0)
			first := proposeActionForTest(t, svc, ctx, binding, proposalForJourney(0))
			revision := first.Revision
			switch condition {
			case "revision":
				f.Exec(t, db, "UPDATE crm_situations SET revision=revision+1 WHERE id=?", binding.SituationID)
			case "expiry":
				svc.now = func() time.Time { return time.Now().Add(48 * time.Hour) }
			case "pause":
				f.Exec(t, db, "UPDATE crm_situations SET lifecycle='paused' WHERE id=?", binding.SituationID)
			case "other approver":
				actor := f.Actor("admin")
				actor.WorkspaceMemberID = f.Success
				ctx = authorization.WithActor(ctx, actor)
			case "dismissed":
				f.Exec(t, db, "UPDATE crm_suggestions SET status='dismissed' WHERE id=?", first.ID)
			case "changed facts":
				f.Exec(t, db, "UPDATE crm_companies SET name='New material facts' WHERE id=?", f.Company)
			case "no revision":
				revision = ""
			}
			if _, err := svc.AcceptSuggestionRevision(ctx, f.Workspace, first.ID, revision, nil); err == nil {
				t.Fatal("invalid approval accepted")
			}
			if executor.calls != 0 {
				t.Fatal("invalid approval reached executor")
			}
		})
	}
}

func TestCRMPlaybookActionsPreserveRejectionAndUnknownResults(t *testing.T) {
	t.Run("rejection", func(t *testing.T) {
		db, svc, binding, _, ctx := playbookActionFixture(t, 0)
		action := proposalForJourney(0)
		first := proposeActionForTest(t, svc, ctx, binding, action)
		f.Exec(t, db, "UPDATE crm_suggestions SET status='dismissed' WHERE id=?", first.ID)
		later := proposeActionForTest(t, svc, ctx, binding, action)
		if later.ID != first.ID || later.Status != "dismissed" {
			t.Fatal("rejection was replaced by a new proposal")
		}
	})
	t.Run("unknown result", func(t *testing.T) {
		_, svc, binding, executor, ctx := playbookActionFixture(t, 0)
		executor.result = crmPlaybookActionResult{status: "in_progress", safeError: f.Ptr("Result needs checking")}
		first := proposeActionForTest(t, svc, ctx, binding, proposalForJourney(0))
		for i := 0; i < 2; i++ {
			result, err := svc.AcceptSuggestionRevision(ctx, f.Workspace, first.ID, first.Revision, nil)
			if err != nil || result.ExecutionStatus != "in_progress" || result.ExecutedAt != nil {
				t.Fatalf("unknown result became success/failure: %v", err)
			}
		}
		if executor.calls != 1 {
			t.Fatal("unknown action was retried")
		}
	})
}

func TestCRMPlaybookActionsRejectUnboundCommandsAndUnsupportedPayload(t *testing.T) {
	_, svc, binding, executor, ctx := playbookActionFixture(t, 0)
	for _, raw := range []string{`{"version":1,"kind":"arbitrary","title":"Title","reason":"Reason"}`, `{"version":1,"kind":"milestone","title":"Title","reason":"Reason","milestone":{"key":"intent_confirmed","evidence":"Reason"},"approved":true}`, `{} {}`} {
		if _, err := svc.Propose(ctx, f.Workspace, binding.RunID, json.RawMessage(raw)); err == nil {
			t.Fatal("unsupported payload accepted")
		}
	}
	if _, err := svc.Context(ctx, f.ForeignWorkspace, binding.RunID); err == nil {
		t.Fatal("foreign context exposed")
	}
	if _, err := svc.Context(ctx, f.Workspace, uuid.NewString()); err == nil {
		t.Fatal("unbound context exposed")
	}
	if executor.calls != 0 {
		t.Fatal("preparation executed work")
	}
}
