package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// DocsCollectionService handles business logic for docs collections.
type DocsCollectionService struct {
	collectionRepo *repository.DocsCollectionRepository
	spaceRepo      *repository.DocsSpaceRepository
}

// NewDocsCollectionService creates a new DocsCollectionService.
func NewDocsCollectionService(collectionRepo *repository.DocsCollectionRepository, spaceRepo *repository.DocsSpaceRepository) *DocsCollectionService {
	return &DocsCollectionService{collectionRepo: collectionRepo, spaceRepo: spaceRepo}
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

	coll := &model.DocsCollection{
		SpaceID:     spaceID,
		WorkspaceID: workspaceID,
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		CreatedBy:   userID,
	}
	if req.Slug != nil && *req.Slug != "" {
		coll.Slug = *req.Slug
	}
	return s.collectionRepo.Create(ctx, coll)
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
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
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
	return s.collectionRepo.Update(ctx, id, updates)
}

// Delete soft-deletes a collection.
func (s *DocsCollectionService) Delete(ctx context.Context, id string) error {
	return s.collectionRepo.Delete(ctx, id)
}

// Restore restores a soft-deleted collection.
func (s *DocsCollectionService) Restore(ctx context.Context, id string) (*model.DocsCollection, error) {
	return s.collectionRepo.Restore(ctx, id)
}
