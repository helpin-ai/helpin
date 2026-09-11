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
	f.Schema(t, db, "CREATE TABLE crm_pipelines (id uuid PRIMARY KEY,workspace_id uuid,default_commercial_motion text)")
	f.Exec(t, db, "INSERT INTO crm_pipelines VALUES (?,?,?)", pipeline, f.Workspace, "new_business")
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
	// Reclassifying an already-won deal must invalidate queued new-customer entry.
	f.Exec(t, db, "UPDATE crm_deals SET commercial_motion='renewal' WHERE id=?", f.Deal)
	candidates, err = store.AutomaticCandidates(ctx, f.Workspace, work[0].ID)
	if err != nil || len(candidates) != 0 {
		t.Fatalf("renewal qualified for handoff: %d %v", len(candidates), err)
	}
	f.Exec(t, db, "UPDATE crm_deals SET commercial_motion=NULL WHERE id=?", f.Deal)
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

func TestExistingBusinessWonDealSkipsNewCustomerHandoff(t *testing.T) {
	for _, tc := range []struct {
		name, defaultMotion, override string
		want                          int64
	}{
		{"existing default", "existing_business", "", 0}, {"renewal default", "renewal", "", 0},
		{"new override", "existing_business", "new_business", 1}, {"existing override", "new_business", "existing_business", 0},
		{"renewal override", "new_business", "renewal", 0}, {"expansion preserved", "expansion", "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, store, book, _, connection := executionFixture(t, 1)
			ctx, now := context.Background(), time.Now().UTC()
			f.Schema(t, db, "CREATE TABLE crm_pipelines (id uuid PRIMARY KEY,workspace_id uuid,default_commercial_motion text)")
			f.Schema(t, db, "CREATE TABLE crm_pipeline_stages (id uuid PRIMARY KEY,pipeline_id uuid,stage_type text)")
			f.Schema(t, db, "CREATE TABLE crm_associations (id uuid PRIMARY KEY,workspace_id uuid,from_object_type text,from_object_id uuid,to_object_type text,to_object_id uuid,association_label text,created_at datetime)")
			pipeline, open, won := uuid.NewString(), uuid.NewString(), uuid.NewString()
			f.Exec(t, db, "INSERT INTO crm_pipelines VALUES (?,?,?)", pipeline, f.Workspace, tc.defaultMotion)
			f.Exec(t, db, "INSERT INTO crm_pipeline_stages VALUES (?,?,'open'), (?,?,'won')", open, pipeline, won, pipeline)
			f.Exec(t, db, "UPDATE crm_deals SET pipeline_id=?,stage_id=?,commercial_motion=? WHERE id=?", pipeline, open, tc.override, f.Deal)
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
			if err := repository.NewCRMDealRepository(db).Update(ctx, &deal); err != nil {
				t.Fatal(err)
			}
			var count int64
			if err := db.Model(&model.CRMSituation{}).Where("origin_kind='deal_won'").Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != tc.want {
				t.Fatalf("handoffs=%d want=%d", count, tc.want)
			}
		})
	}
}
