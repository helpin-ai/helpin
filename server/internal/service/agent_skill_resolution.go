package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/agentskills"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

func (s *AgentService) resolveAgentSkills(ctx context.Context, agent *model.Agent) (agentskills.Resolution, error) {
	if agent == nil {
		return agentskills.Resolution{}, nil
	}
	if s.workspaceSkillRepo == nil {
		return agentskills.Resolve(ctx, agent.WorkspaceID, agent.Skills, nil)
	}
	return agentskills.Resolve(ctx, agent.WorkspaceID, agent.Skills, s.workspaceSkillRepo)
}

func (s *AgentService) validateAndMaterializeAgentSkills(ctx context.Context, agent *model.Agent) error {
	if agent == nil {
		return nil
	}
	resolution, err := s.resolveAgentSkills(ctx, agent)
	if err != nil {
		return err
	}
	agent.Skills = resolution.Refs
	profile := worker.ResolveAgentProfile(agent)
	if err := agentskills.ValidateRuntimeAndTools(agent.RuntimeKind, profile.Tools, resolution.Definitions); err != nil {
		return err
	}
	agent.ResolvedSkillInstructions = agentskills.CompileInstructions(resolution.Definitions)
	return nil
}
