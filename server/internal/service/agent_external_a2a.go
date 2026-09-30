package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Task assignment trigger type recorded on runs started by auto_on_assignment.
const agentRunTriggerTypeTaskAssigned = "task_assigned"

// externalA2ANameSyncer keeps an external agent connection's name equal to its
// linked agent's name.
type externalA2ANameSyncer interface {
	SyncNameForAgent(ctx context.Context, workspaceID, agentID, name string) error
}

// isExternalA2AAgent reports whether runs of agent are delegated to a remote
// A2A agent rather than a model.
func isExternalA2AAgent(agent *model.Agent) bool {
	return agent != nil && strings.TrimSpace(agent.RuntimeKind) == model.AgentRuntimeKindA2A
}

func isExternalA2ARun(run *model.AgentRun) bool {
	return run != nil && strings.TrimSpace(run.RuntimeKind) == model.AgentRuntimeKindA2A
}

func externalA2AAgentIDFromConfig(raw model.JSONBlob) string {
	config, err := model.ParseAgentExecutionConfig(raw)
	if err != nil || config.ExternalA2AAgentID == nil {
		return ""
	}
	return strings.TrimSpace(*config.ExternalA2AAgentID)
}

func externalA2AExecutionConfig(externalAgentID string) model.JSONBlob {
	return model.MarshalAgentExecutionConfig(model.AgentExecutionConfig{ExternalA2AAgentID: &externalAgentID})
}

// runtimeAgentFromExternalA2AAgent projects an a2a agent for Agent Runtime.
// It carries no prompt, model, tools or skills: connection details are
// delivered per turn through the target context instead.
func runtimeAgentFromExternalA2AAgent(agent *model.Agent, appID string) AgentRuntimeAgent {
	config := json.RawMessage(`{}`)
	if id := externalA2AAgentIDFromConfig(agent.ExecutionConfig); id != "" {
		config, _ = json.Marshal(map[string]string{"external_a2a_agent_id": id})
	}
	return AgentRuntimeAgent{
		ID:                    strings.TrimSpace(agent.ID),
		AppID:                 strings.TrimSpace(appID),
		Name:                  strings.TrimSpace(agent.Name),
		RuntimeKind:           model.AgentRuntimeKindA2A,
		AllowedTargets:        parseJSONStringSlice(agent.AllowedTargets),
		ApprovalMode:          "never",
		DefaultInvocationMode: model.InvocationModeAutonomous,
		ExecutionConfig:       config,
	}
}

// sanitizeExternalA2AAgentUpdate keeps the generic agent editor from turning
// an external agent into a model agent. Connection settings are edited through
// the external agents API; only presentation, team scope and trigger fields
// are accepted here. Empty model fields from full-form editors are ignored.
func sanitizeExternalA2AAgentUpdate(agent *model.Agent, req model.UpdateAgentRequest) (model.UpdateAgentRequest, error) {
	managed := func(name string) (model.UpdateAgentRequest, error) {
		return req, fmt.Errorf("%s of an external agent is managed under Settings > External agents", name)
	}
	if value := strings.TrimSpace(derefString(req.RuntimeKind)); value != "" && value != model.AgentRuntimeKindA2A {
		return managed("runtime_kind")
	}
	if strings.TrimSpace(derefString(req.Provider)) != "" || strings.TrimSpace(derefString(req.Model)) != "" ||
		strings.TrimSpace(derefString(req.ModelTier)) != "" || strings.TrimSpace(derefString(req.AIProfileID)) != "" {
		return managed("the model")
	}
	if strings.TrimSpace(derefString(req.SystemPrompt)) != "" || len(req.Skills.Normalize()) > 0 ||
		len(parseJSONStringSlice(req.AllowedTools)) > 0 || len(parseJSONStringSlice(req.AllowedCommands)) > 0 {
		return managed("the instructions and tools")
	}
	if len(req.ExecutionConfig) > 0 {
		if id := externalA2AAgentIDFromConfig(model.JSONBlob(req.ExecutionConfig)); id != "" && id != externalA2AAgentIDFromConfig(agent.ExecutionConfig) {
			return managed("execution_config")
		}
	}
	req.RuntimeKind = nil
	req.Provider, req.Model, req.ModelTier, req.AIProfileID, req.SystemPrompt = nil, nil, nil, nil, nil
	req.Skills, req.AllowedTools, req.AllowedCommands, req.ExecutionConfig = nil, nil, nil, nil
	req.AllowedTargets, req.DefaultInvocationMode = nil, nil
	return req, nil
}

// externalA2AAgentProfile is the Helpin agent shape for a connected agent.
type externalA2AAgentProfile struct {
	WorkspaceID     string
	ExternalAgentID string
	Name            string
	Description     string
	TeamIDs         []string
	ActorID         string
}

// createExternalA2AAgent creates the Helpin agent users assign tickets to. It
// bypasses model routing: a2a agents have no provider, model or tools.
func (s *AgentService) createExternalA2AAgent(ctx context.Context, profile externalA2AAgentProfile) (*model.Agent, error) {
	if s.entitlementSvc != nil {
		if err := s.entitlementSvc.RequireFeature(ctx, profile.WorkspaceID, EntitlementFeatureCustomAgents); err != nil {
			return nil, err
		}
	}
	teamIDs := normalizeServiceTeamIDs(profile.TeamIDs)
	agent := &model.Agent{
		WorkspaceID:           profile.WorkspaceID,
		Name:                  profile.Name,
		Role:                  externalA2AAgentRole(profile.Description),
		Status:                "idle",
		RuntimeKind:           model.AgentRuntimeKindA2A,
		Skills:                model.AgentSkillRefs{},
		TriggerMode:           "auto_on_assignment",
		ExecutionConfig:       externalA2AExecutionConfig(profile.ExternalAgentID),
		TeamID:                firstTeamIDPtr(teamIDs),
		TeamIDs:               teamIDs,
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		AllowedTargets:        mustJSONStringSlice([]string{"task"}),
		ApprovalMode:          "never",
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: model.InvocationModeAutonomous,
	}
	normalizeAgentRecord(agent)
	if err := validateRuntimeForAgent(agent); err != nil {
		return nil, err
	}
	if err := validateTriggerModeForAgent(agent.TriggerMode, agent); err != nil {
		return nil, err
	}
	if err := s.validateModelRouting(agent); err != nil {
		return nil, err
	}
	if err := s.agentRepo.Create(ctx, agent); err != nil {
		return nil, err
	}
	if err := s.ensureCustomAgentDefaultVersion(ctx, agent, profile.ActorID); err != nil {
		slog.WarnContext(ctx, "create external agent version", "workspace_id", agent.WorkspaceID, "agent_id", agent.ID, "error", err)
	}
	if s.activitySvc != nil {
		name := agent.Name
		actorID := profile.ActorID
		_ = s.activitySvc.Log(ctx, agent.WorkspaceID, "agent", agent.ID, &actorID, "created", nil, nil, &name, nil)
	}
	s.publishSimpleEvent("created", "agent", agent.ID, agent.WorkspaceID, profile.ActorID)
	return agent, nil
}

// updateExternalA2AAgentProfile mirrors connection edits onto the linked agent.
func (s *AgentService) updateExternalA2AAgentProfile(ctx context.Context, workspaceID, agentID, actorID string, name, description *string, teamIDs *[]string) error {
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return err
	}
	if agent == nil || !isExternalA2AAgent(agent) {
		return fmt.Errorf("linked external agent not found")
	}
	if name != nil {
		agent.Name = strings.TrimSpace(*name)
	}
	if description != nil {
		agent.Role = externalA2AAgentRole(*description)
	}
	if teamIDs != nil {
		agent.TeamIDs = normalizeServiceTeamIDs(*teamIDs)
		agent.TeamID = firstTeamIDPtr(agent.TeamIDs)
	}
	if err := s.agentRepo.Update(ctx, agent); err != nil {
		return err
	}
	s.publishSimpleEvent("updated", "agent", agent.ID, agent.WorkspaceID, actorID)
	return nil
}

func externalA2AAgentRole(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return "External agent"
	}
	if runes := []rune(description); len(runes) > 200 {
		description = strings.TrimSpace(string(runes[:200])) + "…"
	}
	return description
}

// StartAssignmentRun starts a task run when a task is assigned to an agent
// whose trigger_mode is auto_on_assignment. It is a no-op for other agents
// and when that agent already has an active run on the task.
func (s *AgentService) StartAssignmentRun(ctx context.Context, workspaceID, taskID, agentID, actorID string) (*model.AgentRun, error) {
	if s == nil || s.agentRepo == nil || s.runRepo == nil {
		return nil, nil
	}
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		return nil, err
	}
	if strings.TrimSpace(agent.TriggerMode) != "auto_on_assignment" {
		return nil, nil
	}
	active, err := s.runRepo.ListActiveByTarget(ctx, workspaceID, "task", taskID)
	if err != nil {
		return nil, err
	}
	for _, run := range active {
		if run.AgentID == agentID {
			return nil, nil
		}
	}
	var actor *string
	if strings.TrimSpace(actorID) != "" {
		actor = &actorID
	}
	return s.startTargetRun(ctx, workspaceID, "task", taskID, model.StartAgentRunRequest{AgentID: agentID}, actor, systemRunTriggerContext(agentRunTriggerTypeTaskAssigned), nil, nil)
}
