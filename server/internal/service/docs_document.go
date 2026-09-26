package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/iconcatalog"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/ordering"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// DocsDocumentService handles business logic for documents.
type DocsDocumentService struct {
	docRepo *repository.DocsDocumentRepository
	productAnalyticsEmitter
	spaceRepo      *repository.DocsSpaceRepository
	deletionDeps   DocsDocumentDeletionDependencies
	translationSvc *DocsHelpcenterTranslationService
	helpcenterSvc  *DocsHelpcenterService
	ruleEngine     *AutomationRuleEngine
	wsPublisher    *websocket.Publisher
	entitlementSvc EntitlementPolicy
	useSortKey     bool
}

// NewDocsDocumentService creates a new DocsDocumentService.
func NewDocsDocumentService(docRepo *repository.DocsDocumentRepository, spaceRepo *repository.DocsSpaceRepository, wsPublisher *websocket.Publisher, useSortKey bool) *DocsDocumentService {
	return &DocsDocumentService{docRepo: docRepo, spaceRepo: spaceRepo, wsPublisher: wsPublisher, useSortKey: useSortKey}
}

func (s *DocsDocumentService) SetTranslationService(translationSvc *DocsHelpcenterTranslationService) {
	s.translationSvc = translationSvc
}

func (s *DocsDocumentService) SetDeletionDependencies(deps DocsDocumentDeletionDependencies) {
	s.deletionDeps = deps
}

// SetHelpcenterService wires the help center service lazily so the
// document move path can emit auto_article_move redirects without a
// circular constructor dependency. Call this once in main.go after both
// services have been constructed.
func (s *DocsDocumentService) SetHelpcenterService(helpcenterSvc *DocsHelpcenterService) {
	s.helpcenterSvc = helpcenterSvc
}

// invalidateHelpcenterCache refreshes cached public Help Center pages after a
// document's status, location or existence changes. Public pages require a
// published document and resolve article links at render time, so other
// articles linking to this one must be re-rendered too.
func (s *DocsDocumentService) invalidateHelpcenterCache(ctx context.Context, workspaceID string) {
	if s.helpcenterSvc != nil {
		s.helpcenterSvc.InvalidateHelpcenterCacheForWorkspace(ctx, workspaceID)
	}
}

func (s *DocsDocumentService) SetRuleEngine(engine *AutomationRuleEngine) {
	s.ruleEngine = engine
}

func (s *DocsDocumentService) SetEntitlementService(entitlementSvc EntitlementPolicy) *DocsDocumentService {
	s.entitlementSvc = entitlementSvc
	return s
}

// Create creates a new document.
func (s *DocsDocumentService) Create(ctx context.Context, workspaceID string, req model.CreateDocsDocumentRequest, userID string) (*model.DocsDocument, error) {
	if req.Title == "" {
		return nil, errCommandInput("title is required")
	}
	if s.entitlementSvc != nil {
		count, err := s.docRepo.CountByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		if err := s.entitlementSvc.RequireLimitUsage(ctx, workspaceID, EntitlementLimitDocuments, count, 1); err != nil {
			return nil, err
		}
	}

	// Validate space exists and belongs to workspace.
	space, err := s.spaceRepo.GetByID(ctx, req.SpaceID)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, errCommandNotFound("space")
	}
	if space.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("space does not belong to this workspace")
	}
	normalizedIcon, err := iconcatalog.NormalizeNew(req.Icon)
	if err != nil {
		return nil, fmt.Errorf("icon: %w", err)
	}
	req.Icon = normalizedIcon

	// Sanitize optional UUID fields: treat empty strings as nil.
	collectionID := req.CollectionID
	if collectionID != nil && *collectionID == "" {
		collectionID = nil
	}
	teamID := space.TeamID
	if teamID != nil && *teamID == "" {
		teamID = nil
	}

	// Get next position in the target bucket.
	nextPos, err := s.docRepo.NextPosition(ctx, req.SpaceID, collectionID)
	if err != nil {
		slog.ErrorContext(ctx, "next document position failed", "error", err)
		nextPos = 0
	}

	doc := &model.DocsDocument{
		WorkspaceID:  workspaceID,
		SpaceID:      req.SpaceID,
		CollectionID: collectionID,
		Title:        req.Title,
		Status:       model.DocStatusDraft,
		Visibility:   model.SpaceVisibilityWorkspaceWide,
		OwnerID:      req.OwnerID,
		TeamID:       teamID,
		TemplateKey:  req.TemplateKey,
		Icon:         req.Icon,
		Tags:         model.DocsStringArray(req.Tags),
		Position:     nextPos,
		CreatedBy:    userID,
	}

	if s.useSortKey {
		lastKey, err := s.docRepo.LastSortKeyInBucket(ctx, req.SpaceID, collectionID)
		if err != nil {
			slog.ErrorContext(ctx, "last doc sort key failed", "error", err)
		}
		if key, err := ordering.Between(lastKey, ""); err == nil {
			doc.SortKey = key
		}
	}
	created, err := s.docRepo.Create(ctx, doc)
	if err == nil && created != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "created", "docs_document", created.ID, workspaceID, userID, "docs_space", created.SpaceID, nil)
		s.trackProductEvent(ctx, ProductAnalyticsEvent{
			SemanticKey: "document_created:" + created.ID, UserID: userID,
			WorkspaceID: workspaceID, Name: "document_created", Source: "api",
			OccurredAt: created.CreatedAt,
			Attributes: map[string]any{"entity_id": created.ID, "space_id": created.SpaceID, "module": "docs"},
		})
	}
	return created, err
}

// Get returns a document by ID.
func (s *DocsDocumentService) Get(ctx context.Context, id string) (*model.DocsDocument, error) {
	return s.docRepo.GetByID(ctx, id)
}

// List returns documents with optional filters.
func (s *DocsDocumentService) List(ctx context.Context, workspaceID string, spaceID, collectionID, status, teamID *string, userID, role string, includeArchived bool) ([]model.DocsDocument, error) {
	return s.list(ctx, workspaceID, spaceID, collectionID, status, teamID, nil, userID, role, includeArchived)
}

// ListWithOwner returns documents with optional filters, including owner.
func (s *DocsDocumentService) ListWithOwner(ctx context.Context, workspaceID string, spaceID, collectionID, status, teamID, ownerID *string, userID, role string, includeArchived bool) ([]model.DocsDocument, error) {
	return s.list(ctx, workspaceID, spaceID, collectionID, status, teamID, ownerID, userID, role, includeArchived)
}

func (s *DocsDocumentService) list(ctx context.Context, workspaceID string, spaceID, collectionID, status, teamID, ownerID *string, userID, role string, includeArchived bool) ([]model.DocsDocument, error) {
	// Admins/owners can see all drafts; others only see their own.
	isAdminOrOwner := role == "admin" || role == "owner"
	var draftViewerID string
	if !isAdminOrOwner {
		draftViewerID = userID
	}
	return s.docRepo.ListWithOwner(ctx, workspaceID, spaceID, collectionID, status, teamID, ownerID, draftViewerID, includeArchived)
}

// Update updates a document's metadata.
func (s *DocsDocumentService) Update(ctx context.Context, id string, req model.UpdateDocsDocumentRequest) (*model.DocsDocument, error) {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errCommandNotFound("document")
	}
	if err := checkLocked(doc); err != nil {
		return nil, err
	}

	// Handle collection_id change as a move operation to preserve ordering.
	shouldRefreshTranslations := false
	if req.CollectionID != nil {
		newCollID := req.CollectionID
		if *newCollID == "" {
			newCollID = nil
		}
		oldCollID := doc.CollectionID

		// Only move if collection actually changed.
		collChanged := (oldCollID == nil && newCollID != nil) ||
			(oldCollID != nil && newCollID == nil) ||
			(oldCollID != nil && newCollID != nil && *oldCollID != *newCollID)

		if collChanged {
			if err := s.docRepo.Move(ctx, id, doc.SpaceID, newCollID); err != nil {
				return nil, fmt.Errorf("move document to collection: %w", err)
			}
			shouldRefreshTranslations = true
		}
	}

	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
		shouldRefreshTranslations = true
	}
	// collection_id is handled above via move semantics, skip raw patch.
	if req.OwnerID != nil {
		if *req.OwnerID == "" {
			updates["owner_id"] = nil
		} else {
			updates["owner_id"] = *req.OwnerID
		}
	}
	if req.TemplateKey != nil {
		updates["template_key"] = *req.TemplateKey
	}
	if req.ClearExcerpt {
		updates["excerpt"] = nil
		shouldRefreshTranslations = true
	} else if req.Excerpt != nil {
		updates["excerpt"] = *req.Excerpt
		shouldRefreshTranslations = true
	}
	if normalizedIcon, changed, err := iconcatalog.NormalizeUpdate(req.Icon, doc.Icon); err != nil {
		return nil, fmt.Errorf("icon: %w", err)
	} else if changed {
		if normalizedIcon == nil {
			updates["icon"] = nil
		} else {
			updates["icon"] = *normalizedIcon
		}
	}
	if req.Tags != nil {
		updates["tags"] = model.DocsStringArray(req.Tags)
	}
	if req.IsPinned != nil {
		updates["is_pinned"] = *req.IsPinned
	}
	if len(updates) == 0 {
		updated, err := s.docRepo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if shouldRefreshTranslations && s.translationSvc != nil {
			if err := s.translationSvc.RefreshArticleSource(ctx, id); err != nil {
				slog.WarnContext(ctx, "failed to refresh helpcenter article translation source after document move", "document_id", id, "error", err)
			}
		}
		publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_document", updated.ID, updated.WorkspaceID, "", "docs_space", updated.SpaceID, nil)
		return updated, nil
	}
	updated, err := s.docRepo.Update(ctx, id, updates)
	if err != nil {
		return nil, err
	}
	if shouldRefreshTranslations && s.translationSvc != nil {
		if err := s.translationSvc.RefreshArticleSource(ctx, id); err != nil {
			slog.WarnContext(ctx, "failed to refresh helpcenter article translation source after document update", "document_id", id, "error", err)
		}
	}
	publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_document", updated.ID, updated.WorkspaceID, "", "docs_space", updated.SpaceID, nil)
	return updated, nil
}

// Publish transitions a document to published status.
func (s *DocsDocumentService) Publish(ctx context.Context, id string) (*model.DocsDocument, error) {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errCommandNotFound("document")
	}
	if err := checkLocked(doc); err != nil {
		return nil, err
	}
	if doc.Status == model.DocStatusPublished {
		return doc, nil
	}
	if err := s.docRepo.UpdateStatus(ctx, id, model.DocStatusPublished); err != nil {
		return nil, err
	}
	s.invalidateHelpcenterCache(ctx, doc.WorkspaceID)
	updated, err := s.docRepo.GetByID(ctx, id)
	if err == nil && updated != nil {
		s.trackProductEvent(ctx, ProductAnalyticsEvent{
			SemanticKey: "document_published:" + updated.ID,
			WorkspaceID: updated.WorkspaceID, Name: "document_published", Source: "api",
			Attributes: map[string]any{"entity_id": updated.ID, "space_id": updated.SpaceID, "module": "docs"},
		})
		publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_document", updated.ID, updated.WorkspaceID, "", "docs_space", updated.SpaceID, nil)
		if s.ruleEngine != nil {
			event := model.AutomationEvent{
				WorkspaceID: updated.WorkspaceID,
				TriggerType: model.TriggerDocPublished,
				TargetType:  "document",
				TargetID:    updated.ID,
				PublishedAt: updated.PublishedAt,
			}
			if updated.TeamID != nil {
				event.TeamID = *updated.TeamID
			}
			s.ruleEngine.EvaluateEvent(ctx, event, nil)
		}
	}
	return updated, err
}

// Unpublish transitions a published document back to draft status.
func (s *DocsDocumentService) Unpublish(ctx context.Context, id string) (*model.DocsDocument, error) {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errCommandNotFound("document")
	}
	if err := checkLocked(doc); err != nil {
		return nil, err
	}
	if doc.Status != model.DocStatusPublished {
		return nil, fmt.Errorf("document is not published")
	}
	if err := s.docRepo.UpdateStatus(ctx, id, model.DocStatusDraft); err != nil {
		return nil, err
	}
	s.invalidateHelpcenterCache(ctx, doc.WorkspaceID)
	updated, err := s.docRepo.GetByID(ctx, id)
	if err == nil && updated != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_document", updated.ID, updated.WorkspaceID, "", "docs_space", updated.SpaceID, nil)
	}
	return updated, err
}

// Archive transitions a document to archived status.
func (s *DocsDocumentService) Archive(ctx context.Context, id string) (*model.DocsDocument, error) {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errCommandNotFound("document")
	}
	if err := checkLocked(doc); err != nil {
		return nil, err
	}
	slog.Info("[DEBUG] Archive called",
		"doc_id", id,
		"current_status", doc.Status,
		"workspace_id", doc.WorkspaceID,
	)
	if doc.Status == model.DocStatusArchived {
		return doc, nil
	}
	if err := s.docRepo.UpdateStatus(ctx, id, model.DocStatusArchived); err != nil {
		slog.Error("[DEBUG] Archive UpdateStatus failed", "doc_id", id, "error", err)
		return nil, err
	}
	s.invalidateHelpcenterCache(ctx, doc.WorkspaceID)
	updated, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	slog.Info("[DEBUG] Archive completed",
		"doc_id", id,
		"new_status", updated.Status,
	)
	publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_document", updated.ID, updated.WorkspaceID, "", "docs_space", updated.SpaceID, nil)
	return updated, nil
}

// Unarchive transitions a document from archived back to draft status.
func (s *DocsDocumentService) Unarchive(ctx context.Context, id string) (*model.DocsDocument, error) {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errCommandNotFound("document")
	}
	if doc.Status != model.DocStatusArchived {
		return nil, fmt.Errorf("document is not archived")
	}
	if err := s.docRepo.UpdateStatus(ctx, id, model.DocStatusDraft); err != nil {
		return nil, err
	}
	s.invalidateHelpcenterCache(ctx, doc.WorkspaceID)
	updated, err := s.docRepo.GetByID(ctx, id)
	if err == nil && updated != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_document", updated.ID, updated.WorkspaceID, "", "docs_space", updated.SpaceID, nil)
	}
	return updated, err
}

// Move moves a document to a different space and/or collection.
// Enforces: help_center_article moved to non-external space loses external publication.
// When the owning collection changes and the document is a published
// help center article, emits an auto_article_move redirect via the help
// center service so old public URLs keep resolving.
func (s *DocsDocumentService) Move(ctx context.Context, id string, req model.MoveDocsDocumentRequest) (*model.DocsDocument, error) {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errCommandNotFound("document")
	}

	if err := checkLocked(doc); err != nil {
		return nil, err
	}

	targetSpace, err := s.spaceRepo.GetByID(ctx, req.SpaceID)
	if err != nil {
		return nil, err
	}
	if targetSpace == nil {
		return nil, errCommandNotFound("target space")
	}
	if targetSpace.WorkspaceID != doc.WorkspaceID {
		return nil, fmt.Errorf("target space does not belong to this workspace")
	}

	// Snapshot the old collection before the move so we can emit an
	// accurate redirect. doc.CollectionID is a *string so we copy the
	// underlying value into a new pointer to avoid aliasing once the
	// row is mutated below.
	var oldCollectionID *string
	if doc.CollectionID != nil {
		copied := *doc.CollectionID
		oldCollectionID = &copied
	}

	if err := s.docRepo.Move(ctx, id, req.SpaceID, req.CollectionID); err != nil {
		return nil, err
	}
	s.invalidateHelpcenterCache(ctx, doc.WorkspaceID)
	updated, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if updated != nil {
		if s.helpcenterSvc != nil {
			if redirectErr := s.helpcenterSvc.EmitArticleMoveRedirect(ctx, updated, oldCollectionID); redirectErr != nil {
				slog.WarnContext(ctx, "emit article move redirect failed", "document_id", id, "error", redirectErr)
			}
		}
		publishWorkspaceEventWithParent(s.wsPublisher, "moved", "docs_document", updated.ID, updated.WorkspaceID, "", "docs_space", updated.SpaceID, map[string]any{
			"collection_id": updated.CollectionID,
		})
	}
	return updated, nil
}

// Delete permanently deletes a document and enqueues cleanup for owned imported assets.
func (s *DocsDocumentService) Delete(ctx context.Context, id string) error {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if doc == nil {
		return errCommandNotFound("document")
	}
	if err := checkLocked(doc); err != nil {
		return err
	}
	candidateKeys, err := s.deleteDocumentPermanently(ctx, doc)
	if err != nil {
		return err
	}
	s.invalidateHelpcenterCache(ctx, doc.WorkspaceID)
	publishWorkspaceEventWithParent(s.wsPublisher, "deleted", "docs_document", id, doc.WorkspaceID, "", "docs_space", doc.SpaceID, nil)
	s.enqueueAssetCleanupBestEffort(ctx, doc.WorkspaceID, doc.ID, candidateKeys)
	return nil
}

// Restore restores a soft-deleted document. Does NOT auto-republish externally.
func (s *DocsDocumentService) Restore(ctx context.Context, id string) (*model.DocsDocument, error) {
	doc, err := s.docRepo.Restore(ctx, id)
	if err == nil && doc != nil {
		s.invalidateHelpcenterCache(ctx, doc.WorkspaceID)
		publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_document", doc.ID, doc.WorkspaceID, "", "docs_space", doc.SpaceID, nil)
	}
	return doc, err
}

// ToggleShare enables or disables public sharing for a document.
// When enabling, a share token is generated if not already present.
func (s *DocsDocumentService) ToggleShare(ctx context.Context, id string, enable bool) (*model.DocsDocument, error) {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errCommandNotFound("document")
	}

	updates := map[string]interface{}{
		"is_publicly_shared": enable,
	}

	if enable && doc.ShareToken == nil {
		token, err := generateShareToken()
		if err != nil {
			return nil, fmt.Errorf("generate share token: %w", err)
		}
		updates["share_token"] = token
	}

	updated, err := s.docRepo.Update(ctx, id, updates)
	if err == nil && updated != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_document", updated.ID, updated.WorkspaceID, "", "docs_space", updated.SpaceID, nil)
	}
	return updated, err
}

// GetByShareToken returns a publicly shared document by its token.
func (s *DocsDocumentService) GetByShareToken(ctx context.Context, token string) (*model.DocsDocument, error) {
	return s.docRepo.GetByShareToken(ctx, token)
}

// ToggleLock locks or unlocks a document.
// When locking, records who locked it. When unlocking, only the locker or admin/owner can unlock.
func (s *DocsDocumentService) ToggleLock(ctx context.Context, id string, lock bool, userID, userRole string) (*model.DocsDocument, error) {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errCommandNotFound("document")
	}

	if !lock && doc.IsLocked {
		// Only the locker or admin/owner can unlock
		isAdminOrOwner := userRole == "admin" || userRole == "owner"
		if doc.LockedBy != nil && *doc.LockedBy != userID && !isAdminOrOwner {
			return nil, fmt.Errorf("only the person who locked this document or an admin can unlock it")
		}
	}

	updates := map[string]interface{}{
		"is_locked": lock,
	}
	if lock {
		updates["locked_by"] = userID
	} else {
		updates["locked_by"] = nil
	}

	updated, err := s.docRepo.Update(ctx, id, updates)
	if err == nil && updated != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_document", updated.ID, updated.WorkspaceID, userID, "docs_space", updated.SpaceID, nil)
	}
	return updated, err
}

// checkLocked returns an error if the document is locked, preventing mutation.
func checkLocked(doc *model.DocsDocument) error {
	if doc.IsLocked {
		return ErrDocsDocumentLocked
	}
	return nil
}

// ReorderDocuments reorders documents within a bucket (collection or uncategorized).
func (s *DocsDocumentService) ReorderDocuments(ctx context.Context, spaceID string, req model.ReorderDocsDocumentsRequest) error {
	if err := s.docRepo.Reorder(ctx, spaceID, req.CollectionID, req.DocumentIDs); err != nil {
		return err
	}

	// When the sort_key flag is on, rebuild sort_keys from scratch for
	// the submitted list. Sequential Between(prev, "") calls produce
	// strictly increasing keys matching the client's visual order.
	if s.useSortKey && len(req.DocumentIDs) > 0 {
		prevKey := ""
		for _, id := range req.DocumentIDs {
			key, err := ordering.Between(prevKey, "")
			if err != nil {
				return fmt.Errorf("compute sort key for doc reorder: %w", err)
			}
			if err := s.docRepo.UpdateSortKey(ctx, id, key); err != nil {
				return fmt.Errorf("update sort key for doc %s: %w", id, err)
			}
			prevKey = key
		}
	}

	if len(req.DocumentIDs) == 0 {
		return nil
	}
	if space, err := s.spaceRepo.GetByID(ctx, spaceID); err == nil && space != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "reordered", "docs_document", req.DocumentIDs[0], space.WorkspaceID, "", "docs_space", spaceID, map[string]any{
			"collection_id": req.CollectionID,
		})
	}
	return nil
}

// Sentinel errors for the move endpoint.
var (
	ErrCrossSpaceMove = fmt.Errorf("cross-space moves not supported")
	ErrStaleNeighbors = fmt.Errorf("neighbor sort keys have changed; retry")
	ErrBetweenFailed  = fmt.Errorf("cannot compute sort key between the given neighbors")
)

// MoveItem moves a single doc or collection to a specific position
// within a bucket, computing the fractional sort_key from the
// before/after neighbors. Requires useSortKey to be on.
func (s *DocsDocumentService) MoveItem(ctx context.Context, wsID string, req model.MoveDocsItemRequest) error {
	if !s.useSortKey {
		return fmt.Errorf("sort_key ordering is not enabled")
	}
	if req.Item.Type != "doc" && req.Item.Type != "collection" {
		return fmt.Errorf("invalid item type: %s", req.Item.Type)
	}
	if req.Item.ID == "" {
		return errCommandInput("item id is required")
	}

	// Resolve the before/after sort_keys.
	beforeKey, afterKey := "", ""

	if req.Position.After != nil {
		key, err := s.loadSortKey(ctx, req.Position.After.Type, req.Position.After.ID)
		if err != nil {
			return ErrStaleNeighbors
		}
		afterKey = key
	}
	if req.Position.Before != nil {
		key, err := s.loadSortKey(ctx, req.Position.Before.Type, req.Position.Before.ID)
		if err != nil {
			return ErrStaleNeighbors
		}
		beforeKey = key
	}

	// If both nil, append to end of bucket.
	if afterKey == "" && beforeKey == "" {
		lastKey := maxSortKeyInBucketFromRepos(ctx, s.docRepo, req.TargetBucket.SpaceID, req.TargetBucket.ParentCollectionID)
		afterKey = lastKey
		beforeKey = ""
	}

	newKey, err := ordering.Between(afterKey, beforeKey)
	if err != nil {
		return ErrBetweenFailed
	}

	// Update the item's sort_key (and parent if bucket changed).
	switch req.Item.Type {
	case "doc":
		updates := map[string]interface{}{
			"sort_key": newKey,
		}
		// If moving to a different collection, update collection_id too.
		updates["collection_id"] = req.TargetBucket.ParentCollectionID
		return s.docRepo.UpdateFields(ctx, req.Item.ID, updates)
	case "collection":
		updates := map[string]interface{}{
			"sort_key": newKey,
		}
		updates["parent_collection_id"] = req.TargetBucket.ParentCollectionID
		return s.docRepo.DB().WithContext(ctx).
			Model(&model.DocsCollection{}).
			Where("id = ?", req.Item.ID).
			Updates(updates).Error
	}
	return nil
}

// loadSortKey fetches the current sort_key for an item.
func (s *DocsDocumentService) loadSortKey(ctx context.Context, itemType, itemID string) (string, error) {
	var key string
	var err error
	switch itemType {
	case "doc":
		err = s.docRepo.DB().WithContext(ctx).
			Model(&model.DocsDocument{}).
			Select("sort_key").
			Where("id = ?", itemID).
			Row().Scan(&key)
	case "collection":
		err = s.docRepo.DB().WithContext(ctx).
			Model(&model.DocsCollection{}).
			Select("sort_key").
			Where("id = ?", itemID).
			Row().Scan(&key)
	default:
		return "", fmt.Errorf("unknown type: %s", itemType)
	}
	return key, err
}

// maxSortKeyInBucketFromRepos queries both tables for the max sort_key.
func maxSortKeyInBucketFromRepos(ctx context.Context, docRepo *repository.DocsDocumentRepository, spaceID string, parentID *string) string {
	docKey, _ := docRepo.LastSortKeyInBucket(ctx, spaceID, parentID)
	// Also check collections via the doc repo's DB handle.
	var collKey string
	q := docRepo.DB().WithContext(ctx).
		Model(&model.DocsCollection{}).
		Select("COALESCE(MAX(sort_key), '')").
		Where("space_id = ? AND deleted_at IS NULL", spaceID)
	if parentID != nil {
		q = q.Where("parent_collection_id = ?", *parentID)
	} else {
		q = q.Where("parent_collection_id IS NULL")
	}
	_ = q.Row().Scan(&collKey)
	if collKey == "~" {
		collKey = ""
	}
	if collKey > docKey {
		return collKey
	}
	return docKey
}

func generateShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
