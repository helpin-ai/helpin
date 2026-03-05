package service

import (
	"context"
	"fmt"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

type SearchService struct {
	searchRepo *repository.SearchRepository
}

func NewSearchService(searchRepo *repository.SearchRepository) *SearchService {
	return &SearchService{searchRepo: searchRepo}
}

func (s *SearchService) Search(ctx context.Context, workspaceID, query string) (*model.SearchResponse, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.searchRepo.Search(ctx, workspaceID, query)
}
