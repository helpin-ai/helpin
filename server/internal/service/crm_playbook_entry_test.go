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

func TestCRMPlaybookAutomaticEntryOnlyNewMatchingEvidence(t *testing.T) {
	for index, motion := range []string{"conversion", "onboarding", "retention"} {
		for _, historical := range []bool{false, true} {
			t.Run(motion+map[bool]string{true: " historical", false: " new"}[historical], func(t *testing.T) {
				db, store, book, existing, connection := executionFixture(t, index)
				ctx, now := context.Background(), time.Now().UTC().Truncate(time.Microsecond)
				settings := executionSettings(connection.ID)
				settings.EntryMode = "automatic"
				if _, err := store.Configure(ctx, f.Workspace, book.ID, f.Sales, settings, "enable", now.Add(-time.Minute)); err != nil {
					t.Fatal(err)
				}
				if binding, err := store.Binding(ctx, f.Workspace, existing.ID); err != nil || binding != nil {
					t.Fatalf("existing work started: %v", err)
				}
				detected := now
				if historical {
					detected = now.Add(-24 * time.Hour)
				}
				f.Exec(t, db, "UPDATE crm_signals SET commercial_motion=?,company_id=?,detected_at=?,created_at=? WHERE id=?", motion, f.Company, detected, now, f.Signal)
				source := model.CRMSituationSourceInput{Kind: "signal", SourceID: f.Signal, Situation: f.Situation(motion)}
				sourceRepo := repository.NewCRMSituationRepository(db)
				if err := sourceRepo.ImportSource(ctx, source); err != nil {
					t.Fatal(err)
				}
				if err := sourceRepo.ImportSource(ctx, source); err != nil {
					t.Fatal(err)
				}
				events := repository.NewAutomationScheduledEventRepository(db)
				event, err := events.ClaimNext(ctx, now.Add(time.Second), time.Minute)
				if err != nil {
					t.Fatal(err)
				}
				if historical {
					if event != nil {
						t.Fatal("historical import scheduled automation")
					}
					return
				}
				if event == nil || event.Kind != model.CRMPlaybookEntryDue {
					t.Fatalf("entry event missing: %#v", event)
				}
				candidates, err := store.AutomaticCandidates(ctx, f.Workspace, event.TargetID)
				if err != nil || len(candidates) != 1 {
					t.Fatalf("candidates: %#v %v", candidates, err)
				}
				if err := store.EnrollAutomatically(ctx, *event, &candidates[0], f.Sales, now.Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				binding, err := store.Binding(ctx, f.Workspace, event.TargetID)
				if err != nil || binding == nil || !binding.Enabled {
					t.Fatalf("new work not connected: %#v %v", binding, err)
				}
				item, err := sourceRepo.GetByID(ctx, f.Workspace, event.TargetID)
				if err != nil || item.Situation.PlaybookVersionID == nil || *item.Situation.PlaybookVersionID != *book.PublishedVersionID || item.Situation.Lifecycle != "open" {
					t.Fatalf("canonical progress changed incorrectly: %#v %v", item, err)
				}
				for _, milestone := range item.Situation.PlaybookMilestones {
					if milestone.Status != "pending" {
						t.Fatal("enrollment claimed customer progress")
					}
				}
				var count int64
				db.Model(&model.AutomationScheduledEvent{}).Where("kind=?", model.CRMPlaybookWorkDue).Count(&count)
				if count != 1 {
					t.Fatalf("duplicate starts: %d", count)
				}
				db.Model(&model.AgentRun{}).Count(&count)
				if count != 0 {
					t.Fatal("entry bypassed normal durable dispatch")
				}
			})
		}
	}
}

func TestCRMPlaybookSetupReusesBeaconWithoutChangingSavedConfiguration(t *testing.T) {
	db, store, book, _, connection := executionFixture(t, 0)
	req := model.PrepareCRMPlaybookSetupRequest{ExpectedRevision: book.Revision, PlaybookVersionID: *book.PublishedVersionID}
	factory := func() (*model.Agent, error) { t.Fatal("attempted to replace existing Beacon"); return nil, nil }
	first, err := store.PrepareSetup(context.Background(), f.Workspace, book.ID, f.SalesUser, f.Sales, req, factory)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.PrepareSetup(context.Background(), f.Workspace, book.ID, f.SalesUser, f.Sales, req, factory)
	if err != nil || first.FlowID != second.FlowID || first.AgentID != connection.Snapshot.Agent.ID {
		t.Fatalf("setup duplicated configuration: %v", err)
	}
	var flow model.AutomationRule
	if err := db.Where("id=?", first.FlowID).Take(&flow).Error; err != nil {
		t.Fatal(err)
	}
	if flow.Enabled || flow.TriggerType != model.CRMPlaybookWorkDue {
		t.Fatal("setup enabled a generic Flow")
	}
	var count int64
	db.Model(&model.Agent{}).Where("workspace_id=?", f.Workspace).Count(&count)
	if count != 1 {
		t.Fatalf("created more Beacon agents: %d", count)
	}
	db.Model(&model.CRMPlaybookAutomationSettings{}).Count(&count)
	if count != 0 {
		t.Fatal("setup created live execution gate")
	}
	req.ExpectedRevision++
	if _, err := store.PrepareSetup(context.Background(), f.Workspace, book.ID, f.SalesUser, f.Sales, req, factory); err == nil {
		t.Fatal("accepted stale setup")
	}
}

func TestCRMPlaybookWaitingDoesNotSpendAnotherRun(t *testing.T) {
	db, actions, binding, _, ctx := playbookActionFixture(t, 0)
	proposal := proposeActionForTest(t, actions, ctx, binding, proposalForJourney(0))
	now := time.Now().UTC().Truncate(time.Microsecond)
	f.Exec(t, db, "UPDATE agent_runs SET status='completed' WHERE id=?", binding.RunID)
	f.Exec(t, db, "UPDATE automation_scheduled_events SET status='delivered',lease_token=NULL,lease_until=NULL,completed_at=?", now)
	events := repository.NewAutomationScheduledEventRepository(db)
	if err := events.Enqueue(ctx, model.AutomationScheduledEvent{WorkspaceID: f.Workspace, EventKey: uuid.NewString(), Kind: model.CRMPlaybookWorkDue, TargetType: "crm_situation", TargetID: binding.SituationID, ExpectedRevision: binding.Generation, DueAt: now}); err != nil {
		t.Fatal(err)
	}
	event, err := events.ClaimNext(ctx, now, time.Minute)
	if err != nil || event == nil {
		t.Fatalf("claim: %v", err)
	}
	store := repository.NewCRMPlaybookExecutionRepository(db)
	reserved, err := store.ReserveRun(ctx, *event, now, func(model.CRMPlaybookExecutionSource, string) (model.AutomationRunBinding, error) {
		t.Fatal("waiting for approval attempted another run")
		return model.AutomationRunBinding{}, nil
	})
	if err != nil || reserved != nil {
		t.Fatalf("waiting: %v", err)
	}
	live, _ := store.Binding(ctx, f.Workspace, binding.SituationID)
	if live.Blocker != "awaiting_approval" || live.NoProgressRuns != 0 {
		t.Fatalf("waiting mislabeled as no progress: %#v", live)
	}
	if err := store.MaintainBinding(ctx, f.Workspace, binding.SituationID, now.Add(48*time.Hour), crmPlaybookActionContextFingerprint); err != nil {
		t.Fatal(err)
	}
	expired, err := repository.NewCRMSuggestionRepository(db).GetByID(ctx, f.Workspace, proposal.ID)
	if err != nil || expired.Status != "expired" {
		t.Fatalf("approval did not expire canonically: %#v %v", expired, err)
	}
}

func TestCRMPlaybookNewFactsFenceOldActionsAndScheduleReevaluation(t *testing.T) {
	db, actions, binding, _, ctx := playbookActionFixture(t, 0)
	proposal := proposeActionForTest(t, actions, ctx, binding, proposalForJourney(0))
	store := repository.NewCRMPlaybookExecutionRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.MaintainBinding(ctx, f.Workspace, binding.SituationID, now, crmPlaybookActionContextFingerprint); err != nil {
		t.Fatal(err)
	}
	f.Exec(t, db, "UPDATE crm_companies SET name='Updated customer facts' WHERE id=?", f.Company)
	if err := store.MaintainBinding(ctx, f.Workspace, binding.SituationID, now.Add(2*time.Minute), crmPlaybookActionContextFingerprint); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ValidateRun(ctx, f.Workspace, binding.RunID); err == nil {
		t.Fatal("old run kept authority after context changed")
	}
	action, err := repository.NewCRMSuggestionRepository(db).GetByID(ctx, f.Workspace, proposal.ID)
	if err != nil || action.Status != "superseded" {
		t.Fatalf("stale proposal not retired: %#v %v", action, err)
	}
	live, _ := store.Binding(ctx, f.Workspace, binding.SituationID)
	if live.Generation != binding.Generation+1 || live.NoProgressRuns != 0 {
		t.Fatalf("facts did not reset bounded review: %#v", live)
	}
}
