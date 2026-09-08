package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMSituationActionsIndependentApprovers(t *testing.T) {
	db, sources, work, _, ctx := situationSourceFixture(t)
	stored, _, err := work.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	var successUser string
	if err := db.Table("workspace_members").Select("user_id").Where("id = ?", f.Success).Scan(&successUser).Error; err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{f.SalesUser, successUser} {
		row := sourceJourneySuggestion("conversion")
		row.UserID = f.Ptr(user)
		row.Context["situation_id"] = stored.ID
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		if err := sources.ImportSuggestion(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	for _, member := range []string{f.Sales, f.Success} {
		actor := f.Actor("member")
		actor.WorkspaceMemberID = member
		list, err := work.List(authorization.WithActor(context.Background(), actor), f.Workspace, model.CRMSituationListFilters{})
		if err != nil || list.Total != 1 || list.PendingActionTotal != 2 || list.Data[0].PendingActionCount != 2 {
			t.Fatalf("approver %s lost pending work: %#v %v", member, list, err)
		}
	}
	item, err := work.GetByID(ctx, f.Workspace, stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	first := item.Actions[0]
	if _, err := work.DecideAction(ctx, f.Workspace, stored.ID, first.ID, first.Revision, "dismiss", model.CRMSignalDismissIrrelevant, nil); err != nil {
		t.Fatal(err)
	}
	item, err = work.GetByID(ctx, f.Workspace, stored.ID)
	if err != nil || item.PendingActionCount != 1 || item.Situation.Lifecycle != "open" {
		t.Fatalf("dismiss affected independent work: %#v %v", item, err)
	}
}

func TestCRMSituationActionsRevisionAndPermissions(t *testing.T) {
	db, sources, work, _, ctx := situationSourceFixture(t)
	row := sourceJourneySuggestion("conversion")
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := sources.ImportSuggestion(ctx, row); err != nil {
		t.Fatal(err)
	}
	list, err := work.List(ctx, f.Workspace, model.CRMSituationListFilters{})
	if err != nil || list.Total != 1 {
		t.Fatalf("import: %#v %v", list, err)
	}
	id := list.Data[0].Situation.ID
	item, err := work.GetByID(ctx, f.Workspace, id)
	if err != nil {
		t.Fatal(err)
	}
	revision := item.Actions[0].Revision
	viewer := authorization.WithActor(context.Background(), f.Actor("viewer"))
	if _, err := work.DecideAction(viewer, f.Workspace, id, row.ID, revision, "accept", "", nil); !errors.Is(err, ErrCRMSituationForbidden) {
		t.Fatalf("viewer approval: %v", err)
	}
	if _, err := work.DecideAction(ctx, f.Workspace, id, uuid.NewString(), revision, "accept", "", nil); !errors.Is(err, ErrCRMSituationNotFound) {
		t.Fatalf("unlinked action: %v", err)
	}
	f.Exec(t, db, "UPDATE crm_suggestions SET title = 'Changed proposal' WHERE id = ?", row.ID)
	if _, err := work.DecideAction(ctx, f.Workspace, id, row.ID, revision, "accept", "", nil); !errors.Is(err, ErrCRMSuggestionStale) {
		t.Fatalf("stale approval: %v", err)
	}
}

func TestCRMSituationActionsFailureAndUncertaintyStayVisible(t *testing.T) {
	db, sources, work, decisions, ctx := situationSourceFixture(t)
	row := sourceJourneySuggestion("conversion")
	row.SuggestionType = model.CRMSuggestionDealCreate
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := sources.ImportSuggestion(ctx, row); err != nil {
		t.Fatal(err)
	}
	result, err := decisions.AcceptSuggestion(ctx, f.Workspace, row.ID, nil)
	if err != nil || result.Status != "accepted" || result.ExecutionStatus != "failed" {
		t.Fatalf("failed executor: %#v %v", result, err)
	}
	list, err := work.List(ctx, f.Workspace, model.CRMSituationListFilters{})
	if err != nil || list.Total != 1 || list.Data[0].FailedActionCount != 1 || list.Data[0].EffectiveAttention != "automation_failed" || list.Data[0].Situation.Lifecycle != "open" {
		t.Fatalf("failure disappeared: %#v %v", list, err)
	}
	f.Exec(t, db, "UPDATE crm_suggestions SET execution_status = 'in_progress', updated_at = ? WHERE id = ?", time.Now().UTC().Add(-time.Hour), row.ID)
	list, err = work.List(ctx, f.Workspace, model.CRMSituationListFilters{})
	if err != nil || list.Total != 1 || list.Data[0].UncertainActionCount != 1 {
		t.Fatalf("ambiguous execution disappeared: %#v %v", list, err)
	}
	if _, err := decisions.AcceptSuggestion(ctx, f.Workspace, row.ID, nil); !errors.Is(err, ErrCRMSuggestionStale) {
		t.Fatalf("blindly retried ambiguous action: %v", err)
	}
	pending := "pending"
	if _, err := decisions.Update(ctx, f.Workspace, row.ID, model.UpdateCRMSuggestionRequest{Status: &pending}); !errors.Is(err, ErrCRMSuggestionStale) {
		t.Fatalf("legacy reset allowed duplicate execution: %v", err)
	}
}

func TestCRMSituationActionsExecuteSupportedDealChange(t *testing.T) {
	db, sources, work, decisions, ctx := situationSourceFixture(t)
	pipeline, before, after := uuid.NewString(), uuid.NewString(), uuid.NewString()
	f.Schema(t, db, "CREATE TABLE crm_pipelines (id uuid PRIMARY KEY, workspace_id uuid, name text)")
	f.Schema(t, db, "CREATE TABLE crm_pipeline_stages (id uuid PRIMARY KEY, pipeline_id uuid, name text, stage_type text)")
	f.Exec(t, db, "INSERT INTO crm_pipelines (id,workspace_id,name) VALUES (?,?,?)", pipeline, f.Workspace, "Sales")
	f.Exec(t, db, "INSERT INTO crm_pipeline_stages (id,pipeline_id,name,stage_type) VALUES (?,?,?,?), (?,?,?,?)", before, pipeline, "Open", "open", after, pipeline, "Qualified", "open")
	f.Exec(t, db, "UPDATE crm_deals SET pipeline_id = ?, stage_id = ? WHERE id = ?", pipeline, before, f.Deal)
	row := sourceJourneySuggestion("conversion")
	row.SuggestionType = "deal_advance"
	row.Context["deal_id"], row.Context["target_stage_id"] = f.Deal, after
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := sources.ImportSuggestion(ctx, row); err != nil {
		t.Fatal(err)
	}
	result, err := decisions.AcceptSuggestion(ctx, f.Workspace, row.ID, nil)
	if err != nil || result.ExecutionStatus != "succeeded" || result.ExecutedAt == nil {
		t.Fatalf("supported executor: %#v %v", result, err)
	}
	var stage string
	if err := db.Table("crm_deals").Select("stage_id").Where("id = ?", f.Deal).Scan(&stage).Error; err != nil {
		t.Fatal(err)
	}
	if stage != after {
		t.Fatal("success claimed without the stage change")
	}
	list, err := work.List(ctx, f.Workspace, model.CRMSituationListFilters{Scope: "all", State: "all"})
	if err != nil || list.Total != 1 || list.Data[0].Situation.Lifecycle != "open" || list.Data[0].Situation.OutcomeKind != nil {
		t.Fatalf("execution falsely achieved customer outcome: %#v %v", list, err)
	}
}

func TestCRMSituationActionsConcurrentDecision(t *testing.T) {
	db, _, _, decisions, ctx := situationSourceFixture(t)
	row := sourceJourneySuggestion("conversion")
	row.Context = nil
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	var acceptErr, dismissErr error
	workers.Add(2)
	go func() {
		defer workers.Done()
		_, acceptErr = decisions.AcceptSuggestion(ctx, f.Workspace, row.ID, map[string]interface{}{"note": "Reviewed"})
	}()
	go func() {
		defer workers.Done()
		_, dismissErr = decisions.DismissSuggestion(ctx, f.Workspace, row.ID, model.CRMSignalDismissIrrelevant)
	}()
	workers.Wait()
	if (acceptErr == nil) == (dismissErr == nil) {
		t.Fatalf("exactly one decision must win: accept=%v dismiss=%v", acceptErr, dismissErr)
	}
}

func TestCRMSituationActionsDueCheckpoint(t *testing.T) {
	db, _, work, _, ctx := situationSourceFixture(t)
	stored, _, err := work.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	f.Exec(t, db, "UPDATE crm_situations SET attention = 'waiting_customer', next_checkpoint_at = ? WHERE id = ?", time.Now().UTC().Add(-time.Hour), stored.ID)
	list, err := work.List(ctx, f.Workspace, model.CRMSituationListFilters{})
	if err != nil || list.Total != 1 || list.Data[0].EffectiveAttention != "follow_up_due" {
		t.Fatalf("overdue checkpoint remained hidden: %#v %v", list, err)
	}
}

func TestCRMSituationActionsLegacyApprovalHonorsPause(t *testing.T) {
	db, sources, work, decisions, ctx := situationSourceFixture(t)
	row := sourceJourneySuggestion("conversion")
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := sources.ImportSuggestion(ctx, row); err != nil {
		t.Fatal(err)
	}
	list, err := work.List(ctx, f.Workspace, model.CRMSituationListFilters{})
	if err != nil || list.Total != 1 {
		t.Fatalf("import: %#v %v", list, err)
	}
	f.Exec(t, db, "UPDATE crm_situations SET lifecycle = 'paused' WHERE id = ?", list.Data[0].Situation.ID)
	if _, err := decisions.AcceptSuggestion(ctx, f.Workspace, row.ID, nil); !errors.Is(err, ErrCRMSuggestionStale) {
		t.Fatalf("legacy route bypassed pause: %v", err)
	}
}

func TestCRMSituationActionsRejectPreviouslyExecutedPendingRecord(t *testing.T) {
	db, _, _, decisions, ctx := situationSourceFixture(t)
	row := sourceJourneySuggestion("conversion")
	row.ExecutionStatus = "succeeded"
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := decisions.AcceptSuggestion(ctx, f.Workspace, row.ID, nil); !errors.Is(err, ErrCRMSuggestionStale) {
		t.Fatalf("historically reset record executed again: %v", err)
	}
}
