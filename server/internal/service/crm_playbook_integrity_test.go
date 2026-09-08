package service

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMPlaybookPublicationRollback(t *testing.T) {
	db, svc, ctx := playbookFixture(t)
	pb := readyPlaybook(t, svc, ctx, 0)
	injected := errors.New("injected receipt storage failure")
	const callback = "test:reject_playbook_receipt"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "crm_playbook_changes" {
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
	_, err := svc.Command(ctx, f.Workspace, pb.ID, model.CRMPlaybookCommandRequest{CommandKey: uuid.NewString(), ExpectedRevision: pb.Revision, Operation: "publish"})
	if !errors.Is(err, injected) {
		t.Fatalf("failure not returned: %v", err)
	}
	item, err := svc.Get(ctx, f.Workspace, pb.ID)
	if err != nil || item.Playbook.Revision != pb.Revision || *item.Playbook.PublishedVersionID != *pb.PublishedVersionID {
		t.Fatalf("partial publication committed: %#v %v", item, err)
	}
	versions, err := svc.Versions(ctx, f.Workspace, pb.ID, 0, 100)
	if err != nil || len(versions.Data) != 1 {
		t.Fatalf("orphaned published version: %#v %v", versions, err)
	}
}

func TestCRMPlaybookParticipationRollback(t *testing.T) {
	db, svc, ctx := playbookFixture(t)
	pb := readyPlaybook(t, svc, ctx, 0)
	work := playbookWork(t, db, "conversion")
	injected := errors.New("injected Signal history failure")
	const callback = "test:reject_participant_receipt"
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
	if _, err := svc.Apply(ctx, f.Workspace, pb.ID, playbookApplication(pb, work)); !errors.Is(err, injected) {
		t.Fatalf("failure not returned: %v", err)
	}
	item, err := svc.situations.GetByID(ctx, f.Workspace, work.ID)
	if err != nil || item.Situation.PlaybookID != nil || item.Situation.Revision != 1 {
		t.Fatalf("partial enrollment committed: %#v %v", item, err)
	}
}

func TestCRMPlaybookQueryEligibilityUsesCanonicalReferences(t *testing.T) {
	db, svc, ctx := playbookFixture(t)
	definition := playbookDefinition(0)
	definition.Eligibility.Filter = &model.QueryFilterGroup{Logic: "and", Rules: []model.QueryFilterRule{
		{Field: "company_domain", Operator: "contains", Value: f.Ptr("northstar")},
		{Field: "owner_member_id", Operator: "is", Value: f.Ptr(f.Sales)},
	}}
	pb, _, err := svc.Create(ctx, f.Workspace, model.CreateCRMPlaybookRequest{CreationKey: uuid.NewString(), Definition: definition})
	if err != nil {
		t.Fatal(err)
	}
	work := playbookWork(t, db, "conversion")
	preview, err := svc.Preview(ctx, f.Workspace, pb.ID, "", 1, 1, 25)
	if err != nil || preview.Signals.Total != 1 {
		t.Fatalf("shared filter did not match: %#v %v", preview, err)
	}
	published := mustPlaybookCommand(t, svc, ctx, *pb, "publish", nil, nil)
	yes := true
	ready := mustPlaybookCommand(t, svc, ctx, published, "set_enrollment", nil, &yes)
	// A customer edit between preview and apply must be rechecked, even though
	// neither the Signal revision nor the Playbook revision changed.
	f.Exec(t, db, "UPDATE crm_companies SET domain = 'changed.example' WHERE id = ?", f.Company)
	if _, err := svc.Apply(ctx, f.Workspace, pb.ID, playbookApplication(ready, work)); !errors.Is(err, repository.ErrCRMPlaybookUnavailable) {
		t.Fatalf("stale preview authorized enrollment: %v", err)
	}
	f.Exec(t, db, "UPDATE crm_companies SET domain = 'northstar.example' WHERE id = ?", f.Company)
	f.Exec(t, db, "UPDATE workspace_members SET status = 'inactive' WHERE id = ?", f.Success)
	if _, err := svc.Apply(ctx, f.Workspace, pb.ID, playbookApplication(ready, work)); !errors.Is(err, repository.ErrCRMSituationInvalidReference) {
		t.Fatalf("inactive escalation accepted: %v", err)
	}
}

func TestCRMPlaybookPostgresCompositeVersionIntegrity(t *testing.T) {
	db, svc, ctx := playbookFixture(t)
	if db.Dialector.Name() != "postgres" {
		t.Skip("requires disposable PostgreSQL")
	}
	first, second := readyPlaybook(t, svc, ctx, 0), readyPlaybook(t, svc, ctx, 0)
	work := playbookWork(t, db, "conversion")
	if _, err := svc.Apply(ctx, f.Workspace, first.ID, playbookApplication(first, work)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, query string
		args        []any
	}{
		{"foreign definition version", "UPDATE crm_playbooks SET published_version_id = ? WHERE id = ?", []any{*second.PublishedVersionID, first.ID}},
		{"mismatched Signal policy version", "UPDATE crm_situations SET playbook_version_id = ? WHERE id = ?", []any{*second.PublishedVersionID, work.ID}},
		{"partial binding", "UPDATE crm_situations SET playbook_version_id = NULL WHERE id = ?", []any{work.ID}},
		{"malformed progress", "UPDATE crm_situations SET playbook_milestones = '{}'::jsonb WHERE id = ?", []any{work.ID}},
		{"delete pinned version", "DELETE FROM crm_playbook_versions WHERE id = ?", []any{*first.PublishedVersionID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.Exec(tc.query, tc.args...).Error; err == nil {
				t.Fatal("schema accepted invalid identity or lost pinned history")
			}
		})
	}
}

func TestCRMPlaybookMilestoneCompletionIsNotCustomerClosure(t *testing.T) {
	db, svc, ctx := playbookFixture(t)
	pb := readyPlaybook(t, svc, ctx, 0)
	work := playbookWork(t, db, "conversion")
	if _, err := svc.Apply(ctx, f.Workspace, pb.ID, playbookApplication(pb, work)); err != nil {
		t.Fatal(err)
	}
	revision := int64(2)
	for _, milestone := range pb.Draft.Milestones {
		_, err := svc.AssessMilestone(ctx, f.Workspace, pb.ID, work.ID, model.CRMPlaybookMilestoneRequest{
			CommandKey: uuid.NewString(), ExpectedRevision: revision, MilestoneKey: milestone.Key, Status: "achieved", Summary: "Customer confirmed the milestone",
		})
		if err != nil {
			t.Fatal(err)
		}
		revision++
	}
	item, err := svc.situations.GetByID(ctx, f.Workspace, work.ID)
	if err != nil || item.Situation.Lifecycle != "open" || item.Situation.OutcomeKind != nil {
		t.Fatalf("milestones silently closed objective: %#v %v", item, err)
	}
	closing := workCommand("close", revision)
	closing.Outcome = &model.CRMSituationOutcome{Kind: "achieved", Summary: "The customer objective is confirmed"}
	mustWorkCommand(t, svc.situations, ctx, work.ID, closing)
	item, err = svc.situations.GetByID(ctx, f.Workspace, work.ID)
	if err != nil || item.Situation.PlaybookVersionID == nil || len(item.Situation.PlaybookMilestones) != 2 || item.Situation.Lifecycle != "closed" {
		t.Fatalf("closure lost policy or progress: %#v %v", item, err)
	}
}

func TestCRMPlaybookPostgresWorkspaceRemovalDoesNotLeavePinnedOrphans(t *testing.T) {
	db, svc, ctx := playbookFixture(t)
	if db.Dialector.Name() != "postgres" {
		t.Skip("requires disposable PostgreSQL")
	}
	pb := readyPlaybook(t, svc, ctx, 0)
	work := playbookWork(t, db, "conversion")
	if _, err := svc.Apply(ctx, f.Workspace, pb.ID, playbookApplication(pb, work)); err != nil {
		t.Fatal(err)
	}
	// Generated fixture data only: verify circular publication references do not
	// break the workspace's existing canonical cascade behavior.
	f.Exec(t, db, "DELETE FROM workspaces WHERE id = ?", f.Workspace)
	for _, table := range []string{"crm_playbooks", "crm_playbook_versions", "crm_playbook_changes", "crm_situations"} {
		var count int64
		if err := db.Table(table).Where("workspace_id = ?", f.Workspace).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("orphaned %s: count=%d err=%v", table, count, err)
		}
	}
}
