package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// DocsLinkService handles business logic for document links.
type DocsLinkService struct {
	linkRepo *repository.DocsLinkRepository
}

// NewDocsLinkService creates a new DocsLinkService.
func NewDocsLinkService(linkRepo *repository.DocsLinkRepository) *DocsLinkService {
	return &DocsLinkService{linkRepo: linkRepo}
}

// Create creates a new link between a document and a PM/Support object.
func (s *DocsLinkService) Create(ctx context.Context, workspaceID, documentID string, req model.CreateDocsLinkRequest, userID string) (*model.DocsLink, error) {
	if req.LinkedObjectType == "" || req.LinkedObjectID == "" {
		return nil, fmt.Errorf("linked_object_type and linked_object_id are required")
	}
	if req.LinkContext == "" {
		req.LinkContext = model.LinkContextAttached
	}

	link := &model.DocsLink{
		WorkspaceID:      workspaceID,
		DocumentID:       documentID,
		LinkedObjectType: req.LinkedObjectType,
		LinkedObjectID:   req.LinkedObjectID,
		LinkContext:       req.LinkContext,
		CreatedBy:        userID,
	}
	return s.linkRepo.Create(ctx, link)
}

// ListByDocument returns all links for a document.
func (s *DocsLinkService) ListByDocument(ctx context.Context, documentID string) ([]model.DocsLink, error) {
	return s.linkRepo.ListByDocument(ctx, documentID)
}

// ListByObject returns all document links for a PM/Support object (reverse lookup).
func (s *DocsLinkService) ListByObject(ctx context.Context, workspaceID, objectType, objectID string) ([]model.DocsLink, error) {
	return s.linkRepo.ListByObject(ctx, workspaceID, objectType, objectID)
}

// Delete removes a link.
func (s *DocsLinkService) Delete(ctx context.Context, id string) error {
	link, err := s.linkRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if link == nil {
		return fmt.Errorf("link not found")
	}
	return s.linkRepo.Delete(ctx, id)
}
