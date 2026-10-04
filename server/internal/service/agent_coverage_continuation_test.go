package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCompletedCoverageRunAcceptsFollowupAndRetainsPriorContext(t *testing.T) {
	profiles, primary, _, _ := setupAIProfileTestDB(t)
	ctx := context.Background()
	profile, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Team", Scope: "workspace", Primary: primary})
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
	run, err := svc.createRun(ctx, createRunParams{workspaceID: "workspace", actorID: strPtr("owner"), aiProfileID: profile.ID, agent: agent, targetType: "support_coverage_gap", targetID: "gap", input: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	run.Status = model.AgentRunStatusCompleted
	if err := svc.runRepo.Update(ctx, run); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ContinueTerminalRun(ctx, "workspace", run.ID, "owner", model.ContinueAgentRunRequest{}); err == nil {
		t.Fatal("continued completed work without a new request")
	}
	agent.AllowedTargets = json.RawMessage(`["workspace","support_coverage_gap"]`)
	agent.AllowedTools = json.RawMessage(`["complete_support_coverage_gap"]`)
	if err := db.Model(&model.Agent{}).Where("id = ?", agent.ID).Updates(map[string]any{"allowed_targets": agent.AllowedTargets, "allowed_tools": agent.AllowedTools}).Error; err != nil {
		t.Fatal(err)
	}
	_, coverage, coverageDB := setupCoverageTestEnv(t)
	if err := coverageDB.Create(&model.SupportCoverageGap{ID: "gap", WorkspaceID: "workspace", DedupeKey: "gap", Title: "Invoice corrections", Status: "open", Metadata: json.RawMessage(`{}`)}).Error; err != nil {
		t.Fatal(err)
	}
	svc.SetSupportCoverageService(coverage)
	if err := coverageDB.Model(&model.SupportCoverageGap{}).Where("id = ?", "gap").Update("status", "done").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ContinueTerminalRun(ctx, "workspace", run.ID, "owner", model.ContinueAgentRunRequest{Content: strPtr("Keep the draft internal.")}); err == nil {
		t.Fatal("started new assistant work on a closed gap")
	}
	if err := coverageDB.Model(&model.SupportCoverageGap{}).Where("id = ?", "gap").Update("status", "open").Error; err != nil {
		t.Fatal(err)
	}
	continued, err := svc.ContinueTerminalRun(ctx, "workspace", run.ID, "owner", model.ContinueAgentRunRequest{Content: strPtr("Keep the draft internal.")})
	if err != nil {
		t.Fatal(err)
	}
	if continued.TargetID != run.TargetID || continued.ParentRunID == nil || *continued.ParentRunID != run.ID {
		t.Fatal("follow-up lost its gap or prior run")
	}
	if !strings.Contains(string(continued.Input), "Keep the draft internal") || !strings.Contains(string(continued.Input), run.ID) {
		t.Fatal("follow-up lost its request or prior context")
	}
}
