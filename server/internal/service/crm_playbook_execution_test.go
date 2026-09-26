package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func executionFixture(t *testing.T, index int) (*gorm.DB, *repository.CRMPlaybookExecutionRepository, model.CRMPlaybook, model.CRMSituation, model.CRMPlaybookConnection) {
	t.Helper()
	return executionDefinitionFixture(t, index, playbookDefinition(index))
}

func executionDefinitionFixture(t *testing.T, index int, definition model.CRMPlaybookDefinition, profiles ...*AIProfileService) (*gorm.DB, *repository.CRMPlaybookExecutionRepository, model.CRMPlaybook, model.CRMSituation, model.CRMPlaybookConnection) {
	t.Helper()
	db, svc, ctx := playbookFixture(t)
	flow, agent := f.PlaybookConnectionStorage(t, db)
	f.PlaybookExecutionStorage(t, db)
	f.Exec(t, db, "UPDATE automation_rules SET trigger_type = ? WHERE id = ?", model.CRMPlaybookWorkDue, flow)
	f.Exec(t, db, "UPDATE agents SET allowed_tools = ? WHERE id = ?", []byte(`["get_crm_company","get_crm_contact","get_crm_deal"]`), agent)
	if len(profiles) != 0 {
		svc.SetAIProfileService(profiles[0])
	}
	pb := readyPlaybookDefinition(t, svc, ctx, definition)
	selection := model.CRMPlaybookConnectionSelection{PlaybookVersionID: *pb.PublishedVersionID, ExpectedRevision: pb.Revision, FlowID: flow, AgentID: agent}
	review, err := svc.ReviewConnection(ctx, f.Workspace, pb.ID, selection)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := svc.PublishConnection(ctx, f.Workspace, pb.ID, model.PublishCRMPlaybookConnectionRequest{PlaybookVersionID: selection.PlaybookVersionID, ExpectedRevision: pb.Revision, FlowID: flow, AgentID: agent, CommandKey: uuid.NewString(), ReviewFingerprint: review.ReviewFingerprint})
	if err != nil {
		t.Fatal(err)
	}
	work := playbookWork(t, db, []string{"conversion", "onboarding", "retention"}[index])
	if _, err := svc.Apply(ctx, f.Workspace, pb.ID, playbookApplication(pb, work)); err != nil {
		t.Fatal(err)
	}
	item, err := svc.situations.GetByID(ctx, f.Workspace, work.ID)
	if err != nil {
		t.Fatal(err)
	}
	return db, repository.NewCRMPlaybookExecutionRepository(db), pb, item.Situation, publication.Connection
}

func executionSettings(connection string) model.CRMPlaybookAutomationCommand {
	return model.CRMPlaybookAutomationCommand{CommandKey: uuid.NewString(), ConnectionID: connection, Enabled: true, EntryMode: "manual", MaxRunsPerDay: 4, MaxNoProgressRuns: 3, Confirmed: true}
}

func TestCRMPlaybookExecutionRequiresExplicitSignalAdoption(t *testing.T) {
	for index, motion := range []string{"conversion", "onboarding", "retention"} {
		t.Run(motion, func(t *testing.T) {
			db, repo, pb, work, connection := executionFixture(t, index)
			ctx, now := context.Background(), time.Now().UTC().Truncate(time.Microsecond)
			adopt := model.CRMPlaybookAutomationAdoption{CommandKey: uuid.NewString(), ExpectedRevision: work.Revision, ConnectionID: connection.ID, Enabled: true, Confirmed: true}
			if _, err := repo.Adopt(ctx, f.Workspace, work.ID, f.Sales, adopt, "start", now); !errors.Is(err, repository.ErrCRMPlaybookExecutionBlocked) {
				t.Fatalf("started while disabled: %v", err)
			}
			settings := executionSettings(connection.ID)
			if _, err := repo.Configure(ctx, f.Workspace, pb.ID, f.Sales, settings, "enable", now); err != nil {
				t.Fatal(err)
			}
			var count int64
			if err := db.Model(&model.CRMPlaybookAutomationBinding{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatal("enabling enrolled existing Signals")
			}
			first, err := repo.Adopt(ctx, f.Workspace, work.ID, f.Sales, adopt, "start", now)
			if err != nil {
				t.Fatal(err)
			}
			replay, err := repo.Adopt(ctx, f.Workspace, work.ID, f.Sales, adopt, "start", now.Add(time.Hour))
			if err != nil || replay.Binding.Generation != first.Binding.Generation {
				t.Fatalf("adoption replay: %v", err)
			}
			if err := db.Model(&model.AutomationScheduledEvent{}).Where("kind = ?", model.CRMPlaybookWorkDue).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("duplicate wake-ups: %d", count)
			}
			if _, err := repo.Adopt(ctx, f.Workspace, work.ID, f.Sales, adopt, "changed", now); !errors.Is(err, repository.ErrCRMPlaybookConflict) {
				t.Fatalf("changed intent reused receipt: %v", err)
			}
			var saved model.AutomationRule
			if err := db.Where("id = ?", connection.Snapshot.Flow.ID).Take(&saved).Error; err != nil {
				t.Fatal(err)
			}
			if saved.Enabled {
				t.Fatal("activation changed ordinary Flow settings")
			}
		})
	}
}

func TestCRMPlaybookExecutionDisableFencesExistingWakeups(t *testing.T) {
	db, repo, pb, work, connection := executionFixture(t, 0)
	ctx, now := context.Background(), time.Now().UTC().Truncate(time.Microsecond)
	settings := executionSettings(connection.ID)
	if _, err := repo.Configure(ctx, f.Workspace, pb.ID, f.Sales, settings, "enable", now); err != nil {
		t.Fatal(err)
	}
	adopt := model.CRMPlaybookAutomationAdoption{CommandKey: uuid.NewString(), ExpectedRevision: work.Revision, ConnectionID: connection.ID, Enabled: true, Confirmed: true}
	if _, err := repo.Adopt(ctx, f.Workspace, work.ID, f.Sales, adopt, "start", now); err != nil {
		t.Fatal(err)
	}
	events := repository.NewAutomationScheduledEventRepository(db)
	event, err := events.ClaimNext(ctx, now, time.Minute)
	if err != nil || event == nil {
		t.Fatalf("claim: %v", err)
	}
	settings.CommandKey = uuid.NewString()
	settings.ExpectedRevision = 1
	settings.Enabled = false
	if _, err := repo.Configure(ctx, f.Workspace, pb.ID, f.Sales, settings, "disable", now); err != nil {
		t.Fatal(err)
	}
	called := false
	_, err = repo.ReserveRun(ctx, *event, now, func(model.CRMPlaybookExecutionSource, string) (model.AutomationRunBinding, error) {
		called = true
		return model.AutomationRunBinding{}, nil
	})
	if !errors.Is(err, repository.ErrAutomationEventLeaseLost) || called {
		t.Fatalf("paused claim executed: %v", err)
	}
	settings.CommandKey = uuid.NewString()
	settings.ExpectedRevision = 2
	settings.Enabled = true
	if _, err := repo.Configure(ctx, f.Workspace, pb.ID, f.Sales, settings, "enable again", now); err != nil {
		t.Fatal(err)
	}
	binding, err := repo.Binding(ctx, f.Workspace, work.ID)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Enabled {
		t.Fatal("reenabling silently resumed paused customer work")
	}
}

func TestCRMPlaybookExecutionDispatchReplayAndLiveRevisionGuard(t *testing.T) {
	db, repo, pb, work, connection := executionFixture(t, 0)
	ctx, now := context.Background(), time.Now().UTC().Truncate(time.Microsecond)
	if _, err := repo.Configure(ctx, f.Workspace, pb.ID, f.Sales, executionSettings(connection.ID), "enable", now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Adopt(ctx, f.Workspace, work.ID, f.Sales, model.CRMPlaybookAutomationAdoption{CommandKey: uuid.NewString(), ExpectedRevision: work.Revision, ConnectionID: connection.ID, Enabled: true, Confirmed: true}, "start", now); err != nil {
		t.Fatal(err)
	}
	event, err := repository.NewAutomationScheduledEventRepository(db).ClaimNext(ctx, now, time.Minute)
	if err != nil || event == nil {
		t.Fatalf("claim: %v", err)
	}
	builds := 0
	compile := func(source model.CRMPlaybookExecutionSource, runID string) (model.AutomationRunBinding, error) {
		builds++
		r := model.CRMPlaybookContextRequest{WorkspaceID: f.Workspace, PlaybookID: pb.ID, PlaybookVersionID: *pb.PublishedVersionID, SituationID: work.ID, ExpectedSituationRevision: work.Revision, SpecializationVersion: source.Connection.Snapshot.Specialization.Version, Target: model.AgentRunTargetContext{TargetType: "crm_company", TargetID: f.Company}}
		prepared, err := prepareCRMPlaybookConnectionRuntime(source.Connection, source.Policy, source.Item, r, "helpin")
		if err != nil {
			return model.AutomationRunBinding{}, err
		}
		return model.AutomationRunBinding{AgentID: prepared.helpinAgentID, RuntimeProfileID: prepared.agent.ID, Input: prepared.input}, nil
	}
	first, err := repo.ReserveRun(ctx, *event, now, compile)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := repo.ReserveRun(ctx, *event, now, compile)
	if err != nil || first.RunID != replay.RunID || builds != 1 {
		t.Fatalf("dispatch recompiled/reallocated: %v", err)
	}
	if _, _, err := repo.ValidateRun(ctx, f.Workspace, first.RunID); err != nil {
		t.Fatal(err)
	}
	f.Exec(t, db, "UPDATE crm_situations SET revision = revision + 1 WHERE id = ?", work.ID)
	if _, _, err := repo.ValidateRun(ctx, f.Workspace, first.RunID); !errors.Is(err, repository.ErrCRMPlaybookExecutionBlocked) {
		t.Fatalf("stale run retained authority: %v", err)
	}
	if _, _, err := repo.ValidateRun(ctx, f.ForeignWorkspace, first.RunID); !errors.Is(err, repository.ErrCRMPlaybookExecutionBlocked) {
		t.Fatalf("foreign run retained authority: %v", err)
	}
}
