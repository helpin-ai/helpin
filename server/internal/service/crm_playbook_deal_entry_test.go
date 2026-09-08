package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMPlaybookWonDealUsesSharedEntryAndKeepsSalesAccountable(t *testing.T) {
	db, store, book, _, connection := executionFixture(t, 1)
	ctx, now := context.Background(), time.Now().UTC()
	f.Exec(t, db, "CREATE TABLE crm_pipeline_stages (id uuid PRIMARY KEY, pipeline_id uuid, stage_type text)")
	f.Schema(t, db, "CREATE TABLE crm_associations (id uuid PRIMARY KEY, workspace_id uuid, from_object_type text, from_object_id uuid, to_object_type text, to_object_id uuid, association_label text, created_at datetime)")
	pipeline, open, won := uuid.NewString(), uuid.NewString(), uuid.NewString()
	f.Exec(t, db, "INSERT INTO crm_pipeline_stages VALUES (?,?,'open'), (?,?,'won')", open, pipeline, won, pipeline)
	f.Exec(t, db, "UPDATE crm_deals SET pipeline_id=?,stage_id=? WHERE id=?", pipeline, open, f.Deal)
	f.Exec(t, db, "INSERT INTO crm_associations VALUES (?,?,'deal',?,'company',?,'deal_customer',?)", uuid.NewString(), f.Workspace, f.Deal, f.Company, now)
	settings := executionSettings(connection.ID)
	settings.EntryMode = "automatic"
	if _, err := store.Configure(ctx, f.Workspace, book.ID, f.Sales, settings, "enable", now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	var deal model.CRMDeal
	if err := db.Where("id=?", f.Deal).Take(&deal).Error; err != nil {
		t.Fatal(err)
	}
	deal.StageID = won
	deals := repository.NewCRMDealRepository(db)
	if err := deals.Update(ctx, &deal); err != nil {
		t.Fatal(err)
	}
	if err := deals.Update(ctx, &deal); err != nil {
		t.Fatal(err)
	}
	var work []model.CRMSituation
	if err := db.Where("origin_kind='deal_won'").Find(&work).Error; err != nil || len(work) != 1 {
		t.Fatalf("duplicate/missing handoff: %d %v", len(work), err)
	}
	if work[0].OwnerMemberID == nil || *work[0].OwnerMemberID != f.Sales || work[0].PlaybookID != nil {
		t.Fatal("win transferred ownership or bypassed entry")
	}
	event, err := repository.NewAutomationScheduledEventRepository(db).ClaimNext(ctx, now.Add(time.Minute), time.Minute)
	if err != nil || event == nil || event.Kind != model.CRMPlaybookEntryDue {
		t.Fatalf("no durable shared entry: %#v %v", event, err)
	}
	candidates, err := store.AutomaticCandidates(ctx, f.Workspace, work[0].ID)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("handoff did not qualify: %d %v", len(candidates), err)
	}
	// Reopening before dispatch invalidates eligibility; winning again must not
	// duplicate the already-open customer objective.
	deal.StageID = open
	if err := deals.Update(ctx, &deal); err != nil {
		t.Fatal(err)
	}
	candidates, err = store.AutomaticCandidates(ctx, f.Workspace, work[0].ID)
	if err != nil || len(candidates) != 0 {
		t.Fatalf("reopened deal qualified: %d %v", len(candidates), err)
	}
	deal.StageID = won
	if err := deals.Update(ctx, &deal); err != nil {
		t.Fatal(err)
	}
	candidates, err = store.AutomaticCandidates(ctx, f.Workspace, work[0].ID)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("rewon deal: %d %v", len(candidates), err)
	}
	if err := store.EnrollAutomatically(ctx, *event, &candidates[0], f.Sales, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&model.CRMSituation{}).Where("origin_kind='deal_won'").Count(&count)
	if count != 1 {
		t.Fatal("reopened deal duplicated handoff")
	}
}
