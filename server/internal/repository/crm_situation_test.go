package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMSituationRepositoryIndependentCustomerJourneys(t *testing.T) {
	db := crmsituationtest.Open(t)
	repo := NewCRMSituationRepository(db)
	ctx := context.Background()
	for _, motion := range []string{"conversion", "onboarding", "retention"} {
		t.Run(motion, func(t *testing.T) {
			input := crmsituationtest.Situation(motion)
			stored, created, err := repo.Create(ctx, input, []model.CRMSituationReference{{Kind: "signal", SourceID: crmsituationtest.Signal}})
			if err != nil || !created {
				t.Fatalf("create journey: created=%v err=%v", created, err)
			}
			item, err := repo.GetByID(ctx, crmsituationtest.Workspace, stored.ID)
			if err != nil || item == nil {
				t.Fatalf("read journey: item=%v err=%v", item, err)
			}
			if item.Situation.ID != input.ID || item.Category != model.CRMSituationCategoryForMotion(motion) || len(item.References) != 1 {
				t.Fatalf("unexpected read projection: %#v", item)
			}
		})
	}
	result, err := repo.List(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, situationAllFilters())
	if err != nil || result.Total != 3 {
		t.Fatalf("independent situations: %#v, %v", result, err)
	}
	var reviewed int64
	if err := db.Table("crm_signals").Where("id = ? AND reviewed_at IS NOT NULL", crmsituationtest.Signal).Count(&reviewed).Error; err != nil {
		t.Fatal(err)
	}
	if reviewed != 0 {
		t.Fatal("reading or creating a situation reviewed its evidence")
	}
}

func TestCRMSituationRepositoryCreationReplay(t *testing.T) {
	db := crmsituationtest.Open(t)
	repo := NewCRMSituationRepository(db)
	input := crmsituationtest.Situation("conversion")
	refs := []model.CRMSituationReference{{Kind: "suggestion", SourceID: crmsituationtest.Suggestion}}
	first, _, err := repo.Create(context.Background(), input, refs)
	if err != nil {
		t.Fatal(err)
	}
	input.ID = uuid.NewString()
	again, created, err := repo.Create(context.Background(), input, refs)
	if err != nil || created || again.ID != first.ID {
		t.Fatalf("replay: %#v created=%v err=%v", again, created, err)
	}
	input.CreationFingerprint = "different request"
	if _, _, err := repo.Create(context.Background(), input, nil); !errors.Is(err, ErrCRMSituationConflict) {
		t.Fatalf("conflict = %v", err)
	}
	var count int64
	if err := db.Model(&model.CRMSituationReference{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("reference count = %d", count)
	}
	var status string
	if err := db.Table("crm_suggestions").Select("status").Where("id = ?", crmsituationtest.Suggestion).Scan(&status).Error; err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("referenced suggestion mutated: %s", status)
	}
}

func TestCRMSituationRepositoryRejectsForeignAndInactiveReferences(t *testing.T) {
	tests := []struct {
		name   string
		change func(*model.CRMSituation)
		refs   []model.CRMSituationReference
	}{
		{name: "company", change: func(s *model.CRMSituation) { s.CompanyID = crmsituationtest.Ptr(crmsituationtest.ForeignCompany) }},
		{name: "owner", change: func(s *model.CRMSituation) { s.OwnerMemberID = crmsituationtest.Ptr(crmsituationtest.ForeignMember) }},
		{name: "inactive owner", change: func(s *model.CRMSituation) { s.OwnerMemberID = crmsituationtest.Ptr(crmsituationtest.Inactive) }},
		{name: "action owner", change: func(s *model.CRMSituation) {
			s.NextActionOwnerMemberID = crmsituationtest.Ptr(crmsituationtest.ForeignMember)
		}},
		{name: "evidence", refs: []model.CRMSituationReference{{Kind: "signal", SourceID: crmsituationtest.ForeignSignal}}},
		{name: "unsupported source", refs: []model.CRMSituationReference{{Kind: "agent_run", SourceID: crmsituationtest.Signal}}},
		{name: "missing source", refs: []model.CRMSituationReference{{Kind: "signal", SourceID: uuid.NewString()}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := crmsituationtest.Open(t)
			repo := NewCRMSituationRepository(db)
			input := crmsituationtest.Situation("retention")
			if tt.change != nil {
				tt.change(&input)
			}
			if _, _, err := repo.Create(context.Background(), input, tt.refs); !errors.Is(err, ErrCRMSituationInvalidReference) {
				t.Fatalf("create error = %v", err)
			}
			var count int64
			if err := db.Model(&model.CRMSituation{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatal("invalid creation left a partial situation")
			}
		})
	}
}

func TestCRMSituationRepositoryWorkspaceReadIsolation(t *testing.T) {
	db := crmsituationtest.Open(t)
	repo := NewCRMSituationRepository(db)
	input := crmsituationtest.Situation("retention")
	stored, _, err := repo.Create(context.Background(), input, nil)
	if err != nil {
		t.Fatal(err)
	}
	item, err := repo.GetByID(context.Background(), crmsituationtest.ForeignWorkspace, stored.ID)
	if err != nil || item != nil {
		t.Fatalf("foreign read = %#v %v", item, err)
	}
	filters := situationAllFilters()
	filters.Search = "customer"
	result, err := repo.List(context.Background(), crmsituationtest.ForeignWorkspace, crmsituationtest.ForeignMember, filters)
	if err != nil || result.Total != 0 || result.CategoryCounts["all"] != 0 {
		t.Fatalf("foreign search/count = %#v %v", result, err)
	}
}

func TestCRMSituationRepositoryCountsBeforePagination(t *testing.T) {
	db := crmsituationtest.Open(t)
	repo := NewCRMSituationRepository(db)
	ctx := context.Background()
	for i := 0; i < 57; i++ {
		motion := "conversion"
		if i < 7 {
			motion = "retention"
		}
		input := crmsituationtest.Situation(motion)
		input.Title = fmt.Sprintf("Request %02d", i)
		if _, _, err := repo.Create(ctx, input, nil); err != nil {
			t.Fatal(err)
		}
	}
	filters := situationAllFilters()
	filters.PageSize = 5
	filters.Category = "sales"
	result, err := repo.List(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 50 || len(result.Data) != 5 || result.CategoryCounts["all"] != 57 || result.CategoryCounts["retention"] != 7 {
		t.Fatalf("incorrect facets or pagination: %#v", result)
	}
	filters.Page = 11
	last, err := repo.List(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil || last.Total != 50 || len(last.Data) != 0 {
		t.Fatalf("empty page changed totals: %#v %v", last, err)
	}
	filters.Page, filters.Search = 1, "Request 00"
	searched, err := repo.List(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil || searched.Total != 0 || searched.CategoryCounts["retention"] != 1 {
		t.Fatalf("search facets: %#v %v", searched, err)
	}
}

func TestCRMSituationRepositoryPreservesLegacyMotionFilters(t *testing.T) {
	db := crmsituationtest.Open(t)
	repo := NewCRMSituationRepository(db)
	for _, motion := range []string{"renewal", "retention"} {
		if _, _, err := repo.Create(context.Background(), crmsituationtest.Situation(motion), nil); err != nil {
			t.Fatal(err)
		}
	}
	filters := situationAllFilters()
	filters.Category = "retention"
	filters.Query = &model.QueryFilterGroup{Logic: model.QueryFilterLogicAnd, Rules: []model.QueryFilterRule{{Field: "commercial_motion", Operator: model.QueryFilterOpIs, Value: crmsituationtest.Ptr("renewal")}}}
	result, err := repo.List(context.Background(), crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil || result.Total != 1 || result.CategoryCounts["retention"] != 1 || result.Data[0].Situation.CommercialMotion != "renewal" {
		t.Fatalf("motion filter broadened: %#v %v", result, err)
	}
}

func TestCRMSituationRepositoryOwnershipScopes(t *testing.T) {
	db := crmsituationtest.Open(t)
	repo := NewCRMSituationRepository(db)
	ctx := context.Background()
	owned := crmsituationtest.Situation("conversion")
	owned.NextActionOwnerMemberID = crmsituationtest.Ptr(crmsituationtest.Success)
	delegated := crmsituationtest.Situation("onboarding")
	delegated.OwnerMemberID = crmsituationtest.Ptr(crmsituationtest.Success)
	unassigned := crmsituationtest.Situation("expansion")
	unassigned.OwnerMemberID, unassigned.NextActionOwnerMemberID = nil, nil
	for _, input := range []model.CRMSituation{owned, delegated, unassigned} {
		if _, _, err := repo.Create(ctx, input, nil); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		scope, state string
		total        int64
	}{
		{"mine", "all", 2}, {"mine", "needs_attention", 1}, {"mine", "waiting", 1},
		{"unassigned", "all", 1}, {"my_teams", "all", 0},
	}
	for _, tt := range tests {
		t.Run(tt.scope+tt.state, func(t *testing.T) {
			filters := situationAllFilters()
			filters.Scope, filters.State = tt.scope, tt.state
			result, err := repo.List(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
			if err != nil || result.Total != tt.total {
				t.Fatalf("scope total = %#v err=%v, want %d", result, err, tt.total)
			}
		})
	}
	team := uuid.NewString()
	for _, member := range []string{crmsituationtest.Sales, crmsituationtest.Success} {
		crmsituationtest.Exec(t, db, "INSERT INTO team_workspace_memberships (id,team_id,workspace_member_id) VALUES (?,?,?)", uuid.NewString(), team, member)
	}
	filters := situationAllFilters()
	filters.Scope = "my_teams"
	result, err := repo.List(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil || result.Total != 2 {
		t.Fatalf("team membership scope = %#v %v", result, err)
	}
	crmsituationtest.Exec(t, db, "UPDATE workspace_members SET status = 'inactive' WHERE id = ?", crmsituationtest.Success)
	item, err := repo.GetByID(ctx, crmsituationtest.Workspace, delegated.ID)
	if err != nil || item.OwnerAvailable || item.Situation.OwnerMemberID == nil || *item.Situation.OwnerMemberID != crmsituationtest.Success {
		t.Fatalf("inactive owner was reassigned: %#v %v", item, err)
	}
	filters.Scope = "unassigned"
	result, err = repo.List(ctx, crmsituationtest.Workspace, crmsituationtest.Sales, filters)
	if err != nil || result.Total != 3 {
		t.Fatalf("ownership gaps = %#v %v", result, err)
	}
}

func TestCRMSituationRepositoryRoutingPreservesExistingMotions(t *testing.T) {
	db := crmsituationtest.Open(t)
	repo := NewCRMSituationRepository(db)
	for _, tt := range []struct{ motion, owner string }{
		{"conversion", crmsituationtest.Sales}, {"onboarding", crmsituationtest.Success},
		{"adoption", crmsituationtest.Success}, {"retention", crmsituationtest.Success},
		{"renewal", crmsituationtest.Sales}, {"expansion", crmsituationtest.Sales},
	} {
		t.Run(tt.motion, func(t *testing.T) {
			owner, err := repo.ResolveOwner(context.Background(), crmsituationtest.Workspace, model.CreateCRMSituationRequest{
				CommercialMotion: tt.motion, CompanyID: crmsituationtest.Ptr(crmsituationtest.Company), DealID: crmsituationtest.Ptr(crmsituationtest.Deal),
			})
			if err != nil || owner == nil || *owner != tt.owner {
				t.Fatalf("owner = %v err=%v want=%s", owner, err, tt.owner)
			}
		})
	}
}

func TestCRMSituationSchemaRequiresOutcomeBeforeClosure(t *testing.T) {
	db := crmsituationtest.Open(t)
	input := crmsituationtest.Situation("retention")
	input.Lifecycle = "closed"
	if err := db.Create(&input).Error; err == nil {
		t.Fatal("closed situation accepted without customer outcome")
	}
}

func situationAllFilters() model.CRMSituationListFilters {
	return model.CRMSituationListFilters{Scope: "all", State: "all", Category: "all", Page: 1, PageSize: 25}
}
