package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
	"github.com/d4interactive/teampulse/server/internal/websocket"
	"github.com/d4interactive/teampulse/server/internal/worker"
)

// OrchestrationService handles epic orchestration via AI agents.
type OrchestrationService struct {
	epicRepo    *repository.PMEpicRepository
	storySvc    *PMStoryService
	agentRepo   *repository.AgentRepository
	handoffRepo *repository.AgentHandoffRepository
	activitySvc *PMActivityService
	wsPublisher *websocket.Publisher
	claude      *worker.ClaudeClient
}

// NewOrchestrationService creates a new OrchestrationService.
func NewOrchestrationService(
	epicRepo *repository.PMEpicRepository,
	storySvc *PMStoryService,
	agentRepo *repository.AgentRepository,
	handoffRepo *repository.AgentHandoffRepository,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
	claude *worker.ClaudeClient,
) *OrchestrationService {
	return &OrchestrationService{
		epicRepo:    epicRepo,
		storySvc:    storySvc,
		agentRepo:   agentRepo,
		handoffRepo: handoffRepo,
		activitySvc: activitySvc,
		wsPublisher: wsPublisher,
		claude:      claude,
	}
}

// Orchestrate proposes stories for an epic using the assigned orchestrator agent.
func (s *OrchestrationService) Orchestrate(ctx context.Context, workspaceID, epicID, actorID string, req model.OrchestrateRequest) (*model.OrchestrationProposal, error) {
	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	if epicWithStats == nil {
		return nil, fmt.Errorf("epic not found")
	}
	epic := &epicWithStats.Epic

	if epic.OrchestratorAgentID == nil || *epic.OrchestratorAgentID == "" {
		return nil, fmt.Errorf("no orchestrator agent assigned to this epic")
	}

	agent, err := s.agentRepo.GetByID(ctx, workspaceID, *epic.OrchestratorAgentID)
	if err != nil || agent == nil {
		return nil, fmt.Errorf("orchestrator agent not found")
	}

	if s.claude == nil {
		return nil, fmt.Errorf("AI service not configured (ANTHROPIC_API_KEY required)")
	}

	// Get existing stories for context.
	existingStories, _ := s.epicRepo.ListStories(ctx, epicID)

	// Build prompt.
	systemPrompt := "You are an orchestrator agent that decomposes epics into stories. "
	if agent.SystemPrompt != nil && *agent.SystemPrompt != "" {
		systemPrompt = *agent.SystemPrompt
	}
	systemPrompt += "\nRespond with a JSON object containing: {\"summary\": \"...\", \"proposed_stories\": [{\"name\": \"...\", \"description\": \"...\", \"story_type\": \"feature|bug|chore\", \"estimate\": N}]}. Only output valid JSON, no markdown fences."

	userPrompt := fmt.Sprintf("Epic: %s\n", epic.Name)
	if epic.Description != nil && *epic.Description != "" {
		userPrompt += fmt.Sprintf("Description: %s\n", *epic.Description)
	}
	if len(existingStories) > 0 {
		userPrompt += "\nExisting stories:\n"
		for _, s := range existingStories {
			userPrompt += fmt.Sprintf("- %s (type=%s)\n", s.Name, s.StoryType)
		}
	}
	if req.AdditionalContext != "" {
		userPrompt += "\nAdditional context: " + req.AdditionalContext
	}
	userPrompt += "\nDecompose this epic into implementation stories. Each story should be concrete, implementable, and well-scoped."

	// Call Claude.
	resp, err := s.claude.CreateMessage(ctx, worker.CreateMessageRequest{
		System:    systemPrompt,
		MaxTokens: 4096,
		Messages: []worker.Message{
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("AI call failed: %w", err)
	}

	// Extract text content.
	var responseText string
	for _, block := range resp.Content {
		if block.Type == "text" {
			responseText = block.Text
			break
		}
	}

	// Parse response.
	var proposal model.OrchestrationProposal
	if err := json.Unmarshal([]byte(responseText), &proposal); err != nil {
		// Try to extract from wrapper.
		return nil, fmt.Errorf("failed to parse AI response: %w (response: %s)", err, responseText[:min(len(responseText), 500)])
	}

	proposal.EpicID = epicID
	proposal.TokensUsed = resp.Usage.InputTokens + resp.Usage.OutputTokens

	_ = s.activitySvc.Log(ctx, workspaceID, "epic", epicID, &actorID, "orchestrated", nil, nil, strPtr(fmt.Sprintf("%d stories proposed", len(proposal.ProposedStories))), nil)

	return &proposal, nil
}

// ConfirmOrchestration creates stories from a confirmed proposal.
func (s *OrchestrationService) ConfirmOrchestration(ctx context.Context, workspaceID, epicID, actorID string, req model.ConfirmOrchestrationRequest) ([]model.PMStory, error) {
	ews, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil || ews == nil {
		return nil, fmt.Errorf("epic not found")
	}
	epic := &ews.Epic

	var created []model.PMStory
	for _, ps := range req.ProposedStories {
		storyType := ps.StoryType
		if storyType == "" {
			storyType = "feature"
		}
		createReq := model.CreateStoryRequest{
			WorkspaceID: workspaceID,
			Name:        ps.Name,
			Description: &ps.Description,
			StoryType:   storyType,
			EpicID:      &epicID,
			Estimate:    ps.Estimate,
		}

		detail, err := s.storySvc.Create(ctx, createReq, actorID)
		if err != nil {
			return nil, fmt.Errorf("create story: %w", err)
		}

		// Note: agent assignment for created stories is done via the assign-agent endpoint.

		created = append(created, detail.Story)
	}

	// Log handoff if orchestrator is assigned.
	if epic.OrchestratorAgentID != nil {
		handoff := &model.AgentHandoff{
			WorkspaceID: workspaceID,
			FromAgentID: epic.OrchestratorAgentID,
			EpicID:      &epicID,
			HandoffType: "agent_to_human",
			Reason:      fmt.Sprintf("Orchestrated %d stories", len(created)),
			Context:     json.RawMessage("{}"),
		}
		_ = s.handoffRepo.Create(ctx, handoff)
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "epic", epicID, &actorID, "updated", strPtr("stories"), nil, strPtr(fmt.Sprintf("Created %d stories from orchestration", len(created))), nil)

	return created, nil
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
