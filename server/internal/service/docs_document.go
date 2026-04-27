package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/ordering"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// DocsDocumentService handles business logic for documents.
type DocsDocumentService struct {
	docRepo        *repository.DocsDocumentRepository
	spaceRepo      *repository.DocsSpaceRepository
	deletionDeps   DocsDocumentDeletionDependencies
	translationSvc *DocsHelpcenterTranslationService
	helpcenterSvc  *DocsHelpcenterService
	wsPublisher    *websocket.Publisher
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

// Create creates a new document.
func (s *DocsDocumentService) Create(ctx context.Context, workspaceID string, req model.CreateDocsDocumentRequest, userID string) (*model.DocsDocument, error) {
	if req.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	// Validate space exists and belongs to workspace.
	space, err := s.spaceRepo.GetByID(ctx, req.SpaceID)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, fmt.Errorf("space not found")
	}
	if space.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("space does not belong to this workspace")
	}

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
	}
	return created, err
}

// Get returns a document by ID.
func (s *DocsDocumentService) Get(ctx context.Context, id string) (*model.DocsDocument, error) {
	return s.docRepo.GetByID(ctx, id)
}

// List returns documents with optional filters.
func (s *DocsDocumentService) List(ctx context.Context, workspaceID string, spaceID, collectionID, status, teamID *string, userID, role string, includeArchived bool) ([]model.DocsDocument, error) {
	// Admins/owners can see all drafts; others only see their own.
	isAdminOrOwner := role == "admin" || role == "owner"
	var draftViewerID string
	if !isAdminOrOwner {
		draftViewerID = userID
	}
	return s.docRepo.List(ctx, workspaceID, spaceID, collectionID, status, teamID, draftViewerID, includeArchived)
}

// Update updates a document's metadata.
func (s *DocsDocumentService) Update(ctx context.Context, id string, req model.UpdateDocsDocumentRequest) (*model.DocsDocument, error) {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
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
		updates["owner_id"] = *req.OwnerID
	}
	if req.TemplateKey != nil {
		updates["template_key"] = *req.TemplateKey
	}
	if req.Excerpt != nil {
		updates["excerpt"] = *req.Excerpt
		shouldRefreshTranslations = true
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
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
		return nil, fmt.Errorf("document not found")
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
	updated, err := s.docRepo.GetByID(ctx, id)
	if err == nil && updated != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "updated", "docs_document", updated.ID, updated.WorkspaceID, "", "docs_space", updated.SpaceID, nil)
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
		return nil, fmt.Errorf("document not found")
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
		return nil, fmt.Errorf("document not found")
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
		return nil, fmt.Errorf("document not found")
	}
	if doc.Status != model.DocStatusArchived {
		return nil, fmt.Errorf("document is not archived")
	}
	if err := s.docRepo.UpdateStatus(ctx, id, model.DocStatusDraft); err != nil {
		return nil, err
	}
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
		return nil, fmt.Errorf("document not found")
	}

	if err := checkLocked(doc); err != nil {
		return nil, err
	}

	targetSpace, err := s.spaceRepo.GetByID(ctx, req.SpaceID)
	if err != nil {
		return nil, err
	}
	if targetSpace == nil {
		return nil, fmt.Errorf("target space not found")
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

// Delete permanently deletes a document and its owned, unreferenced imported assets.
func (s *DocsDocumentService) Delete(ctx context.Context, id string) error {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}
	if err := checkLocked(doc); err != nil {
		return err
	}
	if err := s.deleteDocumentPermanently(ctx, doc); err != nil {
		return err
	}
	publishWorkspaceEventWithParent(s.wsPublisher, "deleted", "docs_document", id, doc.WorkspaceID, "", "docs_space", doc.SpaceID, nil)
	return nil
}

// Restore restores a soft-deleted document. Does NOT auto-republish externally.
func (s *DocsDocumentService) Restore(ctx context.Context, id string) (*model.DocsDocument, error) {
	doc, err := s.docRepo.Restore(ctx, id)
	if err == nil && doc != nil {
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
		return nil, fmt.Errorf("document not found")
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
		return nil, fmt.Errorf("document not found")
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
		return fmt.Errorf("document is locked and cannot be modified")
	}
	return nil
}

// ReorderDocuments reorders documents within a bucket (collection or uncategorized).
func (s *DocsDocumentService) ReorderDocuments(ctx context.Context, spaceID string, req model.ReorderDocsDocumentsRequest) error {
	if err := s.docRepo.Reorder(ctx, spaceID, req.CollectionID, req.DocumentIDs); err != nil {
		return err
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

func generateShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
