package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestAgentAIProfileLaunchAndAcceptedRunReuse(t *testing.T) {
	profiles, primary, fallback := setupAIProfileTest(t)
	ctx := context.Background()
	p, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Team", Scope: "workspace", Primary: primary, Fallback: &fallback})
	if err != nil {
		t.Fatal(err)
	}
	if err := profiles.SetDefault(ctx, "workspace", "owner", p.ID); err != nil {
		t.Fatal(err)
	}
	db := setupAutomationDelegationTestDB(t)
	agentID := "22222222-2222-2222-2222-222222222222"
	seedDelegatingWorkspaceAgent(t, db, agentID, "workspace")
	agents := repository.NewAgentRepository(db)
	a, err := agents.GetByID(ctx, "workspace", agentID)
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeAgentRuntimeSignalClient{}
	s := (&AgentService{agentRepo: agents, runRepo: repository.NewAgentRunRepository(db)}).
		SetAIConnectionService(profiles.connections).SetAIProfileService(profiles).
		SetAgentRuntimeClient(client).SetAgentRuntimeLaunchEnabled(true)
	params := createRunParams{workspaceID: "workspace", agent: a, targetType: "workspace", targetID: "workspace", input: []byte(`{}`), trigger: &model.AgentRunTriggerContext{Source: "scheduled"}}
	run, err := s.createRun(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	if len(client.startRunCalls) != 1 {
		t.Fatal("runtime was not called")
	}
	request := client.startRunCalls[0]
	if request.Model == nil || request.Model.Model != "custom-unpriced-model" || request.ModelCredential == nil || request.ModelCredential.APIKey != "Primary-key" {
		t.Fatal("scheduled launch did not send resolved model and credential")
	}
	var input model.AgentRunInputPayload
	if err := json.Unmarshal(run.Input, &input); err != nil {
		t.Fatal(err)
	}
	if input.AISelection == nil || input.AISelection.ProfileID != p.ID || strings.Contains(string(run.Input), "Primary-key") {
		t.Fatal("missing snapshot or persisted secret")
	}
	// The default disappears, but an already accepted run still uses its binding.
	if err := profiles.Delete(ctx, "workspace", "owner", p.ID, p.Revision); err != nil {
		t.Fatal(err)
	}
	reused, err := s.createRun(ctx, params)
	if err != nil || reused.ID != run.ID || len(client.startRunCalls) != 1 {
		t.Fatalf("active run resolved an edited default: %v", err)
	}
}

func TestAgentAIProfileControlsReplaceOnlyModelControls(t *testing.T) {
	profiles, primary, _ := setupAIProfileTest(t)
	ctx := context.Background()
	p, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Team", Scope: "workspace", Primary: primary})
	if err != nil {
		t.Fatal(err)
	}
	s := (&AgentService{}).SetAIConnectionService(profiles.connections).SetAIProfileService(profiles)
	params := createRunParams{workspaceID: "workspace", actorID: strPtr("owner"), aiProfileID: p.ID, input: []byte(`{}`), agent: &model.Agent{ExecutionConfig: model.JSONBlob(`{"reasoning_effort":"high","service_tier":"fast","max_tool_steps":80,"native_context":{"enabled":true}}`)}}
	selected, cred, billing, err := s.prepareAIConnectionRun(ctx, &params)
	if err != nil {
		t.Fatal(err)
	}
	if derefString(selected.Controls.ReasoningEffort) != "low" || selected.Controls.ServiceTier != nil || cred.APIKey != "Primary-key" {
		t.Fatal("legacy controls overrode profile controls")
	}
	var config map[string]any
	if err := json.Unmarshal(billing.ExecutionConfig, &config); err != nil {
		t.Fatal(err)
	}
	if config["max_tool_steps"] != float64(80) || config["native_context"] == nil || config["reasoning_effort"] != "low" || config["service_tier"] != nil {
		t.Fatalf("execution settings changed incorrectly: %s", billing.ExecutionConfig)
	}
	if err := s.validateSharedAIProfile(ctx, "other", &p.ID); err == nil {
		t.Fatal("cross-workspace default accepted")
	}
}
