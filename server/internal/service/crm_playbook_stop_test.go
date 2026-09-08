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

func TestCRMPlaybookStopConditionsPreventRunsWithoutClosingCustomerWork(t *testing.T) {
	for _, reason := range []string{"no_longer_eligible", "contact_restricted"} {
		t.Run(reason, func(t *testing.T) {
			definition := playbookDefinition(0)
			definition.Policy.StopConditions = []string{reason}
			db, store, book, work, connection := executionDefinitionFixture(t, 0, definition)
			f.PlaybookActionStorage(t, db)
			f.Exec(t, db, "UPDATE crm_situations SET contact_id=? WHERE id=?", f.Contact, work.ID)
			ctx, now := context.Background(), time.Now().UTC().Truncate(time.Microsecond)
			if _, err := store.Configure(ctx, f.Workspace, book.ID, f.Sales, executionSettings(connection.ID), "enable", now); err != nil {
				t.Fatal(err)
			}
			adopt := model.CRMPlaybookAutomationAdoption{CommandKey: uuid.NewString(), ExpectedRevision: work.Revision, ConnectionID: connection.ID, Enabled: true, Confirmed: true}
			if _, err := store.Adopt(ctx, f.Workspace, work.ID, f.Sales, adopt, "start", now); err != nil {
				t.Fatal(err)
			}
			source, err := store.Source(ctx, f.Workspace, work.ID)
			if err != nil || source == nil || source.StopReason != "" {
				t.Fatalf("eligible work stopped: %#v %v", source, err)
			}
			if reason == "no_longer_eligible" {
				f.Exec(t, db, "UPDATE crm_situations SET commercial_motion='retention' WHERE id=?", work.ID)
			} else {
				f.Exec(t, db, "UPDATE crm_contacts SET email_status='invalid' WHERE id=?", f.Contact)
			}
			events := repository.NewAutomationScheduledEventRepository(db)
			event, err := events.ClaimNext(ctx, now, time.Minute)
			if err != nil || event == nil {
				t.Fatalf("missing pending work: %v", err)
			}
			reserved, err := store.ReserveRun(ctx, *event, now, func(model.CRMPlaybookExecutionSource, string) (model.AutomationRunBinding, error) {
				t.Fatal("stop condition allowed an AI run")
				return model.AutomationRunBinding{}, nil
			})
			if err != nil || reserved != nil {
				t.Fatalf("stopped dispatch: %#v %v", reserved, err)
			}
			if err := store.MaintainBinding(ctx, f.Workspace, work.ID, now.Add(time.Minute), crmPlaybookActionContextFingerprint); err != nil {
				t.Fatal(err)
			}
			binding, err := store.Binding(ctx, f.Workspace, work.ID)
			if err != nil || binding == nil || binding.Blocker != reason {
				t.Fatalf("stop reason not visible: %#v %v", binding, err)
			}
			item, err := repository.NewCRMSituationRepository(db).GetByID(ctx, f.Workspace, work.ID)
			if err != nil || item.Situation.Lifecycle != "open" {
				t.Fatalf("stop condition fabricated an outcome: %#v %v", item, err)
			}
			for _, milestone := range item.Situation.PlaybookMilestones {
				if milestone.Status != "pending" {
					t.Fatal("stop condition claimed customer progress")
				}
			}
		})
	}
}

func TestCRMPlaybookAutomaticEntryResolvesConfiguredOwnerWithoutAdminFallback(t *testing.T) {
	for _, role := range []string{"signal_owner", "account_owner", "customer_success_owner", "deal_owner"} {
		t.Run(role, func(t *testing.T) {
			definition := playbookDefinition(0)
			definition.Responsibilities.OwnerRole = role
			db, store, book, _, connection := executionDefinitionFixture(t, 0, definition)
			ctx, now := context.Background(), time.Now().UTC().Truncate(time.Microsecond)
			settings := executionSettings(connection.ID)
			settings.EntryMode = "automatic"
			if _, err := store.Configure(ctx, f.Workspace, book.ID, f.Sales, settings, "enable", now.Add(-time.Minute)); err != nil {
				t.Fatal(err)
			}
			f.Exec(t, db, "UPDATE crm_signals SET commercial_motion='conversion',company_id=?,detected_at=?,created_at=? WHERE id=?", f.Company, now, now, f.Signal)
			works := repository.NewCRMSituationRepository(db)
			if err := works.ImportSource(ctx, model.CRMSituationSourceInput{Kind: "signal", SourceID: f.Signal, Situation: f.Situation("conversion")}); err != nil {
				t.Fatal(err)
			}
			event, err := repository.NewAutomationScheduledEventRepository(db).ClaimNext(ctx, now.Add(time.Second), time.Minute)
			if err != nil || event == nil {
				t.Fatalf("missing entry: %v", err)
			}
			candidates, err := store.AutomaticCandidates(ctx, f.Workspace, event.TargetID)
			if err != nil || len(candidates) != 1 {
				t.Fatalf("matching policy: %#v %v", candidates, err)
			}
			if role == "deal_owner" {
				if candidates[0].OwnerMemberID != nil {
					t.Fatal("missing deal owner fell back to administrator")
				}
				if err := store.EnrollAutomatically(ctx, *event, &candidates[0], f.Sales, now.Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				if binding, err := store.Binding(ctx, f.Workspace, event.TargetID); err != nil || binding != nil {
					t.Fatalf("ownerless work started: %#v %v", binding, err)
				}
				return
			}
			want := f.Sales
			if role == "customer_success_owner" {
				want = f.Success
			}
			if candidates[0].OwnerMemberID == nil || *candidates[0].OwnerMemberID != want {
				t.Fatalf("wrong %s routing: %#v", role, candidates[0].OwnerMemberID)
			}
			if err := store.EnrollAutomatically(ctx, *event, &candidates[0], want, now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			item, err := works.GetByID(ctx, f.Workspace, event.TargetID)
			if err != nil || item.Situation.OwnerMemberID == nil || *item.Situation.OwnerMemberID != want || item.Situation.NextActionOwnerMemberID == nil || *item.Situation.NextActionOwnerMemberID != want {
				t.Fatalf("canonical responsibility not routed: %#v %v", item, err)
			}
		})
	}
}
