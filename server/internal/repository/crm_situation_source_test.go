package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMSituationSourceConcurrentImport(t *testing.T) {
	db := f.Open(t)
	repo := NewCRMSituationRepository(db)
	input := model.CRMSituationSourceInput{Kind: "signal", SourceID: f.Signal, Situation: f.Situation("conversion")}
	var workers sync.WaitGroup
	var failures [8]error
	for i := range failures {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			failures[index] = repo.ImportSource(context.Background(), input)
		}(i)
	}
	workers.Wait()
	for _, err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	list, err := repo.List(context.Background(), f.Workspace, f.Sales, situationAllFilters())
	if err != nil || list.Total != 1 {
		t.Fatalf("concurrent import duplicated work: %#v %v", list, err)
	}
	// Replayed source updates must not reopen a closed objective.
	f.Exec(t, db, "UPDATE crm_situations SET lifecycle = 'closed', outcome_kind = 'not_pursued', outcome_summary = 'Human decision', closed_at = ? WHERE id = ?", time.Now().UTC(), list.Data[0].Situation.ID)
	if err := repo.ImportSource(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	item, err := repo.GetByID(context.Background(), f.Workspace, list.Data[0].Situation.ID)
	if err != nil || item.Situation.Lifecycle != "closed" {
		t.Fatalf("source replay reopened work: %#v %v", item, err)
	}
}

func TestCRMSituationSourceForeignDestinationRollsBack(t *testing.T) {
	db := f.Open(t)
	repo := NewCRMSituationRepository(db)
	input := model.CRMSituationSourceInput{Kind: "suggestion", SourceID: f.Suggestion, Situation: f.Situation("conversion"), ExistingSituationID: f.Ptr(uuid.NewString())}
	if err := repo.ImportSource(context.Background(), input); err == nil {
		t.Fatal("unavailable explicit destination accepted")
	}
	for _, table := range []string{"crm_situations", "crm_situation_source_links", "crm_situation_references"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("partial import in %s", table)
		}
	}
}

func TestCRMSituationSourcePreservesRenewalDealMotion(t *testing.T) {
	db := f.Open(t)
	repo := NewCRMSituationRepository(db)
	f.Schema(t, db, "CREATE TABLE crm_pipelines (id uuid PRIMARY KEY, workspace_id uuid, default_commercial_motion text)")
	pipeline := uuid.NewString()
	f.Exec(t, db, "INSERT INTO crm_pipelines (id,workspace_id,default_commercial_motion) VALUES (?,?,?)", pipeline, f.Workspace, "renewal")
	f.Exec(t, db, "UPDATE crm_deals SET pipeline_id = ? WHERE id = ?", pipeline, f.Deal)
	motion, err := repo.SourceMotion(context.Background(), model.CRMSuggestion{WorkspaceID: f.Workspace, SuggestionType: "deal_advance", ObjectType: f.Ptr("deal"), ObjectID: f.Ptr(f.Deal)})
	if err != nil || motion != "renewal" {
		t.Fatalf("renewal proposal was recategorized: %s %v", motion, err)
	}
}
