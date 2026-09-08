package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMSignalInboxPriorityBandsAndDealClassification(t *testing.T) {
	db := crmsituationtest.Open(t)
	crmsituationtest.InboxTables(t, db)
	repo := NewCRMSituationRepository(db)
	ctx := context.Background()
	filters := inboxFilters()
	for i, score := range []float64{15, 8, 7.99} {
		work := crmsituationtest.Situation("conversion")
		work.Priority = score
		work.Title = fmt.Sprintf("Scored %d", i)
		if _, _, err := repo.Create(ctx, work, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, band := range []string{"high", "medium", "low", "unscored"} {
		filters.Navigation.Query = &model.QueryFilterGroup{Logic: model.QueryFilterLogicAnd, Rules: []model.QueryFilterRule{{Field: "priority", Operator: model.QueryFilterOpIs, Value: crmsituationtest.Ptr(band)}}}
		list, err := repo.ListInbox(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
		if err != nil || list.Total != 1 || list.Data[0].PriorityBand != band {
			t.Fatalf("priority %s: %+v %v", band, list, err)
		}
	}
	filters.Navigation.Query = nil
	filters.Navigation.State = "needs_approval"
	pipeline := uuid.NewString()
	crmsituationtest.Exec(t, db, `INSERT INTO crm_pipelines (id,workspace_id,default_commercial_motion) VALUES (?,?,'renewal')`, pipeline, crmsituationtest.Workspace)
	crmsituationtest.Exec(t, db, `UPDATE crm_deals SET pipeline_id = ? WHERE id = ?`, pipeline, crmsituationtest.Deal)
	crmsituationtest.Exec(t, db, `UPDATE crm_suggestions SET suggestion_type = 'deal_advance', object_type = 'deal', object_id = ? WHERE id = ?`, crmsituationtest.Deal, crmsituationtest.Suggestion)
	list, err := repo.ListInbox(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil || list.Total != 1 || list.Data[0].Category != "retention" {
		t.Fatalf("renewal classified as new sales: %+v %v", list, err)
	}
	crmsituationtest.Exec(t, db, `UPDATE crm_deals SET commercial_motion = 'expansion' WHERE id = ?`, crmsituationtest.Deal)
	list, err = repo.ListInbox(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil || list.Data[0].Category != "expansion" {
		t.Fatalf("deal motion lost: %+v %v", list, err)
	}
	crmsituationtest.Exec(t, db, `UPDATE crm_suggestions SET context = ? WHERE id = ?`, `{"commercial_motion":"onboarding"}`, crmsituationtest.Suggestion)
	list, err = repo.ListInbox(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil || list.Data[0].Category != "onboarding_adoption" {
		t.Fatalf("explicit motion lost: %+v %v", list, err)
	}
}

func TestCRMSignalInboxUncertainExecutionIsNotHiddenAsWaiting(t *testing.T) {
	db := crmsituationtest.Open(t)
	crmsituationtest.InboxTables(t, db)
	repo := NewCRMSituationRepository(db)
	crmsituationtest.Exec(t, db, `UPDATE crm_suggestions SET status = 'accepted', suggestion_type = 'deal_create', execution_status = 'in_progress', updated_at = ? WHERE id = ?`, time.Now().UTC().Add(-time.Hour), crmsituationtest.Suggestion)
	filters := inboxFilters()
	filters.Navigation.State = "needs_attention"
	list, err := repo.ListInbox(context.Background(), crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil || list.Total != 1 || list.Data[0].Attention != "automation_failed" {
		t.Fatalf("uncertain execution hidden: %+v %v", list, err)
	}
	filters.Navigation.State = "waiting"
	list, err = repo.ListInbox(context.Background(), crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil || list.Total != 0 {
		t.Fatalf("uncertain execution in waiting: %+v %v", list, err)
	}
}

func inboxFilters() model.CRMSignalInboxFilters {
	return model.CRMSignalInboxFilters{Navigation: model.CRMSituationListFilters{Scope: "all", State: "all", Category: "all", Page: 1, PageSize: 25}, Sort: "priority"}
}

func TestCRMSignalInboxReadOnlyUnionAndEvidence(t *testing.T) {
	db := crmsituationtest.Open(t)
	crmsituationtest.InboxTables(t, db)
	repo := NewCRMSituationRepository(db)
	ctx := context.Background()
	crmsituationtest.Exec(t, db, `UPDATE crm_suggestions SET title = 'Quote seats', suggestion_type = 'deal_create', user_id = ?, signal_ids = ?, context = ?, created_at = CURRENT_TIMESTAMP WHERE id = ?`, crmsituationtest.SalesUser, model.StringArray{crmsituationtest.Signal, crmsituationtest.ForeignSignal}, `{"company_id":"`+crmsituationtest.Company+`"}`, crmsituationtest.Suggestion)
	filters := inboxFilters()
	list, err := repo.ListInbox(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Data) != 1 || list.Data[0].Kind != "recommendation" || list.Data[0].EvidenceReview != "needs_review" || list.Data[0].Priority != nil || list.Data[0].CustomerName != "Northstar" || list.CategoryCounts["retention"] != 1 {
		t.Fatalf("standalone = %+v", list)
	}
	var workCount int64
	db.Table("crm_situations").Count(&workCount)
	if workCount != 0 {
		t.Fatal("GET projected recommendations")
	}
	input := crmsituationtest.Situation("expansion")
	input.Priority = 16
	stored, _, err := repo.Create(ctx, input, []model.CRMSituationReference{{Kind: "suggestion", SourceID: crmsituationtest.Suggestion}})
	if err != nil {
		t.Fatal(err)
	}
	list, err = repo.ListInbox(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Data) != 1 || list.Data[0].ID != stored.ID || list.Data[0].Kind != "situation" || list.Data[0].PriorityBand != "high" || list.Data[0].EvidenceReview != "needs_review" {
		t.Fatalf("linked = %+v", list)
	}
	crmsituationtest.Exec(t, db, `UPDATE crm_signals SET reviewed_at = CURRENT_TIMESTAMP WHERE id = ?`, crmsituationtest.Signal)
	list, err = repo.ListInbox(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil {
		t.Fatal(err)
	}
	if list.Data[0].EvidenceReview != "reviewed" {
		t.Fatalf("foreign unreviewed evidence contaminated result: %+v", list.Data)
	}
	crmsituationtest.Exec(t, db, `UPDATE crm_signals SET superseded_at = CURRENT_TIMESTAMP WHERE id = ?`, crmsituationtest.Signal)
	list, err = repo.ListInbox(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil {
		t.Fatal(err)
	}
	if list.Data[0].EvidenceReview != "none" {
		t.Fatalf("stale evidence counted: %+v", list.Data)
	}
	detail, err := repo.InboxRecommendation(ctx, crmsituationtest.Workspace, crmsituationtest.Suggestion)
	if err != nil || detail == nil || len(detail.LinkedSituations) != 1 || detail.LinkedSituations[0] != stored.ID {
		t.Fatalf("original recommendation link lost: %+v %v", detail, err)
	}
}

func TestCRMSignalInboxApprovalsIncludeFailedAndPausedWork(t *testing.T) {
	db := crmsituationtest.Open(t)
	crmsituationtest.InboxTables(t, db)
	repo := NewCRMSituationRepository(db)
	ctx := context.Background()
	failed := uuid.NewString()
	crmsituationtest.Exec(t, db, `INSERT INTO crm_suggestions (id,workspace_id,title,suggestion_type,status,execution_status,created_at) VALUES (?,?,?,'deal_create','accepted','failed',CURRENT_TIMESTAMP)`, failed, crmsituationtest.Workspace, "Previous attempt")
	crmsituationtest.Exec(t, db, `UPDATE crm_suggestions SET title = 'Approve revised quote', suggestion_type = 'deal_create', context = '{}', created_at = CURRENT_TIMESTAMP WHERE id = ?`, crmsituationtest.Suggestion)
	stored, _, err := repo.Create(ctx, crmsituationtest.Situation("conversion"), []model.CRMSituationReference{{Kind: "suggestion", SourceID: failed}, {Kind: "suggestion", SourceID: crmsituationtest.Suggestion}})
	if err != nil {
		t.Fatal(err)
	}
	filters := inboxFilters()
	filters.Navigation.State = "needs_approval"
	for _, lifecycle := range []string{"open", "paused", "closed"} {
		if lifecycle == "closed" {
			crmsituationtest.Exec(t, db, `UPDATE crm_situations SET lifecycle = 'closed', outcome_kind = 'not_pursued', outcome_summary = 'Customer declined', closed_at = CURRENT_TIMESTAMP WHERE id = ?`, stored.ID)
		} else {
			crmsituationtest.Exec(t, db, `UPDATE crm_situations SET lifecycle = ? WHERE id = ?`, lifecycle, stored.ID)
		}
		list, err := repo.ListInbox(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
		if err != nil {
			t.Fatal(err)
		}
		if list.Total != 1 || list.Data[0].PendingActionCount != 1 || list.Data[0].Attention != "automation_failed" {
			t.Fatalf("%s approval hidden: %+v", lifecycle, list)
		}
	}
}

func TestCRMSignalInboxGlobalFiltersPaginationAndScope(t *testing.T) {
	db := crmsituationtest.Open(t)
	crmsituationtest.InboxTables(t, db)
	repo := NewCRMSituationRepository(db)
	ctx := context.Background()
	crmsituationtest.Exec(t, db, `UPDATE crm_suggestions SET status = 'dismissed' WHERE id = ?`, crmsituationtest.Suggestion)
	for i, kind := range []string{"deal_create", "deal_advance", "follow_up", "risk_alert", "enrichment"} {
		crmsituationtest.Exec(t, db, `INSERT INTO crm_suggestions (id,workspace_id,user_id,title,context,suggestion_type,status,confidence,created_at) VALUES (?,?,?,?,?,?,'pending',?,CURRENT_TIMESTAMP)`, uuid.NewString(), crmsituationtest.Workspace, crmsituationtest.SalesUser, "Review "+kind, `{"search":"100%_literal","company_id":"not-a-uuid"}`, kind, float64(i)/10)
	}
	filters := inboxFilters()
	filters.Navigation.PageSize = 2
	filters.Navigation.Page = 3
	filters.Sort = "recommended"
	list, err := repo.ListInbox(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 5 || len(list.Data) != 1 || list.Data[0].Title != "Review deal_create" || list.CategoryCounts["all"] != 5 {
		t.Fatalf("global page = %+v", list)
	}
	filters.Navigation.Page = 1
	filters.Navigation.Scope = "unassigned"
	list, err = repo.ListInbox(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil || list.Total != 0 {
		t.Fatalf("scope: %+v %v", list, err)
	}
	filters.Navigation.Scope = "mine"
	filters.Navigation.Search = "100%_literal"
	for _, kind := range []string{"deal_create", "deal_advance", "follow_up", "risk_alert", "enrichment"} {
		filters.Navigation.Query = &model.QueryFilterGroup{Logic: model.QueryFilterLogicAnd, Rules: []model.QueryFilterRule{{Field: "has_" + kind, Operator: model.QueryFilterOpIs, Value: crmsituationtest.Ptr("yes")}, {Field: "priority", Operator: model.QueryFilterOpIs, Value: crmsituationtest.Ptr("unscored")}, {Field: "evidence_review", Operator: model.QueryFilterOpIs, Value: crmsituationtest.Ptr("none")}}}
		list, err = repo.ListInbox(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
		if err != nil || list.Total != 1 || list.Data[0].Title != "Review "+kind {
			t.Fatalf("%s filter: %+v %v", kind, list, err)
		}
	}
	filters.Navigation.Search = "100x_literal"
	list, err = repo.ListInbox(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil || list.Total != 0 {
		t.Fatalf("search wildcard escaped: %+v %v", list, err)
	}
}
