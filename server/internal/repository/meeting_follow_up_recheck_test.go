package repository

import (
	"context"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"testing"
)

func TestMeetingFollowUpRecheckScopesCandidatesAndRevalidatesAccess(t *testing.T) {
	db, _ := pmAIFixture(t)
	repo := NewCRMSuggestionRepository(db)
	own := pmAISeed(t, db, nil, pmAIPtr(f.Sales), nil, "private")
	shared := pmAISeed(t, db, nil, pmAIPtr(f.Success), nil, "workspace")
	hidden := pmAISeed(t, db, nil, pmAIPtr(f.Success), nil, "private")
	for _, id := range []string{own, shared, hidden} {
		f.Exec(t, db, "UPDATE crm_suggestions SET context='{}' WHERE id=?", id)
	}
	rows, err := repo.MeetingFollowUpsToRecheck(context.Background(), f.Workspace, f.SalesUser, "")
	if err != nil || len(rows) != 2 {
		t.Fatalf("scoped candidates: %d %v", len(rows), err)
	}
	for _, row := range rows {
		if row.ID == hidden || !row.RoutingEligible {
			t.Fatalf("unexpected candidate %+v", row)
		}
	}
	rows, err = repo.MeetingFollowUpsToRecheck(context.Background(), f.ForeignWorkspace, f.SalesUser, "")
	if err != nil || len(rows) != 0 {
		t.Fatalf("foreign candidates: %d %v", len(rows), err)
	}
	var original model.CRMSuggestion
	if err := db.Where("id = ?", own).Take(&original).Error; err != nil {
		t.Fatal(err)
	}
	f.Exec(t, db, "UPDATE workspace_members SET status='inactive' WHERE id=?", f.Sales)
	changed, err := repo.RouteMeetingFollowUpForActor(context.Background(), original, "internal", f.SalesUser)
	if err != nil || changed {
		t.Fatalf("inactive actor wrote routing: %v %v", changed, err)
	}
	f.Exec(t, db, "UPDATE workspace_members SET status='active' WHERE id=?", f.Sales)
	changed, err = repo.RouteMeetingFollowUpForActor(context.Background(), original, "internal", f.SalesUser)
	if err != nil || !changed {
		t.Fatalf("valid routing not saved: %v %v", changed, err)
	}
	changed, err = repo.RouteMeetingFollowUpForActor(context.Background(), original, "customer", f.SalesUser)
	if err != nil || changed {
		t.Fatalf("stale routing overwrote classification: %v %v", changed, err)
	}
}

func TestMeetingFollowUpRecheckIncludesBlockedReasonsAndBoundsPages(t *testing.T) {
	db, _ := pmAIFixture(t)
	repo := NewCRMSuggestionRepository(db)
	for i := 0; i < 5; i++ {
		id := pmAISeed(t, db, nil, pmAIPtr(f.Sales), nil, "workspace")
		f.Exec(t, db, "UPDATE crm_suggestions SET context=? WHERE id=?", `{"task_id":"existing"}`, id)
	}
	rows, err := repo.MeetingFollowUpsToRecheck(context.Background(), f.Workspace, f.SalesUser, "")
	if err != nil || len(rows) != 4 {
		t.Fatalf("bounded page: %d %v", len(rows), err)
	}
	for _, r := range rows {
		if r.RoutingEligible {
			t.Fatal("linked work eligible")
		}
	}
	rest, err := repo.MeetingFollowUpsToRecheck(context.Background(), f.Workspace, f.SalesUser, rows[2].ID)
	if err != nil || len(rest) != 2 {
		t.Fatalf("cursor: %d %v", len(rest), err)
	}
}

func TestMeetingFollowUpRecheckDeletedCandidateIsChanged(t *testing.T) {
	db, _ := pmAIFixture(t)
	id := pmAISeed(t, db, nil, pmAIPtr(f.Sales), nil, "workspace")
	var original model.CRMSuggestion
	if err := db.Where("id=?", id).Take(&original).Error; err != nil {
		t.Fatal(err)
	}
	f.Exec(t, db, "DELETE FROM crm_suggestions WHERE id=?", id)
	changed, err := NewCRMSuggestionRepository(db).RouteMeetingFollowUpForActor(context.Background(), original, "internal", f.SalesUser)
	if err != nil || changed {
		t.Fatalf("deleted candidate: %v %v", changed, err)
	}
}
