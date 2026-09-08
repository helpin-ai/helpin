package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMSituationServiceCustomerJourneys(t *testing.T) {
	db, svc, ctx := situationServiceFixture(t)
	for _, tt := range []struct{ name, motion, owner string }{
		{"buying intent", "conversion", f.Sales},
		{"customer handoff", "onboarding", f.Success},
		{"renewal risk", "retention", f.Success},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := situationCreateRequest()
			req.CommercialMotion = tt.motion
			req.References = []model.CRMSituationReference{{Kind: "signal", SourceID: f.Signal}, {Kind: "suggestion", SourceID: f.Suggestion}}
			stored, created, err := svc.Create(ctx, f.Workspace, req)
			if err != nil || !created {
				t.Fatalf("create: created=%v err=%v", created, err)
			}
			if stored.OwnerMemberID == nil || *stored.OwnerMemberID != tt.owner ||
				stored.NextActionOwnerMemberID == nil || *stored.NextActionOwnerMemberID != tt.owner ||
				stored.Lifecycle != "open" || stored.Attention != "needs_context" || stored.Priority != 12.75 ||
				stored.CreatedByMemberID == nil || *stored.CreatedByMemberID != f.Sales || stored.OutcomeKind != nil {
				t.Fatalf("incorrect initial situation: %#v", stored)
			}
			item, err := svc.GetByID(ctx, f.Workspace, stored.ID)
			if err != nil || len(item.References) != 2 || item.CompanyName != "Northstar" {
				t.Fatalf("detail: %#v %v", item, err)
			}
		})
	}
	result, err := svc.List(ctx, f.Workspace, model.CRMSituationListFilters{Scope: "all"})
	if err != nil || result.Total != 3 || result.CategoryCounts["all"] != 3 {
		t.Fatalf("separate journeys: %#v %v", result, err)
	}
	var status struct{ Status, ExecutionStatus string }
	if err := db.Table("crm_suggestions").Where("id = ?", f.Suggestion).Take(&status).Error; err != nil {
		t.Fatal(err)
	}
	if status.Status != "pending" || status.ExecutionStatus != "pending" {
		t.Fatalf("creation approved or executed a referenced action: %#v", status)
	}
}

func TestCRMSituationServiceReplayUsesNormalizedIntent(t *testing.T) {
	db, svc, ctx := situationServiceFixture(t)
	req := situationCreateRequest()
	req.Title = "  Buying intent  "
	req.References = []model.CRMSituationReference{{Kind: "suggestion", SourceID: f.Suggestion}, {Kind: "signal", SourceID: f.Signal}}
	first, _, err := svc.Create(ctx, f.Workspace, req)
	if err != nil {
		t.Fatal(err)
	}
	req.Title = "Buying intent"
	req.OwnerMode = "routing"
	req.References = []model.CRMSituationReference{{Kind: "signal", SourceID: f.Signal}, {Kind: "suggestion", SourceID: f.Suggestion}, {Kind: "signal", SourceID: f.Signal}}
	// A routing change must not turn a network retry into reassignment.
	f.Exec(t, db, "UPDATE crm_companies SET owner_member_id = ? WHERE id = ?", f.Success, f.Company)
	again, created, err := svc.Create(ctx, f.Workspace, req)
	if err != nil || created || again.ID != first.ID || *again.OwnerMemberID != f.Sales {
		t.Fatalf("normalized replay changed work: %#v created=%v err=%v", again, created, err)
	}
	req.Objective = "A different customer objective"
	if _, _, err := svc.Create(ctx, f.Workspace, req); !errors.Is(err, repository.ErrCRMSituationConflict) {
		t.Fatalf("different request reused key: %v", err)
	}
	item, err := svc.GetByID(ctx, f.Workspace, first.ID)
	if err != nil || *item.Situation.OwnerMemberID != f.Sales || len(item.References) != 2 {
		t.Fatalf("read rewrote stored ownership or links: %#v %v", item, err)
	}
}

func TestCRMSituationServiceExplicitOwnership(t *testing.T) {
	_, svc, ctx := situationServiceFixture(t)
	for _, mode := range []string{"member", "unassigned"} {
		t.Run(mode, func(t *testing.T) {
			req := situationCreateRequest()
			req.OwnerMode = mode
			if mode == "member" {
				req.OwnerMemberID = f.Ptr(f.Success)
				req.NextActionOwnerMemberID = f.Ptr(f.Sales)
			}
			item, _, err := svc.Create(ctx, f.Workspace, req)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "unassigned" && (item.OwnerMemberID != nil || item.NextActionOwnerMemberID != nil) {
				t.Fatal("explicit unassigned work was routed")
			}
			if mode == "member" && (*item.OwnerMemberID != f.Success || *item.NextActionOwnerMemberID != f.Sales) {
				t.Fatal("explicit responsibilities were lost")
			}
		})
	}
}

func TestCRMSituationServiceAuthorization(t *testing.T) {
	_, svc, ctx := situationServiceFixture(t)
	stored, _, err := svc.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	viewer := authorization.WithActor(context.Background(), f.Actor("viewer"))
	if _, err := svc.GetByID(viewer, f.Workspace, stored.ID); err != nil {
		t.Fatalf("viewer cannot read: %v", err)
	}
	if _, _, err := svc.Create(viewer, f.Workspace, situationCreateRequest()); !errors.Is(err, ErrCRMSituationForbidden) {
		t.Fatalf("viewer can create: %v", err)
	}
	inactive, foreign, malformed := f.Actor("member"), f.Actor("owner"), f.Actor("owner")
	inactive.Status = "inactive"
	foreign.WorkspaceID = f.ForeignWorkspace
	malformed.WorkspaceMemberID = "user-id-not-member-id"
	for _, actor := range []*authorization.Actor{nil, inactive, foreign, malformed, f.Actor("unknown")} {
		t.Run(strings.Join([]string{"actor", actorRole(actor)}, ":"), func(t *testing.T) {
			denied := context.Background()
			if actor != nil {
				denied = authorization.WithActor(denied, actor)
			}
			if _, err := svc.List(denied, f.Workspace, model.CRMSituationListFilters{}); !errors.Is(err, ErrCRMSituationForbidden) {
				t.Fatalf("unauthorized list: %v", err)
			}
			if _, err := svc.GetByID(denied, f.Workspace, stored.ID); !errors.Is(err, ErrCRMSituationForbidden) {
				t.Fatalf("unauthorized detail: %v", err)
			}
			if _, _, err := svc.Create(denied, f.Workspace, situationCreateRequest()); !errors.Is(err, ErrCRMSituationForbidden) {
				t.Fatalf("unauthorized creation: %v", err)
			}
		})
	}
	if _, err := svc.GetByID(ctx, f.Workspace, uuid.NewString()); !errors.Is(err, ErrCRMSituationNotFound) {
		t.Fatalf("missing record: %v", err)
	}
}

func TestCRMSituationServiceInvalidCreation(t *testing.T) {
	_, svc, ctx := situationServiceFixture(t)
	for name, change := range map[string]func(*model.CreateCRMSituationRequest){
		"no key":               func(r *model.CreateCRMSituationRequest) { r.CreationKey = " " },
		"empty title":          func(r *model.CreateCRMSituationRequest) { r.Title = " " },
		"long title":           func(r *model.CreateCRMSituationRequest) { r.Title = strings.Repeat("a", 241) },
		"no objective":         func(r *model.CreateCRMSituationRequest) { r.Objective = "" },
		"no target":            func(r *model.CreateCRMSituationRequest) { r.CompanyID = nil },
		"invalid target":       func(r *model.CreateCRMSituationRequest) { r.CompanyID = f.Ptr("invalid") },
		"nil uuid":             func(r *model.CreateCRMSituationRequest) { r.CompanyID = f.Ptr(uuid.Nil.String()) },
		"category as motion":   func(r *model.CreateCRMSituationRequest) { r.CommercialMotion = "sales" },
		"unknown owner mode":   func(r *model.CreateCRMSituationRequest) { r.OwnerMode = "automatic" },
		"member without owner": func(r *model.CreateCRMSituationRequest) { r.OwnerMode = "member" },
		"routing with owner":   func(r *model.CreateCRMSituationRequest) { r.OwnerMemberID = f.Ptr(f.Sales) },
		"negative priority":    func(r *model.CreateCRMSituationRequest) { r.Priority = -1 },
		"too high priority":    func(r *model.CreateCRMSituationRequest) { r.Priority = 101 },
		"nan priority":         func(r *model.CreateCRMSituationRequest) { r.Priority = math.NaN() },
		"unknown reference": func(r *model.CreateCRMSituationRequest) {
			r.References = []model.CRMSituationReference{{Kind: "run", SourceID: f.Signal}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			req := situationCreateRequest()
			change(&req)
			if _, _, err := svc.Create(ctx, f.Workspace, req); !errors.Is(err, ErrCRMSituationInput) {
				t.Fatalf("invalid input: %v", err)
			}
		})
	}
}

func TestCRMSituationServiceFilters(t *testing.T) {
	_, svc, ctx := situationServiceFixture(t)
	if _, _, err := svc.Create(ctx, f.Workspace, situationCreateRequest()); err != nil {
		t.Fatal(err)
	}
	result, err := svc.List(ctx, f.Workspace, model.CRMSituationListFilters{})
	if err != nil || result.Page != 1 || result.PageSize != 25 || result.Total != 1 {
		t.Fatalf("default mine/attention: %#v %v", result, err)
	}
	for _, filters := range []model.CRMSituationListFilters{
		{Scope: "foreign_team"}, {State: "accepted"}, {Category: "conversion"},
		{Page: -1}, {PageSize: 101}, {Search: strings.Repeat("a", 501)},
		{Query: &model.QueryFilterGroup{Logic: "xor"}},
	} {
		if _, err := svc.List(ctx, f.Workspace, filters); !errors.Is(err, ErrCRMSituationInput) {
			t.Fatalf("invalid filters %#v: %v", filters, err)
		}
	}
	_, err = svc.List(ctx, f.Workspace, model.CRMSituationListFilters{Query: &model.QueryFilterGroup{
		Rules: []model.QueryFilterRule{{Field: "workspace_id", Operator: model.QueryFilterOpIs, Value: f.Ptr(f.ForeignWorkspace)}},
	}})
	var invalid *querybuilder.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("unknown filter field was allowed: %v", err)
	}
}

func situationServiceFixture(t *testing.T) (*gorm.DB, *CRMSituationService, context.Context) {
	t.Helper()
	db := f.Open(t)
	svc := NewCRMSituationService(repository.NewCRMSituationRepository(db), authorization.NewAuthzService(db, nil, nil))
	return db, svc, authorization.WithActor(context.Background(), f.Actor("member"))
}

func situationCreateRequest() model.CreateCRMSituationRequest {
	return model.CreateCRMSituationRequest{
		CreationKey: uuid.NewString(), Title: "Buying intent", Objective: "Agree the customer's next milestone",
		CommercialMotion: "conversion", CompanyID: f.Ptr(f.Company), Priority: 12.75,
	}
}

func actorRole(actor *authorization.Actor) string {
	if actor == nil {
		return "missing"
	}
	return actor.Role + ":" + actor.Status + ":" + actor.WorkspaceID
}
