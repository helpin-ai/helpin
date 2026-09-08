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

func TestCRMSituationRepositoryConcurrentCreation(t *testing.T) {
	db := f.Open(t)
	repo := NewCRMSituationRepository(db)
	input := f.Situation("conversion")
	type result struct {
		item    *model.CRMSituation
		created bool
		err     error
	}
	const attempts = 12
	results := make(chan result, attempts)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < attempts; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			request := input
			request.ID = uuid.NewString()
			<-start
			item, created, err := repo.Create(context.Background(), request,
				[]model.CRMSituationReference{{Kind: "signal", SourceID: f.Signal}})
			results <- result{item, created, err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	var id string
	created := 0
	for response := range results {
		if response.err != nil {
			t.Fatalf("concurrent create: %v", response.err)
		}
		if id != "" && response.item.ID != id {
			t.Fatal("concurrent retries created different situations")
		}
		id = response.item.ID
		if response.created {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("created = %d, want 1", created)
	}
	item, err := repo.GetByID(context.Background(), f.Workspace, id)
	if err != nil || len(item.References) != 1 {
		t.Fatalf("concurrent evidence links: %#v %v", item, err)
	}
}

func TestCRMSituationRepositoryDeletedSourceDoesNotDeleteWork(t *testing.T) {
	db := f.Open(t)
	repo := NewCRMSituationRepository(db)
	input := f.Situation("onboarding")
	if _, _, err := repo.Create(context.Background(), input,
		[]model.CRMSituationReference{{Kind: "signal", SourceID: f.Signal}, {Kind: "suggestion", SourceID: f.Suggestion}}); err != nil {
		t.Fatal(err)
	}
	f.Exec(t, db, "DELETE FROM crm_signals WHERE id = ?", f.Signal)
	item, err := repo.GetByID(context.Background(), f.Workspace, input.ID)
	if err != nil || item == nil || len(item.References) != 1 || item.References[0].Kind != "suggestion" {
		t.Fatalf("deleted source projection: %#v %v", item, err)
	}
	// A source moved out of the workspace must also disappear from disclosure.
	f.Exec(t, db, "UPDATE crm_suggestions SET workspace_id = ? WHERE id = ?", f.ForeignWorkspace, f.Suggestion)
	item, err = repo.GetByID(context.Background(), f.Workspace, input.ID)
	if err != nil || item == nil || len(item.References) != 0 {
		t.Fatalf("foreign source was disclosed: %#v %v", item, err)
	}
}

func TestCRMSituationRepositoryPriorityAndLiteralSearch(t *testing.T) {
	db := f.Open(t)
	repo := NewCRMSituationRepository(db)
	for _, priority := range []float64{12.1, 12.9, 12.5} {
		input := f.Situation("conversion")
		input.Priority = priority
		if priority == 12.5 {
			input.Title = "Usage crossed 90%_threshold"
		}
		if _, _, err := repo.Create(context.Background(), input, nil); err != nil {
			t.Fatal(err)
		}
	}
	filters := situationAllFilters()
	list, err := repo.List(context.Background(), f.Workspace, f.Sales, filters)
	if err != nil || len(list.Data) != 3 || list.Data[0].Situation.Priority != 12.9 || list.Data[1].Situation.Priority != 12.5 {
		t.Fatalf("lost business-priority precision: %#v %v", list, err)
	}
	filters.Search = "%_"
	list, err = repo.List(context.Background(), f.Workspace, f.Sales, filters)
	if err != nil || list.Total != 1 || list.CategoryCounts["all"] != 1 {
		t.Fatalf("search interpreted user wildcards: %#v %v", list, err)
	}
}

func TestCRMSituationSchemaClosureAndReferenceTenant(t *testing.T) {
	db := f.Open(t)
	input := f.Situation("retention")
	closedAt := time.Now().UTC()
	input.Lifecycle, input.OutcomeKind, input.OutcomeSummary, input.ClosedAt = "closed", f.Ptr("achieved"), f.Ptr("Renewal confirmed"), &closedAt
	if err := db.Create(&input).Error; err != nil {
		t.Fatalf("explicit closure: %v", err)
	}
	foreign := model.CRMSituationReference{WorkspaceID: f.ForeignWorkspace, SituationID: input.ID, Kind: "signal", SourceID: f.Signal}
	if err := db.Create(&foreign).Error; err == nil {
		t.Fatal("schema allowed cross-workspace situation reference")
	}
}

func TestCRMSituationPendingActionTotalCountsSharedActionOnce(t *testing.T) {
	db := f.Open(t)
	repo := NewCRMSituationRepository(db)
	for _, motion := range []string{"conversion", "renewal"} {
		if _, _, err := repo.Create(context.Background(), f.Situation(motion),
			[]model.CRMSituationReference{{Kind: "suggestion", SourceID: f.Suggestion}}); err != nil {
			t.Fatal(err)
		}
	}
	filters := situationAllFilters()
	filters.PageSize = 1
	list, err := repo.List(context.Background(), f.Workspace, f.Sales, filters)
	if err != nil || list.Total != 2 || len(list.Data) != 1 || list.PendingActionTotal != 1 {
		t.Fatalf("approval total counted references or only one page: %#v %v", list, err)
	}
}
