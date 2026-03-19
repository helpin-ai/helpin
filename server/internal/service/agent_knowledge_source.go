package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// AgentKnowledgeSourceService handles business logic for agent knowledge source links.
type AgentKnowledgeSourceService struct {
	repo      *repository.AgentKnowledgeSourceRepository
	spaceRepo *repository.DocsSpaceRepository
}

// NewAgentKnowledgeSourceService creates a new AgentKnowledgeSourceService.
func NewAgentKnowledgeSourceService(
	repo *repository.AgentKnowledgeSourceRepository,
	spaceRepo *repository.DocsSpaceRepository,
) *AgentKnowledgeSourceService {
	return &AgentKnowledgeSourceService{
		repo:      repo,
		spaceRepo: spaceRepo,
	}
}

// KnowledgeSourceWithSpace is a knowledge source link enriched with space metadata.
type KnowledgeSourceWithSpace struct {
	model.AgentKnowledgeSource
	SpaceName string `json:"space_name"`
	SpaceType string `json:"space_type"`
}

// List returns knowledge sources for an agent with space metadata.
func (s *AgentKnowledgeSourceService) List(ctx context.Context, workspaceID, agentID string) ([]KnowledgeSourceWithSpace, error) {
	sources, err := s.repo.ListByAgentID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	result := make([]KnowledgeSourceWithSpace, 0, len(sources))
	for _, src := range sources {
		enriched := KnowledgeSourceWithSpace{AgentKnowledgeSource: src}
		if space, err := s.spaceRepo.GetByID(ctx, src.SpaceID); err == nil && space != nil {
			enriched.SpaceName = space.Name
			enriched.SpaceType = space.Type
		}
		result = append(result, enriched)
	}
	return result, nil
}

// Set replaces all knowledge sources for an agent. Delegates workspace tenancy validation to the repo.
func (s *AgentKnowledgeSourceService) Set(ctx context.Context, workspaceID, agentID string, spaceIDs []string) error {
	return s.repo.Set(ctx, workspaceID, agentID, spaceIDs)
}
