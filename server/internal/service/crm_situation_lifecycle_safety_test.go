package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMSituationLifecycleTenantAndOwnerBoundaries(t *testing.T) {
	db, svc, ctx := situationServiceFixture(t)
	stored, _, err := svc.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{f.ForeignMember, f.Inactive, uuid.NewString()} {
		req := workCommand("update", 1)
		req.Changes = &model.CRMSituationChanges{Owner: &model.CRMSituationOwnerChange{MemberID: &owner}}
		if _, err := svc.Command(ctx, f.Workspace, stored.ID, req); !errors.Is(err, repository.ErrCRMSituationInvalidReference) {
			t.Fatalf("unavailable owner %s: %v", owner, err)
		}
	}
	pause := workCommand("pause", 1)
	pause.Reason = "Hold"
	viewer := authorization.WithActor(context.Background(), f.Actor("viewer"))
	if _, err := svc.Command(viewer, f.Workspace, stored.ID, pause); !errors.Is(err, ErrCRMSituationForbidden) {
		t.Fatalf("viewer changed work: %v", err)
	}
	if _, err := svc.History(viewer, f.Workspace, stored.ID, 0, 10); err != nil {
		t.Fatalf("viewer cannot read history: %v", err)
	}
	foreign := f.Situation("conversion")
	foreign.WorkspaceID, foreign.CompanyID, foreign.CreatedByMemberID = f.ForeignWorkspace, f.Ptr(f.ForeignCompany), f.Ptr(f.ForeignMember)
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{foreign.ID, uuid.NewString()} {
		if _, err := svc.Command(ctx, f.Workspace, id, pause); !errors.Is(err, ErrCRMSituationNotFound) {
			t.Fatalf("foreign/missing work changed: %v", err)
		}
		if _, err := svc.History(ctx, f.Workspace, id, 0, 10); !errors.Is(err, ErrCRMSituationNotFound) {
			t.Fatalf("foreign/missing history leaked: %v", err)
		}
	}
	if _, err := svc.Command(ctx, f.ForeignWorkspace, foreign.ID, pause); !errors.Is(err, ErrCRMSituationForbidden) {
		t.Fatalf("foreign workspace accepted: %v", err)
	}
	f.Exec(t, db, "UPDATE workspace_members SET status = 'inactive' WHERE id = ?", f.Sales)
	if _, err := svc.Command(ctx, f.Workspace, stored.ID, pause); !errors.Is(err, repository.ErrCRMSituationInvalidReference) {
		t.Fatalf("stale actor membership accepted: %v", err)
	}
}

func TestCRMSituationLifecycleExplicitUnassignedAndClear(t *testing.T) {
	_, svc, ctx := situationServiceFixture(t)
	stored, _, err := svc.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	req := workCommand("update", 1)
	req.Changes = &model.CRMSituationChanges{Owner: &model.CRMSituationOwnerChange{}, NextActionOwner: &model.CRMSituationOwnerChange{}, NextStep: f.Ptr("")}
	result := mustWorkCommand(t, svc, ctx, stored.ID, req)
	if result.Change.After.OwnerMemberID != nil || result.Change.After.NextActionOwnerMemberID != nil {
		t.Fatal("unassigned intent silently routed")
	}
	list, err := svc.List(ctx, f.Workspace, model.CRMSituationListFilters{Scope: "unassigned"})
	if err != nil || list.Total != 1 {
		t.Fatalf("unassigned work lost: %#v %v", list, err)
	}
	req = workCommand("update", 2)
	req.Changes = &model.CRMSituationChanges{NextStep: f.Ptr("Manually qualify the account")}
	result = mustWorkCommand(t, svc, ctx, stored.ID, req)
	if result.Change.After.OwnerMemberID != nil || result.Change.After.NextActionOwnerMemberID != nil {
		t.Fatal("omitted assignments were overwritten")
	}
}

func TestCRMSituationLifecycleClosurePreservesIndependentWork(t *testing.T) {
	db, svc, ctx := situationServiceFixture(t)
	var ids []string
	for i := 0; i < 2; i++ {
		req := situationCreateRequest()
		req.References = []model.CRMSituationReference{{Kind: "signal", SourceID: f.Signal}, {Kind: "suggestion", SourceID: f.Suggestion}}
		stored, _, err := svc.Create(ctx, f.Workspace, req)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, stored.ID)
	}
	closing := workCommand("close", 1)
	closing.Outcome = &model.CRMSituationOutcome{Kind: "not_pursued", Summary: "The customer declined this objective"}
	closed := mustWorkCommand(t, svc, ctx, ids[0], closing)
	replayed := mustWorkCommand(t, svc, ctx, ids[0], closing)
	if !replayed.Replayed || replayed.Change.ID != closed.Change.ID {
		t.Fatal("closed request retry changed the receipt")
	}
	other, err := svc.GetByID(ctx, f.Workspace, ids[1])
	if err != nil || other.Situation.Lifecycle != "open" || other.PendingActionCount != 1 {
		t.Fatalf("closing changed independent customer work: %#v %v", other, err)
	}
	var signal model.CRMSignal
	if err := db.Where("id = ?", f.Signal).Take(&signal).Error; err != nil {
		t.Fatal(err)
	}
	if signal.ReviewedAt != nil || signal.DismissedAt != nil || signal.ActedAt != nil {
		t.Fatal("closure falsified evidence feedback")
	}
	var action model.CRMSuggestion
	if err := db.Where("id = ?", f.Suggestion).Take(&action).Error; err != nil {
		t.Fatal(err)
	}
	if action.Status != "pending" || action.ExecutionStatus != "pending" || action.ExecutedAt != nil {
		t.Fatal("closure rewrote canonical action state")
	}
	for _, operation := range []string{"pause", "resume", "update", "close"} {
		req := workCommand(operation, 2)
		req.Reason = "Attempt to change a closed outcome"
		if operation == "update" {
			req.Changes = &model.CRMSituationChanges{NextStep: f.Ptr("Another step")}
		} else if operation == "close" {
			req.Outcome = &model.CRMSituationOutcome{Kind: "achieved", Summary: "Changed assessment"}
		}
		if _, err := svc.Command(ctx, f.Workspace, ids[0], req); !errors.Is(err, ErrCRMSituationTransition) {
			t.Fatalf("closed work allowed %s: %v", operation, err)
		}
	}
}

func TestCRMSituationLifecycleDuplicateMustIdentifyOriginal(t *testing.T) {
	_, svc, ctx := situationServiceFixture(t)
	first, _, err := svc.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := svc.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{first.ID, uuid.NewString()} {
		closing := workCommand("close", 1)
		closing.Outcome = &model.CRMSituationOutcome{Kind: "duplicate", Summary: "Same work", DuplicateOfSituationID: &id}
		if _, err := svc.Command(ctx, f.Workspace, first.ID, closing); !errors.Is(err, repository.ErrCRMSituationInvalidReference) {
			t.Fatalf("invalid duplicate destination: %v", err)
		}
	}
	closing := workCommand("close", 1)
	closing.Outcome = &model.CRMSituationOutcome{Kind: "duplicate", Summary: "This repeats the original customer objective", DuplicateOfSituationID: &second.ID}
	mustWorkCommand(t, svc, ctx, first.ID, closing)
	closing.CommandKey = uuid.NewString()
	closing.Outcome.DuplicateOfSituationID = &first.ID
	if _, err := svc.Command(ctx, f.Workspace, second.ID, closing); !errors.Is(err, repository.ErrCRMSituationInvalidReference) {
		t.Fatalf("duplicate cycle allowed: %v", err)
	}
}

func TestCRMSituationLifecycleHistoryPagination(t *testing.T) {
	_, svc, ctx := situationServiceFixture(t)
	stored, _, err := svc.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	for revision := int64(1); revision <= 5; revision++ {
		req := workCommand("update", revision)
		req.Changes = &model.CRMSituationChanges{NextStep: f.Ptr(uuid.NewString())}
		mustWorkCommand(t, svc, ctx, stored.ID, req)
	}
	before := int64(0)
	var revisions []int64
	for {
		page, err := svc.History(ctx, f.Workspace, stored.ID, before, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, change := range page.Data {
			revisions = append(revisions, change.Revision)
		}
		if page.NextBeforeRevision == nil {
			break
		}
		before = *page.NextBeforeRevision
	}
	if len(revisions) != 6 {
		t.Fatalf("history entries = %v", revisions)
	}
	for i, revision := range revisions {
		if revision != int64(6-i) {
			t.Fatalf("history skipped or repeated entries: %v", revisions)
		}
	}
}

func TestCRMSituationLifecyclePauseReportsInFlightWork(t *testing.T) {
	db, sources, svc, decisions, ctx := situationSourceFixture(t)
	row := sourceJourneySuggestion("conversion")
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := sources.ImportSuggestion(ctx, row); err != nil {
		t.Fatal(err)
	}
	list, err := svc.List(ctx, f.Workspace, model.CRMSituationListFilters{})
	if err != nil || len(list.Data) != 1 {
		t.Fatalf("fixture: %#v %v", list, err)
	}
	id := list.Data[0].Situation.ID
	f.Exec(t, db, "UPDATE crm_suggestions SET status = 'accepted', execution_status = 'in_progress' WHERE id = ?", row.ID)
	closing := workCommand("close", 1)
	closing.Outcome = &model.CRMSituationOutcome{Kind: "achieved", Summary: "Customer confirmed the objective"}
	if _, err := svc.Command(ctx, f.Workspace, id, closing); !errors.Is(err, repository.ErrCRMSituationExecutionPending) {
		t.Fatalf("unconfirmed execution hidden by closure: %v", err)
	}
	pause := workCommand("pause", 1)
	pause.Reason = "Stop further actions while we review"
	result := mustWorkCommand(t, svc, ctx, id, pause)
	if result.Change.InFlightActionCount != 1 {
		t.Fatal("pause concealed an already-admitted operation")
	}
	var canonical model.CRMSuggestion
	if err := db.Where("id = ?", row.ID).Take(&canonical).Error; err != nil || canonical.ExecutionStatus != "in_progress" {
		t.Fatalf("pause pretended to cancel executing work: %#v %v", canonical, err)
	}
	// A second pending action cannot start through either the new or legacy API.
	pending := sourceJourneySuggestion("conversion")
	if err := db.Create(&pending).Error; err != nil {
		t.Fatal(err)
	}
	f.Exec(t, db, "INSERT INTO crm_situation_references (workspace_id,situation_id,kind,source_id) VALUES (?,?,'suggestion',?)", f.Workspace, id, pending.ID)
	if _, err := decisions.AcceptSuggestion(ctx, f.Workspace, pending.ID, nil); !errors.Is(err, ErrCRMSuggestionStale) {
		t.Fatalf("legacy approval bypassed lifecycle: %v", err)
	}
	accepted := model.CRMSuggestionStatusAccepted
	if _, err := decisions.Update(ctx, f.Workspace, pending.ID, model.UpdateCRMSuggestionRequest{Status: &accepted}); !errors.Is(err, ErrCRMSuggestionStale) {
		t.Fatalf("legacy status-only approval bypassed lifecycle: %v", err)
	}
	f.Exec(t, db, "UPDATE crm_suggestions SET execution_status = 'succeeded' WHERE id = ?", row.ID)
	closing.ExpectedRevision = 2
	mustWorkCommand(t, svc, ctx, id, closing)
}

func TestCRMSituationLifecycleMissingExecutionStatusStaysVisible(t *testing.T) {
	db, sources, svc, _, ctx := situationSourceFixture(t)
	row := sourceJourneySuggestion("conversion")
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	f.Exec(t, db, "UPDATE crm_suggestions SET status = 'accepted', execution_status = NULL WHERE id = ?", row.ID)
	if err := sources.ReconcileWorkspace(ctx, f.Workspace); err != nil {
		t.Fatal(err)
	}
	list, err := svc.List(ctx, f.Workspace, model.CRMSituationListFilters{})
	if err != nil || list.Total != 1 || list.Data[0].UncertainActionCount != 1 || list.Data[0].EffectiveAttention != "automation_failed" {
		t.Fatalf("missing execution status silently disappeared: %#v %v", list, err)
	}
	closing := workCommand("close", 1)
	closing.Outcome = &model.CRMSituationOutcome{Kind: "achieved", Summary: "Customer confirmed"}
	if _, err := svc.Command(ctx, f.Workspace, list.Data[0].Situation.ID, closing); !errors.Is(err, repository.ErrCRMSituationExecutionPending) {
		t.Fatalf("missing execution status did not block closure: %v", err)
	}
}
