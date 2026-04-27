package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/ordering"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// sameCollectionParentPointer returns true when two *string parent
// references point at the same collection. Nil == nil (both top-level),
// non-nil values compare by dereferenced string, and mismatched
// nil/non-nil combinations are considered different.
func sameCollectionParentPointer(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// isUniqueConstraintViolation returns true when err looks like a
// Postgres unique-constraint violation from the partial unique index
// on docs_collections(workspace_id, slug) where deleted_at is null.
// Postgres returns SQLSTATE 23505 for unique violations; the fallback
// substring match catches sqlite-driver test contexts where the error
// type wrapping is slightly different.
func isUniqueConstraintViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "sqlstate 23505") {
		return true
	}
	if strings.Contains(msg, "unique constraint") {
		return true
	}
	if strings.Contains(msg, "unique") && strings.Contains(msg, "violation") {
		return true
	}
	return false
}

// DocsCollectionService handles business logic for docs collections.
type DocsCollectionService struct {
	collectionRepo  *repository.DocsCollectionRepository
	spaceRepo       *repository.DocsSpaceRepository
	docRepo         *repository.DocsDocumentRepository
	documentSvc     *DocsDocumentService
	helpcenterRepo  *repository.DocsHelpcenterRepository
	translationRepo *repository.DocsHelpcenterTranslationRepository
	translationSvc  *DocsHelpcenterTranslationService
	wsPublisher     *websocket.Publisher
	useSortKey      bool
}

// NewDocsCollectionService creates a new DocsCollectionService.
func NewDocsCollectionService(collectionRepo *repository.DocsCollectionRepository, spaceRepo *repository.DocsSpaceRepository, wsPublisher *websocket.Publisher, useSortKey bool) *DocsCollectionService {
	return &DocsCollectionService{collectionRepo: collectionRepo, spaceRepo: spaceRepo, wsPublisher: wsPublisher, useSortKey: useSortKey}
}

func (s *DocsCollectionService) SetTranslationService(translationSvc *DocsHelpcenterTranslationService) {
	s.translationSvc = translationSvc
}

func (s *DocsCollectionService) SetPermanentDeleteDependencies(docRepo *repository.DocsDocumentRepository, documentSvc *DocsDocumentService, translationRepo *repository.DocsHelpcenterTranslationRepository) {
	s.docRepo = docRepo
	s.documentSvc = documentSvc
	s.translationRepo = translationRepo
}

func (s *DocsCollectionService) SetHelpcenterRepository(helpcenterRepo *repository.DocsHelpcenterRepository) {
	s.helpcenterRepo = helpcenterRepo
}

// maxCollectionDepth is the deepest allowed value of DocsCollection.Depth.
// With the root at depth 0 this gives three navigable tiers per space:
// top-level, child, and grandchild.
const maxCollectionDepth = 1

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

	slug := slugify(req.Name)
	if req.Slug != nil && *req.Slug != "" {
		slug = slugify(*req.Slug)
	}

	publicID, err := s.generateUniqueCollectionPublicID(ctx, "")
	if err != nil {
		return nil, err
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
		PublicID:           publicID,
		Slug:               slug,
		Description:        req.Description,
		Icon:               req.Icon,
		Position:           nextPos,
		CreatedBy:          userID,
	}

	if s.useSortKey {
		lastKey, err := s.collectionRepo.LastSortKeyInBucket(ctx, spaceID, parentID)
		if err != nil {
			slog.ErrorContext(ctx, "last collection sort key failed", "error", err)
		}
		if key, err := ordering.Between(lastKey, ""); err == nil {
			coll.SortKey = key
		}
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
	//
	// When the resolved parent matches the current parent we skip the
	// reparent call entirely — repo.Reparent appends to the end of the
	// target sibling bucket, so a no-op "update" that happened to echo
	// the current parent_collection_id would silently reshuffle sibling
	// order. Comparing the pointer values handles all three cases
	// (both nil, both non-nil equal, mismatch) cleanly.
	if req.ParentCollectionID != nil {
		newParentID, err := s.resolveParentForReparent(ctx, current, *req.ParentCollectionID)
		if err != nil {
			return nil, err
		}
		if !sameCollectionParentPointer(current.ParentCollectionID, newParentID) {
			if err := s.collectionRepo.Reparent(ctx, id, newParentID); err != nil {
				return nil, err
			}
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

func (s *DocsCollectionService) generateUniqueCollectionPublicID(ctx context.Context, excludeID string) (string, error) {
	const maxAttempts = 16
	for attempt := 0; attempt < maxAttempts; attempt++ {
		publicID, err := generateDocsHelpcenterPublicID()
		if err != nil {
			return "", err
		}
		taken, err := s.collectionRepo.PublicIDExists(ctx, publicID, excludeID)
		if err != nil {
			return "", err
		}
		if !taken {
			return publicID, nil
		}
	}
	return "", fmt.Errorf("generate unique docs collection public id: exhausted %d attempts", maxAttempts)
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

// Delete permanently deletes a collection subtree when permanent-delete
// dependencies are wired. Without those dependencies it preserves the legacy
// repository-level soft-delete behavior used by older tests and callers.
func (s *DocsCollectionService) Delete(ctx context.Context, workspaceID, id string) error {
	collection, err := s.collectionRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if collection == nil {
		return ErrDocsCollectionNotFound
	}
	if workspaceID != "" && collection.WorkspaceID != workspaceID {
		return ErrDocsCrossWorkspace
	}
	if s.documentSvc != nil && s.docRepo != nil {
		collections, err := s.collectionRepo.ListBySpace(ctx, collection.SpaceID)
		if err != nil {
			return err
		}
		subtreeIDs := docsCollectionSubtreeIDs(id, collections)
		// Collect all doc ids across the subtree in one pass, then delete the
		// whole batch at once. This avoids the per-doc survivor scan that
		// dominates delete time for large collections.
		var documentIDs []string
		for _, collectionID := range subtreeIDs {
			docs, err := s.docRepo.List(ctx, collection.WorkspaceID, &collection.SpaceID, &collectionID, nil, nil, "", true)
			if err != nil {
				return err
			}
			for _, doc := range docs {
				documentIDs = append(documentIDs, doc.ID)
			}
		}
		if err := s.documentSvc.DeleteDocumentsPermanently(ctx, collection.WorkspaceID, documentIDs); err != nil {
			return err
		}
		if s.translationRepo != nil {
			if err := s.translationRepo.DeleteCollectionTranslationsByCollectionIDs(ctx, subtreeIDs); err != nil {
				return err
			}
		}
		if err := s.collectionRepo.HardDeleteByIDs(ctx, subtreeIDs); err != nil {
			return err
		}
		publishWorkspaceEventWithParent(s.wsPublisher, "deleted", "docs_collection", id, collection.WorkspaceID, "", "docs_space", collection.SpaceID, nil)
		return nil
	}
	if err := s.collectionRepo.Delete(ctx, id); err != nil {
		return err
	}
	publishWorkspaceEventWithParent(s.wsPublisher, "deleted", "docs_collection", id, collection.WorkspaceID, "", "docs_space", collection.SpaceID, nil)
	return nil
}

func docsCollectionSubtreeIDs(rootID string, collections []model.DocsCollection) []string {
	ids := []string{rootID}
	seen := map[string]struct{}{rootID: {}}
	for {
		added := false
		for _, collection := range collections {
			if _, ok := seen[collection.ID]; ok || collection.ParentCollectionID == nil {
				continue
			}
			if _, parentInSubtree := seen[*collection.ParentCollectionID]; parentInSubtree {
				seen[collection.ID] = struct{}{}
				ids = append(ids, collection.ID)
				added = true
			}
		}
		if !added {
			return ids
		}
	}
}

func (s *DocsCollectionService) GetDeleteImpact(ctx context.Context, workspaceID, id string) (*model.DocsCollectionDeleteImpact, error) {
	collection, err := s.collectionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if collection == nil {
		return nil, ErrDocsCollectionNotFound
	}
	if workspaceID != "" && collection.WorkspaceID != workspaceID {
		return nil, ErrDocsCrossWorkspace
	}

	collections, err := s.collectionRepo.ListBySpace(ctx, collection.SpaceID)
	if err != nil {
		return nil, err
	}
	subtreeIDs := docsCollectionSubtreeIDs(id, collections)
	impact := &model.DocsCollectionDeleteImpact{
		CollectionID:    collection.ID,
		CollectionName:  collection.Name,
		SpaceID:         collection.SpaceID,
		CollectionCount: len(subtreeIDs),
	}
	if s.docRepo == nil {
		return impact, nil
	}

	documentIDs := []string{}
	for _, collectionID := range subtreeIDs {
		docs, err := s.docRepo.List(ctx, collection.WorkspaceID, &collection.SpaceID, &collectionID, nil, nil, "", true)
		if err != nil {
			return nil, err
		}
		for _, doc := range docs {
			documentIDs = append(documentIDs, doc.ID)
			impact.DocumentCount++
			switch doc.Status {
			case model.DocStatusArchived:
				impact.ArchivedDocumentCount++
			case model.DocStatusPublished:
				impact.PublishedDocumentCount++
			}
		}
	}
	if s.helpcenterRepo != nil {
		publicCount, err := s.helpcenterRepo.CountPublicArticlesByDocumentIDs(ctx, documentIDs)
		if err != nil {
			return nil, err
		}
		impact.PublicDocumentCount = publicCount
	}
	return impact, nil
}

// Restore restores a soft-deleted collection.
func (s *DocsCollectionService) Restore(ctx context.Context, id string) (*model.DocsCollection, error) {
	collection, err := s.collectionRepo.Restore(ctx, id)
	if err == nil && collection != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_collection", collection.ID, collection.WorkspaceID, "", "docs_space", collection.SpaceID, nil)
	}
	return collection, err
}

// ReorderCollections reorders one (space_id, parent_collection_id)
// sibling bucket. An empty or nil ParentCollectionID targets the
// top-level bucket in the space. Collection IDs that do not belong to
// the targeted bucket are silently skipped by the repository, so a
// stale payload cannot displace sibling groups.
func (s *DocsCollectionService) ReorderCollections(ctx context.Context, spaceID string, req model.ReorderDocsCollectionsRequest) error {
	if len(req.CollectionIDs) == 0 {
		return nil
	}
	var parentID *string
	if req.ParentCollectionID != nil && *req.ParentCollectionID != "" {
		parentID = req.ParentCollectionID
	}
	if err := s.collectionRepo.ReorderSiblings(ctx, spaceID, parentID, req.CollectionIDs); err != nil {
		return err
	}

	if s.useSortKey {
		prevKey := ""
		for _, id := range req.CollectionIDs {
			key, err := ordering.Between(prevKey, "")
			if err != nil {
				return fmt.Errorf("compute sort key for collection reorder: %w", err)
			}
			if err := s.collectionRepo.UpdateSortKey(ctx, id, key); err != nil {
				return fmt.Errorf("update sort key for collection %s: %w", id, err)
			}
			prevKey = key
		}
	}

	if space, err := s.spaceRepo.GetByID(ctx, spaceID); err == nil && space != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "reordered", "docs_collection", req.CollectionIDs[0], space.WorkspaceID, "", "docs_space", spaceID, nil)
	}
	return nil
}

// ReorderChildren reassigns positions across a mixed list of collections
// and articles sharing the same parent (or the space root). Both types
// get sequential positions in one transaction so cross-type drag-and-drop
// persists correctly.
func (s *DocsCollectionService) ReorderChildren(ctx context.Context, spaceID string, req model.ReorderDocsChildrenRequest) error {
	if len(req.Items) == 0 {
		return nil
	}
	var parentID *string
	if req.ParentCollectionID != nil && *req.ParentCollectionID != "" {
		parentID = req.ParentCollectionID
	}
	ordered := make([]repository.OrderedChild, 0, len(req.Items))
	for _, item := range req.Items {
		kind := repository.ChildKind(item.Kind)
		if kind != repository.ChildKindCollection && kind != repository.ChildKindArticle {
			return fmt.Errorf("invalid child kind: %q", item.Kind)
		}
		if item.ID == "" {
			return fmt.Errorf("child id is required")
		}
		ordered = append(ordered, repository.OrderedChild{Kind: kind, ID: item.ID})
	}
	if err := s.collectionRepo.ReorderChildren(ctx, spaceID, parentID, ordered); err != nil {
		return err
	}

	// Rebuild sort_keys from scratch for the full submitted mixed list.
	if s.useSortKey {
		prevKey := ""
		for _, child := range ordered {
			key, err := ordering.Between(prevKey, "")
			if err != nil {
				return fmt.Errorf("compute sort key for children reorder: %w", err)
			}
			switch child.Kind {
			case repository.ChildKindCollection:
				if err := s.collectionRepo.UpdateSortKey(ctx, child.ID, key); err != nil {
					return fmt.Errorf("update sort key for collection %s: %w", child.ID, err)
				}
			case repository.ChildKindArticle:
				if err := s.docRepo.UpdateSortKey(ctx, child.ID, key); err != nil {
					// docRepo may be nil if not wired via SetPermanentDeleteDependencies.
					slog.ErrorContext(ctx, "update sort key for doc in children reorder", "doc_id", child.ID, "error", err)
				}
			}
			prevKey = key
		}
	}

	if space, err := s.spaceRepo.GetByID(ctx, spaceID); err == nil && space != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "reordered", "docs_children", req.Items[0].ID, space.WorkspaceID, "", "docs_space", spaceID, nil)
	}
	return nil
}
