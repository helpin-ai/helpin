package repository

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMSituationLifecycleAuditFailureRollsBackState(t *testing.T) {
	db := f.Open(t)
	repo := NewCRMSituationRepository(db)
	stored, _, err := repo.Create(context.Background(), f.Situation("conversion"), nil)
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected history failure")
	const callback = "test:reject_situation_history"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "crm_situation_changes" {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Callback().Create().Remove(callback); err != nil {
			t.Error(err)
		}
	})
	req := model.CRMSituationCommandRequest{CommandKey: uuid.NewString(), ExpectedRevision: 1, Operation: "pause", Reason: "Hold"}
	_, err = repo.ApplyCommand(context.Background(), f.Workspace, stored.ID, f.Sales, req, "fingerprint", pauseWorkState)
	if !errors.Is(err, injected) {
		t.Fatalf("history error not returned: %v", err)
	}
	item, err := repo.GetByID(context.Background(), f.Workspace, stored.ID)
	if err != nil || item.Situation.Revision != 1 || item.Situation.Lifecycle != "open" {
		t.Fatalf("state committed without audit: %#v %v", item, err)
	}
}

func TestCRMSituationLifecycleConcurrentRetryHasOneReceipt(t *testing.T) {
	db := f.Open(t)
	repo := NewCRMSituationRepository(db)
	stored, _, err := repo.Create(context.Background(), f.Situation("conversion"), nil)
	if err != nil {
		t.Fatal(err)
	}
	req := model.CRMSituationCommandRequest{CommandKey: uuid.NewString(), ExpectedRevision: 1, Operation: "pause", Reason: "Hold"}
	var workers sync.WaitGroup
	results := make([]*model.CRMSituationCommandResult, 8)
	errs := make([]error, len(results))
	for i := range results {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			results[index], errs[index] = repo.ApplyCommand(context.Background(), f.Workspace, stored.ID, f.Sales, req, "fingerprint", pauseWorkState)
		}(i)
	}
	workers.Wait()
	commits := 0
	for i, result := range results {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if !result.Replayed {
			commits++
		}
		if result.Change.ID != results[0].Change.ID || result.Change.Revision != 2 {
			t.Fatal("retries returned different receipts")
		}
	}
	if commits != 1 {
		t.Fatalf("committed receipts = %d", commits)
	}
}

func TestCRMSituationLifecyclePauseSerializesActionAdmission(t *testing.T) {
	db := f.Open(t)
	repo := NewCRMSituationRepository(db)
	actions := NewCRMSuggestionRepository(db)
	for i := 0; i < 12; i++ {
		action := model.CRMSuggestion{ID: uuid.NewString(), WorkspaceID: f.Workspace, Status: "pending", SuggestionType: "deal_advance", ExecutionStatus: "pending"}
		if err := db.Create(&action).Error; err != nil {
			t.Fatal(err)
		}
		stored, _, err := repo.Create(context.Background(), f.Situation("conversion"), []model.CRMSituationReference{{Kind: "suggestion", SourceID: action.ID}})
		if err != nil {
			t.Fatal(err)
		}
		// Fetch canonical timestamp precision, as the service does before approval.
		proposal, err := actions.GetByID(context.Background(), f.Workspace, action.ID)
		if err != nil {
			t.Fatal(err)
		}
		var workers sync.WaitGroup
		var paused *model.CRMSituationCommandResult
		var pauseErr, claimErr error
		var claimed bool
		workers.Add(2)
		go func() {
			defer workers.Done()
			req := model.CRMSituationCommandRequest{CommandKey: uuid.NewString(), ExpectedRevision: 1, Operation: "pause", Reason: "Hold"}
			paused, pauseErr = repo.ApplyCommand(context.Background(), f.Workspace, stored.ID, f.Sales, req, "fingerprint", pauseWorkState)
		}()
		go func() {
			defer workers.Done()
			claimed, claimErr = actions.ClaimPending(context.Background(), f.Workspace, proposal)
		}()
		workers.Wait()
		if pauseErr != nil || claimErr != nil {
			t.Fatalf("pause=%v claim=%v", pauseErr, claimErr)
		}
		if claimed != (paused.Change.InFlightActionCount == 1) {
			t.Fatalf("action admitted after pause or hidden in-flight action: claimed=%v receipt=%#v", claimed, paused)
		}
		if again, err := actions.ClaimPending(context.Background(), f.Workspace, proposal); err != nil || again {
			t.Fatalf("new admission after pause: claimed=%v err=%v", again, err)
		}
	}
}

func TestCRMSituationLifecycleSourceCreationAttribution(t *testing.T) {
	db := f.Open(t)
	repo := NewCRMSituationRepository(db)
	for _, source := range []struct{ kind, id string }{{"signal", f.Signal}, {"suggestion", f.Suggestion}} {
		input := model.CRMSituationSourceInput{Kind: source.kind, SourceID: source.id, Situation: f.Situation("conversion")}
		for i := 0; i < 2; i++ {
			if err := repo.ImportSource(context.Background(), input); err != nil {
				t.Fatal(err)
			}
		}
		var link model.CRMSituationSourceLink
		if err := db.Where("kind = ? AND source_id = ?", source.kind, source.id).Take(&link).Error; err != nil {
			t.Fatal(err)
		}
		history, err := repo.History(context.Background(), f.Workspace, link.SituationID, 0, 10)
		if err != nil || len(history.Data) != 1 || history.Data[0].ActorKind != source.kind || history.Data[0].ActorMemberID != nil {
			t.Fatalf("source creation invented human attribution or duplicate history: %#v %v", history, err)
		}
	}
}

func pauseWorkState(current model.CRMSituation) (model.CRMSituationWorkState, error) {
	state := model.CRMSituationState(current)
	state.Lifecycle = "paused"
	return state, nil
}
