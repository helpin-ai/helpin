package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMPlaybookAutomationPreviewSelectsPublishedJob(t *testing.T) {
	for _, index := range []int{0, 1, 2} {
		t.Run(playbookDefinition(index).Journey, func(t *testing.T) {
			_, svc, ctx := playbookFixture(t)
			pb := readyPlaybook(t, svc, ctx, index)
			before, err := svc.Get(ctx, f.Workspace, pb.ID)
			if err != nil {
				t.Fatal(err)
			}
			preview, err := svc.AutomationPreview(ctx, f.Workspace, pb.ID, *pb.PublishedVersionID, pb.Revision)
			if err != nil {
				t.Fatal(err)
			}
			if preview.ExecutionEnabled || preview.Status != "not_connected" || preview.Scope != "published" || preview.PresetKey != model.AgentPresetCRMOperator || len(preview.Skills) != 2 || len(preview.SpecializationVersion) != 64 {
				t.Fatalf("untruthful setup preview: %#v", preview)
			}
			if preview.Journey != pb.Draft.Journey || preview.PlaybookVersionID == nil || *preview.PlaybookVersionID != *pb.PublishedVersionID {
				t.Fatal("preview selected the wrong published job")
			}
			after, err := svc.Get(ctx, f.Workspace, pb.ID)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("preview changed policy, history or participants: %v", err)
			}
		})
	}
}

func TestCRMPlaybookAutomationPreviewDoesNotUseNewDraftForPublishedPolicy(t *testing.T) {
	_, svc, ctx := playbookFixture(t)
	pb := readyPlaybook(t, svc, ctx, 0)
	changed := playbookDefinition(2)
	pb = mustPlaybookCommand(t, svc, ctx, pb, "update_draft", &changed, nil)
	published, err := svc.AutomationPreview(ctx, f.Workspace, pb.ID, *pb.PublishedVersionID, pb.Revision)
	if err != nil || published.Journey != "buying_intent" {
		t.Fatalf("draft rewrote published specialization: %#v %v", published, err)
	}
	draft, err := svc.AutomationPreview(ctx, f.Workspace, pb.ID, "", pb.Revision)
	if err != nil || draft.Journey != "renewal_recovery" || draft.Scope != "draft" || draft.PlaybookVersionID != nil {
		t.Fatalf("draft is mislabeled as published: %#v %v", draft, err)
	}
	if draft.SpecializationVersion == published.SpecializationVersion {
		t.Fatal("different jobs have the same specialization identity")
	}
}

func TestCRMPlaybookAutomationPreviewPreservesManualCustomPlaybooks(t *testing.T) {
	_, svc, ctx := playbookFixture(t)
	d := playbookDefinition(0)
	d.Journey = "custom"
	pb, _, err := svc.Create(ctx, f.Workspace, model.CreateCRMPlaybookRequest{CreationKey: uuid.NewString(), Definition: d})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := svc.AutomationPreview(ctx, f.Workspace, pb.ID, "", pb.Revision)
	if err != nil || preview.Status != "unsupported_journey" || preview.ExecutionEnabled || len(preview.Skills) != 0 || preview.SpecializationVersion != "" {
		t.Fatalf("custom policy implicitly selected a job: %#v %v", preview, err)
	}
}

func TestCRMPlaybookAutomationPreviewPermissionsAndFreshness(t *testing.T) {
	_, svc, ctx := playbookFixture(t)
	pb := readyPlaybook(t, svc, ctx, 0)
	for _, test := range []struct {
		name, workspace, id, version string
		revision                     int64
		ctx                          context.Context
		want                         error
	}{
		{name: "anonymous", workspace: f.Workspace, id: pb.ID, revision: pb.Revision, ctx: context.Background(), want: ErrCRMPlaybookForbidden},
		{name: "foreign workspace", workspace: uuid.NewString(), id: pb.ID, revision: pb.Revision, ctx: ctx, want: ErrCRMPlaybookForbidden},
		{name: "invalid id", workspace: f.Workspace, id: "bad", revision: pb.Revision, ctx: ctx, want: ErrCRMPlaybookInput},
		{name: "invalid revision", workspace: f.Workspace, id: pb.ID, revision: 0, ctx: ctx, want: ErrCRMPlaybookInput},
		{name: "stale revision", workspace: f.Workspace, id: pb.ID, revision: pb.Revision - 1, ctx: ctx, want: repository.ErrCRMPlaybookStale},
		{name: "foreign version", workspace: f.Workspace, id: pb.ID, version: uuid.NewString(), revision: pb.Revision, ctx: ctx, want: repository.ErrCRMPlaybookStale},
		{name: "missing", workspace: f.Workspace, id: uuid.NewString(), revision: 1, ctx: ctx, want: ErrCRMPlaybookNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := svc.AutomationPreview(test.ctx, test.workspace, test.id, test.version, test.revision)
			if !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
	viewer := authorization.WithActor(context.Background(), f.Actor("viewer"))
	if _, err := svc.AutomationPreview(viewer, f.Workspace, pb.ID, "", pb.Revision); err != nil {
		t.Fatalf("CRM reader cannot inspect product-owned job metadata: %v", err)
	}
}
