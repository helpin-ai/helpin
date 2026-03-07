package repository

import (
	"context"
	"fmt"
	"sync"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type SearchRepository struct {
	db *gorm.DB
}

func NewSearchRepository(db *gorm.DB) *SearchRepository {
	return &SearchRepository{db: db}
}

const searchLimit = 20

func (r *SearchRepository) Search(ctx context.Context, workspaceID, query string) (*model.SearchResponse, error) {
	if query == "" {
		return &model.SearchResponse{
			Stories:    []model.SearchResult{},
			Epics:      []model.SearchResult{},
			Sprints:    []model.SearchResult{},
			Objectives: []model.SearchResult{},
			Members:    []model.SearchResult{},
		}, nil
	}

	pattern := "%" + query + "%"

	var (
		stories    []model.SearchResult
		epics      []model.SearchResult
		sprints    []model.SearchResult
		objectives []model.SearchResult
		members    []model.SearchResult
		wg         sync.WaitGroup
		mu         sync.Mutex
		firstErr   error
	)

	setErr := func(err error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
	}

	wg.Add(5)

	go func() {
		defer wg.Done()
		if err := r.db.WithContext(ctx).
			Raw(`SELECT id, name, 'story' AS type, display_id, team_id
				FROM pm_stories
				WHERE workspace_id = ? AND archived = false
				  AND (name ILIKE ? OR CAST(display_id AS TEXT) ILIKE ?)
				ORDER BY updated_at DESC
				LIMIT ?`, workspaceID, pattern, pattern, searchLimit).
			Scan(&stories).Error; err != nil {
			setErr(fmt.Errorf("search stories: %w", err))
		}
	}()

	go func() {
		defer wg.Done()
		if err := r.db.WithContext(ctx).
			Raw(`SELECT id, name, 'epic' AS type
				FROM pm_epics
				WHERE workspace_id = ? AND archived = false
				  AND name ILIKE ?
				ORDER BY updated_at DESC
				LIMIT ?`, workspaceID, pattern, searchLimit).
			Scan(&epics).Error; err != nil {
			setErr(fmt.Errorf("search epics: %w", err))
		}
	}()

	go func() {
		defer wg.Done()
		if err := r.db.WithContext(ctx).
			Raw(`SELECT id, name, 'sprint' AS type
				FROM pm_sprints
				WHERE workspace_id = ? AND archived = false
				  AND name ILIKE ?
				ORDER BY updated_at DESC
				LIMIT ?`, workspaceID, pattern, searchLimit).
			Scan(&sprints).Error; err != nil {
			setErr(fmt.Errorf("search sprints: %w", err))
		}
	}()

	go func() {
		defer wg.Done()
		if err := r.db.WithContext(ctx).
			Raw(`SELECT id, name, 'objective' AS type
				FROM pm_objectives
				WHERE workspace_id = ? AND archived = false
				  AND name ILIKE ?
				ORDER BY updated_at DESC
				LIMIT ?`, workspaceID, pattern, searchLimit).
			Scan(&objectives).Error; err != nil {
			setErr(fmt.Errorf("search objectives: %w", err))
		}
	}()

	go func() {
		defer wg.Done()
		if err := r.db.WithContext(ctx).
			Raw(`SELECT wm.id, wm.display_name AS name, 'member' AS type,
					COALESCE(twm.team_id::text, '') AS team_id,
					COALESCE(wt.name, '') AS team_name
				FROM workspace_members wm
				LEFT JOIN team_workspace_memberships twm ON twm.workspace_member_id = wm.id
				LEFT JOIN workspace_teams wt ON wt.id = twm.team_id
				WHERE wm.workspace_id = ? AND wm.status IN ('active', 'pending')
				  AND (wm.display_name ILIKE ? OR wm.email ILIKE ?)
				ORDER BY wm.display_name ASC
				LIMIT ?`, workspaceID, pattern, pattern, searchLimit).
			Scan(&members).Error; err != nil {
			setErr(fmt.Errorf("search members: %w", err))
		}
	}()

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	if stories == nil {
		stories = []model.SearchResult{}
	}
	if epics == nil {
		epics = []model.SearchResult{}
	}
	if sprints == nil {
		sprints = []model.SearchResult{}
	}
	if objectives == nil {
		objectives = []model.SearchResult{}
	}
	if members == nil {
		members = []model.SearchResult{}
	}

	return &model.SearchResponse{
		Stories:    stories,
		Epics:      epics,
		Sprints:    sprints,
		Objectives: objectives,
		Members:    members,
	}, nil
}
