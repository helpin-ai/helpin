package service

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func playbookFixture(t *testing.T) (*gorm.DB, *CRMPlaybookService, context.Context) {
	t.Helper()
	db, crm, _ := situationServiceFixture(t)
	authz := authorization.NewAuthzService(db, nil, nil)
	return db, NewCRMPlaybookService(repository.NewCRMPlaybookRepository(db), authz, crm), authorization.WithActor(context.Background(), f.Actor("admin"))
}

func playbookDefinition(index int) model.CRMPlaybookDefinition {
	d := playbookTemplates()[index]
	d.Responsibilities.EscalationMemberID = f.Ptr(f.Success)
	return d
}

func mustPlaybookCommand(t *testing.T, svc *CRMPlaybookService, ctx context.Context, pb model.CRMPlaybook, operation string, definition *model.CRMPlaybookDefinition, accepting *bool) model.CRMPlaybook {
	t.Helper()
	result, err := svc.Command(ctx, f.Workspace, pb.ID, model.CRMPlaybookCommandRequest{CommandKey: uuid.NewString(), ExpectedRevision: pb.Revision, Operation: operation, Definition: definition, AcceptingCustomers: accepting})
	if err != nil {
		t.Fatal(err)
	}
	return result.Change.After
}

func readyPlaybook(t *testing.T, svc *CRMPlaybookService, ctx context.Context, index int) model.CRMPlaybook {
	t.Helper()
	return readyPlaybookDefinition(t, svc, ctx, playbookDefinition(index))
}

func readyPlaybookDefinition(t *testing.T, svc *CRMPlaybookService, ctx context.Context, definition model.CRMPlaybookDefinition) model.CRMPlaybook {
	t.Helper()
	pb, _, err := svc.Create(ctx, f.Workspace, model.CreateCRMPlaybookRequest{CreationKey: uuid.NewString(), Definition: definition})
	if err != nil {
		t.Fatal(err)
	}
	published := mustPlaybookCommand(t, svc, ctx, *pb, "publish", nil, nil)
	yes := true
	return mustPlaybookCommand(t, svc, ctx, published, "set_enrollment", nil, &yes)
}

func playbookWork(t *testing.T, db *gorm.DB, motion string) model.CRMSituation {
	t.Helper()
	stored, _, err := repository.NewCRMSituationRepository(db).Create(context.Background(), f.Situation(motion), nil)
	if err != nil {
		t.Fatal(err)
	}
	return *stored
}

func playbookApplication(pb model.CRMPlaybook, work model.CRMSituation) model.ApplyCRMPlaybookRequest {
	return model.ApplyCRMPlaybookRequest{CommandKey: uuid.NewString(), SituationID: work.ID, VersionID: *pb.PublishedVersionID,
		ExpectedPlaybookRevision: pb.Revision, ExpectedSituationRevision: work.Revision, Confirmed: true}
}

func TestCRMPlaybookDraftPublishAndReplay(t *testing.T) {
	db, svc, ctx := playbookFixture(t)
	req := model.CreateCRMPlaybookRequest{CreationKey: uuid.NewString(), Definition: playbookDefinition(0)}
	pb, created, err := svc.Create(ctx, f.Workspace, req)
	if err != nil || !created || pb.AcceptingCustomers || pb.PublishedVersionID != nil || pb.Revision != 1 {
		t.Fatalf("draft: %#v %v", pb, err)
	}
	command := model.CRMPlaybookCommandRequest{CommandKey: uuid.NewString(), ExpectedRevision: 1, Operation: "publish"}
	first, err := svc.Command(ctx, f.Workspace, pb.ID, command)
	if err != nil || first.Change.After.AcceptingCustomers || first.Change.After.PublishedVersionID == nil {
		t.Fatalf("publishing implicitly enabled enrollment: %#v %v", first, err)
	}
	changed := first.Change.After.Draft
	changed.Name = "Updated draft"
	changed.Objective = "A changed future objective"
	current := mustPlaybookCommand(t, svc, ctx, first.Change.After, "update_draft", &changed, nil)
	item, err := svc.Get(ctx, f.Workspace, pb.ID)
	if err != nil || item.PublishedVersion.Definition.Name != pb.Draft.Name || item.Playbook.Draft.Name != changed.Name {
		t.Fatalf("draft edit mutated publication: %#v %v", item, err)
	}
	replay, err := svc.Command(ctx, f.Workspace, pb.ID, command)
	if err != nil || !replay.Replayed || !reflect.DeepEqual(first.Change, replay.Change) {
		t.Fatalf("publication replay changed: %#v %v", replay, err)
	}
	createdAgain, fresh, err := svc.Create(ctx, f.Workspace, req)
	if err != nil || fresh || createdAgain.Revision != 1 || createdAgain.Draft.Name != pb.Draft.Name {
		t.Fatalf("creation replay rewrote or misrepresented later settings: %#v %v", createdAgain, err)
	}
	current = mustPlaybookCommand(t, svc, ctx, current, "publish", nil, nil)
	versions, err := svc.Versions(ctx, f.Workspace, pb.ID, 0, 1)
	if err != nil || len(versions.Data) != 1 || versions.Data[0].Version != 2 || versions.NextBeforeVersion == nil {
		t.Fatalf("version pagination: %#v %v", versions, err)
	}
	older, err := svc.Versions(ctx, f.Workspace, pb.ID, *versions.NextBeforeVersion, 1)
	if err != nil || len(older.Data) != 1 || older.Data[0].Definition.Name != pb.Draft.Name {
		t.Fatalf("old policy lost: %#v %v", older, err)
	}
	history, err := svc.History(ctx, f.Workspace, pb.ID, 0, 2)
	if err != nil || len(history.Data) != 2 || history.NextBeforeRevision == nil || history.Data[0].Revision != current.Revision {
		t.Fatalf("history pagination: %#v %v", history, err)
	}
	var workCount int64
	if err := db.Model(&model.CRMSituation{}).Count(&workCount).Error; err != nil || workCount != 0 {
		t.Fatalf("configuration enrolled customers: %d %v", workCount, err)
	}
	command.Reason = "Different intent"
	if _, err := svc.Command(ctx, f.Workspace, pb.ID, command); !errors.Is(err, repository.ErrCRMPlaybookConflict) {
		t.Fatalf("conflicting command allowed: %v", err)
	}
}

func TestCRMPlaybookConcurrentDraftEdits(t *testing.T) {
	_, svc, ctx := playbookFixture(t)
	pb := readyPlaybook(t, svc, ctx, 0)
	errs := make([]error, 8)
	var workers sync.WaitGroup
	for i := range errs {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			_, errs[index] = svc.Command(ctx, f.Workspace, pb.ID, model.CRMPlaybookCommandRequest{CommandKey: uuid.NewString(), ExpectedRevision: pb.Revision, Operation: "publish"})
		}(i)
	}
	workers.Wait()
	winners := 0
	for _, err := range errs {
		if err == nil {
			winners++
		} else if !errors.Is(err, repository.ErrCRMPlaybookStale) {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent publication winners=%d", winners)
	}
	versions, err := svc.Versions(ctx, f.Workspace, pb.ID, 0, 100)
	if err != nil || len(versions.Data) != 2 {
		t.Fatalf("duplicate publications: %#v %v", versions, err)
	}
}

func TestCRMPlaybookPermissionsAndWorkspaceBoundaries(t *testing.T) {
	_, svc, admin := playbookFixture(t)
	pb := readyPlaybook(t, svc, admin, 0)
	for _, role := range []string{"viewer", "member", "admin", "owner"} {
		t.Run(role, func(t *testing.T) {
			ctx := authorization.WithActor(context.Background(), f.Actor(role))
			if _, err := svc.Get(ctx, f.Workspace, pb.ID); err != nil {
				t.Fatal(err)
			}
			_, _, err := svc.Create(ctx, f.Workspace, model.CreateCRMPlaybookRequest{CreationKey: uuid.NewString(), Definition: playbookDefinition(0)})
			allowed := role == "admin" || role == "owner"
			if allowed && err != nil {
				t.Fatal(err)
			}
			if !allowed && !errors.Is(err, ErrCRMPlaybookForbidden) {
				t.Fatalf("role %s configured a Playbook: %v", role, err)
			}
		})
	}
	foreignActor := f.Actor("admin")
	foreignActor.WorkspaceID, foreignActor.WorkspaceMemberID = f.ForeignWorkspace, f.ForeignMember
	foreign := authorization.WithActor(context.Background(), foreignActor)
	if _, err := svc.Get(foreign, f.ForeignWorkspace, pb.ID); !errors.Is(err, ErrCRMPlaybookNotFound) {
		t.Fatalf("foreign definition exposed: %v", err)
	}
	if _, err := svc.Get(admin, f.ForeignWorkspace, pb.ID); !errors.Is(err, ErrCRMPlaybookForbidden) {
		t.Fatalf("forged workspace accepted: %v", err)
	}
	if _, err := svc.List(context.Background(), f.Workspace, model.CRMPlaybookListFilters{}); !errors.Is(err, ErrCRMPlaybookForbidden) {
		t.Fatalf("missing actor accepted: %v", err)
	}
}

func TestCRMPlaybookPublishingValidation(t *testing.T) {
	for _, tc := range []struct {
		name        string
		change      func(*model.CRMPlaybookDefinition)
		createFails bool
	}{
		{"missing escalation", func(d *model.CRMPlaybookDefinition) { d.Responsibilities.EscalationMemberID = nil }, false},
		{"missing objective", func(d *model.CRMPlaybookDefinition) { d.Objective = "" }, false},
		{"missing milestones", func(d *model.CRMPlaybookDefinition) { d.Milestones = nil }, false},
		{"missing success criteria", func(d *model.CRMPlaybookDefinition) { d.Milestones[0].SuccessCriteria = "" }, false},
		{"duplicate milestone keys", func(d *model.CRMPlaybookDefinition) { d.Milestones[1].Key = d.Milestones[0].Key }, true},
		{"unsupported automatic sending", func(d *model.CRMPlaybookDefinition) { d.Policy.OutboundMessages = "automatic" }, true},
		{"unsupported stop", func(d *model.CRMPlaybookDefinition) { d.Policy.StopConditions = []string{"agent_says_done"} }, true},
		{"unbounded follow-up", func(d *model.CRMPlaybookDefinition) { d.Policy.CheckAfterHours = -1 }, true},
		{"foreign escalation", func(d *model.CRMPlaybookDefinition) { d.Responsibilities.EscalationMemberID = f.Ptr(f.ForeignMember) }, true},
		{"invalid query", func(d *model.CRMPlaybookDefinition) {
			d.Eligibility.Filter = &model.QueryFilterGroup{Rules: []model.QueryFilterRule{{Field: "secret", Operator: "is", Value: f.Ptr("x")}}}
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, svc, ctx := playbookFixture(t)
			d := playbookDefinition(0)
			tc.change(&d)
			pb, _, err := svc.Create(ctx, f.Workspace, model.CreateCRMPlaybookRequest{CreationKey: uuid.NewString(), Definition: d})
			if tc.createFails {
				if err == nil {
					t.Fatal("invalid draft accepted")
				}
				return
			}
			if err != nil {
				t.Fatalf("incomplete draft could not be saved: %v", err)
			}
			if _, err := svc.Command(ctx, f.Workspace, pb.ID, model.CRMPlaybookCommandRequest{CommandKey: uuid.NewString(), ExpectedRevision: 1, Operation: "publish"}); !errors.Is(err, ErrCRMPlaybookInput) {
				t.Fatalf("incomplete policy published: %v", err)
			}
		})
	}
}
