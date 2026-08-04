package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestStartTargetRunSprint(t *testing.T) {
	sprintService, db, workspaceID := newSprintTestEnvWithDB(t)
	createAgentRunActivityTables(t, db)
	seedPMSprintCommandTeam(t, db, workspaceID, "team-run-sprint")
	teamID := "team-run-sprint"
	sprint, err := sprintService.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: workspaceID,
		Name:        "Run Sprint",
		StartDate:   commandMustDate(t, "2027-03-01"),
		EndDate:     commandMustDate(t, "2027-03-15"),
		TeamID:      &teamID,
	}, "actor-1")
	if err != nil {
		t.Fatalf("create sprint: %v", err)
	}

	if err := db.Create(&model.Agent{
		ID:                    "agent-sprint-run",
		WorkspaceID:           workspaceID,
		Name:                  "Sprint Runner",
		Role:                  "Sprint operator",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		Skills:                model.AgentSkillRefs{},
		TriggerMode:           "manual",
		ExecutionConfig:       model.JSONBlob(`{}`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        json.RawMessage(`["sprint"]`),
		ApprovalMode:          "never",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}).Error; err != nil {
		t.Fatalf("seed sprint agent: %v", err)
	}

	runtimeClient := &fakeAgentRuntimeSignalClient{}
	activityRepo := repository.NewPMActivityRepository(db)
	agentService := (&AgentService{
		agentRepo:   repository.NewAgentRepository(db),
		runRepo:     repository.NewAgentRunRepository(db),
		activitySvc: NewPMActivityService(activityRepo),
	}).SetPMSprintService(sprintService).
		SetAgentRuntimeClient(runtimeClient).
		SetAgentRuntimeLaunchEnabled(true)

	run, err := agentService.StartTargetRun(context.Background(), workspaceID, "sprint", sprint.Sprint.ID, model.StartAgentRunRequest{AgentID: "agent-sprint-run"}, "actor-1")
	if err != nil {
		t.Fatalf("StartTargetRun: %v", err)
	}
	if run.TargetType != "sprint" || run.TargetID != sprint.Sprint.ID {
		t.Fatalf("unexpected persisted target: %#v", run)
	}
	if len(runtimeClient.startRunCalls) != 1 || runtimeClient.startRunCalls[0].Target.Type != "sprint" || runtimeClient.startRunCalls[0].Target.ID != sprint.Sprint.ID {
		t.Fatalf("unexpected runtime launch: %#v", runtimeClient.startRunCalls)
	}
	entries, total, err := activityRepo.List(context.Background(), "sprint", sprint.Sprint.ID, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("list sprint activity: %v", err)
	}
	if total < 2 || entries[0].Activity.Action != "updated" {
		t.Fatalf("expected sprint run activity after create, got total=%d entries=%#v", total, entries)
	}

	if err := db.Create(&model.Agent{
		ID: "agent-task-only", WorkspaceID: workspaceID, Name: "Task only", Status: "idle", RuntimeKind: "native_sdk",
		Skills: model.AgentSkillRefs{}, TriggerMode: "manual", ExecutionConfig: model.JSONBlob(`{}`), AllowedTools: json.RawMessage(`[]`),
		AllowedCommands: json.RawMessage(`[]`), AllowedTargets: json.RawMessage(`["task"]`), ApprovalMode: "never", MaxConcurrentRuns: 1,
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}).Error; err != nil {
		t.Fatalf("seed task-only agent: %v", err)
	}
	_, err = agentService.StartTargetRun(context.Background(), workspaceID, "sprint", sprint.Sprint.ID, model.StartAgentRunRequest{AgentID: "agent-task-only"}, "actor-1")
	if err == nil || !strings.Contains(err.Error(), "agent can only be assigned") {
		t.Fatalf("expected target allowance error, got %v", err)
	}

	seedWorkspace(t, db, "ws-run-sprint-other", "Other", "run-sprint-other", "owner-2")
	seedPMSprintCommandTeam(t, db, "ws-run-sprint-other", "team-run-sprint-other")
	otherTeamID := "team-run-sprint-other"
	other, err := sprintService.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: "ws-run-sprint-other", Name: "Other", StartDate: commandMustDate(t, "2027-04-01"), EndDate: commandMustDate(t, "2027-04-15"), TeamID: &otherTeamID,
	}, "actor-2")
	if err != nil {
		t.Fatalf("create other sprint: %v", err)
	}
	_, err = agentService.StartTargetRun(context.Background(), workspaceID, "sprint", other.Sprint.ID, model.StartAgentRunRequest{AgentID: "agent-sprint-run"}, "actor-1")
	if err == nil || !strings.Contains(err.Error(), "sprint not found") {
		t.Fatalf("expected workspace isolation error, got %v", err)
	}
}
