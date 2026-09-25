package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AskAgentDefaults contains only the launch default, not agent management data.
type AskAgentDefaults struct {
	AIProfileID *string `json:"ai_profile_id"`
}

// AskAgentDefaults reads the system agent selected by startChatRun, without
// creating agents or requiring provider credentials just to open the picker.
// The caller must be an authenticated workspace member; Automation access is
// intentionally not required. Profile eligibility is still checked at launch.
func (s *AgentService) AskAgentDefaults(ctx context.Context, workspaceID string) (*AskAgentDefaults, error) {
	agent, err := s.agentRepo.GetSystemByPreset(ctx, workspaceID, model.AgentPresetAskAgent)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return &AskAgentDefaults{}, nil
	}
	return &AskAgentDefaults{AIProfileID: agent.AIProfileID}, nil
}
