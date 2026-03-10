package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// DocsDocumentService handles business logic for documents.
type DocsDocumentService struct {
	docRepo   *repository.DocsDocumentRepository
	spaceRepo *repository.DocsSpaceRepository
}

// NewDocsDocumentService creates a new DocsDocumentService.
func NewDocsDocumentService(docRepo *repository.DocsDocumentRepository, spaceRepo *repository.DocsSpaceRepository) *DocsDocumentService {
	return &DocsDocumentService{docRepo: docRepo, spaceRepo: spaceRepo}
}

// Create creates a new document.
func (s *DocsDocumentService) Create(ctx context.Context, workspaceID string, req model.CreateDocsDocumentRequest, userID string) (*model.DocsDocument, error) {
	if req.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if !model.IsValidDocType(req.DocType) {
		return nil, fmt.Errorf("invalid doc_type: %s", req.DocType)
	}
	if req.TemplateKey != nil && !model.IsValidTemplateKey(*req.TemplateKey) {
		return nil, fmt.Errorf("invalid template_key: %s", *req.TemplateKey)
	}

	// Validate owner requirement.
	if model.OwnerRequired(req.DocType) && req.OwnerID == nil {
		return nil, fmt.Errorf("owner_id is required for doc_type %s", req.DocType)
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

	// Only help_center_article allowed in external_capable spaces (other types also allowed).
	// Internal spaces cannot hold help_center_article docs.
	if model.IsExternalCapableDocType(req.DocType) && space.Type != model.SpaceTypeExternalCapable {
		return nil, fmt.Errorf("doc_type %s requires an external-capable space", req.DocType)
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

	doc := &model.DocsDocument{
		WorkspaceID:  workspaceID,
		SpaceID:      req.SpaceID,
		CollectionID: collectionID,
		Title:        req.Title,
		DocType:      req.DocType,
		Status:       model.DocStatusDraft,
		Visibility:   model.SpaceVisibilityWorkspaceWide,
		OwnerID:      req.OwnerID,
		TeamID:       teamID,
		TemplateKey:  req.TemplateKey,
		Icon:         req.Icon,
		Tags:         model.DocsStringArray(req.Tags),
		CreatedBy:    userID,
	}
	return s.docRepo.Create(ctx, doc)
}

// Get returns a document by ID.
func (s *DocsDocumentService) Get(ctx context.Context, id string) (*model.DocsDocument, error) {
	return s.docRepo.GetByID(ctx, id)
}

// List returns documents with optional filters.
func (s *DocsDocumentService) List(ctx context.Context, workspaceID string, spaceID, collectionID, docType, status, teamID *string, userID, role string, includeArchived bool) ([]model.DocsDocument, error) {
	// Admins/owners can see all drafts; others only see their own.
	isAdminOrOwner := role == "admin" || role == "owner"
	var draftViewerID string
	if !isAdminOrOwner {
		draftViewerID = userID
	}
	return s.docRepo.List(ctx, workspaceID, spaceID, collectionID, docType, status, teamID, draftViewerID, includeArchived)
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

	if req.TemplateKey != nil && !model.IsValidTemplateKey(*req.TemplateKey) {
		return nil, fmt.Errorf("invalid template_key: %s", *req.TemplateKey)
	}

	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.CollectionID != nil {
		updates["collection_id"] = *req.CollectionID
	}
	if req.OwnerID != nil {
		updates["owner_id"] = *req.OwnerID
	}
	if req.TemplateKey != nil {
		updates["template_key"] = *req.TemplateKey
	}
	if req.Excerpt != nil {
		updates["excerpt"] = *req.Excerpt
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
		return doc, nil
	}
	return s.docRepo.Update(ctx, id, updates)
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
	if doc.Status == model.DocStatusPublished {
		return doc, nil
	}
	if err := s.docRepo.UpdateStatus(ctx, id, model.DocStatusPublished); err != nil {
		return nil, err
	}
	return s.docRepo.GetByID(ctx, id)
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
	if doc.Status != model.DocStatusPublished {
		return nil, fmt.Errorf("document is not published")
	}
	if err := s.docRepo.UpdateStatus(ctx, id, model.DocStatusDraft); err != nil {
		return nil, err
	}
	return s.docRepo.GetByID(ctx, id)
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
	return s.docRepo.GetByID(ctx, id)
}

// Move moves a document to a different space and/or collection.
// Enforces: help_center_article moved to non-external space loses external publication.
func (s *DocsDocumentService) Move(ctx context.Context, id string, req model.MoveDocsDocumentRequest) (*model.DocsDocument, error) {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
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

	// help_center_article can only live in external_capable spaces.
	if model.IsExternalCapableDocType(doc.DocType) && targetSpace.Type != model.SpaceTypeExternalCapable {
		// Move is allowed, but we flag that external publication will be disabled.
		// The caller (handler/helpcenter service) should unpublish externally.
	}

	if err := s.docRepo.Move(ctx, id, req.SpaceID, req.CollectionID); err != nil {
		return nil, err
	}
	return s.docRepo.GetByID(ctx, id)
}

// Delete soft-deletes a document.
func (s *DocsDocumentService) Delete(ctx context.Context, id string) error {
	doc, err := s.docRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}
	return s.docRepo.Delete(ctx, id)
}

// Restore restores a soft-deleted document. Does NOT auto-republish externally.
func (s *DocsDocumentService) Restore(ctx context.Context, id string) (*model.DocsDocument, error) {
	return s.docRepo.Restore(ctx, id)
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

	return s.docRepo.Update(ctx, id, updates)
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

	return s.docRepo.Update(ctx, id, updates)
}

func generateShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
