package service

import (
	"context"
	"encoding/json"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// DocsContentService handles business logic for document content.
type DocsContentService struct {
	contentRepo *repository.DocsContentRepository
}

// NewDocsContentService creates a new DocsContentService.
func NewDocsContentService(contentRepo *repository.DocsContentRepository) *DocsContentService {
	return &DocsContentService{contentRepo: contentRepo}
}

// Get returns the content for a document.
func (s *DocsContentService) Get(ctx context.Context, documentID string) (*model.DocsContent, error) {
	return s.contentRepo.GetByDocumentID(ctx, documentID)
}

// Save creates or updates document content.
// Automatically extracts content_text and computes word_count in the repository layer.
func (s *DocsContentService) Save(ctx context.Context, documentID string, content json.RawMessage) (*model.DocsContent, error) {
	return s.contentRepo.Upsert(ctx, documentID, content)
}
