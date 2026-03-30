package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
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
func (s *DocsSearchService) Search(ctx context.Context, workspaceID, query string, spaceIDs []string, status *string, limit int) ([]repository.DocsSearchResult, error) {
	return s.searchRepo.Search(ctx, workspaceID, query, spaceIDs, status, limit)
}

// PublicSearch searches published help center article translations for a single locale.
func (s *DocsSearchService) PublicSearch(ctx context.Context, workspaceID, locale, query, spaceSlug string, limit int) ([]model.PublicSearchResultResponse, error) {
	results, err := s.searchRepo.PublicSearch(ctx, workspaceID, locale, query, spaceSlug, limit)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].RequestedLocale = locale
	}
	return results, nil
}
