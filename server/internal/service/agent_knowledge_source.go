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
	repo           *repository.AgentKnowledgeSourceRepository
	spaceRepo      *repository.DocsSpaceRepository
	collectionRepo *repository.DocsCollectionRepository
	documentRepo   *repository.DocsDocumentRepository
	embeddingSvc   *DocsEmbeddingService
}

// NewAgentKnowledgeSourceService creates a new AgentKnowledgeSourceService.
func NewAgentKnowledgeSourceService(
	repo *repository.AgentKnowledgeSourceRepository,
	spaceRepo *repository.DocsSpaceRepository,
	embeddingSvc *DocsEmbeddingService,
) *AgentKnowledgeSourceService {
	return NewAgentKnowledgeSourceServiceWithScopes(repo, spaceRepo, nil, nil, embeddingSvc)
}

// NewAgentKnowledgeSourceServiceWithScopes creates a knowledge source service
// that supports space-, collection-, and article-level sources.
func NewAgentKnowledgeSourceServiceWithScopes(
	repo *repository.AgentKnowledgeSourceRepository,
	spaceRepo *repository.DocsSpaceRepository,
	collectionRepo *repository.DocsCollectionRepository,
	documentRepo *repository.DocsDocumentRepository,
	embeddingSvc *DocsEmbeddingService,
) *AgentKnowledgeSourceService {
	return &AgentKnowledgeSourceService{
		repo:           repo,
		spaceRepo:      spaceRepo,
		collectionRepo: collectionRepo,
		documentRepo:   documentRepo,
		embeddingSvc:   embeddingSvc,
	}
}

// KnowledgeSourceWithSpace is a knowledge source link enriched with space metadata.
type KnowledgeSourceWithSpace struct {
	model.AgentKnowledgeSource
	SpaceName      string `json:"space_name"`
	SpaceType      string `json:"space_type"`
	CollectionName string `json:"collection_name,omitempty"`
	DocumentTitle  string `json:"document_title,omitempty"`
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
		if err != nil || space == nil || space.WorkspaceID != workspaceID || !isDocsKnowledgeSpaceType(space.Type) {
			continue
		}
		enriched := KnowledgeSourceWithSpace{AgentKnowledgeSource: src}
		enriched.SpaceName = space.Name
		enriched.SpaceType = space.Type
		if src.CollectionID != nil && s.collectionRepo != nil {
			collection, err := s.collectionRepo.GetByID(ctx, *src.CollectionID)
			if err == nil && collection != nil && collection.WorkspaceID == workspaceID {
				enriched.CollectionName = collection.Name
			}
		}
		if src.DocumentID != nil && s.documentRepo != nil {
			document, err := s.documentRepo.GetByID(ctx, *src.DocumentID)
			if err == nil && document != nil && document.WorkspaceID == workspaceID {
				enriched.DocumentTitle = document.Title
			}
		}
		result = append(result, enriched)
	}
	return result, nil
}

// Set replaces all knowledge sources for an agent and queues vector syncs for selected help-center spaces.
func (s *AgentKnowledgeSourceService) Set(ctx context.Context, workspaceID, agentID string, spaceIDs []string) error {
	scopes := make([]model.KnowledgeSourceScopeRequest, 0, len(spaceIDs))
	for _, spaceID := range spaceIDs {
		scopes = append(scopes, model.KnowledgeSourceScopeRequest{
			ScopeType: model.KnowledgeSourceScopeSpace,
			SpaceID:   spaceID,
		})
	}
	return s.SetScoped(ctx, workspaceID, agentID, scopes)
}

// SetScoped replaces all knowledge sources for an agent and queues vector syncs for selected help-center spaces.
func (s *AgentKnowledgeSourceService) SetScoped(ctx context.Context, workspaceID, agentID string, requested []model.KnowledgeSourceScopeRequest) error {
	pendingSyncSpaceIDs := map[string]struct{}{}

	if err := s.repo.Transaction(ctx, func(txRepo *repository.AgentKnowledgeSourceRepository) error {
		txSvc := *s
		txSvc.repo = txRepo
		nextPendingSyncSpaceIDs, err := txSvc.setScopedInTx(ctx, workspaceID, agentID, requested)
		if err != nil {
			return err
		}
		pendingSyncSpaceIDs = nextPendingSyncSpaceIDs
		return nil
	}); err != nil {
		return err
	}

	for spaceID := range pendingSyncSpaceIDs {
		if s.embeddingSvc != nil {
			if err := s.embeddingSvc.QueueSpaceSync(ctx, workspaceID, spaceID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *AgentKnowledgeSourceService) setScopedInTx(ctx context.Context, workspaceID, agentID string, requested []model.KnowledgeSourceScopeRequest) (map[string]struct{}, error) {
	existing, err := s.repo.ListByAgentID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	nextByKey := make(map[string]model.KnowledgeSourceScopeRequest, len(requested))
	for _, raw := range requested {
		scope, err := s.normalizeAndValidateScope(ctx, workspaceID, raw)
		if err != nil {
			return nil, err
		}
		if scope.SpaceID == "" {
			continue
		}
		nextByKey[knowledgeSourceScopeKey(scope.ScopeType, scope.SpaceID, scope.CollectionID, scope.DocumentID)] = scope
	}

	existingByKey := make(map[string]model.AgentKnowledgeSource, len(existing))
	for _, source := range existing {
		existingByKey[knowledgeSourceScopeKey(source.ScopeType, source.SpaceID, source.CollectionID, source.DocumentID)] = source
	}

	pendingSyncSpaceIDs := make(map[string]struct{}, len(nextByKey))

	for key, source := range existingByKey {
		if _, ok := nextByKey[key]; ok {
			continue
		}
		if err := s.repo.DeleteByID(ctx, source.ID); err != nil {
			return nil, err
		}
	}

	for key, scope := range nextByKey {
		if _, ok := existingByKey[key]; ok {
			continue
		}

		reusedState, err := s.reusableStateForScope(ctx, workspaceID, scope, agentID)
		if err != nil {
			return nil, err
		}

		source := &model.AgentKnowledgeSource{
			AgentID:      agentID,
			ScopeType:    scope.ScopeType,
			SpaceID:      scope.SpaceID,
			CollectionID: scope.CollectionID,
			DocumentID:   scope.DocumentID,
			WorkspaceID:  workspaceID,
			SyncStatus:   model.KnowledgeSourceSyncQueued,
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
			pendingSyncSpaceIDs[scope.SpaceID] = struct{}{}
		}
		if err := s.repo.Create(ctx, source); err != nil {
			return nil, err
		}
	}

	return pendingSyncSpaceIDs, nil
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
	if !isDocsKnowledgeSpaceType(space.Type) {
		return fmt.Errorf("only docs spaces can be reindexed")
	}

	return s.embeddingSvc.QueueSpaceSync(ctx, workspaceID, spaceID)
}

func (s *AgentKnowledgeSourceService) normalizeAndValidateScope(ctx context.Context, workspaceID string, raw model.KnowledgeSourceScopeRequest) (model.KnowledgeSourceScopeRequest, error) {
	scopeType := strings.TrimSpace(raw.ScopeType)
	if scopeType == "" {
		scopeType = model.KnowledgeSourceScopeSpace
	}
	scope := model.KnowledgeSourceScopeRequest{
		ScopeType: scopeType,
		SpaceID:   strings.TrimSpace(raw.SpaceID),
	}
	if raw.CollectionID != nil {
		collectionID := strings.TrimSpace(*raw.CollectionID)
		if collectionID != "" {
			scope.CollectionID = &collectionID
		}
	}
	if raw.DocumentID != nil {
		documentID := strings.TrimSpace(*raw.DocumentID)
		if documentID != "" {
			scope.DocumentID = &documentID
		}
	}
	if scope.SpaceID == "" {
		return model.KnowledgeSourceScopeRequest{}, nil
	}

	space, err := s.spaceRepo.GetByID(ctx, scope.SpaceID)
	if err != nil {
		return model.KnowledgeSourceScopeRequest{}, err
	}
	if space == nil || space.WorkspaceID != workspaceID {
		return model.KnowledgeSourceScopeRequest{}, fmt.Errorf("one or more space_ids do not belong to this workspace")
	}
	if !isDocsKnowledgeSpaceType(space.Type) {
		return model.KnowledgeSourceScopeRequest{}, fmt.Errorf("support AI knowledge sources must be docs spaces")
	}

	switch scope.ScopeType {
	case model.KnowledgeSourceScopeSpace:
		scope.CollectionID = nil
		scope.DocumentID = nil
	case model.KnowledgeSourceScopeCollection:
		if scope.CollectionID == nil {
			return model.KnowledgeSourceScopeRequest{}, fmt.Errorf("collection_id is required for collection knowledge sources")
		}
		if s.collectionRepo == nil {
			return model.KnowledgeSourceScopeRequest{}, fmt.Errorf("docs collection repository is not configured")
		}
		collection, err := s.collectionRepo.GetByID(ctx, *scope.CollectionID)
		if err != nil {
			return model.KnowledgeSourceScopeRequest{}, err
		}
		if collection == nil || collection.WorkspaceID != workspaceID || collection.SpaceID != scope.SpaceID {
			return model.KnowledgeSourceScopeRequest{}, fmt.Errorf("collection_id does not belong to this docs space")
		}
		scope.DocumentID = nil
	case model.KnowledgeSourceScopeArticle:
		if scope.DocumentID == nil {
			return model.KnowledgeSourceScopeRequest{}, fmt.Errorf("document_id is required for article knowledge sources")
		}
		if s.documentRepo == nil {
			return model.KnowledgeSourceScopeRequest{}, fmt.Errorf("docs document repository is not configured")
		}
		document, err := s.documentRepo.GetByID(ctx, *scope.DocumentID)
		if err != nil {
			return model.KnowledgeSourceScopeRequest{}, err
		}
		if document == nil || document.WorkspaceID != workspaceID || document.SpaceID != scope.SpaceID {
			return model.KnowledgeSourceScopeRequest{}, fmt.Errorf("document_id does not belong to this docs space")
		}
		scope.CollectionID = document.CollectionID
	default:
		return model.KnowledgeSourceScopeRequest{}, fmt.Errorf("unsupported knowledge source scope_type %q", scope.ScopeType)
	}

	return scope, nil
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

func (s *AgentKnowledgeSourceService) reusableStateForScope(ctx context.Context, workspaceID string, scope model.KnowledgeSourceScopeRequest, agentID string) (*model.AgentKnowledgeSource, error) {
	sources, err := s.repo.ListBySpaceID(ctx, workspaceID, scope.SpaceID)
	if err != nil {
		return nil, err
	}

	targetKey := knowledgeSourceScopeKey(scope.ScopeType, scope.SpaceID, scope.CollectionID, scope.DocumentID)
	var best *model.AgentKnowledgeSource
	bestRank := -1
	bestUpdatedAt := time.Time{}

	for _, source := range sources {
		if source.AgentID == agentID {
			continue
		}
		if knowledgeSourceScopeKey(source.ScopeType, source.SpaceID, source.CollectionID, source.DocumentID) != targetKey {
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

func knowledgeSourceScopeKey(scopeType, spaceID string, collectionID, documentID *string) string {
	if strings.TrimSpace(scopeType) == "" {
		scopeType = model.KnowledgeSourceScopeSpace
	}
	collection := ""
	if collectionID != nil {
		collection = strings.TrimSpace(*collectionID)
	}
	document := ""
	if documentID != nil {
		document = strings.TrimSpace(*documentID)
	}
	return strings.Join([]string{scopeType, strings.TrimSpace(spaceID), collection, document}, ":")
}

func isDocsKnowledgeSpaceType(spaceType string) bool {
	return spaceType == model.SpaceTypeExternalCapable || spaceType == model.SpaceTypeInternal
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
