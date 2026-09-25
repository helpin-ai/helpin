package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// KnowledgeEmbeddingBackfill queues indexing for workspace knowledge that has
// no vectors yet, through the same durable docs and content sync workflows
// that normal edits use. Already indexed knowledge is not re-embedded.
type KnowledgeEmbeddingBackfill struct {
	docs    *DocsEmbeddingService
	content *SupportContentSyncService
}

// NewKnowledgeEmbeddingBackfill creates a backfill over the docs and content
// sync services; either may be nil.
func NewKnowledgeEmbeddingBackfill(docs *DocsEmbeddingService, content *SupportContentSyncService) *KnowledgeEmbeddingBackfill {
	return &KnowledgeEmbeddingBackfill{docs: docs, content: content}
}

// QueueMissingEmbeddings queues every unindexed docs space and content source.
func (b *KnowledgeEmbeddingBackfill) QueueMissingEmbeddings(ctx context.Context, workspaceID string) error {
	if b == nil {
		return nil
	}
	return errors.Join(b.docs.QueueMissingEmbeddings(ctx, workspaceID), b.content.QueueMissingEmbeddings(ctx, workspaceID))
}

// QueueMissingEmbeddings queues a sync for each docs space that is linked as
// agent knowledge but was never indexed (disabled, failed, or no chunks), and
// for each help-center space without searchable chunks.
func (s *DocsEmbeddingService) QueueMissingEmbeddings(ctx context.Context, workspaceID string) error {
	if s == nil || s.knowledgeRepo == nil {
		return nil
	}
	spaceIDs := []string{}
	seen := map[string]bool{}
	add := func(spaceID string) {
		if spaceID != "" && !seen[spaceID] {
			seen[spaceID] = true
			spaceIDs = append(spaceIDs, spaceID)
		}
	}
	sources, err := s.knowledgeRepo.ListByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list knowledge sources: %w", err)
	}
	for _, source := range sources {
		if docsKnowledgeSourceNeedsIndex(source) {
			add(source.SpaceID)
		}
	}
	if s.spaceRepo != nil && s.chunkRepo != nil {
		spaces, err := s.spaceRepo.ListPublicByWorkspace(ctx, workspaceID)
		if err != nil {
			return err
		}
		indexed, err := s.chunkRepo.ListSearchableExternalSpaceIDs(ctx, workspaceID)
		if err != nil {
			return err
		}
		searchable := make(map[string]bool, len(indexed))
		for _, id := range indexed {
			searchable[id] = true
		}
		for _, space := range spaces {
			if !searchable[space.ID] {
				add(space.ID)
			}
		}
	}
	var errs []error
	for _, spaceID := range spaceIDs {
		if err := s.QueueSpaceSync(ctx, workspaceID, spaceID); err != nil {
			errs = append(errs, fmt.Errorf("queue docs space %s: %w", spaceID, err))
		}
	}
	return errors.Join(errs...)
}

func docsKnowledgeSourceNeedsIndex(source model.AgentKnowledgeSource) bool {
	switch source.SyncStatus {
	case model.KnowledgeSourceSyncDisabled, model.KnowledgeSourceSyncFailed:
		return true
	case model.KnowledgeSourceSyncQueued, model.KnowledgeSourceSyncRunning:
		return false
	default:
		return source.IndexedChunks == 0
	}
}

// QueueMissingEmbeddings queues a sync for content sources that were disabled
// or failed and hold no chunks, which is how they end when no embedding
// provider existed.
func (s *SupportContentSyncService) QueueMissingEmbeddings(ctx context.Context, workspaceID string) error {
	if s == nil || s.sourceRepo == nil {
		return nil
	}
	sources, err := s.sourceRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list content sources: %w", err)
	}
	var errs []error
	for _, source := range sources {
		if source.IndexedChunks > 0 {
			continue
		}
		if source.SyncStatus != model.KnowledgeSourceSyncDisabled && source.SyncStatus != model.KnowledgeSourceSyncFailed {
			continue
		}
		if err := s.QueueSourceSync(ctx, workspaceID, source.ID); err != nil {
			errs = append(errs, fmt.Errorf("queue content source %s: %w", source.ID, err))
		}
	}
	return errors.Join(errs...)
}

var _ WorkspaceEmbeddingBackfiller = (*KnowledgeEmbeddingBackfill)(nil)
