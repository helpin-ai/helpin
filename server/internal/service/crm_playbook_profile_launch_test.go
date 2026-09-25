package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMProfileLaunchAndAcceptedRetry(t *testing.T) {
	profiles, _, _, connectionsDB := setupAIProfileTestDB(t)
	ctx := context.Background()
	f.Exec(t, connectionsDB, "INSERT INTO workspace_members(workspace_id,user_id,status) VALUES (?,?,'active')", f.Workspace, f.SalesUser)
	f.Exec(t, connectionsDB, "INSERT INTO workspaces(id) VALUES (?)", f.Workspace)
	connection := &model.AIConnection{ID: uuid.NewString(), WorkspaceID: f.Workspace, Scope: "workspace", Name: "Reviewed key", Provider: "openai", Status: "connected"}
	if err := profiles.connections.seal(connection, aiConnectionSecret{APIKey: "reviewed-key"}); err != nil {
		t.Fatal(err)
	}
	if err := profiles.connections.repo.Create(ctx, connection); err != nil {
		t.Fatal(err)
	}
	profile := &model.AIProfile{ID: uuid.NewString(), WorkspaceID: f.Workspace, Scope: "workspace", Name: "Reviewed", Revision: 1, Primary: model.AIProfileRoute{ConnectionID: connection.ID, Model: sdk.RunModel{Provider: "openai", Model: "reviewed-model"}}}
	if err := profiles.repo.Create(ctx, profile); err != nil {
		t.Fatal(err)
	}
	if err := profiles.repo.SetDefault(ctx, f.Workspace, profile); err != nil {
		t.Fatal(err)
	}
	db, host, binding, _ := boundHostDefinitionFixture(t, 0, playbookDefinition(0), profiles)
	f.Exec(t, db, "DELETE FROM agent_runs WHERE id=?", binding.RunID)
	calls := 0
	profiles.SetAdmissionPolicy(connectionPolicyFunc(func(context.Context, string, *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
		calls++
		if calls > 1 {
			return nil, errors.New("BYOK disabled after acceptance")
		}
		return &model.AIExecutionPolicySnapshot{Mode: "ee", FundingMode: aiusage.FundingCustomerFlat, FlatTariff: testFlatTariff(2_000_000)}, nil
	}))
	client := &playbookRuntimeEcho{lostResponse: true}
	agents := (&AgentService{agentRepo: host.agentRepo, runRepo: host.runRepo}).SetAgentRuntimeClient(client).SetAgentRuntimeLaunchEnabled(true).SetAIConnectionService(profiles.connections).SetAIProfileService(profiles)
	consumer := &recordingAIUsageConsumer{}
	launcher := NewCRMPlaybookAgentLauncher(agents, repository.NewCRMPlaybookExecutionRepository(db), NewTokenPricedAIUsageMeter(consumer))
	launcher.SetExecutionService(host.playbookExecution)
	host.playbookExecution.launcher = launcher
	run, err := launcher.StartPlaybookRun(ctx, binding)
	if !errors.Is(err, ErrCRMPlaybookLaunchUncertain) {
		t.Fatalf("launch: %v", err)
	}
	if len(client.startRunCalls) != 1 {
		t.Fatal("runtime not called")
	}
	request := client.startRunCalls[0]
	if request.Model == nil || request.Model.Model != "reviewed-model" || request.ModelCredential == nil || request.ModelCredential.APIKey != "reviewed-key" {
		t.Fatal("CRM lost reviewed model or credential")
	}
	var accepted model.AgentRunInputPayload
	if err := json.Unmarshal(run.Input, &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.AISelection == nil || accepted.AISelection.Policy.FlatTariff == nil {
		t.Fatal("missing accepted tariff")
	}
	consumer.preflightErr = errors.New("must not preflight twice")
	replay, err := launcher.StartPlaybookRun(ctx, binding)
	if err != nil || replay.ID != run.ID || replay.ExternalRuntimeID == nil {
		t.Fatalf("accepted retry: %v", err)
	}
	if calls != 1 || len(client.startRunCalls) != 1 {
		t.Fatal("accepted retry readmitted or resubmitted execution")
	}
}
