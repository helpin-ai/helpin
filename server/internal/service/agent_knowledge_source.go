package service

import (
	"context"
	"fmt"
	"strings"
	"time"

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

	pendingSyncSpaceIDs := make([]string, 0, len(nextBySpace))

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

		reusedState, err := s.reusableStateForSpace(ctx, workspaceID, spaceID, agentID)
		if err != nil {
			return err
		}

		source := &model.AgentKnowledgeSource{
			AgentID:     agentID,
			SpaceID:     spaceID,
			WorkspaceID: workspaceID,
			SyncStatus:  model.KnowledgeSourceSyncQueued,
		}
		if reusedState != nil {
			source.SyncStatus = reusedState.SyncStatus
			source.SyncProgress = reusedState.SyncProgress
			source.IndexedDocuments = reusedState.IndexedDocuments
			source.IndexedChunks = reusedState.IndexedChunks
			source.LastSyncError = reusedState.LastSyncError
			source.LastSyncStartedAt = reusedState.LastSyncStartedAt
			source.LastSyncCompletedAt = reusedState.LastSyncCompletedAt
		} else {
			pendingSyncSpaceIDs = append(pendingSyncSpaceIDs, spaceID)
		}
		if err := s.repo.Create(ctx, source); err != nil {
			return err
		}
	}

	for _, spaceID := range pendingSyncSpaceIDs {
		if s.embeddingSvc != nil {
			if err := s.embeddingSvc.QueueSpaceSync(ctx, workspaceID, spaceID); err != nil {
				return err
			}
		}
	}
	return nil
}

// Reindex queues a fresh embedding sync for one selected help-center space.
func (s *AgentKnowledgeSourceService) Reindex(ctx context.Context, workspaceID, agentID, spaceID string) error {
	if strings.TrimSpace(spaceID) == "" {
		return fmt.Errorf("space_id is required")
	}

	if s.embeddingSvc == nil {
		return fmt.Errorf("docs embedding service is not configured")
	}

	sources, err := s.repo.ListByAgentID(ctx, agentID)
	if err != nil {
		return err
	}

	selected := false
	for _, source := range sources {
		if source.WorkspaceID == workspaceID && source.SpaceID == spaceID {
			selected = true
			break
		}
	}
	if !selected {
		return fmt.Errorf("knowledge source not found for this agent")
	}

	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		return err
	}
	if space == nil || space.WorkspaceID != workspaceID {
		return fmt.Errorf("space not found in workspace")
	}
	if space.Type != model.SpaceTypeExternalCapable {
		return fmt.Errorf("only help center spaces can be reindexed")
	}

	return s.embeddingSvc.QueueSpaceSync(ctx, workspaceID, spaceID)
}

func (s *AgentKnowledgeSourceService) reusableStateForSpace(ctx context.Context, workspaceID, spaceID, agentID string) (*model.AgentKnowledgeSource, error) {
	sources, err := s.repo.ListBySpaceID(ctx, workspaceID, spaceID)
	if err != nil {
		return nil, err
	}

	var best *model.AgentKnowledgeSource
	bestRank := -1
	bestUpdatedAt := time.Time{}

	for _, source := range sources {
		if source.AgentID == agentID {
			continue
		}
		rank := knowledgeSourceStatusRank(source.SyncStatus)
		if best == nil || rank > bestRank || (rank == bestRank && source.UpdatedAt.After(bestUpdatedAt)) {
			candidate := source
			best = &candidate
			bestRank = rank
			bestUpdatedAt = source.UpdatedAt
		}
	}

	return best, nil
}

func knowledgeSourceStatusRank(status string) int {
	switch status {
	case model.KnowledgeSourceSyncRunning:
		return 5
	case model.KnowledgeSourceSyncQueued:
		return 4
	case model.KnowledgeSourceSyncReady:
		return 3
	case model.KnowledgeSourceSyncStale:
		return 2
	case model.KnowledgeSourceSyncFailed:
		return 1
	case model.KnowledgeSourceSyncDisabled:
		return 0
	default:
		return -1
	}
}
