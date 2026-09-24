package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PauseRun requests a durable manual pause and exposes the pending transition.
func (s *AgentService) PauseRun(ctx context.Context, workspaceID, runID, actorID string) (*model.AgentRun, error) {
	run, err := s.runRepo.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("agent run not found")
	}
	if run.Status == model.AgentRunStatusPaused && run.PauseReason == model.AgentRunPauseReasonManual {
		return run, nil
	}
	if run.Status != model.AgentRunStatusQueued && run.Status != model.AgentRunStatusRunning {
		return nil, fmt.Errorf("only queued or running runs can be paused")
	}
	if s.agentRuntimeClient == nil || model.IsLocalAgentRun(run) {
		return nil, fmt.Errorf("manual pause requires an agent runtime run")
	}
	runtimeRunID, mapped := agentRuntimeRunID(run)
	if !mapped {
		runtimeRunID = strings.TrimSpace(run.ID)
	}
	if err := s.runRepo.UpdateStage(ctx, workspaceID, run.ID, "pausing", nil); err != nil {
		return nil, err
	}
	if _, err := s.agentRuntimeClient.PauseRun(ctx, runtimeRunID); err != nil {
		resetErr := s.runRepo.UpdateStage(ctx, workspaceID, run.ID, "", nil)
		return nil, errors.Join(fmt.Errorf("pause agent runtime run: %w", err), resetErr)
	}
	if refreshed, err := s.runRepo.GetByID(ctx, workspaceID, run.ID); err == nil && refreshed != nil {
		run = refreshed
	}
	s.publishRunEvent(run, actorID)
	return run, nil
}

// ResumeManuallyPausedRun continues the same runtime run without a new message.
func (s *AgentService) ResumeManuallyPausedRun(ctx context.Context, workspaceID, runID, actorID string) (*model.AgentRun, error) {
	run, err := s.runRepo.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("agent run not found")
	}
	if run.Status != model.AgentRunStatusPaused || run.PauseReason != model.AgentRunPauseReasonManual {
		return nil, fmt.Errorf("run is not manually paused")
	}
	if s.agentRuntimeClient == nil || model.IsLocalAgentRun(run) {
		return nil, fmt.Errorf("manual resume requires an agent runtime run")
	}
	runtimeRunID, ok := agentRuntimeRunID(run)
	if !ok {
		return nil, fmt.Errorf("run is not managed by the agent runtime")
	}
	now := time.Now().UTC()
	if err := s.runRepo.UpdateStage(ctx, workspaceID, run.ID, "resuming", &now); err != nil {
		return nil, err
	}
	if _, err := s.agentRuntimeClient.ResumeRun(ctx, runtimeRunID, AgentRuntimeResumeRunRequest{
		Intent: agentruntime.ResumeIntentContinue, ExternalActorID: actorID,
	}); err != nil {
		resetErr := s.runRepo.UpdateStage(ctx, workspaceID, run.ID, "", nil)
		return nil, errors.Join(fmt.Errorf("resume agent runtime run: %w", err), resetErr)
	}
	if refreshed, err := s.runRepo.GetByID(ctx, workspaceID, run.ID); err == nil && refreshed != nil {
		run = refreshed
	}
	s.publishRunEvent(run, actorID)
	return run, nil
}
