package service

import (
	"context"
	"fmt"
	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
	"strings"
)

type agentRuntimeSteeringClient interface {
	SteerRun(context.Context, string, sdk.SteerRunRequest) (*sdk.RunSteeringCommand, error)
	RunSteering(context.Context, string) ([]sdk.RunSteeringCommand, error)
}

func (c *AgentRuntimeClient) SteerRun(ctx context.Context, id string, req sdk.SteerRunRequest) (*sdk.RunSteeringCommand, error) {
	return c.client.SteerRun(ctx, id, req)
}
func (c *AgentRuntimeClient) RunSteering(ctx context.Context, id string) ([]sdk.RunSteeringCommand, error) {
	return c.client.RunSteering(ctx, id)
}

func (s *AgentService) steeringRun(ctx context.Context, workspaceID, runID string) (string, agentRuntimeSteeringClient, error) {
	run, err := s.runRepo.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return "", nil, err
	}
	if run == nil {
		return "", nil, fmt.Errorf("agent run not found")
	}
	if err := rejectUnclaimedCRMPlaybookRun(run.Input); err != nil {
		return "", nil, err
	}
	runtimeID, ok := agentRuntimeRunID(run)
	if !ok || model.IsLocalAgentRun(run) {
		return "", nil, fmt.Errorf("run does not support steering")
	}
	client, ok := s.agentRuntimeClient.(agentRuntimeSteeringClient)
	if !ok {
		return "", nil, fmt.Errorf("runtime steering is unavailable")
	}
	return runtimeID, client, nil
}
func (s *AgentService) SteerRun(ctx context.Context, workspaceID, runID, actorID string, req model.SteerAgentRunRequest) (*sdk.RunSteeringCommand, error) {
	if strings.TrimSpace(actorID) == "" || strings.TrimSpace(req.CommandID) == "" || len(req.CommandID) > 128 || req.ExpectedRevision < 0 || len(req.Content) > 32768 || len(req.DeniedTools) > 128 || (strings.TrimSpace(req.Content) == "" && len(req.DeniedTools) == 0) {
		return nil, fmt.Errorf("invalid steering request")
	}
	id, client, err := s.steeringRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	return client.SteerRun(ctx, id, sdk.SteerRunRequest{CommandID: req.CommandID, ExpectedRevision: req.ExpectedRevision, Content: req.Content, DeniedTools: req.DeniedTools, ExternalActorID: actorID})
}
func (s *AgentService) RunSteering(ctx context.Context, workspaceID, runID string) ([]sdk.RunSteeringCommand, error) {
	id, client, err := s.steeringRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	return client.RunSteering(ctx, id)
}
