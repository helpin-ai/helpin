package service

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMPlaybookParticipationPinsVersionAndUsesCanonicalLifecycle(t *testing.T) {
	for index, motion := range []string{"conversion", "onboarding", "retention"} {
		t.Run(motion, func(t *testing.T) {
			db, svc, ctx := playbookFixture(t)
			pb := readyPlaybook(t, svc, ctx, index)
			work := playbookWork(t, db, motion)
			request := playbookApplication(pb, work)
			first, err := svc.Apply(ctx, f.Workspace, pb.ID, request)
			if err != nil {
				t.Fatal(err)
			}
			state := first.Change.After
			if state.PlaybookVersionID == nil || *state.PlaybookVersionID != *pb.PublishedVersionID || first.Change.Revision != 2 {
				t.Fatalf("policy was not pinned: %#v", state)
			}
			if len(state.PlaybookMilestones) != len(pb.Draft.Milestones) || *state.OwnerMemberID != f.Sales || state.NextStep != work.NextStep || state.OutcomeKind != nil {
				t.Fatalf("attachment changed work: %#v", state)
			}
			// The draft and new publication cannot rewrite the active customer's policy.
			draft := pb.Draft
			draft.Objective = "New policy for future customers"
			draft.Milestones = draft.Milestones[:1]
			pb = mustPlaybookCommand(t, svc, ctx, pb, "update_draft", &draft, nil)
			pb = mustPlaybookCommand(t, svc, ctx, pb, "publish", nil, nil)
			no := false
			pb = mustPlaybookCommand(t, svc, ctx, pb, "set_enrollment", nil, &no)
			replay, err := svc.Apply(ctx, f.Workspace, pb.ID, request)
			if err != nil || !replay.Replayed || !reflect.DeepEqual(first.Change, replay.Change) {
				t.Fatalf("replay reset active work: %#v %v", replay, err)
			}
			assessment := model.CRMPlaybookMilestoneRequest{CommandKey: uuid.NewString(), ExpectedRevision: 2, MilestoneKey: state.PlaybookMilestones[1].Key, Status: "achieved", Summary: "Customer confirmed this milestone"}
			progress, err := svc.AssessMilestone(ctx, f.Workspace, pb.ID, work.ID, assessment)
			if err != nil || *progress.Change.After.PlaybookVersionID != request.VersionID || progress.Change.After.OutcomeKind != nil {
				t.Fatalf("progress used new version or closed the objective: %#v %v", progress, err)
			}
			milestone := progress.Change.After.PlaybookMilestones[1]
			if milestone.Basis == nil || *milestone.Basis != "human_assessment" || milestone.AssessedAt == nil || *milestone.AssessedByMemberID != f.Sales {
				t.Fatalf("progress lacks provenance: %#v", milestone)
			}
			if _, err := svc.AssessMilestone(ctx, f.Workspace, pb.ID, work.ID, assessment); err != nil {
				t.Fatal(err)
			}
			pause := workCommand("pause", 3)
			pause.Reason = "Customer requested a hold"
			mustWorkCommand(t, svc.situations, ctx, work.ID, pause)
			participants, err := svc.Participants(ctx, f.Workspace, pb.ID, model.CRMSituationListFilters{State: "paused"})
			if err != nil || participants.Total != 1 || participants.Data[0].Situation.PlaybookVersionID == nil {
				t.Fatalf("canonical pause missing: %#v %v", participants, err)
			}
			item, err := svc.Get(ctx, f.Workspace, pb.ID)
			if err != nil || item.PausedCount != 1 || item.OpenCount != 0 || item.ClosedCount != 0 {
				t.Fatalf("counts copied stale lifecycle: %#v %v", item, err)
			}
			other := playbookWork(t, db, motion)
			if _, err := svc.Apply(ctx, f.Workspace, pb.ID, playbookApplication(pb, other)); !errors.Is(err, repository.ErrCRMPlaybookUnavailable) {
				t.Fatalf("stopped enrollment admitted work: %v", err)
			}
			assessment.CommandKey, assessment.ExpectedRevision = uuid.NewString(), 4
			if _, err := svc.AssessMilestone(ctx, f.Workspace, pb.ID, work.ID, assessment); !errors.Is(err, repository.ErrCRMPlaybookUnavailable) {
				t.Fatalf("paused progress advanced: %v", err)
			}
		})
	}
}

func TestCRMPlaybookPreviewIsReadOnlyAndVersionExplicit(t *testing.T) {
	db, svc, ctx := playbookFixture(t)
	pb := readyPlaybook(t, svc, ctx, 0)
	for range 3 {
		playbookWork(t, db, "conversion")
	}
	playbookWork(t, db, "retention")
	for range 2 {
		preview, err := svc.Preview(ctx, f.Workspace, pb.ID, "", pb.Revision, 2, 1)
		if err != nil || preview.Signals.Total != 3 || len(preview.Signals.Data) != 1 || preview.ExecutionEnabled || preview.VersionID != nil || preview.Scope != "existing_signals" {
			t.Fatalf("preview: %#v %v", preview, err)
		}
	}
	var enrolled, versions, changes, suggestions int64
	for _, q := range []struct {
		table, condition string
		value            *int64
	}{
		{"crm_situations", "playbook_id IS NOT NULL", &enrolled}, {"crm_playbook_versions", "1 = 1", &versions},
		{"crm_playbook_changes", "1 = 1", &changes}, {"crm_suggestions", "1 = 1", &suggestions},
	} {
		if err := db.Table(q.table).Where(q.condition).Count(q.value).Error; err != nil {
			t.Fatal(err)
		}
	}
	if enrolled != 0 || versions != 1 || changes != 3 || suggestions != 1 {
		t.Fatalf("preview wrote data: enrolled=%d versions=%d changes=%d suggestions=%d", enrolled, versions, changes, suggestions)
	}
	draft := pb.Draft
	draft.Eligibility.CommercialMotions = []string{"retention"}
	pb = mustPlaybookCommand(t, svc, ctx, pb, "update_draft", &draft, nil)
	previewDraft, err := svc.Preview(ctx, f.Workspace, pb.ID, "", pb.Revision, 1, 25)
	if err != nil || previewDraft.Signals.Total != 1 {
		t.Fatalf("draft preview: %#v %v", previewDraft, err)
	}
	published, err := svc.Preview(ctx, f.Workspace, pb.ID, *pb.PublishedVersionID, pb.Revision, 1, 25)
	if err != nil || published.Signals.Total != 3 || published.VersionID == nil {
		t.Fatalf("draft shadowed published eligibility: %#v %v", published, err)
	}
	if _, err := svc.Preview(ctx, f.Workspace, pb.ID, "", pb.Revision-1, 1, 25); !errors.Is(err, repository.ErrCRMPlaybookStale) {
		t.Fatalf("stale preview accepted: %v", err)
	}
}

func TestCRMPlaybookConcurrentApplicationAndIndependentEpisodes(t *testing.T) {
	db, svc, ctx := playbookFixture(t)
	pb := readyPlaybook(t, svc, ctx, 0)
	work := playbookWork(t, db, "conversion")
	request := playbookApplication(pb, work)
	results := make([]*model.CRMSituationCommandResult, 8)
	errs := make([]error, 8)
	var workers sync.WaitGroup
	for i := range results {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			results[index], errs[index] = svc.Apply(ctx, f.Workspace, pb.ID, request)
		}(i)
	}
	workers.Wait()
	fresh := 0
	for i, result := range results {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if !result.Replayed {
			fresh++
		}
	}
	if fresh != 1 {
		t.Fatalf("duplicate enrollments: %d", fresh)
	}
	other := playbookWork(t, db, "conversion")
	if _, err := svc.Apply(ctx, f.Workspace, pb.ID, playbookApplication(pb, other)); err != nil {
		t.Fatalf("same company suppressed independent request: %v", err)
	}
	list, err := svc.Participants(ctx, f.Workspace, pb.ID, model.CRMSituationListFilters{PageSize: 1})
	if err != nil || list.Total != 2 || len(list.Data) != 1 {
		t.Fatalf("participant pagination: %#v %v", list, err)
	}
	books, err := svc.List(ctx, f.Workspace, model.CRMPlaybookListFilters{PageSize: 1})
	if err != nil || books.Total != 1 || books.Data[0].OpenCount != 2 {
		t.Fatalf("Playbook counts: %#v %v", books, err)
	}
}

func TestCRMPlaybookApplicationRechecksEligibilityAndInvalidatesCheckpoint(t *testing.T) {
	db, svc, ctx := playbookFixture(t)
	pb := readyPlaybook(t, svc, ctx, 0)
	work := playbookWork(t, db, "retention")
	if _, err := svc.Apply(ctx, f.Workspace, pb.ID, playbookApplication(pb, work)); !errors.Is(err, repository.ErrCRMPlaybookUnavailable) {
		t.Fatalf("ineligible work admitted: %v", err)
	}
	work = playbookWork(t, db, "conversion")
	now := time.Now().UTC().Truncate(time.Microsecond)
	change := workCommand("update", 1)
	change.Changes = &model.CRMSituationChanges{Checkpoint: &model.CRMSituationCheckpointChange{At: &now}}
	mustWorkCommand(t, svc.situations, ctx, work.ID, change)
	work.Revision = 2
	events := repository.NewAutomationScheduledEventRepository(db)
	claim, err := events.ClaimNext(ctx, now, time.Minute)
	if err != nil || claim == nil {
		t.Fatalf("checkpoint claim: %#v %v", claim, err)
	}
	request := playbookApplication(pb, work)
	if _, err := svc.Apply(ctx, f.Workspace, pb.ID, request); err != nil {
		t.Fatal(err)
	}
	if err := events.Complete(ctx, *claim, "follow_up_due", now); !errors.Is(err, repository.ErrAutomationEventLeaseLost) {
		t.Fatalf("old checkpoint survived policy attachment: %v", err)
	}
	request.CommandKey, request.ExpectedSituationRevision = uuid.NewString(), 3
	if _, err := svc.Apply(ctx, f.Workspace, pb.ID, request); !errors.Is(err, repository.ErrCRMPlaybookConflict) {
		t.Fatalf("second Playbook attached to same objective: %v", err)
	}
	viewer := authorization.WithActor(context.Background(), f.Actor("viewer"))
	if _, err := svc.Apply(viewer, f.Workspace, pb.ID, request); !errors.Is(err, ErrCRMPlaybookForbidden) {
		t.Fatalf("viewer enrolled customer: %v", err)
	}
}
