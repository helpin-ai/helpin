package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestNewInheritedRunUsesCurrentPolicy(t *testing.T) {
	for _, child := range []bool{false, true} {
		name := "continuation"
		if child {
			name = "personal child"
		}
		t.Run(name, func(t *testing.T) {
			profiles, primary, _, db := setupAIProfileTestDB(t)
			ctx := context.Background()
			scope := "workspace"
			if child {
				c, err := profiles.connections.Create(ctx, "workspace", "owner", model.CreateAIConnectionRequest{Name: "Personal", Provider: "openai", APIKey: "personal-key"})
				if err != nil {
					t.Fatal(err)
				}
				primary.ConnectionID, scope = c.Connection.ID, "personal"
			}
			profile, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Selected", Scope: scope, Primary: primary})
			if err != nil {
				t.Fatal(err)
			}
			tariff := testFlatTariff(1_000_000)
			denied := errors.New("BYOK disabled")
			enabled := true
			profiles.SetAdmissionPolicy(connectionPolicyFunc(func(context.Context, string, *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
				if !enabled {
					return nil, denied
				}
				return &model.AIExecutionPolicySnapshot{Mode: "ee", FundingMode: aiusage.FundingCustomerFlat, FlatTariff: tariff}, nil
			}))
			selected, _, err := profiles.Resolve(ctx, "workspace", "owner", AIProfileSelectionRequest{ProfileID: profile.ID})
			if err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(model.AgentRunInputPayload{AISelection: selected, ModelConnectionID: selected.Route.ConnectionID, ModelName: selected.Route.Model.Model})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("INSERT INTO agent_runs (id,workspace_id,triggered_by_user_id,input,status) VALUES (?,?,?,?,?)", "parent", "workspace", "owner", payload, "completed").Error; err != nil {
				t.Fatal(err)
			}
			svc := (&AgentService{runRepo: repository.NewAgentRunRepository(db)}).SetAIConnectionService(profiles.connections).SetAIProfileService(profiles)
			input := []byte(`{"event":{"reason":"continued_from_terminal_run"}}`)
			if child {
				input = []byte(`{}`)
			}
			params := createRunParams{workspaceID: "workspace", parentRunID: strPtr("parent"), actorID: strPtr("owner"), input: input, agent: &model.Agent{}, trigger: &model.AgentRunTriggerContext{Source: "agent"}}
			tariff = testFlatTariff(2_000_000)
			tariff.Version = "rotated"
			if _, _, _, err := svc.prepareAIProfileRun(ctx, &params); err != nil {
				t.Fatal(err)
			}
			var accepted model.AgentRunInputPayload
			if err := json.Unmarshal(params.input, &accepted); err != nil {
				t.Fatal(err)
			}
			if accepted.AISelection.Policy.FlatTariff.Version != "rotated" || accepted.AISelection.Route.ConnectionID != selected.Route.ConnectionID {
				t.Fatal("new run lost route or reused old tariff")
			}
			if selected.Policy.FlatTariff.Version == "rotated" {
				t.Fatal("parent pricing mutated")
			}
			enabled = false
			if _, _, _, err := svc.prepareAIProfileRun(ctx, &params); !errors.Is(err, denied) {
				t.Fatalf("new run bypassed policy: %v", err)
			}
			if _, err := profiles.Restore(ctx, "workspace", "owner", selected); err != nil {
				t.Fatalf("existing run cannot resume: %v", err)
			}
		})
	}
}

func TestLegacyProfileLaunchValidation(t *testing.T) {
	for _, scenario := range []string{"valid", "conflict", "model without connection", "removed automated actor", "disconnected"} {
		t.Run(scenario, func(t *testing.T) {
			profiles, primary, _, db := setupAIProfileTestDB(t)
			svc := (&AgentService{}).SetAIConnectionService(profiles.connections).SetAIProfileService(profiles)
			params := createRunParams{workspaceID: "workspace", actorID: strPtr("owner"), input: []byte(`{}`), agent: &model.Agent{}, modelConnectionID: primary.ConnectionID, modelName: "legacy-model"}
			switch scenario {
			case "conflict":
				params.aiProfileID = "profile"
			case "model without connection":
				params.modelConnectionID = ""
			case "removed automated actor":
				params.trigger = &model.AgentRunTriggerContext{Source: "scheduled"}
				if err := db.Exec("UPDATE workspace_members SET status='removed' WHERE user_id='owner'").Error; err != nil {
					t.Fatal(err)
				}
			case "disconnected":
				if err := profiles.connections.Disconnect(context.Background(), "workspace", "owner", primary.ConnectionID); err != nil {
					t.Fatal(err)
				}
			}
			route, cred, _, err := svc.prepareAIProfileRun(context.Background(), &params)
			if scenario != "valid" {
				if err == nil || route != nil || cred != nil {
					t.Fatal("invalid launch returned credentials")
				}
				return
			}
			if err != nil || route.Model != "legacy-model" || cred.APIKey != "Primary-key" {
				t.Fatalf("legacy route failed: %v", err)
			}
		})
	}
}

func TestAttributedUnattendedProfileRequiresActiveMember(t *testing.T) {
	profiles, primary, _, db := setupAIProfileTestDB(t)
	ctx := context.Background()
	p, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Team", Scope: "workspace", Primary: primary})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE workspace_members SET status='removed' WHERE user_id='owner'").Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := profiles.Resolve(ctx, "workspace", "owner", AIProfileSelectionRequest{ProfileID: p.ID, Unattended: true}); err == nil {
		t.Fatal("removed actor launched automation")
	}
	if _, _, err := profiles.Resolve(ctx, "workspace", "", AIProfileSelectionRequest{ProfileID: p.ID, Unattended: true}); err != nil {
		t.Fatalf("system automation depends on creator: %v", err)
	}
}

func TestPersonalProfileRejectsSharedPrimary(t *testing.T) {
	profiles, primary, _, _ := setupAIProfileTestDB(t)
	_, err := profiles.Save(context.Background(), "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Personal", Scope: "personal", Primary: primary})
	if err == nil {
		t.Fatal("shared primary accepted for personal profile")
	}
}

func TestAgentPreflightPreservesCompatibleEndpoint(t *testing.T) {
	endpoint := &sdk.ModelEndpoint{ID: "local", BaseURL: "http://127.0.0.1:8181/v1", AuthMode: "none"}
	selected := &model.AIExecutionSelection{Route: model.AIProfileRoute{Model: sdk.RunModel{Provider: "openai_compatible", Model: "custom", Endpoint: endpoint}}, Policy: &model.AIExecutionPolicySnapshot{FundingMode: aiusage.FundingCustomerFlat, FlatTariff: testFlatTariff(1_000_000)}}
	input, err := json.Marshal(model.AgentRunInputPayload{AISelection: selected})
	if err != nil {
		t.Fatal(err)
	}
	meter := &recordingAIUsageConsumer{}
	run := &model.AgentRun{ID: "run", WorkspaceID: "workspace", Input: input, OutputSummary: []byte(`{}`)}
	if err := PreflightAgentRunAIUsage(context.Background(), NewTokenPricedAIUsageMeter(meter), run, &model.Agent{}); err != nil {
		t.Fatal(err)
	}
	if !sameModelEndpoint(meter.preflight.Endpoint, endpoint) || meter.preflight.Provider != "openai_compatible" {
		t.Fatal("agent preflight dropped resolved endpoint")
	}
}

type profileRaceRuntime struct {
	fakeAgentRuntimeSignalClient
	beforeRegister, duringStart func()
}

func (c *profileRaceRuntime) UpsertAgent(ctx context.Context, a AgentRuntimeAgent) (*AgentRuntimeAgent, error) {
	if c.beforeRegister != nil {
		c.beforeRegister()
	}
	return c.fakeAgentRuntimeSignalClient.UpsertAgent(ctx, a)
}
func (c *profileRaceRuntime) StartRun(ctx context.Context, r AgentRuntimeStartRunRequest) (*AgentRuntimeRun, error) {
	if c.duringStart != nil {
		c.duringStart()
	}
	return c.fakeAgentRuntimeSignalClient.StartRun(ctx, r)
}

func TestProfileDisconnectDuringLaunch(t *testing.T) {
	for _, stage := range []string{"registration", "submission"} {
		t.Run(stage, func(t *testing.T) {
			profiles, primary, _, _ := setupAIProfileTestDB(t)
			ctx := context.Background()
			p, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Team", Scope: "workspace", Primary: primary})
			if err != nil {
				t.Fatal(err)
			}
			var revoked atomic.Int32
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				revoked.Add(1)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{}`)
			}))
			defer remote.Close()
			profiles.connections.runtime, err = NewAgentRuntimeClient(remote.URL, "helpin", "token", remote.Client())
			if err != nil {
				t.Fatal(err)
			}
			db := setupAutomationDelegationTestDB(t)
			seedDelegatingWorkspaceAgent(t, db, "agent", "workspace")
			repo := repository.NewAgentRepository(db)
			agent, err := repo.GetByID(ctx, "workspace", "agent")
			if err != nil {
				t.Fatal(err)
			}
			disconnect := func() {
				if err := profiles.connections.Disconnect(ctx, "workspace", "owner", primary.ConnectionID); err != nil {
					t.Fatal(err)
				}
			}
			client := &profileRaceRuntime{}
			if stage == "registration" {
				client.beforeRegister = disconnect
			} else {
				client.duringStart = disconnect
			}
			svc := (&AgentService{agentRepo: repo, runRepo: repository.NewAgentRunRepository(db)}).SetAIConnectionService(profiles.connections).SetAIProfileService(profiles).SetAgentRuntimeClient(client).SetAgentRuntimeLaunchEnabled(true)
			_, err = svc.createRun(ctx, createRunParams{workspaceID: "workspace", actorID: strPtr("owner"), aiProfileID: p.ID, agent: agent, targetType: "workspace", targetID: "workspace", input: []byte(`{}`)})
			if err == nil {
				t.Fatal("disconnected launch succeeded")
			}
			if stage == "registration" {
				if len(client.startRunCalls) != 0 {
					t.Fatal("disconnected key submitted")
				}
				return
			}
			if len(client.startRunCalls) != 1 || len(client.cancelCalls) != 1 || revoked.Load() != 1 {
				t.Fatalf("in-flight run not stopped: starts=%d cancels=%d revokes=%d", len(client.startRunCalls), len(client.cancelCalls), revoked.Load())
			}
		})
	}
}

func TestOrdinaryContinuationWithProfile(t *testing.T) {
	profiles, primary, _, _ := setupAIProfileTestDB(t)
	ctx := context.Background()
	p, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Team", Scope: "workspace", Primary: primary})
	if err != nil {
		t.Fatal(err)
	}
	db := setupAutomationDelegationTestDB(t)
	seedDelegatingWorkspaceAgent(t, db, "agent", "workspace")
	repo := repository.NewAgentRepository(db)
	agent, err := repo.GetByID(ctx, "workspace", "agent")
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeAgentRuntimeSignalClient{}
	svc := (&AgentService{agentRepo: repo, runRepo: repository.NewAgentRunRepository(db)}).SetAIConnectionService(profiles.connections).SetAIProfileService(profiles).SetAgentRuntimeClient(client).SetAgentRuntimeLaunchEnabled(true)
	run, err := svc.createRun(ctx, createRunParams{workspaceID: "workspace", actorID: strPtr("owner"), aiProfileID: p.ID, agent: agent, targetType: "workspace", targetID: "workspace", input: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	run.Status = model.AgentRunStatusFailed
	if err := svc.runRepo.Update(ctx, run); err != nil {
		t.Fatal(err)
	}
	continued, err := svc.ContinueTerminalRun(ctx, "workspace", run.ID, "owner", model.ContinueAgentRunRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if continued.ID == run.ID || len(client.startRunCalls) != 2 {
		t.Fatal("continuation did not create a new run")
	}
	if client.startRunCalls[1].ModelCredential.APIKey != "Primary-key" || client.startRunCalls[1].Model.Model != primary.Model.Model {
		t.Fatal("continuation lost profile binding")
	}
}

func TestDockChatLaunchWithProfile(t *testing.T) {
	profiles, primary, _, _ := setupAIProfileTestDB(t)
	ctx := context.Background()
	p, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Chat", Scope: "workspace", Primary: primary})
	if err != nil {
		t.Fatal(err)
	}
	db := setupAutomationDelegationTestDB(t)
	seedDelegatingWorkspaceAgent(t, db, "ask-agent", "workspace")
	if err := db.Exec("UPDATE agents SET preset_key=? WHERE id='ask-agent'", model.AgentPresetAskAgent).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE dock_chats (
 coverage_gap_id TEXT, initial_context TEXT,
execution_enabled boolean NOT NULL DEFAULT false,
 id TEXT PRIMARY KEY, workspace_id TEXT, user_id TEXT, title TEXT,
 visibility TEXT, module_id TEXT, support_conversation_id TEXT, active_run_id TEXT,
 next_message_sequence INTEGER, last_message_at DATETIME, archived_at DATETIME,
 created_at DATETIME, updated_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	chat := &model.DockChat{ID: "chat", WorkspaceID: "workspace", UserID: "owner", Visibility: "private"}
	if err := db.Create(chat).Error; err != nil {
		t.Fatal(err)
	}
	client := &fakeAgentRuntimeSignalClient{}
	agents := (&AgentService{agentRepo: repository.NewAgentRepository(db), runRepo: repository.NewAgentRunRepository(db)}).SetAIConnectionService(profiles.connections).SetAIProfileService(profiles).SetAgentRuntimeClient(client).SetAgentRuntimeLaunchEnabled(true)
	dock := &DockChatService{agentService: agents, chatRepo: repository.NewDockChatRepository(db)}
	if err := dock.startChatRun(ctx, chat, "owner", "Review this task", nil, nil, "", nil, "", "", p.ID); err != nil {
		t.Fatal(err)
	}
	if chat.ActiveRunID == nil || len(client.startRunCalls) != 1 {
		t.Fatal("chat run not started")
	}
	req := client.startRunCalls[0]
	if req.Model == nil || req.Model.Model != primary.Model.Model || req.ModelCredential == nil || req.ModelCredential.APIKey != "Primary-key" {
		t.Fatal("Dock lost profile selection")
	}
}
