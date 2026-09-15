package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	sdk "github.com/helpin-ai/agent-runtime-go"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMPlaybookReviewFreezesSharedAIProfile(t *testing.T) {
	db, playbooks, ctx := playbookFixture(t)
	flow, agent := f.PlaybookConnectionStorage(t, db)
	profiles, _, _ := setupAIProfileTest(t)
	connection := &model.AIConnection{ID: uuid.NewString(), WorkspaceID: f.Workspace, Scope: "workspace", Name: "Shared", Provider: "openai", Status: "connected"}
	if err := profiles.connections.seal(connection, aiConnectionSecret{APIKey: "shared-secret"}); err != nil {
		t.Fatal(err)
	}
	if err := profiles.connections.repo.Create(ctx, connection); err != nil {
		t.Fatal(err)
	}
	p := &model.AIProfile{ID: uuid.NewString(), WorkspaceID: f.Workspace, Scope: "workspace", Name: "Reviewed", Revision: 1, Primary: model.AIProfileRoute{ConnectionID: connection.ID, Model: sdk.RunModel{Provider: "openai", Model: "reviewed-model", Controls: &sdk.ModelControls{ReasoningEffort: strPtr("low")}}}}
	if err := profiles.repo.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := profiles.repo.SetDefault(ctx, f.Workspace, p); err != nil {
		t.Fatal(err)
	}
	playbooks.SetAIProfileService(profiles)
	pb := readyPlaybook(t, playbooks, ctx, 0)
	selection := model.CRMPlaybookConnectionSelection{PlaybookVersionID: *pb.PublishedVersionID, ExpectedRevision: pb.Revision, FlowID: flow, AgentID: agent}
	review, err := playbooks.ReviewConnection(ctx, f.Workspace, pb.ID, selection)
	if err != nil {
		t.Fatal(err)
	}
	req := model.PublishCRMPlaybookConnectionRequest{PlaybookVersionID: selection.PlaybookVersionID, ExpectedRevision: selection.ExpectedRevision, FlowID: flow, AgentID: agent, CommandKey: uuid.NewString(), ReviewFingerprint: review.ReviewFingerprint}
	result, err := playbooks.PublishConnection(ctx, f.Workspace, pb.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := repository.NewCRMPlaybookRepository(db).Connection(ctx, f.Workspace, pb.ID, result.Connection.ID)
	if err != nil {
		t.Fatal(err)
	}
	frozen := stored.Snapshot.AISelection
	if frozen == nil || frozen.ProfileID != p.ID || frozen.Route.Model.Model != "reviewed-model" || frozen.ProfileRevision != 1 || derefString(stored.Snapshot.Agent.Model) != "reviewed-model" {
		t.Fatal("review did not freeze model and profile")
	}
	encoded, _ := json.Marshal(stored.Snapshot)
	if strings.Contains(string(encoded), "shared-secret") {
		t.Fatal("review snapshot exposed a credential")
	}
	p.Revision = 2
	p.Primary.Model.Model = "changed-model"
	if err := profiles.repo.Update(ctx, p, 1); err != nil {
		t.Fatal(err)
	}
	current, err := playbooks.ReviewConnection(ctx, f.Workspace, pb.ID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if current.ReviewFingerprint == review.ReviewFingerprint {
		t.Fatal("route change did not invalidate review fingerprint")
	}
	stored, err = repository.NewCRMPlaybookRepository(db).Connection(ctx, f.Workspace, pb.ID, result.Connection.ID)
	if err != nil || stored.Snapshot.AISelection.Route.Model.Model != "reviewed-model" {
		t.Fatal("profile edit altered an approved setup")
	}
}
