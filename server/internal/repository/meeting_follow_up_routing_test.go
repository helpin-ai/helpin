package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestMeetingFollowUpRoutingPreservesConcurrentDecisionsAndDrafts(t *testing.T) {
	for _, change := range []string{"accepted", "dismissed", "edited", "unchanged"} {
		t.Run(change, func(t *testing.T) {
			db := f.Open(t)
			repo := NewCRMSuggestionRepository(db)
			when := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
			f.Exec(t, db, `UPDATE crm_suggestions SET suggestion_type='follow_up', object_type='meeting', object_id=?, context='{"draft_body":"Original draft","keep":"value"}', updated_at=?, created_at=? WHERE id=?`, uuid.NewString(), when, when, f.Suggestion)
			var original model.CRMSuggestion
			if err := db.Where("id=?", f.Suggestion).Take(&original).Error; err != nil {
				t.Fatal(err)
			}
			if change == "accepted" || change == "dismissed" {
				f.Exec(t, db, "UPDATE crm_suggestions SET status=? WHERE id=?", change, f.Suggestion)
			}
			if change == "edited" {
				f.Exec(t, db, "UPDATE crm_suggestions SET description='Edited draft' WHERE id=?", f.Suggestion)
			}
			if err := repo.RouteMeetingFollowUp(context.Background(), original, "internal"); err != nil {
				t.Fatal(err)
			}
			var after model.CRMSuggestion
			if err := db.Where("id=?", f.Suggestion).Take(&after).Error; err != nil {
				t.Fatal(err)
			}
			if change != "unchanged" && after.Context[model.MeetingFollowUpScopeKey] != nil {
				t.Fatalf("changed draft was reclassified: %+v", after)
			}
			if change == "unchanged" && after.Context[model.MeetingFollowUpScopeKey] != "internal" {
				t.Fatal("pending draft was not classified")
			}
			if after.Context["keep"] != "value" || !after.UpdatedAt.Equal(when) || !after.CreatedAt.Equal(when) || after.ExecutionStatus != original.ExecutionStatus {
				t.Fatalf("classification rewrote unrelated fields: %+v", after)
			}
		})
	}
}

func TestMeetingFollowUpCRMListsOnlyHideExplicitEligibleInternalDrafts(t *testing.T) {
	db := f.Open(t)
	f.InboxTables(t, db)
	f.Exec(t, db, "DELETE FROM crm_suggestions")
	ws, object := f.Workspace, "meeting"
	for _, tc := range []struct{ scope, kind string }{{"internal", "follow_up"}, {"customer", "follow_up"}, {"uncertain", "follow_up"}, {"", "follow_up"}, {"internal", "risk_alert"}} {
		suggestion := model.CRMSuggestion{ID: uuid.NewString(), WorkspaceID: ws, ObjectType: &object, ObjectID: f.Ptr(uuid.NewString()), SuggestionType: tc.kind, Title: tc.scope + tc.kind, Status: "pending", Context: model.JSONB{model.MeetingFollowUpScopeKey: tc.scope}, SignalIDs: model.StringArray{}}
		if err := db.Create(&suggestion).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewCRMSuggestionRepository(db)
	items, total, err := repo.List(context.Background(), ws, model.CRMSuggestionListFilters{}, model.PMPagination{Page: 1, PerPage: 25})
	if err != nil || total != 4 || len(items) != 4 {
		t.Fatalf("CRM list: total=%d err=%v", total, err)
	}
	_, canonicalTotal, err := repo.List(context.Background(), ws, model.CRMSuggestionListFilters{IncludeInternalMeetingFollowUps: true}, model.PMPagination{Page: 1, PerPage: 25})
	if err != nil || canonicalTotal != 5 {
		t.Fatalf("canonical dedup list: total=%d err=%v", canonicalTotal, err)
	}
	inbox, err := NewCRMSituationRepository(db).ListInbox(context.Background(), ws, f.Sales, inboxFilters())
	if err != nil || inbox.Total != 4 {
		t.Fatalf("CRM inbox=%+v err=%v", inbox, err)
	}
}

func TestMeetingFollowUpBackfillOnlySelectsPendingUnclassifiedAndBounded(t *testing.T) {
	db := f.Open(t)
	f.Exec(t, db, "DELETE FROM crm_suggestions")
	object := "meeting"
	for i := 0; i < 6; i++ {
		item := model.CRMSuggestion{ID: uuid.NewString(), WorkspaceID: f.Workspace, ObjectType: &object, ObjectID: f.Ptr(uuid.NewString()), SuggestionType: "follow_up", Title: "Follow-up", Status: "pending", Context: model.JSONB{}, SignalIDs: model.StringArray{}}
		if i == 0 {
			item.Status = "accepted"
		}
		if i == 1 {
			item.Status = "dismissed"
		}
		if i == 2 {
			item.Context[model.MeetingFollowUpRoutingVersionKey] = model.MeetingFollowUpRoutingVersion
		}
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
	}
	items, err := NewCRMSuggestionRepository(db).MeetingFollowUpsToRoute(context.Background(), 200)
	if err != nil || len(items) != 3 {
		t.Fatalf("batch=%+v err=%v", items, err)
	}
	for _, item := range items {
		if item.Status != "pending" || item.Context[model.MeetingFollowUpRoutingVersionKey] != nil {
			t.Fatalf("unsafe candidate: %+v", item)
		}
	}
}

func TestMeetingFollowUpCRMRetainsAdoptedWorkAndMovesPristineProjection(t *testing.T) {
	db := f.Open(t)
	f.InboxTables(t, db)
	ws := f.Workspace
	f.Exec(t, db, `UPDATE crm_suggestions SET suggestion_type='follow_up', object_type='meeting', object_id=?, context='{}' WHERE id=?`, uuid.NewString(), f.Suggestion)
	repo := NewCRMSituationRepository(db)
	input := model.CRMSituationSourceInput{Kind: "suggestion", SourceID: f.Suggestion, Situation: model.CRMSituation{WorkspaceID: ws, Title: "Team follow-up", Objective: "Review", CommercialMotion: "needs_context", Lifecycle: "open", Attention: "needs_context"}}
	if err := repo.ImportSource(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	f.Exec(t, db, `UPDATE crm_suggestions SET context='{"meeting_follow_up_scope":"internal"}' WHERE id=?`, f.Suggestion)
	list, err := repo.ListInbox(context.Background(), ws, f.Sales, inboxFilters())
	if err != nil || list.Total != 0 {
		t.Fatalf("pristine projection still in CRM: %+v %v", list, err)
	}
	f.Exec(t, db, `UPDATE crm_situations SET revision=2 WHERE creation_key=?`, "source:suggestion:"+f.Suggestion)
	list, err = repo.ListInbox(context.Background(), ws, f.Sales, inboxFilters())
	if err != nil || list.Total != 1 {
		t.Fatalf("adopted customer work disappeared: %+v %v", list, err)
	}
}

func TestMeetingFollowUpInternalProjectionIsNotRecreated(t *testing.T) {
	db := f.Open(t)
	f.Exec(t, db, `UPDATE crm_suggestions SET suggestion_type='follow_up', object_type='meeting', object_id=?, context='{"meeting_follow_up_scope":"internal"}' WHERE id=?`, uuid.NewString(), f.Suggestion)
	input := model.CRMSituationSourceInput{Kind: "suggestion", SourceID: f.Suggestion, Situation: model.CRMSituation{WorkspaceID: f.Workspace, Title: "Team follow-up", Objective: "Review", CommercialMotion: "needs_context", Lifecycle: "open", Attention: "needs_context"}}
	for range 2 {
		if err := NewCRMSituationRepository(db).ImportSource(context.Background(), input); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Model(&model.CRMSituation{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("internal follow-up was projected back into CRM")
	}
}

func TestMeetingFollowUpReviewedInMyWorkCannotResurrectAfterOwnerLeaves(t *testing.T) {
	db := f.Open(t)
	f.Exec(t, db, `CREATE TABLE crm_meetings (id TEXT PRIMARY KEY, workspace_id TEXT, owner_member_id TEXT, created_by TEXT, visibility TEXT)`)
	meeting := uuid.NewString()
	f.Exec(t, db, `INSERT INTO crm_meetings VALUES (?, ?, ?, ?, 'workspace')`, meeting, f.Workspace, f.Sales, f.SalesUser)
	f.Exec(t, db, `UPDATE crm_suggestions SET suggestion_type='follow_up', object_type='meeting', object_id=?, context='{"meeting_follow_up_scope":"internal","meeting_follow_up_reviewed_in":"my_work"}', status='accepted', execution_status='manual_required' WHERE id=?`, meeting, f.Suggestion)
	f.Exec(t, db, "UPDATE workspace_members SET status='inactive' WHERE id=?", f.Sales)
	input := model.CRMSituationSourceInput{Kind: "suggestion", SourceID: f.Suggestion, Situation: model.CRMSituation{WorkspaceID: f.Workspace, Title: "Team follow-up", Objective: "Review", CommercialMotion: "needs_context", Lifecycle: "open", Attention: "needs_context"}}
	if err := NewCRMSituationRepository(db).ImportSource(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.CRMSituation{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("completed My Work review reappeared in CRM after owner left")
	}
	_, total, err := NewCRMSuggestionRepository(db).List(context.Background(), f.Workspace, model.CRMSuggestionListFilters{}, model.PMPagination{Page: 1, PerPage: 25})
	if err != nil || total != 0 {
		t.Fatalf("reviewed suggestion reappeared in CRM: %d %v", total, err)
	}
}

func TestMeetingFollowUpBackfillRevisitsOlderClassificationPolicy(t *testing.T) {
	db := f.Open(t)
	f.Exec(t, db, "DELETE FROM crm_suggestions")
	object := "meeting"
	for _, scope := range []string{"customer", "uncertain"} {
		item := model.CRMSuggestion{ID: uuid.NewString(), WorkspaceID: f.Workspace, ObjectType: &object, ObjectID: f.Ptr(uuid.NewString()), SuggestionType: "follow_up", Title: "Team recap", Status: "pending", Context: model.JSONB{model.MeetingFollowUpRoutingVersionKey: "v1", model.MeetingFollowUpScopeKey: scope}, SignalIDs: model.StringArray{}}
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewCRMSuggestionRepository(db)
	items, err := repo.MeetingFollowUpsToRoute(context.Background(), 3)
	if err != nil || len(items) != 2 {
		t.Fatalf("old classifications were not automatically revisited: %d %v", len(items), err)
	}
	for _, item := range items {
		if err := repo.RouteMeetingFollowUp(context.Background(), item, "internal"); err != nil {
			t.Fatal(err)
		}
	}
	items, err = repo.MeetingFollowUpsToRoute(context.Background(), 3)
	if err != nil || len(items) != 0 {
		t.Fatalf("current policy was repeatedly classified: %d %v", len(items), err)
	}
}
