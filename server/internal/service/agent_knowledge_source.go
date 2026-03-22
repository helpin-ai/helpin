package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// AgentKnowledgeSourceService handles business logic for agent knowledge source links.
type AgentKnowledgeSourceService struct {
	repo         *repository.AgentKnowledgeSourceRepository
	spaceRepo    *repository.DocsSpaceRepository
	embeddingSvc *DocsEmbeddingService
}

// NewAgentKnowledgeSourceService creates a new AgentKnowledgeSourceService.
func NewAgentKnowledgeSourceService(
	repo *repository.AgentKnowledgeSourceRepository,
	spaceRepo *repository.DocsSpaceRepository,
	embeddingSvc *DocsEmbeddingService,
) *AgentKnowledgeSourceService {
	return &AgentKnowledgeSourceService{
		repo:         repo,
		spaceRepo:    spaceRepo,
		embeddingSvc: embeddingSvc,
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
		space, err := s.spaceRepo.GetByID(ctx, src.SpaceID)
		if err != nil || space == nil || space.WorkspaceID != workspaceID || space.Type != model.SpaceTypeExternalCapable {
			continue
		}
		enriched := KnowledgeSourceWithSpace{AgentKnowledgeSource: src}
		enriched.SpaceName = space.Name
		enriched.SpaceType = space.Type
		result = append(result, enriched)
	}
	return result, nil
}

// Set replaces all knowledge sources for an agent and queues vector syncs for selected help-center spaces.
func (s *AgentKnowledgeSourceService) Set(ctx context.Context, workspaceID, agentID string, spaceIDs []string) error {
	existing, err := s.repo.ListByAgentID(ctx, agentID)
	if err != nil {
		return err
	}

	nextBySpace := make(map[string]bool, len(spaceIDs))
	for _, rawSpaceID := range spaceIDs {
		spaceID := strings.TrimSpace(rawSpaceID)
		if spaceID == "" || nextBySpace[spaceID] {
			continue
		}
		space, err := s.spaceRepo.GetByID(ctx, spaceID)
		if err != nil {
			return err
		}
		if space == nil || space.WorkspaceID != workspaceID {
			return fmt.Errorf("one or more space_ids do not belong to this workspace")
		}
		if space.Type != model.SpaceTypeExternalCapable {
			return fmt.Errorf("support AI knowledge sources must be help center spaces")
		}
		nextBySpace[spaceID] = true
	}

	existingBySpace := make(map[string]model.AgentKnowledgeSource, len(existing))
	for _, source := range existing {
		existingBySpace[source.SpaceID] = source
	}

	for spaceID := range existingBySpace {
		if nextBySpace[spaceID] {
			continue
		}
		if err := s.repo.DeleteByAgentAndSpace(ctx, agentID, spaceID); err != nil {
			return err
		}
	}

	for spaceID := range nextBySpace {
		if _, ok := existingBySpace[spaceID]; ok {
			continue
		}
		source := &model.AgentKnowledgeSource{
			AgentID:     agentID,
			SpaceID:     spaceID,
			WorkspaceID: workspaceID,
			SyncStatus:  model.KnowledgeSourceSyncQueued,
		}
		if err := s.repo.Create(ctx, source); err != nil {
			return err
		}
	}

	for spaceID := range nextBySpace {
		if s.embeddingSvc != nil {
			if err := s.embeddingSvc.QueueSpaceSync(ctx, workspaceID, spaceID); err != nil {
				return err
			}
		}
	}
	return nil
}
