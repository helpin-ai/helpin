package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

// DocsSearchService handles document search.
type DocsSearchService struct {
	searchRepo *repository.DocsSearchRepository
}

// NewDocsSearchService creates a new DocsSearchService.
func NewDocsSearchService(searchRepo *repository.DocsSearchRepository) *DocsSearchService {
	return &DocsSearchService{searchRepo: searchRepo}
}

// Search performs a workspace-scoped full-text search.
// spaceIDs provides permission-aware filtering — only spaces the user can access.
func (s *DocsSearchService) Search(ctx context.Context, workspaceID, query string, spaceIDs []string, docType, status *string, limit int) ([]repository.DocsSearchResult, error) {
	return s.searchRepo.Search(ctx, workspaceID, query, spaceIDs, docType, status, limit)
}

// PublicSearch searches published help center articles, optionally filtered by space.
func (s *DocsSearchService) PublicSearch(ctx context.Context, workspaceID, query, spaceID string, limit int) ([]repository.DocsSearchResult, error) {
	return s.searchRepo.PublicSearch(ctx, workspaceID, query, spaceID, limit)
}
