package service

import (
	"context"
	"encoding/json"
	"errors"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *AgentService) SetAIConnectionService(connections *AIConnectionService) *AgentService {
	s.aiConnections = connections
	return s
}
func (c *AgentRuntimeClient) UpdateRunModelCredential(ctx context.Context, runID string, credential sdk.ModelCredential) error {
	_, err := c.client.UpdateRunModelCredential(ctx, runID, sdk.UpdateRunModelCredentialRequest{Credential: credential})
	return err
}
func (c *AgentRuntimeClient) RevokeRunModelCredential(ctx context.Context, runID string) error {
	return c.client.RevokeRunModelCredential(ctx, runID)
}

func (s *AgentService) prepareAIConnectionRun(ctx context.Context, params *createRunParams) (*sdk.RunModel, *sdk.ModelCredential, *model.Agent, error) {
	if s.aiProfiles != nil {
		return s.prepareAIProfileRun(ctx, params)
	}
	if params.parentRunID != nil {
		parent, err := s.runRepo.GetByID(ctx, params.workspaceID, *params.parentRunID)
		if err != nil {
			return nil, nil, nil, err
		}
		if parent != nil {
			var input model.AgentRunInputPayload
			if err := decodeAIConnectionRunInput(parent.Input, &input); err != nil {
				return nil, nil, nil, err
			}
			if input.ModelConnectionID != "" {
				if derefString(params.actorID) != derefString(parent.TriggeredByUserID) {
					return nil, nil, nil, ErrAIConnection
				}
				if params.modelConnectionID != "" && params.modelConnectionID != input.ModelConnectionID {
					return nil, nil, nil, errors.New("start a new run to change AI connections")
				}
				if params.modelName != "" && params.modelName != input.ModelName {
					return nil, nil, nil, errors.New("start a new run to change models")
				}
				params.modelConnectionID = input.ModelConnectionID
				params.modelName = input.ModelName
			}
		}
	}
	if params.modelConnectionID == "" {
		if params.modelName != "" {
			return nil, nil, nil, errors.New("select an AI connection before selecting a model")
		}
		return nil, nil, params.agent, nil
	}
	if params.actorID == nil || *params.actorID == "" || s.aiConnections == nil {
		return nil, nil, nil, ErrAIConnection
	}
	if params.parentRunID == nil && params.trigger != nil && params.trigger.Source != model.AgentRunTriggerSourceManual {
		return nil, nil, nil, errors.New("personal AI connections are available for manual runs only")
	}
	connection, credential, err := s.aiConnections.Credential(ctx, params.workspaceID, *params.actorID, params.modelConnectionID, false)
	if err != nil {
		return nil, nil, nil, err
	}
	modelName := params.modelName
	if modelName == "" && derefString(params.agent.Provider) == connection.Provider {
		modelName = derefString(params.agent.Model)
	}
	selection, tier, err := s.aiConnections.ResolveModel(connection.Provider, modelName)
	if err != nil {
		return nil, nil, nil, err
	}
	var input map[string]any
	if json.Unmarshal(params.input, &input) != nil {
		return nil, nil, nil, errors.New("invalid run input")
	}
	if input == nil {
		input = map[string]any{}
	}
	input["model_connection_id"] = connection.ID
	input["model_provider"] = selection.Provider
	input["model_name"] = selection.Model
	input["credential_source"] = "app"
	params.input, err = json.Marshal(input)
	if err != nil {
		return nil, nil, nil, err
	}
	billingAgent := *params.agent
	billingAgent.Provider = &selection.Provider
	billingAgent.Model = &selection.Model
	billingAgent.ModelTier = string(tier)
	return selection, credential, &billingAgent, nil
}

func requireAIConnectionRunOwner(run *model.AgentRun, actor string) error {
	var input model.AgentRunInputPayload
	if err := decodeAIConnectionRunInput(run.Input, &input); err != nil {
		return err
	}
	if input.AISelection != nil && input.AISelection.ConnectionScope == "workspace" {
		return nil
	}
	if input.ModelConnectionID != "" && (actor == "" || actor != derefString(run.TriggeredByUserID)) {
		return ErrAIConnection
	}
	return nil
}

// Older default-key runs may have no input document.
func decodeAIConnectionRunInput(raw json.RawMessage, input *model.AgentRunInputPayload) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, input)
}
