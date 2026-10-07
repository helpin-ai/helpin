package service

import (
	"context"
	"encoding/json"
	"errors"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// SetAIProfileService enables explicit connection resolution for all agent launches.
func (s *AgentService) SetAIProfileService(profiles *AIProfileService) *AgentService {
	s.aiProfiles = profiles
	return s
}

func (s *AgentService) validateSharedAIProfile(ctx context.Context, workspace string, id *string) error {
	if derefString(id) == "" {
		return nil
	}
	if s.aiProfiles == nil {
		return errors.New("AI profiles are not configured")
	}
	p, err := s.aiProfiles.repo.Get(ctx, workspace, *id)
	if err != nil {
		return err
	}
	if p == nil || p.Scope != "workspace" || p.UserID != nil {
		return errors.New("agent defaults require a shared workspace AI profile")
	}
	return nil
}

func (s *AgentService) prepareAIProfileRun(ctx context.Context, params *createRunParams) (*sdk.RunModel, *sdk.ModelCredential, *model.Agent, error) {
	if params.aiProfileID != "" && (params.modelConnectionID != "" || params.modelName != "") {
		return nil, nil, nil, errors.New("select a profile or legacy connection, not both")
	}
	user := derefString(params.actorID)
	if user != "" {
		if err := s.aiProfiles.requireMember(ctx, params.workspaceID, user); err != nil {
			return nil, nil, nil, err
		}
	}
	var input model.AgentRunInputPayload
	if err := decodeAIConnectionRunInput(params.input, &input); err != nil {
		return nil, nil, nil, err
	}
	bound, commandBarStep := ctx.Value(commandBarAIContextKey{}).(*model.AIExecutionSelection)
	if commandBarStep && bound != nil {
		credential, err := s.aiProfiles.Restore(ctx, params.workspaceID, user, bound)
		if err != nil {
			return nil, nil, nil, err
		}
		selection, err := s.aiProfiles.AdmitReviewedSelection(ctx, params.workspaceID, bound)
		if err != nil {
			return nil, nil, nil, err
		}
		return s.applyAISelection(params, selection, credential)
	}
	if params.parentRunID != nil && !commandBarStep {
		parent, err := s.runRepo.GetByID(ctx, params.workspaceID, *params.parentRunID)
		if err != nil {
			return nil, nil, nil, err
		}
		if parent == nil {
			return nil, nil, nil, errors.New("parent run not found")
		}
		var previous model.AgentRunInputPayload
		if err := decodeAIConnectionRunInput(parent.Input, &previous); err != nil {
			return nil, nil, nil, err
		}
		continuation := input.Event != nil && derefString(input.Event.Reason) == "continued_from_terminal_run"
		_, personal := previous.AISelection.PersonalOwner()
		legacyPersonal := previous.AISelection == nil && previous.ModelConnectionID != ""
		if continuation || personal || legacyPersonal {
			if err := requireAIConnectionRunOwner(parent, user); err != nil {
				return nil, nil, nil, err
			}
			if params.aiProfileID != "" && (previous.AISelection == nil || previous.AISelection.ProfileID != params.aiProfileID) {
				return nil, nil, nil, errors.New("a continuation cannot change AI profiles")
			}
			if (params.modelConnectionID != "" && params.modelConnectionID != previous.ModelConnectionID) || (params.modelName != "" && params.modelName != previous.ModelName) {
				return nil, nil, nil, errors.New("a continuation cannot change its AI connection or model")
			}
			if previous.AISelection != nil {
				credential, err := s.aiProfiles.Restore(ctx, params.workspaceID, user, previous.AISelection)
				if err != nil {
					return nil, nil, nil, err
				}
				// This is a new run, unlike restoring an accepted run after a pause.
				selection, err := s.aiProfiles.AdmitReviewedSelection(ctx, params.workspaceID, previous.AISelection)
				if err != nil {
					return nil, nil, nil, err
				}
				return s.applyAISelection(params, selection, credential)
			}
			params.modelConnectionID, params.modelName = previous.ModelConnectionID, previous.ModelName
		}
	}
	unattended := user == "" || (params.trigger != nil && params.trigger.Source != model.AgentRunTriggerSourceManual)
	var selection *model.AIExecutionSelection
	var credential *sdk.ModelCredential
	var err error
	if params.modelConnectionID != "" {
		selection, credential, err = s.resolveLegacyAISelection(ctx, params, unattended)
	} else {
		if params.modelName != "" {
			return nil, nil, nil, errors.New("select an AI connection before selecting a model")
		}
		selection, credential, err = s.aiProfiles.Resolve(ctx, params.workspaceID, user, AIProfileSelectionRequest{ProfileID: params.aiProfileID, AgentProfileID: derefString(params.agent.AIProfileID), Unattended: unattended, UsePersonalDefault: params.parentRunID == nil && params.dockChatID != nil && params.agent.EffectivePresetKey() == model.AgentPresetAskAgent})
	}
	if err != nil {
		return nil, nil, nil, err
	}
	return s.applyAISelection(params, selection, credential)
}

func (s *AgentService) resolveLegacyAISelection(ctx context.Context, params *createRunParams, unattended bool) (*model.AIExecutionSelection, *sdk.ModelCredential, error) {
	user := derefString(params.actorID)
	c, err := s.aiConnections.repo.Get(ctx, params.modelConnectionID)
	if err != nil {
		return nil, nil, err
	}
	if c == nil || c.WorkspaceID != params.workspaceID {
		return nil, nil, ErrAIConnection
	}
	if unattended && c.Scope != "workspace" && params.parentRunID == nil {
		return nil, nil, errors.New("personal AI connections are available for manual runs only")
	}
	name := params.modelName
	if name == "" && c.Provider == derefString(params.agent.Provider) {
		name = derefString(params.agent.Model)
	}
	controls := &sdk.ModelControls{}
	if len(params.agent.ExecutionConfig) > 0 {
		if err := json.Unmarshal(params.agent.ExecutionConfig, controls); err != nil {
			return nil, nil, err
		}
	}
	route := model.AIProfileRoute{ConnectionID: c.ID, Model: sdk.RunModel{Provider: c.Provider, Model: name, Controls: controls, Endpoint: c.Endpoint}}
	if err := s.aiProfiles.validateRoute(ctx, params.workspaceID, user, c.Scope, &route); err != nil {
		return nil, nil, err
	}
	policy, err := s.aiProfiles.selectionPolicy(ctx, params.workspaceID, route)
	if err != nil {
		return nil, nil, err
	}
	if err := s.aiProfiles.checkExecutionRoute(ctx, route); err != nil {
		return nil, nil, err
	}
	_, credential, err := s.aiProfiles.routeCredential(ctx, params.workspaceID, user, unattended && c.Scope == "workspace", route)
	if err != nil {
		return nil, nil, err
	}
	return &model.AIExecutionSelection{Route: route, ConnectionScope: c.Scope, OwnerID: c.UserID, Source: "legacy_override", Policy: policy}, credential, nil
}

func (s *AgentService) applyAISelection(params *createRunParams, selection *model.AIExecutionSelection, credential *sdk.ModelCredential) (*sdk.RunModel, *sdk.ModelCredential, *model.Agent, error) {
	var input map[string]any
	if err := json.Unmarshal(params.input, &input); err != nil {
		return nil, nil, nil, err
	}
	if input == nil {
		input = map[string]any{}
	}
	input["ai_selection"] = selection
	input["model_connection_id"], input["model_provider"], input["model_name"] = selection.Route.ConnectionID, selection.Route.Model.Provider, selection.Route.Model.Model
	input["credential_source"] = "app"
	var err error
	params.input, err = json.Marshal(input)
	if err != nil {
		return nil, nil, nil, err
	}
	params.modelConnectionID, params.modelName = selection.Route.ConnectionID, selection.Route.Model.Model
	billingAgent := *params.agent
	billingAgent.Provider, billingAgent.Model = &selection.Route.Model.Provider, &selection.Route.Model.Model
	billingAgent.ExecutionConfig, err = executionConfigWithModelControls(billingAgent.ExecutionConfig, selection.Route.Model.Controls)
	if err != nil {
		return nil, nil, nil, err
	}
	billingAgent.ModelTier = deriveAgentModelTier(billingAgent.Provider, billingAgent.Model, billingAgent.ExecutionConfig)
	return &selection.Route.Model, credential, &billingAgent, nil
}

// Replace only model controls; preserve native context, tool limits, and future
// execution fields in the agent configuration.
func executionConfigWithModelControls(raw model.JSONBlob, controls *sdk.ModelControls) (model.JSONBlob, error) {
	config := map[string]any{}
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &config); err != nil {
			return nil, err
		}
	}
	if config == nil {
		config = map[string]any{}
	}
	for _, key := range []string{"reasoning_effort", "service_tier", "openrouter"} {
		delete(config, key)
	}
	if controls != nil {
		encoded, err := json.Marshal(controls)
		if err != nil {
			return nil, err
		}
		var replacement map[string]any
		if err := json.Unmarshal(encoded, &replacement); err != nil {
			return nil, err
		}
		for key, value := range replacement {
			config[key] = value
		}
	}
	return json.Marshal(config)
}

func (s *AgentService) recheckRunAISelection(ctx context.Context, run *model.AgentRun, actor string) error {
	var input model.AgentRunInputPayload
	if err := decodeAIConnectionRunInput(run.Input, &input); err != nil {
		return err
	}
	if input.AISelection != nil {
		if s.aiProfiles == nil {
			return ErrAIConnection
		}
		_, err := s.aiProfiles.Restore(ctx, run.WorkspaceID, actor, input.AISelection)
		return err
	}
	if input.ModelConnectionID != "" {
		if s.aiConnections == nil {
			return ErrAIConnection
		}
		_, _, err := s.aiConnections.Credential(ctx, run.WorkspaceID, actor, input.ModelConnectionID, false)
		return err
	}
	return nil
}
