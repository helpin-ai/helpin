package service

import (
	"context"
	"errors"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"testing"
)

type playbookRuntimeEcho struct {
	fakeAgentRuntimeSignalClient
	lostResponse bool
}

func (c *playbookRuntimeEcho) StartRun(_ context.Context, req AgentRuntimeStartRunRequest) (*AgentRuntimeRun, error) {
	c.startRunCalls = append(c.startRunCalls, req)
	run := &AgentRuntimeRun{ID: "remote-" + req.HostRunID, HostRunID: req.HostRunID, AppID: c.AppID(), AgentID: req.AgentID, Target: req.Target, Status: "running"}
	c.getRuns = map[string]*AgentRuntimeRun{req.HostRunID: run}
	if c.lostResponse {
		return nil, errors.New("response lost after provider acceptance")
	}
	return run, nil
}

func TestCRMPlaybookLauncherUsesExistingRuntimeOnceForAllJourneys(t *testing.T) {
	for index, name := range []string{"buying_intent", "sales_handoff", "renewal_recovery"} {
		for _, uncertain := range []bool{false, true} {
			t.Run(name+map[bool]string{false: " confirmed", true: " uncertain"}[uncertain], func(t *testing.T) {
				db, host, binding, _ := boundHostFixture(t, index)
				// Remove only the fixture's pre-seeded run: the actual launcher
				// must create the normal host run from its durable reservation.
				f.Exec(t, db, "DELETE FROM agent_runs WHERE id=?", binding.RunID)
				client := &playbookRuntimeEcho{lostResponse: uncertain}
				agents := (&AgentService{agentRepo: host.agentRepo, runRepo: host.runRepo}).SetAgentRuntimeClient(client).SetAgentRuntimeLaunchEnabled(true)
				consumer := &recordingAIUsageConsumer{}
				launcher := NewCRMPlaybookAgentLauncher(agents, repository.NewCRMPlaybookExecutionRepository(db), NewTokenPricedAIUsageMeter(consumer))
				launcher.SetExecutionService(host.playbookExecution)
				host.playbookExecution.launcher = launcher
				run, err := launcher.StartPlaybookRun(context.Background(), binding)
				// Accepted executions must not re-enter billing admission on retry.
				consumer.preflightErr = errors.New("billing configuration changed after acceptance")
				if uncertain {
					if !errors.Is(err, ErrCRMPlaybookLaunchUncertain) {
						t.Fatalf("unknown start: %v", err)
					}
					// Wrong private profile cannot be correlated even with the
					// right host ID. Recovery is read-only and never starts twice.
					remote := client.getRuns[binding.RunID]
					remote.AgentID = binding.AgentID
					if _, err := launcher.StartPlaybookRun(context.Background(), binding); !errors.Is(err, ErrCRMPlaybookLaunchUncertain) {
						t.Fatalf("wrong profile accepted: %v", err)
					}
					remote.AgentID = binding.RuntimeProfileID
					f.Exec(t, db, "UPDATE crm_playbook_automation_bindings SET blocker='start_uncertain' WHERE situation_id=?", binding.SituationID)
				} else if err != nil {
					t.Fatal(err)
				}
				run, err = launcher.StartPlaybookRun(context.Background(), binding)
				if err != nil || run.ExternalRuntimeID == nil {
					t.Fatalf("could not correlate run: %#v %v", run, err)
				}
				live, err := repository.NewCRMPlaybookExecutionRepository(db).Binding(context.Background(), f.Workspace, binding.SituationID)
				if err != nil || live.Blocker == "start_uncertain" {
					t.Fatalf("confirmed recovery kept an uncertain-result warning: %#v %v", live, err)
				}
				if len(client.startRunCalls) != 1 || len(client.upsertAgents) != 1 {
					t.Fatal("replayed launch created another execution")
				}
				if run.AgentID != binding.AgentID || client.startRunCalls[0].AgentID != binding.RuntimeProfileID {
					t.Fatal("shared Beacon identity was replaced")
				}
				if client.startRunCalls[0].TurnPolicy.Mode != agentRuntimeTurnCompleteOnFinish {
					t.Fatal("ordinary conversational approval path used")
				}
				item, err := repository.NewCRMSituationRepository(db).GetByID(context.Background(), f.Workspace, binding.SituationID)
				if err != nil || item.Situation.Lifecycle != "open" {
					t.Fatal("launch changed customer outcome")
				}
				for _, milestone := range item.Situation.PlaybookMilestones {
					if milestone.Status != "pending" {
						t.Fatal("launch claimed customer progress")
					}
				}
			})
		}
	}
}

func TestCRMPlaybookRuntimeCorrelationRejectsEveryScopeMismatch(t *testing.T) {
	b := model.AutomationRunBinding{RuntimeProfileID: "profile"}
	run := model.AgentRun{ID: "host", TargetType: "crm_company", TargetID: "company"}
	remote := AgentRuntimeRun{ID: "remote", HostRunID: "host", AppID: "helpin", AgentID: "profile", Target: AgentRuntimeTargetRef{Type: "crm_company", ID: "company"}}
	if !matchesPlaybookRuntimeRun(&remote, &run, &b, "helpin") {
		t.Fatal("valid correlation rejected")
	}
	for _, change := range []func(*AgentRuntimeRun){func(r *AgentRuntimeRun) { r.AppID = "other" }, func(r *AgentRuntimeRun) { r.HostRunID = "other" }, func(r *AgentRuntimeRun) { r.AgentID = "other" }, func(r *AgentRuntimeRun) { r.Target.ID = "other" }, func(r *AgentRuntimeRun) { r.Target.Type = "workspace" }} {
		wrong := remote
		change(&wrong)
		if matchesPlaybookRuntimeRun(&wrong, &run, &b, "helpin") {
			t.Fatal("scope mismatch accepted")
		}
	}
}
