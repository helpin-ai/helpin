package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// PMTriageScope must be derived from the authenticated workspace actor.
// Empty TeamIDs denies access unless AllTeams is explicitly granted.
type PMTriageScope struct {
	WorkspaceID string
	TeamIDs     []string
	AllTeams    bool
}

// FindTriageCandidates retrieves a bounded pool of relevant non-archived tasks before
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
	// Measure rarity inside the authorized scope, never across private tasks.
	var counts []string
	var countArgs []interface{}
	for i, term := range terms {
		counts = append(counts, fmt.Sprintf("COALESCE(SUM(CASE WHEN LOWER(name) LIKE ? OR LOWER(COALESCE(description, '')) LIKE ? THEN 1 ELSE 0 END), 0) AS term_%d", i))
		countArgs = append(countArgs, "%"+term+"%", "%"+term+"%")
	}
	countRow, err := query.Session(&gorm.Session{}).Select("COUNT(*) AS total, "+strings.Join(counts, ", "), countArgs...).Rows()
	if err != nil {
		return nil, fmt.Errorf("count triage terms: %w", err)
	}
	frequencies := make([]int64, len(terms)+1)
	destinations := make([]interface{}, len(frequencies))
	for i := range frequencies {
		destinations[i] = &frequencies[i]
	}
	if countRow.Next() {
		err = countRow.Scan(destinations...)
	}
	rowErr := countRow.Err()
	countRow.Close()
	if err != nil {
		return nil, err
	}
	if rowErr != nil {
		return nil, rowErr
	}
	weights := make([]float64, len(terms))
	for i := range terms {
		weights[i] = 1 + math.Log(1+float64(frequencies[0])/float64(1+frequencies[i+1]))
	}
	var predicates, scores []string
	var predicateArgs, scoreArgs []interface{}
	for i, term := range terms {
		// Terms contain only letters and digits, so LIKE wildcards cannot be injected.
		pattern := "%" + term + "%"
		predicates = append(predicates, "(LOWER(name) LIKE ? OR LOWER(COALESCE(description, '')) LIKE ?)")
		predicateArgs = append(predicateArgs, pattern, pattern)
		scores = append(scores, "CASE WHEN LOWER(name) LIKE ? THEN CAST(? AS DOUBLE PRECISION) ELSE 0 END + CASE WHEN LOWER(COALESCE(description, '')) LIKE ? THEN CAST(? AS DOUBLE PRECISION) ELSE 0 END")
		scoreArgs = append(scoreArgs, pattern, 3*weights[i], pattern, weights[i])
	}
	err = query.Select("id", "workspace_id", "team_id", "display_id", "name", "description", "updated_at").Where("("+strings.Join(predicates, " OR ")+")", predicateArgs...).
		Order(clause.OrderBy{Expression: clause.Expr{SQL: "(" + strings.Join(scores, " + ") + ") DESC, updated_at DESC, id ASC", Vars: scoreArgs}}).
		Limit(50).Find(&tasks).Error
	if err != nil {
		return nil, fmt.Errorf("find triage candidates: %w", err)
	}
	// Re-rank real words rather than substring or HTML-markup hits. Recency
	// only breaks equal relevance scores; old resolved work stays eligible.
	ranked := make([]struct {
		task  model.PMTask
		score float64
	}, 0, len(tasks))
	for _, task := range tasks {
		titleWords := triageWordSet(task.Name)
		description := ""
		if task.Description != nil {
			description = tiptap.RichTextToMarkdown(*task.Description)
		}
		descriptionWords := triageWordSet(description)
		score := 0.0
		for i, term := range terms {
			if titleWords[term] {
				score += 3 * weights[i]
			}
			if descriptionWords[term] {
				score += weights[i]
			}
		}
		if score > 0 {
			ranked = append(ranked, struct {
				task  model.PMTask
				score float64
			}{task, score})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if !ranked[i].task.UpdatedAt.Equal(ranked[j].task.UpdatedAt) {
			return ranked[i].task.UpdatedAt.After(ranked[j].task.UpdatedAt)
		}
		return ranked[i].task.ID < ranked[j].task.ID
	})
	tasks = tasks[:0]
	for _, candidate := range ranked {
		tasks = append(tasks, candidate.task)
	}
	return tasks, nil
}

func triageSearchTerms(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	stop := map[string]bool{"the": true, "and": true, "for": true, "this": true, "that": true, "with": true, "from": true, "have": true, "has": true, "was": true, "were": true, "are": true, "not": true, "when": true, "then": true, "can": true, "could": true, "would": true, "should": true, "please": true, "you": true, "your": true, "our": true, "task": true, "bug": true, "feature": true, "request": true}
	seen := map[string]bool{}
	var terms []string
	for _, word := range words {
		word = triageSingular(word)
		if len([]rune(word)) < 3 || len(word) > 80 || stop[word] || seen[word] {
			continue
		}
		seen[word] = true
		terms = append(terms, word)

	}
	if len(terms) <= 32 {
		return terms
	}
	// Keep the leading title terms, then sample the remaining description
	// evenly so a diagnostic near the end is not always discarded.
	selected := append([]string(nil), terms[:8]...)
	for i := 0; i < 24; i++ {
		selected = append(selected, terms[8+i*(len(terms)-9)/23])
	}
	return selected
}

func triageWordSet(text string) map[string]bool {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	result := make(map[string]bool, len(words))
	for _, word := range words {
		result[triageSingular(word)] = true
	}
	return result
}

// Fold simple plurals without changing short identifiers or words like access/status.
func triageSingular(word string) string {
	if len(word) > 4 && strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") && !strings.HasSuffix(word, "us") {
		return strings.TrimSuffix(word, "s")
	}
	return word
}
