package service

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMPlaybookConnectionRuntimePreparation(t *testing.T) {
	for i, motion := range []string{"conversion", "onboarding", "retention"} {
		t.Run(motion, func(t *testing.T) {
			db, svc, ctx := playbookFixture(t)
			flow, agent := f.PlaybookConnectionStorage(t, db)
			pb := readyPlaybook(t, svc, ctx, i)
			selection := model.CRMPlaybookConnectionSelection{PlaybookVersionID: *pb.PublishedVersionID, ExpectedRevision: pb.Revision, FlowID: flow, AgentID: agent}
			review, err := svc.ReviewConnection(ctx, f.Workspace, pb.ID, selection)
			if err != nil {
				t.Fatal(err)
			}
			published, err := svc.PublishConnection(ctx, f.Workspace, pb.ID, model.PublishCRMPlaybookConnectionRequest{PlaybookVersionID: selection.PlaybookVersionID, ExpectedRevision: selection.ExpectedRevision, FlowID: selection.FlowID, AgentID: selection.AgentID, CommandKey: uuid.NewString(), ReviewFingerprint: review.ReviewFingerprint})
			if err != nil {
				t.Fatal(err)
			}
			stored, err := repository.NewCRMPlaybookRepository(db).Connection(ctx, f.Workspace, pb.ID, published.Connection.ID)
			if err != nil {
				t.Fatal(err)
			}
			work := playbookWork(t, db, motion)
			if _, err := svc.Apply(ctx, f.Workspace, pb.ID, playbookApplication(pb, work)); err != nil {
				t.Fatal(err)
			}
			item, err := svc.situations.GetByID(ctx, f.Workspace, work.ID)
			if err != nil {
				t.Fatal(err)
			}
			policy, err := svc.Get(ctx, f.Workspace, pb.ID)
			if err != nil {
				t.Fatal(err)
			}
			req := model.CRMPlaybookContextRequest{WorkspaceID: f.Workspace, PlaybookID: pb.ID, PlaybookVersionID: *pb.PublishedVersionID, SituationID: work.ID,
				ExpectedSituationRevision: item.Situation.Revision, SpecializationVersion: stored.Snapshot.Specialization.Version, Target: model.AgentRunTargetContext{TargetType: "crm_company", TargetID: f.Company}}
			before, err := json.Marshal(item)
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := prepareCRMPlaybookConnectionRuntime(*stored, *policy.PublishedVersion, *item, req, "test-app")
			if err != nil {
				t.Fatal(err)
			}
			if prepared.executionEnabled || len(prepared.packages) != 2 || prepared.agent.ID == agent || prepared.helpinAgentID != agent || prepared.input.CRMPlaybook.ConnectionID != stored.ID || prepared.input.CRMPlaybook.ConnectionVersion != 1 {
				t.Fatal("invalid runtime preparation")
			}
			f.Exec(t, db, "UPDATE agents SET system_prompt = 'Unreviewed replacement' WHERE id = ?", agent)
			again, err := prepareCRMPlaybookConnectionRuntime(*stored, *policy.PublishedVersion, *item, req, "test-app")
			if err != nil || !reflect.DeepEqual(prepared, again) || prepared.agent.SystemPrompt != "Workspace-reviewed Beacon instructions" {
				t.Fatal("preparation consulted latest configuration")
			}
			after, err := svc.situations.GetByID(ctx, f.Workspace, work.ID)
			if err != nil {
				t.Fatal(err)
			}
			afterBytes, err := json.Marshal(after)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(afterBytes) {
				t.Fatal("preparation rewrote customer progress")
			}
			for _, mutate := range []func(*model.CRMPlaybookConnection, *model.CRMSituationItem, *model.CRMPlaybookContextRequest){
				func(c *model.CRMPlaybookConnection, _ *model.CRMSituationItem, _ *model.CRMPlaybookContextRequest) {
					c.Fingerprint = "tampered"
				},
				func(c *model.CRMPlaybookConnection, _ *model.CRMSituationItem, _ *model.CRMPlaybookContextRequest) {
					c.WorkspaceID = f.ForeignWorkspace
				},
				func(c *model.CRMPlaybookConnection, _ *model.CRMSituationItem, _ *model.CRMPlaybookContextRequest) {
					c.Snapshot.RuntimeAgent = json.RawMessage(`{}`)
				},
				func(c *model.CRMPlaybookConnection, _ *model.CRMSituationItem, _ *model.CRMPlaybookContextRequest) {
					c.ExecutionEnabled = true
				},
				func(_ *model.CRMPlaybookConnection, item *model.CRMSituationItem, _ *model.CRMPlaybookContextRequest) {
					item.Situation.Lifecycle = model.CRMSituationPaused
				},
				func(_ *model.CRMPlaybookConnection, _ *model.CRMSituationItem, r *model.CRMPlaybookContextRequest) {
					r.ExpectedSituationRevision++
				},
				func(_ *model.CRMPlaybookConnection, _ *model.CRMSituationItem, r *model.CRMPlaybookContextRequest) {
					r.Target.TargetID = f.ForeignCompany
				},
			} {
				c, item, r := *stored, *item, req
				mutate(&c, &item, &r)
				if _, err := prepareCRMPlaybookConnectionRuntime(c, *policy.PublishedVersion, item, r, "test-app"); err == nil {
					t.Fatal("unsafe preparation accepted")
				}
			}
		})
	}
}
