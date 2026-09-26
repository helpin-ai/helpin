package service

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/aimodel"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func presetDefaultAITier(family string) string {
	if p, ok := agentPresetDefinition(family); ok && p.ModelTier != "" {
		return p.ModelTier
	}
	return string(aimodel.TierSmall)
}

func standardModelForTier(tier string) (sdk.RunModel, error) {
	route, ok := selectableAgentTierRoutes[aimodel.Tier(strings.TrimSpace(tier))]
	if !ok {
		return sdk.RunModel{}, fmt.Errorf("unknown standard AI size %q", tier)
	}
	controls := &sdk.ModelControls{}
	if q := providerQuantizationsForAgentRoute(route); len(q) > 0 {
		controls.OpenRouter = &sdk.OpenRouterModelControls{Provider: &sdk.OpenRouterProviderPreferences{Quantizations: q}}
	}
	return sdk.RunModel{Provider: route.Provider, Model: route.Model, Controls: controls}, nil
}

func (s *AgentService) assignInitialStandardProfile(ctx context.Context, agent *model.Agent, tier string) error {
	if s.aiProfiles == nil || agent.AIProfileID != nil {
		return nil
	}
	profiles, err := NewAIStandardProfiles(repository.NewAIStandardProfileRepository(s.agentRepo.DB()), "", "customer", nil)
	if err != nil {
		return err
	}
	if err := profiles.EnsureWorkspace(ctx, agent.WorkspaceID); err != nil {
		return err
	}
	// The standard profile was just re-mapped to connected providers, so the
	// agent records the route it will actually run rather than the fixed default.
	route, err := profiles.StandardRoute(ctx, agent.WorkspaceID, tier)
	if err != nil {
		return err
	}
	id := model.StandardAIProfileID(agent.WorkspaceID, tier)
	agent.AIProfileID = &id
	agent.ModelTier = tier
	agent.Provider, agent.Model = &route.Provider, &route.Model
	agent.ExecutionConfig, err = executionConfigWithModelControls(agent.ExecutionConfig, route.Controls)
	return err
}
