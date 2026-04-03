package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type SearchService struct {
	searchRepo    *repository.SearchRepository
	workspaceRepo *repository.WorkspaceRepository
}

func NewSearchService(searchRepo *repository.SearchRepository, workspaceRepo *repository.WorkspaceRepository) *SearchService {
	return &SearchService{searchRepo: searchRepo, workspaceRepo: workspaceRepo}
}

func (s *SearchService) Search(ctx context.Context, workspaceID, query string) (*model.SearchResponse, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	resp, err := s.searchRepo.Search(ctx, workspaceID, query)
	if err != nil {
		return nil, err
	}
	s.populateTaskKeys(ctx, workspaceID, resp)
	return resp, nil
}

// populateTaskKeys sets TaskKey on task search results.
func (s *SearchService) populateTaskKeys(ctx context.Context, workspaceID string, resp *model.SearchResponse) {
	if resp == nil || len(resp.Tasks) == 0 {
		return
	}
	ws, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil || ws == nil {
		return
	}
	for i := range resp.Tasks {
		if resp.Tasks[i].DisplayID > 0 {
			resp.Tasks[i].TaskKey = model.FormatTaskKey(ws.WorkspaceKey, resp.Tasks[i].DisplayID)
		}
	}
}
