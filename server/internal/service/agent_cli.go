package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type localRunPreparation struct {
	runID, connectionID, executionID string
	review                           bool
	validate                         func(context.Context, *model.AgentRun) error
}

// AdmitLocalRun shares target validation, agent materialization, model selection,
// transcript creation, and billing preflight with cloud launches, without dispatch.
func (s *AgentService) AdmitLocalRun(ctx context.Context, c *model.CLIConnection, e *model.CLIExecution, req model.CLIAdmissionRequest, actor *authorization.Actor, validate func(context.Context, *model.AgentRun) error) (*model.AgentRun, error) {
	targetType, targetID, ok := strings.Cut(req.Target, ":")
	if !ok || targetID == "" {
		return nil, ErrCLIInvalid
	}
	switch targetType {
	case "task", "repository", "workspace":
	default:
		return nil, ErrCLIInvalid
	}
	if err := s.RequireActorCanUseAgent(ctx, c.WorkspaceID, req.AgentID, actor); err != nil {
		return nil, ErrCLIForbidden
	}
	if targetType == "workspace" && targetID != c.WorkspaceID {
		return nil, ErrCLIForbidden
	}
	if targetType == "task" {
		task, err := s.taskRepo.GetRawByID(ctx, targetID)
		if err != nil {
			return nil, err
		}
		if task == nil || task.WorkspaceID != c.WorkspaceID {
			return nil, ErrCLIForbidden
		}
		if actor.Role != "owner" && actor.Role != "admin" && task.TeamID != nil && !actor.IsMemberOfTeam(*task.TeamID) {
			return nil, ErrCLIForbidden
		}
	}
	local := &localRunPreparation{runID: e.RunID, connectionID: c.ID, executionID: e.ID, review: req.Review, validate: validate}
	return s.startTargetRunWithOptions(ctx, c.WorkspaceID, targetType, targetID, model.StartAgentRunRequest{AgentID: req.AgentID, AdditionalContext: &req.Instructions, DeliveryMode: "preview"}, &c.UserID, manualRunTriggerContext(), nil, nil, startTargetRunOptions{local: local})
}
func localAgentPolicy(agent *model.Agent, review bool) (model.CLIAgentSnapshot, []string, error) {
	if agent == nil || agent.RuntimeKind != "native_sdk" {
		return model.CLIAgentSnapshot{}, nil, ErrCLIInvalid
	}
	var saved struct {
		Workspace struct {
			Access string `json:"access"`
		} `json:"workspace"`
	}
	if len(agent.ExecutionConfig) > 0 {
		if err := json.Unmarshal(agent.ExecutionConfig, &saved); err != nil {
			return model.CLIAgentSnapshot{}, nil, ErrCLIInvalid
		}
	}
	review = review || saved.Workspace.Access == "read_only"
	supported := map[string]bool{"read_files": true, "list_directory": true, "repository_search": true, "run_command": true, "request_user_input": true, "request_approval": true}
	if !review {
		supported["write_file"] = true
		supported["edit_file"] = true
		supported["apply_patch"] = true
	}
	allowed := make([]string, 0)
	for _, name := range agentcontract.NormalizeToolNames(parseJSONStringSlice(agent.AllowedTools)) {
		if supported[name] {
			allowed = append(allowed, name)
		}
	}
	if len(allowed) == 0 {
		return model.CLIAgentSnapshot{}, nil, ErrCLIInvalid
	}
	profile := agentcontract.ResolveAgentProfile(agent, resolveInvocationMode(agent))
	approval := profile.ApprovalMode
	if profile.ApprovalRequired && approval == "never" {
		approval = "mutating_tools"
	}
	if approval == "risk_based" {
		approval = "mutating_tools"
	}
	switch approval {
	case "never", "mutating_tools", "always":
	case "":
		approval = "mutating_tools"
	default:
		return model.CLIAgentSnapshot{}, nil, ErrCLIInvalid
	}
	access := "read_write"
	if review {
		access = "read_only"
	}
	config, err := json.Marshal(map[string]any{"workspace": map[string]string{"mode": "host_prepared", "access": access}})
	if err != nil {
		return model.CLIAgentSnapshot{}, nil, err
	}
	return model.CLIAgentSnapshot{ID: agent.ID, Name: agent.Name, RuntimeKind: "native_sdk", SystemPrompt: runtimeAgentFromHelpinAgent(agent, "helpin").SystemPrompt, ApprovalMode: approval, ExecutionConfig: config}, allowed, nil
}
func prepareLocalRun(run *model.AgentRun, p createRunParams) error {
	snapshot, allowed, err := localAgentPolicy(p.agent, p.local.review)
	if err != nil {
		return err
	}
	admission := model.CLIAdmission{RunID: run.ID, Agent: snapshot, AllowedTools: allowed, Context: initialAgentRunContext(run)}
	var input model.AgentRunInputPayload
	if err = json.Unmarshal(run.Input, &input); err != nil {
		return err
	}
	input.ExecutionLocation = "local"
	input.LocalExecution = &model.CLILocalRun{ConnectionID: p.local.connectionID, ExecutionID: p.local.executionID, Admission: admission}
	input.DeliveryMode = "preview"
	input.AllowedTools = allowed
	run.Input, err = json.Marshal(input)
	return err
}
func (s *AgentService) cancelLocalRun(ctx context.Context, run *model.AgentRun, actorID string) (*model.AgentRun, error) {
	now := time.Now().UTC()
	run.Status = "cancelled"
	run.CompletedAt = &now
	run.PauseReason = model.AgentRunPauseReasonNone
	run.ExecutionStage = strPtr("cancelled")
	if s.aiUsageMeter != nil && s.aiUsageMeter.usage != nil {
		if metering, ok := agentRunMeteringContext(run); ok && metering.ReservationID != "" {
			if err := s.aiUsageMeter.usage.Release(ctx, metering.ReservationID, "local_admission_cancelled"); err != nil {
				return nil, err
			}
		}
	}
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}
	s.publishRunEvent(run, actorID)
	return run, nil
}
