package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// DocsCollectionService handles business logic for docs collections.
type DocsCollectionService struct {
	collectionRepo *repository.DocsCollectionRepository
	spaceRepo      *repository.DocsSpaceRepository
	translationSvc *DocsHelpcenterTranslationService
}

// NewDocsCollectionService creates a new DocsCollectionService.
func NewDocsCollectionService(collectionRepo *repository.DocsCollectionRepository, spaceRepo *repository.DocsSpaceRepository) *DocsCollectionService {
	return &DocsCollectionService{collectionRepo: collectionRepo, spaceRepo: spaceRepo}
}

func (s *DocsCollectionService) SetTranslationService(translationSvc *DocsHelpcenterTranslationService) {
	s.translationSvc = translationSvc
}

// Create creates a new collection inside a space.
func (s *DocsCollectionService) Create(ctx context.Context, workspaceID, spaceID string, req model.CreateDocsCollectionRequest, userID string) (*model.DocsCollection, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, fmt.Errorf("space not found")
	}
	if space.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("space does not belong to this workspace")
	}

	// Append to end of space.
	nextPos, err := s.collectionRepo.NextPosition(ctx, spaceID)
	if err != nil {
		nextPos = 0
	}

	coll := &model.DocsCollection{
		SpaceID:     spaceID,
		WorkspaceID: workspaceID,
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		Position:    nextPos,
		CreatedBy:   userID,
	}
	if req.Slug != nil && *req.Slug != "" {
		coll.Slug = *req.Slug
	}
	created, err := s.collectionRepo.Create(ctx, coll)
	if err != nil {
		return nil, err
	}
	if s.translationSvc != nil {
		if err := s.translationSvc.RefreshCollectionSource(ctx, created.ID); err != nil {
			slog.WarnContext(ctx, "failed to refresh helpcenter collection translation source after create", "collection_id", created.ID, "error", err)
		}
	}
	return created, nil
}

// Get returns a collection by ID.
func (s *DocsCollectionService) Get(ctx context.Context, id string) (*model.DocsCollection, error) {
	return s.collectionRepo.GetByID(ctx, id)
}

// List returns all collections in a space.
func (s *DocsCollectionService) List(ctx context.Context, spaceID string) ([]model.DocsCollection, error) {
	return s.collectionRepo.ListBySpace(ctx, spaceID)
}

// Update updates a collection.
func (s *DocsCollectionService) Update(ctx context.Context, id string, req model.UpdateDocsCollectionRequest) (*model.DocsCollection, error) {
	updates := map[string]interface{}{}
	shouldRefreshTranslations := false
	if req.Name != nil {
		updates["name"] = *req.Name
		shouldRefreshTranslations = true
	}
	if req.Description != nil {
		updates["description"] = *req.Description
		shouldRefreshTranslations = true
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.Position != nil {
		updates["position"] = *req.Position
	}
	if len(updates) == 0 {
		return s.collectionRepo.GetByID(ctx, id)
	}
	updated, err := s.collectionRepo.Update(ctx, id, updates)
	if err != nil {
		return nil, err
	}
	if shouldRefreshTranslations && s.translationSvc != nil {
		if err := s.translationSvc.RefreshCollectionSource(ctx, id); err != nil {
			slog.WarnContext(ctx, "failed to refresh helpcenter collection translation source after update", "collection_id", id, "error", err)
		}
	}
	return updated, nil
}

// Delete soft-deletes a collection.
func (s *DocsCollectionService) Delete(ctx context.Context, id string) error {
	return s.collectionRepo.Delete(ctx, id)
}

// Restore restores a soft-deleted collection.
func (s *DocsCollectionService) Restore(ctx context.Context, id string) (*model.DocsCollection, error) {
	return s.collectionRepo.Restore(ctx, id)
}

// ReorderCollections reorders collections within a space.
func (s *DocsCollectionService) ReorderCollections(ctx context.Context, spaceID string, req model.ReorderDocsCollectionsRequest) error {
	if len(req.CollectionIDs) == 0 {
		return nil
	}
	return s.collectionRepo.Reorder(ctx, spaceID, req.CollectionIDs)
}
