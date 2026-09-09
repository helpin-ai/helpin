package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

func pmAIFixture(t *testing.T) (*gorm.DB, *PMAISuggestionRepository) {
	t.Helper()
	db := f.Open(t)
	f.Exec(t, db, `CREATE TABLE crm_meetings(id uuid PRIMARY KEY,workspace_id uuid,owner_member_id uuid,created_by uuid,title text,visibility text,actual_start_at datetime,scheduled_start_at datetime,created_at datetime)`)
	return db, NewPMAISuggestionRepository(db)
}
func pmAISeed(t *testing.T, db *gorm.DB, user, owner, creator *string, visibility string) string {
	t.Helper()
	id, meetingID := uuid.NewString(), uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	f.Exec(t, db, `INSERT INTO crm_meetings(id,workspace_id,owner_member_id,created_by,title,visibility,created_at) VALUES(?,?,?,?,?,?,?)`, meetingID, f.Workspace, owner, creator, "Weekly planning", visibility, now)
	objectType := "meeting"
	suggestion := model.CRMSuggestion{ID: id, WorkspaceID: f.Workspace, UserID: user, SuggestionType: "follow_up", ObjectType: &objectType, ObjectID: &meetingID, Title: "Share next steps", Context: model.JSONB{"meeting_follow_up_scope": "internal", "draft_subject": "Planning follow-up", "draft_body": "Share the design notes."}, SignalIDs: model.StringArray{}, Status: "pending", ExecutionStatus: "pending", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&suggestion).Error; err != nil {
		t.Fatal(err)
	}
	return id
}
func pmAIPtr(s string) *string { return &s }

func TestPMAISuggestionsPersonalScopeAndPrivateMeeting(t *testing.T) {
	db, repo := pmAIFixture(t)
	ctx := context.Background()
	assigned := pmAISeed(t, db, pmAIPtr(f.SalesUser), pmAIPtr(f.Success), nil, "workspace")
	owned := pmAISeed(t, db, nil, pmAIPtr(f.Sales), nil, "workspace")
	created := pmAISeed(t, db, nil, nil, pmAIPtr(f.SalesUser), "workspace")
	privateOwned := pmAISeed(t, db, nil, pmAIPtr(f.Sales), nil, "private")
	hidden := []string{
		pmAISeed(t, db, pmAIPtr(f.ForeignUser), pmAIPtr(f.Sales), nil, "workspace"),
		pmAISeed(t, db, nil, pmAIPtr(f.Success), pmAIPtr(f.SalesUser), "workspace"),
		pmAISeed(t, db, nil, nil, nil, "workspace"),
		pmAISeed(t, db, pmAIPtr(f.SalesUser), pmAIPtr(f.Success), nil, "private"),
		pmAISeed(t, db, pmAIPtr(f.SalesUser), pmAIPtr(f.Success), nil, "participants"),
	}
	for _, id := range []string{assigned, owned, created, privateOwned} {
		item, err := repo.Get(ctx, f.Workspace, f.SalesUser, id)
		if err != nil || item == nil || item.DraftBody != "Share the design notes." {
			t.Fatalf("expected personal %s: %#v %v", id, item, err)
		}
	}
	for _, id := range hidden {
		item, err := repo.Get(ctx, f.Workspace, f.SalesUser, id)
		if err != nil || item != nil {
			t.Fatalf("leaked hidden %s: %#v %v", id, item, err)
		}
		result, err := repo.Decide(ctx, f.Workspace, f.SalesUser, id, "forged", "accept")
		if err != nil || result != nil {
			t.Fatalf("hidden mutation: %#v %v", result, err)
		}
	}
	items, total, err := repo.List(ctx, f.Workspace, f.SalesUser, 1)
	if err != nil || len(items) != 4 || total != 4 {
		t.Fatalf("personal list %d %d %v", len(items), total, err)
	}
	item, err := repo.Get(ctx, f.ForeignWorkspace, f.SalesUser, assigned)
	if err != nil || item != nil {
		t.Fatalf("foreign workspace leaked: %#v %v", item, err)
	}
	f.Exec(t, db, "UPDATE workspace_members SET status='inactive' WHERE id=?", f.Sales)
	items, total, err = repo.List(ctx, f.Workspace, f.SalesUser, 1)
	if err != nil || len(items) != 0 || total != 0 {
		t.Fatalf("inactive list: %#v %d %v", items, total, err)
	}
	item, err = repo.Get(ctx, f.Workspace, f.SalesUser, assigned)
	if err != nil || item != nil {
		t.Fatalf("inactive detail: %#v %v", item, err)
	}
	result, err := repo.Decide(ctx, f.Workspace, f.SalesUser, assigned, "forged", "dismiss")
	if err != nil || result != nil {
		t.Fatalf("inactive mutation: %#v %v", result, err)
	}
}

func TestPMAISuggestionsPaginationAndRouting(t *testing.T) {
	db, repo := pmAIFixture(t)
	ctx := context.Background()
	for i := 0; i < 27; i++ {
		pmAISeed(t, db, nil, pmAIPtr(f.Sales), nil, "workspace")
	}
	for _, payload := range []string{`{"meeting_follow_up_scope":"customer"}`, `{"meeting_follow_up_scope":"uncertain"}`, `{}`, `{"meeting_follow_up_scope":"internal","task_id":"existing-task"}`} {
		id := pmAISeed(t, db, nil, pmAIPtr(f.Sales), nil, "workspace")
		f.Exec(t, db, "UPDATE crm_suggestions SET context=? WHERE id=?", payload, id)
	}
	first, total, err := repo.List(ctx, f.Workspace, f.SalesUser, 1)
	if err != nil || len(first) != 25 || total != 27 {
		t.Fatalf("first page %d %d %v", len(first), total, err)
	}
	second, total, err := repo.List(ctx, f.Workspace, f.SalesUser, 2)
	if err != nil || len(second) != 2 || total != 27 {
		t.Fatalf("second page %d %d %v", len(second), total, err)
	}
	seen := map[string]bool{}
	for _, item := range first {
		seen[item.ID] = true
	}
	for _, item := range second {
		if seen[item.ID] {
			t.Fatal("duplicate across pages")
		}
	}
}

func TestPMAISuggestionsRevisionAndDecision(t *testing.T) {
	db, repo := pmAIFixture(t)
	ctx := context.Background()
	for _, decision := range []string{"accept", "dismiss"} {
		t.Run(decision, func(t *testing.T) {
			id := pmAISeed(t, db, nil, pmAIPtr(f.Sales), nil, "workspace")
			item, err := repo.Get(ctx, f.Workspace, f.SalesUser, id)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Decide(ctx, f.Workspace, f.SalesUser, id, "stale", decision); !errors.Is(err, ErrPMAISuggestionStale) {
				t.Fatalf("stale decision: %v", err)
			}
			result, err := repo.Decide(ctx, f.Workspace, f.SalesUser, id, item.Revision, decision)
			if err != nil || result == nil {
				t.Fatalf("decision %#v %v", result, err)
			}
			var canonical model.CRMSuggestion
			if err := db.First(&canonical, "id = ?", id).Error; err != nil {
				t.Fatal(err)
			}
			if canonical.Context["meeting_follow_up_reviewed_in"] != "my_work" {
				t.Fatal("missing durable PM review marker")
			}
			if canonical.ExecutedAt != nil {
				t.Fatal("review executed action")
			}
			if decision == "accept" && (canonical.Status != "accepted" || canonical.ExecutionStatus != "manual_required") {
				t.Fatalf("wrong accept %#v", canonical)
			}
			if decision == "dismiss" && (canonical.Status != "dismissed" || canonical.DismissalReason == nil || *canonical.DismissalReason != "not_relevant") {
				t.Fatalf("wrong dismiss %#v", canonical)
			}
			if _, err := repo.Decide(ctx, f.Workspace, f.SalesUser, id, item.Revision, decision); !errors.Is(err, ErrPMAISuggestionStale) {
				t.Fatalf("repeat: %v", err)
			}
			item, err = repo.Get(ctx, f.Workspace, f.SalesUser, id)
			if err != nil || item != nil {
				t.Fatalf("resolved remains pending %#v %v", item, err)
			}
		})
	}
}

func TestPMAISuggestionsProjectionHistoryAndGuards(t *testing.T) {
	db, repo := pmAIFixture(t)
	ctx := context.Background()
	id := pmAISeed(t, db, nil, pmAIPtr(f.Sales), nil, "workspace")
	situation := f.Situation("needs_context")
	situation.CompanyID = nil
	situation.ContactID = nil
	situation.DealID = nil
	situation.CreatedByMemberID = nil
	situation.OriginKind = "suggestion"
	situation.CreationKey = "source:suggestion:" + id
	situation.CreationFingerprint = situation.CreationKey
	stored, _, err := NewCRMSituationRepository(db).Create(ctx, situation, []model.CRMSituationReference{{Kind: "suggestion", SourceID: id}})
	if err != nil {
		t.Fatal(err)
	}
	item, err := repo.Get(ctx, f.Workspace, f.SalesUser, id)
	if err != nil || item == nil {
		t.Fatalf("pristine projection hidden %#v %v", item, err)
	}
	f.Exec(t, db, "UPDATE crm_situations SET revision=2 WHERE id=?", stored.ID)
	if _, err := repo.Decide(ctx, f.Workspace, f.SalesUser, id, item.Revision, "accept"); !errors.Is(err, ErrPMAISuggestionStale) {
		t.Fatalf("adopted projection not guarded %v", err)
	}
	f.Exec(t, db, "UPDATE crm_situations SET revision=1 WHERE id=?", stored.ID)
	if _, err := repo.Decide(ctx, f.Workspace, f.SalesUser, id, item.Revision, "accept"); err != nil {
		t.Fatal(err)
	}
	var after model.CRMSituation
	if err := db.First(&after, "id=?", stored.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Lifecycle != "closed" || after.Revision != 2 || after.OutcomeSummary == nil {
		t.Fatalf("projection did not close %#v", after)
	}
	var count int64
	if err := db.Model(&model.CRMSituationReference{}).Where("source_id=?", id).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("lost source history")
	}
	var change model.CRMSituationChange
	if err := db.Where("situation_id=? AND revision=2", stored.ID).Take(&change).Error; err != nil {
		t.Fatal(err)
	}
	if change.ActorMemberID == nil || *change.ActorMemberID != f.Sales || change.Operation != "close" {
		t.Fatalf("missing human receipt %#v", change)
	}
	if _, err := repo.Decide(ctx, f.Workspace, f.SalesUser, id, item.Revision, "accept"); !errors.Is(err, ErrPMAISuggestionStale) {
		t.Fatalf("projection repeat %v", err)
	}
}
