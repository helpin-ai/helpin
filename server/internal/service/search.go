package service

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var searchTaskKeyPattern = regexp.MustCompile(`^([A-Z]{2,5})-(\d+)$`)

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

	resolvedWorkspaceID := workspaceID
	resolvedDisplayID := 0

	if m := searchTaskKeyPattern.FindStringSubmatch(strings.ToUpper(query)); m != nil {
		keyPrefix := m[1]
		displayID, _ := strconv.Atoi(m[2])

		ws, err := s.workspaceRepo.GetByID(ctx, workspaceID)
		if err == nil && ws != nil {
			if strings.EqualFold(keyPrefix, ws.WorkspaceKey) {
				resolvedDisplayID = displayID
			} else {
				resolved, err := s.workspaceRepo.FindWorkspaceByKeyOrAlias(ctx, keyPrefix)
				if err == nil && resolved != nil && resolved.ID == workspaceID {
					resolvedDisplayID = displayID
				}
			}
		}

		if resolvedDisplayID > 0 {
			resolvedWorkspaceID = workspaceID
		}
	}

	resp, err := s.searchRepo.Search(ctx, resolvedWorkspaceID, query, resolvedDisplayID)
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
