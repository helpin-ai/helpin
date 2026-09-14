package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func (s *AIProfileBootstrapService) assignBootstrapProfiles(ctx context.Context, store *repository.AIProfileBootstrapRepository, workspace string, tierIDs []string, connection func(string) (*model.AIConnection, error), result *AIProfileBootstrapResult) error {
	profiles, err := store.SharedProfiles(ctx, workspace)
	if err != nil {
		return err
	}
	byRoute := map[string]string{}
	add := func(p model.AIProfile) error {
		// A fallback changes execution behavior; it is not equivalent to a legacy
		// agent's single route. Ownership and liveness are filtered by the store.
		if p.Fallback != nil {
			return nil
		}
		key, err := bootstrapRouteKey(p.Primary)
		if err != nil {
			return err
		}
		if byRoute[key] == "" {
			byRoute[key] = p.ID
		}
		return nil
	}
	// Prefer the actual saved tier profiles, including edits, over custom names.
	for _, id := range tierIDs {
		for _, p := range profiles {
			if p.ID == id {
				if err := add(p); err != nil {
					return err
				}
			}
		}
	}
	for _, p := range profiles {
		if err := add(p); err != nil {
			return err
		}
	}
	agents, err := store.Agents(ctx, workspace)
	if err != nil {
		return err
	}
	for _, agent := range agents {
		if agent.AIProfileID != nil {
			continue
		}
		runModel, err := bootstrapAgentModel(agent)
		if err != nil {
			return fmt.Errorf("agent %s: %w", agent.ID, err)
		}
		c, err := connection(runModel.Provider)
		if err != nil {
			return err
		}
		route := model.AIProfileRoute{ConnectionID: c.ID, Model: runModel}
		key, err := bootstrapRouteKey(route)
		if err != nil {
			return err
		}
		id := byRoute[key]
		if id == "" {
			p := bootstrapProfile(workspace, "route:"+key, bootstrapRouteName(runModel), route)
			created, err := store.EnsureProfile(ctx, p)
			if err != nil {
				return err
			}
			if !created {
				return errors.New("a bootstrap profile was edited or deleted; select an active shared profile for the agent")
			}
			id, byRoute[key] = p.ID, p.ID
			result.ProfilesCreated++
		}
		if err := store.AssignAgent(ctx, workspace, agent.ID, id); err != nil {
			return err
		}
		result.AgentsMigrated++
		if strings.TrimSpace(derefString(agent.Model)) == "" && strings.TrimSpace(agent.ModelTier) == "" {
			result.AgentsDefaultedToSmall++
		}
	}
	return nil
}

// Compare the full connection and model selection, excluding execution settings.
// Normalize empty controls and provider-native service-tier aliases only; never
// merge different reasoning, endpoints, credentials or routing preferences.
func bootstrapRouteKey(route model.AIProfileRoute) (string, error) {
	controls := sdk.ModelControls{}
	if route.Model.Controls != nil {
		controls = *route.Model.Controls
	}
	if effort := strings.ToLower(strings.TrimSpace(derefString(controls.ReasoningEffort))); effort != "" {
		controls.ReasoningEffort = &effort
	} else {
		controls.ReasoningEffort = nil
	}
	if tier := sdk.NormalizeServiceTier(derefString(controls.ServiceTier)); tier != "" {
		controls.ServiceTier = &tier
	} else {
		controls.ServiceTier = nil
	}
	route.Model.Controls = &controls
	raw, err := json.Marshal(route)
	return string(raw), err
}

func bootstrapRouteName(route sdk.RunModel) string {
	name := route.Model + " · " + route.Provider
	if route.Controls != nil {
		if effort := strings.TrimSpace(derefString(route.Controls.ReasoningEffort)); effort != "" {
			name += " · " + effort + " reasoning"
		}
		if tier := sdk.NormalizeServiceTier(derefString(route.Controls.ServiceTier)); tier != "" {
			name += " · " + tier
		}
		if preferences := route.Controls.OpenRouter; preferences != nil && preferences.Provider != nil && len(preferences.Provider.Quantizations) != 0 {
			name += " · " + strings.Join(preferences.Provider.Quantizations, ", ")
		}
	}
	return name
}
