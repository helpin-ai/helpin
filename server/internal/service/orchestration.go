package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// OrchestrationService handles orchestrator assignment metadata for epics.
type OrchestrationService struct {
	epicRepo    *repository.PMEpicRepository
	agentRepo   *repository.AgentRepository
	activitySvc *PMActivityService
	wsPublisher *websocket.Publisher
}

// NewOrchestrationService creates a new OrchestrationService.
func NewOrchestrationService(
	epicRepo *repository.PMEpicRepository,
	agentRepo *repository.AgentRepository,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
) *OrchestrationService {
	return &OrchestrationService{
		epicRepo:    epicRepo,
		agentRepo:   agentRepo,
		activitySvc: activitySvc,
		wsPublisher: wsPublisher,
	}
}

// AssignOrchestrator assigns an orchestrator agent to an epic.
func (s *OrchestrationService) AssignOrchestrator(ctx context.Context, workspaceID, epicID, agentID, actorID string) error {
	ews, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil || ews == nil {
		return fmt.Errorf("epic not found")
	}
	epic := &ews.Epic

	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		return fmt.Errorf("agent not found")
	}
	if err := validateAgentTarget(agent, "epic"); err != nil {
		return err
	}

	epic.OrchestratorAgentID = &agentID
	if err := s.epicRepo.Update(ctx, epic); err != nil {
		return fmt.Errorf("update epic: %w", err)
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "epic", epicID, &actorID, "updated", strPtr("orchestrator_agent_id"), nil, &agent.Name, nil)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "epic",
		EntityID:    epicID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})

	return nil
}
