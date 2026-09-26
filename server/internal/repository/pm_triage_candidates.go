package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMTriageScope must be derived from the authenticated workspace actor.
// Empty TeamIDs denies access unless AllTeams is explicitly granted.
type PMTriageScope struct {
	WorkspaceID string
	TeamIDs     []string
	AllTeams    bool
}

// FindTriageCandidates retrieves at most ten relevant non-archived tasks before
// any content is sent to the decision provider. It never broadens an empty scope.
func (r *PMTaskRepository) FindTriageCandidates(ctx context.Context, scope PMTriageScope, sourceID, text string) ([]model.PMTask, error) {
	if strings.TrimSpace(scope.WorkspaceID) == "" {
		return nil, errors.New("triage workspace required")
	}
	tasks := []model.PMTask{}
	if !scope.AllTeams && len(scope.TeamIDs) == 0 {
		return tasks, nil
	}
	terms := triageSearchTerms(text)
	if len(terms) == 0 {
		return tasks, nil
	}
	query := r.db.WithContext(ctx).Model(&model.PMTask{}).
		Select("id", "workspace_id", "team_id", "display_id", "name", "description", "updated_at").
		Where("workspace_id = ? AND archived = ?", scope.WorkspaceID, false)
	if !scope.AllTeams {
		query = query.Where("team_id IN ?", scope.TeamIDs)
	}
	if sourceID != "" {
		query = query.Where("id <> ?", sourceID)
	}
	var predicates, scores []string
	var predicateArgs, scoreArgs []interface{}
	for _, term := range terms {
		// Terms contain only letters and digits, so LIKE wildcards cannot be injected.
		pattern := "%" + term + "%"
		predicates = append(predicates, "(LOWER(name) LIKE ? OR LOWER(COALESCE(description, '')) LIKE ?)")
		predicateArgs = append(predicateArgs, pattern, pattern)
		scores = append(scores, "CASE WHEN LOWER(name) LIKE ? THEN 3 ELSE 0 END + CASE WHEN LOWER(COALESCE(description, '')) LIKE ? THEN 1 ELSE 0 END")
		scoreArgs = append(scoreArgs, pattern, pattern)
	}
	err := query.Where("("+strings.Join(predicates, " OR ")+")", predicateArgs...).
		Order(clause.OrderBy{Expression: clause.Expr{SQL: "(" + strings.Join(scores, " + ") + ") DESC, updated_at DESC, id ASC", Vars: scoreArgs}}).
		Limit(10).Find(&tasks).Error
	if err != nil {
		return nil, fmt.Errorf("find triage candidates: %w", err)
	}
	return tasks, nil
}

func triageSearchTerms(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	stop := map[string]bool{"the": true, "and": true, "for": true, "this": true, "that": true, "with": true, "from": true, "have": true, "has": true, "was": true, "were": true, "are": true, "not": true, "when": true, "then": true, "can": true, "could": true, "would": true, "should": true, "please": true, "you": true, "your": true, "our": true, "task": true, "bug": true, "feature": true, "request": true}
	seen := map[string]bool{}
	var terms []string
	for _, word := range words {
		if len([]rune(word)) < 3 || len(word) > 80 || stop[word] || seen[word] {
			continue
		}
		seen[word] = true
		terms = append(terms, word)
		if len(terms) == 12 {
			break
		}
	}
	return terms
}
