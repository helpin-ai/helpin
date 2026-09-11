package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMSituationSourceCustomerJourneys(t *testing.T) {
	db, sources, work, decisions, ctx := situationSourceFixture(t)
	for _, motion := range []string{"conversion", "onboarding", "renewal"} {
		t.Run(motion, func(t *testing.T) {
			signal := sourceJourneySignal(motion)
			if err := db.Create(&signal).Error; err != nil {
				t.Fatal(err)
			}
			if err := sources.ReconcileWorkspace(ctx, f.Workspace); err != nil {
				t.Fatal(err)
			}
			var link model.CRMSituationSourceLink
			if err := db.Where("kind = 'signal' AND source_id = ?", signal.ID).Take(&link).Error; err != nil {
				t.Fatal(err)
			}
			suggestion := sourceJourneySuggestion(motion)
			suggestion.Context["situation_id"] = link.SituationID
			suggestion.SignalIDs = model.StringArray{signal.ID}
			if err := db.Create(&suggestion).Error; err != nil {
				t.Fatal(err)
			}
			if err := sources.ReconcileWorkspace(ctx, f.Workspace); err != nil {
				t.Fatal(err)
			}
			item, err := work.GetByID(ctx, f.Workspace, link.SituationID)
			if err != nil || item.PendingActionCount != 1 || item.EffectiveAttention != "needs_approval" || len(item.Actions) != 1 {
				t.Fatalf("review projection: %#v %v", item, err)
			}
			result, err := work.DecideAction(ctx, f.Workspace, link.SituationID, suggestion.ID, item.Actions[0].Revision, "accept", "", nil)
			if err != nil || result.ExecutionStatus != "manual_required" || result.ExecutedAt != nil {
				t.Fatalf("manual follow-through: %#v %v", result, err)
			}
			item, err = work.GetByID(ctx, f.Workspace, link.SituationID)
			if err != nil || item.Situation.Lifecycle != "open" || item.Situation.OutcomeKind != nil || item.PendingActionCount != 0 || item.ManualActionCount != 1 {
				t.Fatalf("decision falsely completed work: %#v %v", item, err)
			}
			if _, err := decisions.AcceptSuggestion(ctx, f.Workspace, suggestion.ID, nil); !errors.Is(err, ErrCRMSuggestionStale) {
				t.Fatalf("repeat approval: %v", err)
			}
			if err := sources.ReconcileWorkspace(ctx, f.Workspace); err != nil {
				t.Fatal(err)
			}
		})
	}
	list, err := work.List(ctx, f.Workspace, model.CRMSituationListFilters{Scope: "all", State: "all"})
	if err != nil || list.Total != 3 {
		t.Fatalf("independent customer journeys: %#v %v", list, err)
	}
	var reviewed int64
	if err := db.Model(&model.CRMSignal{}).Where("reviewed_at IS NOT NULL OR acted_at IS NOT NULL").Count(&reviewed).Error; err != nil {
		t.Fatal(err)
	}
	if reviewed != 0 {
		t.Fatal("action decision changed evidence feedback")
	}
}

func TestCRMSituationSourceStandaloneAndPagination(t *testing.T) {
	db, sources, work, _, ctx := situationSourceFixture(t)
	for i := 0; i < 205; i++ {
		suggestion := sourceJourneySuggestion("")
		suggestion.Title = fmt.Sprintf("Standalone review %03d", i)
		suggestion.ObjectType, suggestion.ObjectID, suggestion.Context = nil, nil, nil
		if err := db.Create(&suggestion).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := sources.ReconcileWorkspace(ctx, f.Workspace); err != nil {
			t.Fatal(err)
		}
	}
	list, err := work.List(ctx, f.Workspace, model.CRMSituationListFilters{Scope: "all", State: "all", Page: 9})
	if err != nil || list.Total != 205 || len(list.Data) != 5 || list.UncategorizedCount != 205 || list.PendingActionTotal != 205 {
		t.Fatalf("standalone work lost or duplicated: %#v %v", list, err)
	}
	if list.Data[0].Situation.CreatedByMemberID != nil || list.Data[0].Situation.CompanyID != nil {
		t.Fatal("import invented a human creator or customer")
	}
}

func TestCRMSituationSourcePolicyGates(t *testing.T) {
	db, _, _, _, ctx := situationSourceFixture(t)
	signals := NewCRMSignalService(repository.NewCRMSignalRepository(db), nil)
	base := sourceJourneySignal("conversion")
	for _, tt := range []struct {
		name   string
		change func(*model.CRMSignal)
	}{
		{"foreign workspace", func(s *model.CRMSignal) { s.WorkspaceID = f.ForeignWorkspace }},
		{"unknown trust", func(s *model.CRMSignal) { s.EvidenceIdentityTrust = "unknown" }},
		{"low priority", func(s *model.CRMSignal) { s.Confidence = .001 }},
		{"no version", func(s *model.CRMSignal) { s.RuleVersion = nil }},
		{"wrong version", func(s *model.CRMSignal) { v := 2; s.RuleVersion = &v }},
		{"dismissed", func(s *model.CRMSignal) { now := time.Now(); s.DismissedAt = &now }},
		{"superseded", func(s *model.CRMSignal) { now := time.Now(); s.SupersededAt = &now }},
		{"acted", func(s *model.CRMSignal) { now := time.Now(); s.ActedAt = &now }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := base
			tt.change(&input)
			rows, err := signals.QualifySituationSignals(ctx, f.Workspace, []model.CRMSignal{input})
			if err != nil || len(rows) != 0 {
				t.Fatalf("policy bypass: %#v %v", rows, err)
			}
		})
	}
	// No deliveries/PM tables exist: neither notification history nor unrelated
	// task presence is consulted when recording qualified customer work.
	rows, err := signals.QualifySituationSignals(ctx, f.Workspace, []model.CRMSignal{base})
	if err != nil || len(rows) != 1 {
		t.Fatalf("qualified signal: %#v %v", rows, err)
	}
	f.Exec(t, db, "UPDATE crm_signal_rule_configs SET shadow_mode = TRUE")
	rows, err = signals.QualifySituationSignals(ctx, f.Workspace, []model.CRMSignal{base})
	if err != nil || len(rows) != 0 {
		t.Fatalf("rule shadow bypass: %#v %v", rows, err)
	}
	f.Exec(t, db, "UPDATE crm_signal_rule_configs SET shadow_mode = FALSE")
	f.Exec(t, db, "INSERT INTO crm_signal_rollout_settings (id,workspace_id,mode) VALUES (?,?,?)", uuid.NewString(), f.Workspace, "shadow")
	rows, err = signals.QualifySituationSignals(ctx, f.Workspace, []model.CRMSignal{base})
	if err != nil || len(rows) != 0 {
		t.Fatalf("workspace shadow bypass: %#v %v", rows, err)
	}
}

func TestCRMSituationSourceHistoricalDecisionsStayHistorical(t *testing.T) {
	db, sources, work, _, ctx := situationSourceFixture(t)
	for _, status := range []string{"accepted", "dismissed"} {
		suggestion := sourceJourneySuggestion("conversion")
		suggestion.Status, suggestion.ExecutionStatus = status, "succeeded"
		if err := db.Create(&suggestion).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := sources.ReconcileWorkspace(ctx, f.Workspace); err != nil {
		t.Fatal(err)
	}
	list, err := work.List(ctx, f.Workspace, model.CRMSituationListFilters{Scope: "all", State: "all"})
	if err != nil || list.Total != 0 {
		t.Fatalf("backfill reopened history: %#v %v", list, err)
	}
}

func situationSourceFixture(t *testing.T) (*gorm.DB, *CRMSituationSourceService, *CRMSituationService, *CRMSuggestionService, context.Context) {
	t.Helper()
	db, work, ctx := situationServiceFixture(t)
	f.Exec(t, db, "DELETE FROM crm_suggestions")
	f.Exec(t, db, "DELETE FROM crm_signals")
	f.EnablePolicy(t, db)
	repo := repository.NewCRMSituationRepository(db)
	sources := NewCRMSituationSourceService(repo, NewCRMSignalService(repository.NewCRMSignalRepository(db), nil))
	decisions := NewCRMSuggestionService(repository.NewCRMSuggestionRepository(db), repository.NewCRMDealRepository(db), nil)
	decisions.SetSituationSources(sources)
	decisions.SetSituationGuard(repo)
	work.SetActions(decisions)
	return db, sources, work, decisions, ctx
}

func sourceJourneySignal(motion string) model.CRMSignal {
	version := 1
	return model.CRMSignal{ID: uuid.NewString(), WorkspaceID: f.Workspace, CompanyID: f.Ptr(f.Company),
		CommercialMotion: motion, SignalType: "buying_intent", SourceType: "manual", Summary: "Customer requests a concrete next step",
		RuleKey: f.Ptr("customer_event"), RuleVersion: &version, Confidence: 1, EvidenceIdentityTrust: "verified",
		DetectedAt: time.Now().UTC(), Metadata: model.JSONB{}}
}

func sourceJourneySuggestion(motion string) model.CRMSuggestion {
	return model.CRMSuggestion{ID: uuid.NewString(), WorkspaceID: f.Workspace, SuggestionType: "follow_up",
		ObjectType: f.Ptr("company"), ObjectID: f.Ptr(f.Company), Title: "Agree the next milestone",
		Context: model.JSONB{"commercial_motion": motion}, SignalIDs: model.StringArray{}, Status: "pending", ExecutionStatus: "pending"}
}

func TestExistingBusinessSourceDoesNotFallBackToConversion(t *testing.T) {
	db, sources, _, _, ctx := situationSourceFixture(t)
	f.InboxTables(t, db)
	pipeline := uuid.NewString()
	f.Exec(t, db, "INSERT INTO crm_pipelines VALUES (?,?,'existing_business')", pipeline, f.Workspace)
	f.Exec(t, db, "UPDATE crm_deals SET pipeline_id=? WHERE id=?", pipeline, f.Deal)
	suggestion := sourceJourneySuggestion("")
	suggestion.SuggestionType = "deal_advance"
	suggestion.ObjectType = f.Ptr("deal")
	suggestion.ObjectID = f.Ptr(f.Deal)
	if err := db.Create(&suggestion).Error; err != nil {
		t.Fatal(err)
	}
	if err := sources.ImportSuggestion(ctx, suggestion); err != nil {
		t.Fatal(err)
	}
	var work model.CRMSituation
	if err := db.Take(&work).Error; err != nil {
		t.Fatal(err)
	}
	if work.CommercialMotion != "needs_context" {
		t.Fatalf("motion=%s", work.CommercialMotion)
	}
}
