package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	_mcpAgentRunStartsPerHour      = 10
	_mcpAgentRunActivePerUser      = 3
	_mcpAgentRunActivePerWorkspace = 10
)

type mcpStartAgentRunInput struct {
	AgentID           string  `json:"agent_id"`
	TargetType        string  `json:"target_type"`
	TargetID          string  `json:"target_id"`
	AdditionalContext *string `json:"additional_context"`
	RepositoryID      *string `json:"repository_id"`
	BaseBranch        *string `json:"base_branch"`
}

// startMCPAgentRun starts a durable agent run for the connected actor with the
// same launch contract as the app: the agent must be usable by the actor and
// allowed on the target, and repository agents need a task repository, which
// the caller may choose here exactly as the app's repository picker does.
func (s *MCPService) startMCPAgentRun(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	arguments json.RawMessage,
) (*MCPToolResult, error) {
	var input mcpStartAgentRunInput
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, err
	}
	input.TargetType = strings.TrimSpace(input.TargetType)
	input.TargetID = strings.TrimSpace(input.TargetID)
	if err := s.agents.RequireActorCanUseAgent(ctx, principal.WorkspaceID, input.AgentID, actor); err != nil {
		return nil, ErrMCPForbidden
	}
	repositoryID := strings.TrimSpace(derefString(input.RepositoryID))
	if (repositoryID != "" || input.BaseBranch != nil) && input.TargetType != "task" {
		return nil, newMCPToolError(MCPErrorCodeRepositoryNotApplicable,
			"repository_id and base_branch apply only to task targets. Remove them and retry.")
	}
	if input.BaseBranch != nil && repositoryID == "" {
		return nil, fmt.Errorf("%w: base_branch requires repository_id", ErrMCPInvalidArguments)
	}
	if repositoryID != "" && !s.principalCanWritePM(principal) {
		return nil, ErrMCPForbidden
	}
	if err := s.requireMCPAgentRunCapacity(ctx, principal); err != nil {
		return nil, err
	}
	if input.TargetType == "task" {
		taskID, err := s.resolveMCPAgentRunTaskTarget(ctx, principal, actor, input.TargetID)
		if err != nil {
			return nil, err
		}
		input.TargetID = taskID
	}
	if repositoryID != "" {
		if err := s.agents.SelectTaskRunRepository(ctx, principal.WorkspaceID, input.TargetID, input.AgentID, repositoryID, input.BaseBranch, principal.UserID); err != nil {
			return nil, mcpAgentRunStartError(err)
		}
	}
	run, err := s.agents.StartTargetRun(ctx, principal.WorkspaceID, input.TargetType, input.TargetID, model.StartAgentRunRequest{
		AgentID: input.AgentID, AdditionalContext: input.AdditionalContext,
	}, principal.UserID)
	if err != nil {
		return nil, mcpAgentRunStartError(err)
	}
	if !mcpPrincipalCanSeeRun(principal, run) {
		return nil, newMCPToolError(MCPErrorCodeRunNotOwned,
			"Another private run is already active for this agent and target. Wait for it to finish or choose another target.")
	}
	attribution := &model.MCPAgentRunAttribution{RunID: run.ID, WorkspaceID: principal.WorkspaceID, ClientName: principal.ClientName}
	if principal.ConnectionID != "" {
		attribution.ConnectionID = &principal.ConnectionID
	}
	if principal.ServicePrincipalID != "" {
		attribution.ServicePrincipalID = &principal.ServicePrincipalID
	}
	if err := s.repo.CreateRunAttribution(ctx, attribution); err != nil {
		return nil, err
	}
	return &MCPToolResult{Summary: "Agent run started. Poll get_agent_run with the returned run_id.", Data: map[string]any{
		"run_id": run.ID, "status": run.Status, "agent_id": run.AgentID,
		"target_type": run.TargetType, "target_id": run.TargetID,
	}}, nil
}

// principalCanWritePM reports whether the connection may change PM records,
// which choosing a task's delivery repository does.
func (s *MCPService) principalCanWritePM(principal *model.MCPPrincipal) bool {
	return s.config.PMWriteEnabled && !principal.ReadOnly &&
		containsMCPValue(principal.Toolsets, MCPToolsetPM) &&
		containsMCPValue(principal.Scopes, MCPScopePMWrite)
}

func (s *MCPService) requireMCPAgentRunCapacity(ctx context.Context, principal *model.MCPPrincipal) error {
	recentStarts, err := s.repo.CountRecentToolCalls(ctx, principal.WorkspaceID, principal.UserID, "start_agent_run", time.Now().Add(-time.Hour))
	if err != nil {
		return err
	}
	if recentStarts >= _mcpAgentRunStartsPerHour {
		return &MCPToolError{Code: MCPErrorCodeAgentRunLimit, Err: ErrMCPRateLimited,
			Message: "You started 10 agent runs over MCP in the last hour, the hourly limit. Retry later."}
	}
	userActive, workspaceActive, err := s.repo.CountActiveMCPAgentRuns(ctx, principal.WorkspaceID, principal.UserID)
	if err != nil {
		return err
	}
	if userActive >= _mcpAgentRunActivePerUser {
		return &MCPToolError{Code: MCPErrorCodeAgentRunLimit, Err: ErrMCPRateLimited,
			Message: "You already have 3 active agent runs started over MCP. Wait for one to finish or cancel one with cancel_agent_run."}
	}
	if workspaceActive >= _mcpAgentRunActivePerWorkspace {
		return &MCPToolError{Code: MCPErrorCodeAgentRunLimit, Err: ErrMCPRateLimited,
			Message: "This workspace already has 10 active agent runs started over MCP. Retry after some finish."}
	}
	return nil
}

// resolveMCPAgentRunTaskTarget accepts a human task key such as HEL-120 as a
// task target, matching every other MCP tool that takes a task.
func (s *MCPService) resolveMCPAgentRunTaskTarget(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	targetID string,
) (string, error) {
	if s.commands == nil || !searchTaskKeyPattern.MatchString(strings.ToUpper(targetID)) {
		return targetID, nil
	}
	taskID, err := s.commands.resolveCommandTaskKey(ctx, model.InternalCommandContext{
		WorkspaceID: principal.WorkspaceID, ActorID: principal.UserID, ActorRole: actor.Role,
	}, targetID)
	if err != nil {
		return "", newMCPToolError(MCPErrorCodeTargetNotFound, "No task with key "+strings.ToUpper(targetID)+" exists in the connected workspace.")
	}
	return taskID, nil
}

// mcpAgentRunStartError turns launch-precondition failures into typed,
// actionable MCP errors. Other failures keep the generic public message.
func mcpAgentRunStartError(err error) error {
	switch {
	case errors.Is(err, ErrAgentRunTargetNotFound):
		return newMCPToolError(MCPErrorCodeTargetNotFound,
			"The target was not found in the connected workspace. Use an ID from list_tasks, list_epics, or search_workspace; tasks also accept keys such as HEL-12.")
	case errors.Is(err, ErrAgentRunTargetNotAllowed):
		return newMCPToolError(MCPErrorCodeAgentTargetNotAllowed,
			"This agent cannot run on this target ("+err.Error()+"). Call list_agents and choose an agent whose allowed_targets include the target type and team.")
	case errors.Is(err, ErrAgentRunTargetBusy):
		return newMCPToolError(MCPErrorCodeTargetBusy,
			"Another agent run is already active on this target. Wait for it to finish or cancel it, then retry.")
	case errors.Is(err, ErrTaskDeliveryTargetRequired):
		return newMCPToolError(MCPErrorCodeRepositoryRequired,
			"This agent works in a repository and the task has none. Call list_repositories, then retry start_agent_run with repository_id (and optionally base_branch).")
	case errors.Is(err, ErrDeliveryRepositoryUnavailable):
		return newMCPToolError(MCPErrorCodeRepositoryRequired,
			"The task's repository is not connected or enabled for delivery. Call list_repositories and retry start_agent_run with an available repository_id.")
	default:
		return err
	}
}
