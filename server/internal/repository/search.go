package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"unicode"

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

func normalizeSearchText(value string) string {
	var b strings.Builder
	previousWasSpace := true
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			previousWasSpace = false
			continue
		}
		if !previousWasSpace {
			b.WriteByte(' ')
			previousWasSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func normalizedSearchColumn(column string) string {
	return "regexp_replace(lower(coalesce(" + column + ", '')), '[^[:alnum:]]+', ' ', 'g')"
}

func taskSearchPredicate() string {
	return "(" + normalizedSearchColumn("name") + " LIKE ? OR " +
		normalizedSearchColumn("description") + " LIKE ? OR CAST(display_id AS TEXT) LIKE ?)"
}

func (r *SearchRepository) Search(ctx context.Context, workspaceID, query string, taskKeyDisplayID int) (*model.SearchResponse, error) {
	return r.SearchLimit(ctx, workspaceID, query, taskKeyDisplayID, searchLimit)
}

// SearchLimit searches each supported entity group with a bounded per-group
// result limit. Callers that paginate a merged result set can request enough
// candidates to cover the desired offset without changing legacy callers.
func (r *SearchRepository) SearchLimit(ctx context.Context, workspaceID, query string, taskKeyDisplayID, limit int) (*model.SearchResponse, error) {
	if limit <= 0 {
		limit = searchLimit
	}
	if limit > 600 {
		limit = 600
	}
	normalizedQuery := normalizeSearchText(query)
	if normalizedQuery == "" && taskKeyDisplayID <= 0 {
		return &model.SearchResponse{
			Tasks:      []model.SearchResult{},
			Epics:      []model.SearchResult{},
			Sprints:    []model.SearchResult{},
			Objectives: []model.SearchResult{},
			Members:    []model.SearchResult{},
			Documents:  []model.SearchResult{},
		}, nil
	}

	pattern := "%" + normalizedQuery + "%"
	var (
		stories    []model.SearchResult
		epics      []model.SearchResult
		sprints    []model.SearchResult
		objectives []model.SearchResult
		members    []model.SearchResult
		documents  []model.SearchResult
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

	wg.Add(6)

	go func() {
		defer wg.Done()

		// When a task key display_id is provided, do an exact lookup
		// plus the normal text search, then merge with the exact match first.
		var exactMatch []model.SearchResult
		if taskKeyDisplayID > 0 {
			if err := r.db.WithContext(ctx).
				Raw(`SELECT id, name, 'task' AS type, display_id, team_id
					FROM pm_tasks
					WHERE workspace_id = ? AND archived = false
					  AND display_id = ?
					LIMIT 1`, workspaceID, taskKeyDisplayID).
				Scan(&exactMatch).Error; err != nil {
				setErr(fmt.Errorf("search task by key: %w", err))
				return
			}
		}

		var textResults []model.SearchResult
		if err := r.db.WithContext(ctx).
			Raw(`SELECT id, name, 'task' AS type, display_id, team_id
				FROM pm_tasks
				WHERE workspace_id = ? AND archived = false
				  AND `+taskSearchPredicate()+`
				ORDER BY updated_at DESC
				LIMIT ?`, workspaceID, pattern, pattern, pattern, limit).
			Scan(&textResults).Error; err != nil {
			setErr(fmt.Errorf("search stories: %w", err))
			return
		}

		// Prepend exact match, deduplicating against text results.
		if len(exactMatch) > 0 {
			exactID := exactMatch[0].ID
			deduped := make([]model.SearchResult, 0, len(textResults))
			for _, r := range textResults {
				if r.ID != exactID {
					deduped = append(deduped, r)
				}
			}
			stories = append(exactMatch, deduped...)
		} else {
			stories = textResults
		}
	}()

	go func() {
		defer wg.Done()
		if err := r.db.WithContext(ctx).
			Raw(`SELECT id, name, 'epic' AS type
				FROM pm_epics
				WHERE workspace_id = ? AND archived = false
				  AND `+normalizedSearchColumn("name")+` LIKE ?
				ORDER BY updated_at DESC
				LIMIT ?`, workspaceID, pattern, limit).
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
				  AND `+normalizedSearchColumn("name")+` LIKE ?
				ORDER BY updated_at DESC
				LIMIT ?`, workspaceID, pattern, limit).
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
				  AND `+normalizedSearchColumn("name")+` LIKE ?
				ORDER BY updated_at DESC
				LIMIT ?`, workspaceID, pattern, limit).
			Scan(&objectives).Error; err != nil {
			setErr(fmt.Errorf("search objectives: %w", err))
		}
	}()

	go func() {
		defer wg.Done()
		if err := r.db.WithContext(ctx).
			Raw(`SELECT wm.id, wm.display_name AS name, 'member' AS type,
					COALESCE(string_agg(DISTINCT twm.team_id::text, ', '), '') AS team_id,
					COALESCE(string_agg(DISTINCT wt.name, ', '), '') AS team_name
				FROM workspace_members wm
				LEFT JOIN team_workspace_memberships twm ON twm.workspace_member_id = wm.id
				LEFT JOIN workspace_teams wt ON wt.id = twm.team_id
				WHERE wm.workspace_id = ? AND wm.status IN ('active', 'pending')
				  AND (`+normalizedSearchColumn("wm.display_name")+` LIKE ? OR `+normalizedSearchColumn("wm.email")+` LIKE ?)
				GROUP BY wm.id, wm.display_name
				ORDER BY wm.display_name ASC
				LIMIT ?`, workspaceID, pattern, pattern, limit).
			Scan(&members).Error; err != nil {
			setErr(fmt.Errorf("search members: %w", err))
		}
	}()

	go func() {
		defer wg.Done()
		if err := r.db.WithContext(ctx).
			Raw(`SELECT id, title AS name, 'document' AS type
				FROM docs_documents
				WHERE workspace_id = ? AND deleted_at IS NULL
				  AND status != 'archived'
				  AND `+normalizedSearchColumn("title")+` LIKE ?
				ORDER BY updated_at DESC
				LIMIT ?`, workspaceID, pattern, limit).
			Scan(&documents).Error; err != nil {
			setErr(fmt.Errorf("search documents: %w", err))
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
	if documents == nil {
		documents = []model.SearchResult{}
	}

	return &model.SearchResponse{
		Tasks:      stories,
		Epics:      epics,
		Sprints:    sprints,
		Objectives: objectives,
		Members:    members,
		Documents:  documents,
	}, nil
}
