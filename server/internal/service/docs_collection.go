package service

import (
	"context"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// DocsCollectionService handles business logic for docs collections.
type DocsCollectionService struct {
	collectionRepo *repository.DocsCollectionRepository
	spaceRepo      *repository.DocsSpaceRepository
	translationSvc *DocsHelpcenterTranslationService
	wsPublisher    *websocket.Publisher
}

// NewDocsCollectionService creates a new DocsCollectionService.
func NewDocsCollectionService(collectionRepo *repository.DocsCollectionRepository, spaceRepo *repository.DocsSpaceRepository, wsPublisher *websocket.Publisher) *DocsCollectionService {
	return &DocsCollectionService{collectionRepo: collectionRepo, spaceRepo: spaceRepo, wsPublisher: wsPublisher}
}

func (s *DocsCollectionService) SetTranslationService(translationSvc *DocsHelpcenterTranslationService) {
	s.translationSvc = translationSvc
}

// maxCollectionDepth is the deepest allowed value of DocsCollection.Depth.
// With the root at depth 0 this gives three navigable tiers per space:
// top-level, child, and grandchild.
const maxCollectionDepth = 2

// Create creates a new collection inside a space. If ParentCollectionID is
// provided and non-empty, the collection is inserted as a child of that
// parent within the same space; the depth is derived from the parent and
// validated against maxCollectionDepth.
func (s *DocsCollectionService) Create(ctx context.Context, workspaceID, spaceID string, req model.CreateDocsCollectionRequest, userID string) (*model.DocsCollection, error) {
	if req.Name == "" {
		return nil, ErrDocsCollectionNameRequired
	}

	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, ErrDocsSpaceNotFound
	}
	if space.WorkspaceID != workspaceID {
		return nil, ErrDocsCrossWorkspace
	}

	parentID, parentDepth, err := s.resolveParentForCreate(ctx, spaceID, req.ParentCollectionID)
	if err != nil {
		return nil, err
	}
	newDepth := 0
	if parentID != nil {
		newDepth = parentDepth + 1
	}
	if newDepth > maxCollectionDepth {
		return nil, ErrDocsCollectionDepthExceeded
	}

	// Append to end of the target sibling bucket.
	nextPos, err := s.collectionRepo.NextPositionInBucket(ctx, spaceID, parentID)
	if err != nil {
		nextPos = 0
	}

	coll := &model.DocsCollection{
		SpaceID:            spaceID,
		WorkspaceID:        workspaceID,
		ParentCollectionID: parentID,
		Depth:              newDepth,
		Name:               req.Name,
		Slug:               slugify(req.Name),
		Description:        req.Description,
		Icon:               req.Icon,
		Position:           nextPos,
		CreatedBy:          userID,
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
		// Auto-generate translations for all enabled locales if space is external
		go func() {
			if err := s.translationSvc.AutoGenerateCollectionTranslations(ctx, created.ID); err != nil {
				slog.WarnContext(ctx, "failed to auto-generate collection translations", "collection_id", created.ID, "error", err)
			}
		}()
	}
	publishWorkspaceEventWithParent(s.wsPublisher, "created", "docs_collection", created.ID, workspaceID, userID, "docs_space", spaceID, nil)
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

// ListByWorkspace returns all collections across all spaces in a workspace.
func (s *DocsCollectionService) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.DocsCollection, error) {
	return s.collectionRepo.ListByWorkspace(ctx, workspaceID)
}

// Update updates a collection. If ParentCollectionID is set in the request,
// the collection is reparented first (validated + Reparent), then the
// remaining field updates are applied.
func (s *DocsCollectionService) Update(ctx context.Context, id string, req model.UpdateDocsCollectionRequest) (*model.DocsCollection, error) {
	current, err := s.collectionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrDocsCollectionNotFound
	}

	// Step 1: reparent if requested. The nil-pointer sentinel means "leave
	// parent unchanged"; a pointer to the empty string means "reparent to
	// the top of the space"; anything else is an explicit parent ID.
	if req.ParentCollectionID != nil {
		newParentID, err := s.resolveParentForReparent(ctx, current, *req.ParentCollectionID)
		if err != nil {
			return nil, err
		}
		if err := s.collectionRepo.Reparent(ctx, id, newParentID); err != nil {
			return nil, err
		}
	}

	// Step 2: apply the remaining partial updates.
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
		updated, err := s.collectionRepo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if updated != nil {
			publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_collection", updated.ID, updated.WorkspaceID, "", "docs_space", updated.SpaceID, nil)
		}
		return updated, nil
	}
	updated, err := s.collectionRepo.Update(ctx, id, updates)
	if err != nil {
		return nil, err
	}
	if req.Position != nil && updated != nil {
		// Position was changed by hand — re-normalize the owning bucket so
		// any ties introduced by manual position overrides are compacted.
		if err := s.collectionRepo.NormalizeBucket(ctx, updated.SpaceID, updated.ParentCollectionID); err != nil {
			slog.WarnContext(ctx, "normalize bucket after position update failed", "collection_id", id, "error", err)
		}
	}
	if shouldRefreshTranslations && s.translationSvc != nil {
		if err := s.translationSvc.RefreshCollectionSource(ctx, id); err != nil {
			slog.WarnContext(ctx, "failed to refresh helpcenter collection translation source after update", "collection_id", id, "error", err)
		}
	}
	publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_collection", updated.ID, updated.WorkspaceID, "", "docs_space", updated.SpaceID, nil)
	return updated, nil
}

// resolveParentForCreate validates the requested parent for a new
// collection. It returns the canonical parent ID (nil for top-level), the
// parent's depth (0 when no parent), and a typed sentinel error if the
// parent is missing or lives in a different space.
func (s *DocsCollectionService) resolveParentForCreate(ctx context.Context, spaceID string, requested *string) (*string, int, error) {
	if requested == nil || *requested == "" {
		return nil, 0, nil
	}
	parent, err := s.collectionRepo.GetByID(ctx, *requested)
	if err != nil {
		return nil, 0, err
	}
	if parent == nil {
		return nil, 0, ErrDocsCollectionParentNotFound
	}
	if parent.SpaceID != spaceID {
		return nil, 0, ErrDocsCollectionParentDifferentSpace
	}
	id := parent.ID
	return &id, parent.Depth, nil
}

// resolveParentForReparent validates a reparent target for an existing
// collection. It enforces:
//   - parent exists and is in the same space
//   - target is not the collection itself
//   - target is not one of the collection's descendants (no cycles)
//   - moving under the target does not push the collection's subtree past
//     maxCollectionDepth
//
// All rejection cases return typed sentinel errors so handlers can map
// them to precise HTTP responses.
func (s *DocsCollectionService) resolveParentForReparent(ctx context.Context, current *model.DocsCollection, requested string) (*string, error) {
	if requested == "" {
		// Reparent to the top of the space — no parent, no further checks.
		return nil, nil
	}
	if requested == current.ID {
		return nil, ErrDocsCollectionSelfParent
	}

	parent, err := s.collectionRepo.GetByID(ctx, requested)
	if err != nil {
		return nil, err
	}
	if parent == nil {
		return nil, ErrDocsCollectionParentNotFound
	}
	if parent.SpaceID != current.SpaceID {
		return nil, ErrDocsCollectionParentDifferentSpace
	}

	descendants, err := s.collectionRepo.ListDescendants(ctx, current.ID)
	if err != nil {
		return nil, err
	}
	for _, d := range descendants {
		if d.ID == requested {
			return nil, ErrDocsCollectionCycle
		}
	}

	// The moved collection takes on parent.Depth + 1. Its deepest
	// descendant takes on that depth plus the subtree's relative depth.
	newRootDepth := parent.Depth + 1
	if newRootDepth > maxCollectionDepth {
		return nil, ErrDocsCollectionDepthExceeded
	}
	delta := newRootDepth - current.Depth
	for _, d := range descendants {
		if d.Depth+delta > maxCollectionDepth {
			return nil, ErrDocsCollectionDepthExceeded
		}
	}

	id := parent.ID
	return &id, nil
}

// Delete soft-deletes a collection.
func (s *DocsCollectionService) Delete(ctx context.Context, id string) error {
	collection, err := s.collectionRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if collection == nil {
		return ErrDocsCollectionNotFound
	}
	if err := s.collectionRepo.Delete(ctx, id); err != nil {
		return err
	}
	publishWorkspaceEventWithParent(s.wsPublisher, "deleted", "docs_collection", id, collection.WorkspaceID, "", "docs_space", collection.SpaceID, nil)
	return nil
}

// Restore restores a soft-deleted collection.
func (s *DocsCollectionService) Restore(ctx context.Context, id string) (*model.DocsCollection, error) {
	collection, err := s.collectionRepo.Restore(ctx, id)
	if err == nil && collection != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_collection", collection.ID, collection.WorkspaceID, "", "docs_space", collection.SpaceID, nil)
	}
	return collection, err
}

// ReorderCollections reorders collections within a space.
func (s *DocsCollectionService) ReorderCollections(ctx context.Context, spaceID string, req model.ReorderDocsCollectionsRequest) error {
	if len(req.CollectionIDs) == 0 {
		return nil
	}
	if err := s.collectionRepo.Reorder(ctx, spaceID, req.CollectionIDs); err != nil {
		return err
	}
	if space, err := s.spaceRepo.GetByID(ctx, spaceID); err == nil && space != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "reordered", "docs_collection", req.CollectionIDs[0], space.WorkspaceID, "", "docs_space", spaceID, nil)
	}
	return nil
}
