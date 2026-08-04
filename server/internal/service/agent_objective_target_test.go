package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestStartTargetRunObjective(t *testing.T) {
	env := newPMObjectiveCommandTestEnv(t)
	createAgentRunActivityTables(t, env.db)
	mustExec(t, env.db, `CREATE TABLE agent_team_access (agent_id TEXT NOT NULL, team_id TEXT NOT NULL, created_at DATETIME, PRIMARY KEY (agent_id, team_id))`)
	env.seedObjective(t, "objective-run-target", env.workspaceID, "Grow retention", []string{env.teamA, env.teamB})
	env.seedObjective(t, "objective-run-target-2", env.workspaceID, "Grow activation", []string{env.teamA, env.teamB})
	env.seedObjective(t, "objective-run-other", env.otherWorkspaceID, "Other objective", []string{env.teamOther})

	agentRepo := repository.NewAgentRepository(env.db)
	if err := agentRepo.Create(context.Background(), &model.Agent{
		ID: "agent-objective-run", WorkspaceID: env.workspaceID, Name: "Objective Runner", Role: "Objective operator",
		Status: "idle", RuntimeKind: "native_sdk", Skills: model.AgentSkillRefs{}, TriggerMode: "manual",
		ExecutionConfig: model.JSONBlob(`{}`), AllowedTools: json.RawMessage(`[]`), AllowedCommands: json.RawMessage(`[]`),
		AllowedTargets: json.RawMessage(`["objective"]`), ApprovalMode: "never", MaxConcurrentRuns: 1,
		DefaultInvocationMode: model.InvocationModeAutonomous, TeamIDs: []string{"objective-team-unrelated", env.teamB},
	}); err != nil {
		t.Fatalf("seed objective agent: %v", err)
	}
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	agentService := (&AgentService{agentRepo: agentRepo, runRepo: repository.NewAgentRunRepository(env.db)}).
		SetPMObjectiveService(env.objectives).
		SetAgentRuntimeClient(runtimeClient).
		SetAgentRuntimeLaunchEnabled(true)
	run, err := agentService.StartTargetRun(context.Background(), env.workspaceID, "objective", "objective-run-target", model.StartAgentRunRequest{AgentID: "agent-objective-run"}, "objective-admin")
	if err != nil {
		t.Fatalf("StartTargetRun: %v", err)
	}
	if run.TargetType != "objective" || run.TargetID != "objective-run-target" || len(runtimeClient.startRunCalls) != 1 || runtimeClient.startRunCalls[0].Target.Type != "objective" {
		t.Fatalf("unexpected objective launch: run=%#v calls=%#v", run, runtimeClient.startRunCalls)
	}

	if err := agentRepo.Create(context.Background(), &model.Agent{
		ID: "agent-objective-wrong-team", WorkspaceID: env.workspaceID, Name: "Wrong Team", Status: "idle", RuntimeKind: "native_sdk",
		Skills: model.AgentSkillRefs{}, TriggerMode: "manual", ExecutionConfig: model.JSONBlob(`{}`), AllowedTools: json.RawMessage(`[]`),
		AllowedCommands: json.RawMessage(`[]`), AllowedTargets: json.RawMessage(`["objective"]`), ApprovalMode: "never", MaxConcurrentRuns: 1,
		DefaultInvocationMode: model.InvocationModeAutonomous, TeamIDs: []string{"objective-team-unrelated"},
	}); err != nil {
		t.Fatalf("seed wrong-team agent: %v", err)
	}
	if _, err := agentService.StartTargetRun(context.Background(), env.workspaceID, "objective", "objective-run-target-2", model.StartAgentRunRequest{AgentID: "agent-objective-wrong-team"}, "objective-admin"); err == nil || !strings.Contains(err.Error(), "cannot run on objective targets") {
		t.Fatalf("objective team mismatch error = %v", err)
	}
	if err := agentRepo.Create(context.Background(), &model.Agent{
		ID: "agent-objective-task-only", WorkspaceID: env.workspaceID, Name: "Task only", Status: "idle", RuntimeKind: "native_sdk",
		Skills: model.AgentSkillRefs{}, TriggerMode: "manual", ExecutionConfig: model.JSONBlob(`{}`), AllowedTools: json.RawMessage(`[]`),
		AllowedCommands: json.RawMessage(`[]`), AllowedTargets: json.RawMessage(`["task"]`), ApprovalMode: "never", MaxConcurrentRuns: 1,
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}); err != nil {
		t.Fatalf("seed task-only agent: %v", err)
	}
	if _, err := agentService.StartTargetRun(context.Background(), env.workspaceID, "objective", "objective-run-target-2", model.StartAgentRunRequest{AgentID: "agent-objective-task-only"}, "objective-admin"); err == nil || !strings.Contains(err.Error(), "agent can only be assigned") {
		t.Fatalf("objective allowed-target error = %v", err)
	}
	if _, err := agentService.StartTargetRun(context.Background(), env.workspaceID, "objective", "objective-run-other", model.StartAgentRunRequest{AgentID: "agent-objective-run"}, "objective-admin"); err == nil || !strings.Contains(err.Error(), "objective not found") {
		t.Fatalf("cross-workspace objective error = %v", err)
	}
}

func TestAgentRuntimeHostResolveObjectiveTarget(t *testing.T) {
	env := newPMObjectiveCommandTestEnv(t)
	now := time.Now().UTC()
	color := "#336699"
	if err := env.db.Create(&model.PMLabel{ID: "objective-runtime-label", WorkspaceID: env.workspaceID, TeamID: &env.teamA, Name: "Growth", Color: &color}).Error; err != nil {
		t.Fatalf("seed label: %v", err)
	}
	mustExec(t, env.db, `INSERT INTO pm_epics (id, workspace_id, name, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, "objective-runtime-epic", env.workspaceID, "Activation epic", env.teamA, false, now, now)
	start, deadline := commandMustDate(t, "2026-09-01"), commandMustDate(t, "2026-12-31")
	description, healthComment := "Increase retained teams", "Leading indicators healthy"
	objective, err := env.objectives.Create(context.Background(), model.CreateObjectiveRequest{
		WorkspaceID: env.workspaceID, Name: "Retention", Description: &description, ObjectiveType: model.PMObjectiveTypeStrategic,
		State: strPtr(model.PMObjectiveStateActive), PlannedStartDate: &start, Deadline: &deadline,
		Health: strPtr(model.PMObjectiveHealthOnTrack), HealthComment: &healthComment,
		TeamIDs: []string{env.teamA, env.teamB}, OwnerMemberIDs: []string{"objective-member-owner"},
		LabelIDs: []string{"objective-runtime-label"}, EpicIDs: []string{"objective-runtime-epic"},
	}, "objective-admin")
	if err != nil {
		t.Fatalf("create objective: %v", err)
	}
	if _, err := env.objectives.CreateKeyResult(context.Background(), objective.Objective.ID, model.CreateKeyResultRequest{
		Name: "Weekly retained teams", ResultType: model.PMKeyResultTypeNumeric, InitialValue: 10, CurrentValue: 20, TargetValue: 50,
	}, "objective-admin"); err != nil {
		t.Fatalf("create key result: %v", err)
	}

	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetPMObjectiveService(env.objectives)
	resolved, err := host.ResolveTargetContext(context.Background(), agentruntime.TargetContextRequest{
		AppID: "helpin", Target: agentruntime.TargetRef{Type: "objective", ID: objective.Objective.ID},
		Metadata: map[string]interface{}{"workspace_id": env.workspaceID},
	})
	if err != nil {
		t.Fatalf("ResolveTargetContext: %v", err)
	}
	if resolved.Summary != "Objective: Retention" || resolved.Target.Display == nil || resolved.Target.Display.Title != "Retention" {
		t.Fatalf("unexpected objective display: %#v", resolved)
	}
	for key, want := range map[string]any{
		"description": description, "objective_type": model.PMObjectiveTypeStrategic, "state": model.PMObjectiveStateActive,
		"planned_start_date": "2026-09-01", "deadline": "2026-12-31", "health": model.PMObjectiveHealthOnTrack,
		"health_comment": healthComment,
	} {
		if resolved.Data[key] != want {
			t.Errorf("objective context %s = %#v, want %#v", key, resolved.Data[key], want)
		}
	}
	for _, key := range []string{"teams", "owners", "owner_member_ids", "labels", "epics", "key_results", "stats"} {
		if resolved.Data[key] == nil {
			t.Errorf("objective context missing %s: %#v", key, resolved.Data)
		}
	}

	_, err = host.ResolveTargetContext(context.Background(), agentruntime.TargetContextRequest{
		AppID: "helpin", Target: agentruntime.TargetRef{Type: "objective", ID: objective.Objective.ID},
		Metadata: map[string]interface{}{"workspace_id": env.otherWorkspaceID},
	})
	if !errors.Is(err, ErrAgentRuntimeHostForbidden) {
		t.Fatalf("cross-workspace objective error = %v, want forbidden", err)
	}
}

func TestEnrichRunTargetsResolvesObjectiveTitle(t *testing.T) {
	env := newPMObjectiveCommandTestEnv(t)
	env.seedObjective(t, "objective-run-title", env.workspaceID, "Objective title", []string{env.teamA})
	svc := (&AgentService{}).SetPMObjectiveService(env.objectives)
	runs := []model.AgentRun{{WorkspaceID: env.workspaceID, TargetType: "objective", TargetID: "objective-run-title"}}
	svc.enrichRunTargets(context.Background(), env.workspaceID, runs)
	if runs[0].TargetInfo == nil || runs[0].TargetInfo.Title != "Objective title" {
		t.Fatalf("objective target info = %#v", runs[0].TargetInfo)
	}
}
