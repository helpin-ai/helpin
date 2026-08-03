package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CuratedGuidanceSearchResult is an eligible guidance match.
type CuratedGuidanceSearchResult struct {
	ID            string  `json:"id"`
	WorkspaceID   string  `json:"workspace_id"`
	AgentID       string  `json:"agent_id"`
	Title         string  `json:"title"`
	Answer        string  `json:"answer"`
	Intent        string  `json:"intent"`
	Language      string  `json:"language"`
	LexicalScore  float64 `json:"lexical_score"`
	VectorScore   float64 `json:"vector_score"`
	CombinedScore float64 `json:"combined_score"`
}

type CuratedGuidanceRepository struct {
	db *gorm.DB
}

func curatedGuidancePostgresSearchSQL(vectorSelect, languageSQL, matchSQL, vectorOrder string) string {
	return fmt.Sprintf(`
		WITH ranked AS (
			SELECT id, workspace_id, agent_id, title, answer, intent, language, updated_at,
			       ts_rank(
			         setweight(to_tsvector('english', COALESCE(title, '')), 'A') ||
			         setweight(to_tsvector('english', COALESCE(array_to_string(question_patterns, ' '), '')), 'A') ||
			         setweight(to_tsvector('english', COALESCE(answer, '')), 'B'),
			         to_tsquery('english', ?)
			       ) AS lexical_score,
			       %s
			FROM curated_guidance
			WHERE workspace_id = ? AND agent_id = ? AND status = ?
			  AND audience_policy_id IS NULL AND brand_id IS NULL
			  AND (valid_from IS NULL OR valid_from <= ?)
			  AND (valid_until IS NULL OR valid_until > ?)
			  %s
			  %s
		)
		SELECT id, workspace_id, agent_id, title, answer, intent, language,
		       lexical_score, vector_score
		FROM ranked
		ORDER BY %s, updated_at DESC
		LIMIT ?`, vectorSelect, languageSQL, matchSQL, vectorOrder)
}

func NewCuratedGuidanceRepository(db *gorm.DB) *CuratedGuidanceRepository {
	return &CuratedGuidanceRepository{db: db}
}

func (r *CuratedGuidanceRepository) List(ctx context.Context, workspaceID, agentID string) ([]model.CuratedGuidance, error) {
	items := []model.CuratedGuidance{}
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND agent_id = ?", workspaceID, agentID).
		Order("updated_at DESC, id DESC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list curated guidance: %w", err)
	}
	return items, nil
}

func (r *CuratedGuidanceRepository) GetByID(ctx context.Context, workspaceID, agentID, id string) (*model.CuratedGuidance, error) {
	var item model.CuratedGuidance
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND agent_id = ? AND id = ?", workspaceID, agentID, id).
		First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get curated guidance: %w", err)
	}
	return &item, nil
}

func (r *CuratedGuidanceRepository) Create(ctx context.Context, item *model.CuratedGuidance) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create curated guidance: %w", err)
	}
	return nil
}

func (r *CuratedGuidanceRepository) Save(ctx context.Context, item *model.CuratedGuidance) error {
	if err := r.db.WithContext(ctx).Save(item).Error; err != nil {
		return fmt.Errorf("save curated guidance: %w", err)
	}
	return nil
}

func (r *CuratedGuidanceRepository) Delete(ctx context.Context, workspaceID, agentID, id string) error {
	result := r.db.WithContext(ctx).
		Where("workspace_id = ? AND agent_id = ? AND id = ?", workspaceID, agentID, id).
		Delete(&model.CuratedGuidance{})
	if result.Error != nil {
		return fmt.Errorf("delete curated guidance: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *CuratedGuidanceRepository) Search(
	ctx context.Context,
	workspaceID string,
	agentID string,
	language string,
	query string,
	queryEmbedding string,
	embeddingModel string,
	limit int,
) ([]CuratedGuidanceSearchResult, error) {
	if limit <= 0 {
		limit = 8
	}
	if query == "" {
		return []CuratedGuidanceSearchResult{}, nil
	}

	base := r.db.WithContext(ctx).
		Model(&model.CuratedGuidance{}).
		Where("workspace_id = ? AND agent_id = ?", workspaceID, agentID).
		Where("status = ?", model.CuratedGuidanceStatusActive).
		Where("audience_policy_id IS NULL AND brand_id IS NULL").
		Where("(valid_from IS NULL OR valid_from <= ?)", time.Now()).
		Where("(valid_until IS NULL OR valid_until > ?)", time.Now())
	if language != "" {
		base = base.Where("(language = '' OR language = ?)", language)
	} else {
		base = base.Where("language = ''")
	}

	if r.db.Dialector.Name() != "postgres" {
		var items []model.CuratedGuidance
		if err := base.Order("updated_at DESC").Limit(limit * 4).Find(&items).Error; err != nil {
			return nil, fmt.Errorf("search curated guidance: %w", err)
		}
		results := make([]CuratedGuidanceSearchResult, 0, len(items))
		for _, item := range items {
			haystack := strings.ToLower(item.Title + " " + strings.Join([]string(item.QuestionPatterns), " ") + " " + item.Answer)
			matched := 0
			for _, term := range strings.Fields(strings.ToLower(query)) {
				term = strings.Trim(term, ".,!?;:()[]{}\"'")
				if len(term) > 1 && strings.Contains(haystack, term) {
					matched++
				}
			}
			if matched == 0 {
				continue
			}
			score := float64(matched) / float64(max(len(strings.Fields(query)), 1))
			results = append(results, CuratedGuidanceSearchResult{
				ID: item.ID, WorkspaceID: item.WorkspaceID, AgentID: item.AgentID,
				Title: item.Title, Answer: item.Answer, Intent: item.Intent, Language: item.Language,
				LexicalScore: score, CombinedScore: score + 10,
			})
		}
		return results, nil
	}

	tsQuery := toTSQuery(query)
	if tsQuery == "" {
		return []CuratedGuidanceSearchResult{}, nil
	}
	params := []any{tsQuery, workspaceID, agentID, model.CuratedGuidanceStatusActive, time.Now(), time.Now()}
	languageSQL := ""
	if language != "" {
		languageSQL = " AND (language = '' OR language = ?)"
		params = append(params, language)
	} else {
		languageSQL = " AND language = ''"
	}
	vectorSelect := "0::double precision AS vector_score"
	vectorOrder := "lexical_score DESC"
	matchSQL := `AND (
		    to_tsvector('english', COALESCE(title, '')) ||
		    to_tsvector('english', COALESCE(array_to_string(question_patterns, ' '), '')) ||
		    to_tsvector('english', COALESCE(answer, ''))
		  ) @@ to_tsquery('english', ?)`
	if queryEmbedding != "" && embeddingModel != "" {
		vectorSelect = "CASE WHEN embedding IS NULL THEN 0 ELSE GREATEST(0, 1 - (embedding <=> CAST(? AS vector))) END AS vector_score"
		params = append([]any{tsQuery, queryEmbedding}, params[1:]...)
		vectorOrder = "(lexical_score + vector_score) DESC"
		matchSQL = ""
	}
	if matchSQL != "" {
		params = append(params, tsQuery)
	}
	params = append(params, limit)
	sql := curatedGuidancePostgresSearchSQL(vectorSelect, languageSQL, matchSQL, vectorOrder)
	var results []CuratedGuidanceSearchResult
	if err := r.db.WithContext(ctx).Raw(sql, params...).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("search curated guidance: %w", err)
	}
	for idx := range results {
		results[idx].CombinedScore = results[idx].LexicalScore + results[idx].VectorScore + 10
	}
	return results, nil
}
