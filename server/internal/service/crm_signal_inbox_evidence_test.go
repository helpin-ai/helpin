package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMInboxEvidenceWithoutAutomationPolicy(t *testing.T) {
	db, repo, svc, ctx := inboxEvidenceFixture(t)
	for _, motion := range []string{"expansion", "expansion", "renewal", "retention"} {
		insertInboxEvidence(t, db, motion)
	}
	for i := 0; i < 2; i++ {
		suggestion := sourceJourneySuggestion("")
		suggestion.ObjectID, suggestion.ObjectType, suggestion.Context = nil, nil, nil
		if err := db.Create(&suggestion).Error; err != nil {
			t.Fatal(err)
		}
	}
	feed, err := NewCRMSignalService(repository.NewCRMSignalRepository(db), nil).ListWorkspaceSignalLanes(ctx, f.Workspace, model.CRMSignalListFilters{}, model.PMPagination{Page: 1, PerPage: 100}, nil)
	if err != nil || feed.Total != 3 {
		t.Fatalf("legacy feed: %+v %v", feed, err)
	}
	for i := 0; i < 2; i++ {
		// The default includes everyone, including standalone unassigned approvals.
		list, err := svc.ListInbox(ctx, f.Workspace, model.CRMSignalInboxFilters{})
		if err != nil || list.Total != 6 || list.CategoryCounts["expansion"] != 1 || list.CategoryCounts["retention"] != 2 || list.UncategorizedCount != 3 {
			t.Fatalf("evidence disappeared without automation policy: %+v %v", list, err)
		}
		groups := map[string]model.CRMSignalInboxItem{}
		for _, item := range list.Data {
			if item.Kind == "evidence" {
				groups[item.ID] = item
			}
		}
		for _, expected := range feed.Data {
			item, found := groups[expected.ID]
			if !found || item.Priority == nil || *item.Priority != expected.Priority {
				t.Fatalf("group/score changed: %+v, want %+v", item, expected)
			}
			detail, err := svc.InboxSignalGroup(ctx, f.Workspace, item.ID)
			if err != nil || len(detail.Signals) != len(expected.Signals) {
				t.Fatalf("evidence detail: %+v %v", detail, err)
			}
		}
	}
	for _, table := range []string{"crm_situations", "crm_situation_source_links", "automation_scheduled_events", "crm_signal_routing_policies"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("GET wrote %s: %d %v", table, count, err)
		}
	}
	var rule model.CRMSignalRuleConfig
	if err := db.First(&rule).Error; err != nil || !rule.ShadowMode {
		t.Fatalf("rule activation changed: %+v %v", rule, err)
	}
	// Visibility is independent; the existing automatic-import gate stays closed.
	sources := NewCRMSituationSourceService(repo, NewCRMSignalService(repository.NewCRMSignalRepository(db), nil))
	qualified, err := sources.signals.QualifySituationSignals(ctx, f.Workspace, []model.CRMSignal{sourceJourneySignal("expansion")})
	if err != nil || len(qualified) != 0 {
		t.Fatalf("visibility granted automatic enrollment: %+v %v", qualified, err)
	}
}

func TestCRMInboxEvidenceFiltersAndPagination(t *testing.T) {
	db, _, svc, ctx := inboxEvidenceFixture(t)
	insertInboxEvidence(t, db, "expansion")
	insertInboxEvidence(t, db, "renewal")
	insertInboxEvidence(t, db, "retention")
	for _, tt := range []struct {
		name, scope, state, category, search string
		total                                int64
	}{
		{name: "everyone", total: 4},
		// Existing ownership routes expansion/renewal to Sales and retention to Success.
		{name: "assigned to sales", scope: "mine", total: 2},
		{name: "unassigned approval", scope: "unassigned", total: 1},
		{name: "approval only", state: "needs_approval", total: 1},
		{name: "closed", state: "closed", total: 0},
		{name: "paused", state: "paused", total: 0},
		{name: "waiting", state: "waiting", total: 0},
		{name: "retention category includes renewal", category: "retention", total: 2},
		{name: "literal search", search: "100%_pricing", total: 3},
		{name: "search must escape wildcard", search: "100%_pricingX", total: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			list, err := svc.ListInbox(ctx, f.Workspace, model.CRMSignalInboxFilters{Navigation: model.CRMSituationListFilters{Scope: tt.scope, State: tt.state, Category: tt.category, Search: tt.search}})
			if err != nil || list.Total != tt.total {
				t.Fatalf("filtered inbox: %+v %v", list, err)
			}
		})
	}
	for _, sortOrder := range []string{"priority", "newest", "oldest", "recommended"} {
		seen := map[string]bool{}
		for page := 1; page <= 2; page++ {
			list, err := svc.ListInbox(ctx, f.Workspace, model.CRMSignalInboxFilters{Sort: sortOrder, Navigation: model.CRMSituationListFilters{Page: page, PageSize: 2}})
			if err != nil || list.Total != 4 || len(list.Data) != 2 || list.CategoryCounts["retention"] != 2 {
				t.Fatalf("global pagination: %+v %v", list, err)
			}
			for _, item := range list.Data {
				if seen[item.ID] {
					t.Fatal("duplicate across pages")
				}
				seen[item.ID] = true
			}
		}
	}
	for _, field := range []string{"evidence_review", "attention", "priority", "has_follow_up"} {
		value := map[string]string{"evidence_review": "needs_review", "attention": "needs_context", "priority": "high", "has_follow_up": "yes"}[field]
		list, err := svc.ListInbox(ctx, f.Workspace, model.CRMSignalInboxFilters{Navigation: model.CRMSituationListFilters{Query: &model.QueryFilterGroup{Logic: "and", Rules: []model.QueryFilterRule{{Field: field, Operator: "is", Value: &value}}}}})
		want := int64(3)
		if field == "has_follow_up" {
			want = 0
		}
		if err != nil || list.Total != want {
			t.Fatalf("%s filter: %+v %v", field, list, err)
		}
	}
}

func TestCRMInboxEvidenceDoesNotDuplicateTrackedOrHistoricalSources(t *testing.T) {
	for _, lifecycle := range []string{"open", "paused", "closed", "recommendation"} {
		t.Run(lifecycle, func(t *testing.T) {
			db, repo, svc, ctx := inboxEvidenceFixture(t)
			signal := insertInboxEvidence(t, db, "expansion")
			list, err := svc.ListInbox(ctx, f.Workspace, model.CRMSignalInboxFilters{})
			if err != nil {
				t.Fatal(err)
			}
			groupID := list.Data[0].ID
			if lifecycle == "recommendation" {
				f.Exec(t, db, `UPDATE crm_suggestions SET signal_ids = ? WHERE id = ?`, model.StringArray{signal.ID}, f.Suggestion)
			} else {
				work := f.Situation("expansion")
				stored, _, err := repo.Create(ctx, work, []model.CRMSituationReference{{Kind: "signal", SourceID: signal.ID}})
				if err != nil {
					t.Fatal(err)
				}
				if lifecycle == "closed" {
					f.Exec(t, db, `UPDATE crm_situations SET lifecycle = 'closed', outcome_kind = 'not_pursued', outcome_summary = 'Customer declined', closed_at = CURRENT_TIMESTAMP WHERE id = ?`, stored.ID)
				} else {
					f.Exec(t, db, `UPDATE crm_situations SET lifecycle = ? WHERE id = ?`, lifecycle, stored.ID)
				}
			}
			list, err = svc.ListInbox(ctx, f.Workspace, model.CRMSignalInboxFilters{Navigation: model.CRMSituationListFilters{State: "all"}})
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range list.Data {
				if item.Kind == "evidence" {
					t.Fatalf("tracked evidence reappeared: %+v", item)
				}
			}
			if _, err := svc.InboxSignalGroup(ctx, f.Workspace, groupID); !errors.Is(err, ErrCRMSituationNotFound) {
				t.Fatalf("stale group: %v", err)
			}
		})
	}
}

func TestCRMInboxEvidenceRespectsVisibilityAndTenantBoundaries(t *testing.T) {
	db, _, svc, ctx := inboxEvidenceFixture(t)
	visible := insertInboxEvidence(t, db, "expansion")
	for _, kind := range []string{"dismissed", "superseded", "acted", "low_score", "context_only", "foreign"} {
		signal := sourceJourneySignal("retention")
		now := time.Now().UTC()
		switch kind {
		case "dismissed":
			signal.DismissedAt = &now
		case "superseded":
			signal.SupersededAt = &now
		case "acted":
			signal.ActedAt = &now
		case "low_score":
			signal.Confidence = .001
		case "context_only":
			signal.SourceType, signal.DetectorKind = "support", "rule_derived"
		case "foreign":
			signal.WorkspaceID, signal.CompanyID = f.ForeignWorkspace, f.Ptr(f.ForeignCompany)
		}
		if err := db.Create(&signal).Error; err != nil {
			t.Fatal(err)
		}
	}
	list, err := svc.ListInbox(ctx, f.Workspace, model.CRMSignalInboxFilters{})
	if err != nil || list.Total != 2 || list.Data[0].Kind != "evidence" {
		t.Fatalf("hidden evidence leaked: %+v %v", list, err)
	}
	groupID := list.Data[0].ID
	detail, err := svc.InboxSignalGroup(ctx, f.Workspace, groupID)
	if err != nil || len(detail.Signals) != 1 || detail.Signals[0].ID != visible.ID {
		t.Fatalf("group evidence: %+v %v", detail, err)
	}
	if _, err := svc.InboxSignalGroup(context.Background(), f.Workspace, groupID); !errors.Is(err, ErrCRMSituationForbidden) {
		t.Fatalf("missing auth: %v", err)
	}
	if _, err := svc.InboxSignalGroup(ctx, f.ForeignWorkspace, groupID); !errors.Is(err, ErrCRMSituationForbidden) {
		t.Fatalf("foreign auth: %v", err)
	}
	if _, err := svc.InboxSignalGroup(ctx, f.Workspace, "invalid"); !errors.Is(err, ErrCRMSituationInput) {
		t.Fatalf("invalid ID: %v", err)
	}
	f.Exec(t, db, `INSERT INTO crm_signal_rollout_settings (id,workspace_id,mode) VALUES (?,?,'shadow')`, uuid.NewString(), f.Workspace)
	list, err = svc.ListInbox(ctx, f.Workspace, model.CRMSignalInboxFilters{})
	if err != nil || list.Total != 1 {
		t.Fatalf("workspace opt-out bypassed: %+v %v", list, err)
	}
}

func inboxEvidenceFixture(t *testing.T) (*gorm.DB, *repository.CRMSituationRepository, *CRMSituationService, context.Context) {
	t.Helper()
	db := f.Open(t)
	f.InboxTables(t, db)
	f.EnablePolicy(t, db)
	f.Schema(t, db, `CREATE TABLE crm_signal_routing_settings (id uuid PRIMARY KEY, workspace_id uuid, default_signal_owner_member_id uuid, minimum_lane_priority double precision, created_at datetime, updated_at datetime)`)
	f.Exec(t, db, `DELETE FROM crm_signals`)
	f.Exec(t, db, `DELETE FROM crm_signal_routing_policies`)
	f.Exec(t, db, `UPDATE crm_signal_rule_configs SET shadow_mode = TRUE`)
	repo := repository.NewCRMSituationRepository(db)
	repo.SetInboxSignalComposer(ComposeCRMInboxSignalGroups)
	svc := NewCRMSituationService(repo, authorization.NewAuthzService(db, nil, nil))
	return db, repo, svc, authorization.WithActor(context.Background(), f.Actor("member"))
}

func insertInboxEvidence(t *testing.T, db *gorm.DB, motion string) model.CRMSignal {
	t.Helper()
	signal := sourceJourneySignal(motion)
	signal.Summary = "Customer requests 100%_pricing details"
	if err := db.Create(&signal).Error; err != nil {
		t.Fatal(err)
	}
	return signal
}
