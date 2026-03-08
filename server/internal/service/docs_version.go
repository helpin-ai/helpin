package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// DocsVersionService handles business logic for document versioning.
type DocsVersionService struct {
	versionRepo *repository.DocsVersionRepository
	contentRepo *repository.DocsContentRepository
}

// NewDocsVersionService creates a new DocsVersionService.
func NewDocsVersionService(versionRepo *repository.DocsVersionRepository, contentRepo *repository.DocsContentRepository) *DocsVersionService {
	return &DocsVersionService{versionRepo: versionRepo, contentRepo: contentRepo}
}

// List returns all versions for a document.
func (s *DocsVersionService) List(ctx context.Context, documentID string) ([]model.DocsVersion, error) {
	return s.versionRepo.ListByDocument(ctx, documentID)
}

// CreateSnapshot creates a manual version snapshot from current content.
func (s *DocsVersionService) CreateSnapshot(ctx context.Context, documentID, userID string, label *string) (*model.DocsVersion, error) {
	content, err := s.contentRepo.GetByDocumentID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, fmt.Errorf("no content to snapshot")
	}
	return s.versionRepo.Create(ctx, documentID, userID, content.Content, content.ContentText, label)
}

// SnapshotOnPublish creates a version snapshot labeled "Published".
func (s *DocsVersionService) SnapshotOnPublish(ctx context.Context, documentID, userID string) (*model.DocsVersion, error) {
	label := "Published"
	return s.CreateSnapshot(ctx, documentID, userID, &label)
}

// Revert replaces current content with a previous version's content and creates a "Reverted" snapshot.
func (s *DocsVersionService) Revert(ctx context.Context, documentID, versionID, userID string) (*model.DocsContent, error) {
	version, err := s.versionRepo.GetByID(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if version == nil {
		return nil, fmt.Errorf("version not found")
	}
	if version.DocumentID != documentID {
		return nil, fmt.Errorf("version does not belong to this document")
	}

	// Snapshot current state before reverting.
	label := "Before revert"
	_, _ = s.CreateSnapshot(ctx, documentID, userID, &label)

	// Overwrite current content with the version's content.
	updated, err := s.contentRepo.Upsert(ctx, documentID, version.Content)
	if err != nil {
		return nil, err
	}

	// Create a "Reverted" snapshot.
	revertLabel := "Reverted"
	_, _ = s.versionRepo.Create(ctx, documentID, userID, version.Content, version.ContentText, &revertLabel)

	return updated, nil
}
