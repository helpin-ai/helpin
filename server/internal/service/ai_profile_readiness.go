package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// CheckRuntimeReadiness enables capability validation in both production launch
// paths. Capabilities describe configuration, never successful authentication.
func (s *AIProfileService) CheckRuntimeReadiness() *AIProfileService {
	s.checkRuntimeReadiness = true
	return s
}

func (s *AIProfileService) checkExecutionRoute(ctx context.Context, route model.AIProfileRoute) error {
	if !s.checkRuntimeReadiness {
		return nil
	}
	if s.connections == nil || s.connections.runtime == nil {
		return errors.New("agent runtime is not configured")
	}
	c := s.connections.runtime
	capabilities, err := c.client.GetCapabilities(ctx)
	if err != nil {
		return fmt.Errorf("check agent runtime capabilities: %w", err)
	}
	if err := validateRuntimeRoute(capabilities, c.AppID(), route.Model.Provider); err != nil {
		return err
	}
	if route.Model.Provider == "openai_compatible" {
		for _, app := range capabilities.Apps {
			if app.AppID != c.AppID() {
				continue
			}
			for _, endpoint := range app.ModelEndpoints {
				if sameModelEndpoint(&endpoint, route.Model.Endpoint) {
					for _, provider := range capabilities.Providers {
						if provider.Name == "openai_compatible" && slices.Contains(provider.AuthModes, endpoint.AuthMode) {
							return nil
						}
					}
					return errors.New("compatible endpoint authentication mode is unsupported")
				}
			}
		}
		return errors.New("compatible endpoint is no longer approved or its binding changed")
	}
	return nil
}

func validateRuntimeRoute(capabilities *sdk.Capabilities, appID, provider string) error {
	if capabilities == nil {
		return errors.New("agent runtime capabilities are unavailable")
	}
	if !slices.Contains(capabilities.RuntimeKinds, "native_sdk") {
		return errors.New("agent runtime does not support native execution")
	}
	// openrouter_responses is an existing alias of the same transport. ChatGPT
	// is a separate route and must never inherit direct OpenAI capabilities.
	name := provider
	if name == "openrouter_responses" {
		name = "openrouter"
	}
	auth := "api_key"
	if provider == "openai_chatgpt" {
		auth = "oauth"
	}
	ready := false
	for _, route := range capabilities.Providers {
		if route.Name == name {
			ready = route.RunCredentialsConfigured && slices.Contains(route.AuthModes, auth)
			break
		}
	}
	if provider == "openai_compatible" {
		for _, route := range capabilities.Providers {
			if route.Name == name {
				ready = route.RunCredentialsConfigured && slices.Contains(route.Protocols, "chat_completions")
			}
		}
	}
	if !ready {
		return fmt.Errorf("agent runtime is not configured for %s run credentials", provider)
	}
	if auth == "oauth" {
		for _, app := range capabilities.Apps {
			if app.AppID != appID {
				continue
			}
			for _, component := range app.Components {
				if component.Kind == "model_credentials" && component.Configured && component.AuthConfigured {
					return nil
				}
			}
		}
		return errors.New("ChatGPT requires the app credential refresh callback in agent runtime configuration")
	}
	return nil
}
