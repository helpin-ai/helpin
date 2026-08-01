package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportContentChunkSearchResult is a chunk-level search hit for crawled content.
type SupportContentChunkSearchResult struct {
	ID              string  `json:"id"`
	WorkspaceID     string  `json:"workspace_id"`
	ContentSourceID string  `json:"content_source_id"`
	PageID          string  `json:"page_id"`
	ChunkIndex      int     `json:"chunk_index"`
	SectionKey      string  `json:"section_key"`
	HeadingPath     string  `json:"heading_path"`
	Title           string  `json:"title"`
	URL             string  `json:"url"`
	Content         string  `json:"content"`
	LexicalScore    float64 `json:"lexical_score"`
	VectorScore     float64 `json:"vector_score"`
	CombinedScore   float64 `json:"combined_score"`
}

// SupportContentChunkRepository manages pgvector-backed content chunks.
type SupportContentChunkRepository struct {
	db *gorm.DB
}

func NewSupportContentChunkRepository(db *gorm.DB) *SupportContentChunkRepository {
	return &SupportContentChunkRepository{db: db}
}

func (r *SupportContentChunkRepository) ReplacePageChunks(ctx context.Context, pageID string, chunks []model.SupportContentChunk) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("page_id = ?", pageID).Delete(&model.SupportContentChunk{}).Error; err != nil {
			return fmt.Errorf("delete content page chunks: %w", err)
		}
		if len(chunks) == 0 {
			return nil
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "page_id"}, {Name: "chunk_index"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"section_key", "heading_path", "title", "url", "content", "search_content",
				"previous_chunk_index", "next_chunk_index", "content_hash", "embedding",
				"embedding_provider", "embedding_model", "embedding_version", "embedding_dimensions",
				"updated_at",
			}),
		}).Create(&chunks).Error; err != nil {
			return fmt.Errorf("upsert content page chunks: %w", err)
		}
		return nil
	})
}

func (r *SupportContentChunkRepository) DeleteByContentSourceID(ctx context.Context, contentSourceID string) error {
	if err := r.db.WithContext(ctx).
		Where("content_source_id = ?", contentSourceID).
		Delete(&model.SupportContentChunk{}).Error; err != nil {
		return fmt.Errorf("delete content chunks by source: %w", err)
	}
	return nil
}

func (r *SupportContentChunkRepository) DeleteByContentSourceExceptPages(ctx context.Context, workspaceID, contentSourceID string, keepPageIDs []string) error {
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND content_source_id = ?", workspaceID, contentSourceID)
	if len(keepPageIDs) > 0 {
		query = query.Where("page_id NOT IN ?", keepPageIDs)
	}
	if err := query.Delete(&model.SupportContentChunk{}).Error; err != nil {
		return fmt.Errorf("delete stale content chunks: %w", err)
	}
	return nil
}

// ListPageNeighbors returns exact adjacent chunks for an already eligible page hit.
func (r *SupportContentChunkRepository) ListPageNeighbors(ctx context.Context, workspaceID, agentID, pageID string, indexes []int) ([]SupportContentChunkSearchResult, error) {
	if len(indexes) == 0 {
		return []SupportContentChunkSearchResult{}, nil
	}
	results := []SupportContentChunkSearchResult{}
	if err := r.db.WithContext(ctx).
		Table("support_content_chunks AS c").
		Select("c.id, c.workspace_id, c.content_source_id, c.page_id, c.chunk_index, c.section_key, c.heading_path, c.title, c.url, c.content").
		Joins("JOIN agent_content_sources acs ON acs.content_source_id = c.content_source_id AND acs.workspace_id = c.workspace_id").
		Joins("JOIN support_content_sources scs ON scs.id = c.content_source_id AND scs.workspace_id = c.workspace_id").
		Joins("JOIN support_content_pages p ON p.id = c.page_id AND p.workspace_id = c.workspace_id").
		Where("c.workspace_id = ? AND acs.agent_id = ? AND c.page_id = ? AND c.chunk_index IN ?", workspaceID, agentID, pageID, indexes).
		Where("scs.sync_status <> ? AND p.http_status >= 200 AND p.http_status < 300", model.KnowledgeSourceSyncDisabled).
		Order("c.chunk_index ASC").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("list content chunk neighbors: %w", err)
	}
	return results, nil
}

func (r *SupportContentChunkRepository) HybridSearch(ctx context.Context, workspaceID string, sourceIDs []string, query string, queryEmbedding string, limit int) ([]SupportContentChunkSearchResult, error) {
	return r.hybridSearch(ctx, workspaceID, "", sourceIDs, query, queryEmbedding, defaultChunkEmbeddingModel, limit)
}

// HybridSearchForAgent enforces the agent-to-source link inside candidate SQL.
func (r *SupportContentChunkRepository) HybridSearchForAgent(ctx context.Context, workspaceID, agentID string, sourceIDs []string, query, queryEmbedding, embeddingModel string, limit int) ([]SupportContentChunkSearchResult, error) {
	return r.hybridSearch(ctx, workspaceID, agentID, sourceIDs, query, queryEmbedding, embeddingModel, limit)
}

func (r *SupportContentChunkRepository) HybridSearchWithEmbeddingModel(ctx context.Context, workspaceID string, sourceIDs []string, query string, queryEmbedding string, embeddingModel string, limit int) ([]SupportContentChunkSearchResult, error) {
	return r.hybridSearch(ctx, workspaceID, "", sourceIDs, query, queryEmbedding, embeddingModel, limit)
}

func (r *SupportContentChunkRepository) hybridSearch(ctx context.Context, workspaceID, agentID string, sourceIDs []string, query, queryEmbedding, embeddingModel string, limit int) ([]SupportContentChunkSearchResult, error) {
	if limit <= 0 {
		limit = 8
	}
	if len(sourceIDs) == 0 || query == "" {
		return []SupportContentChunkSearchResult{}, nil
	}
	if embeddingModel == "" {
		embeddingModel = defaultChunkEmbeddingModel
	}

	lexical, err := r.lexicalSearch(ctx, workspaceID, agentID, sourceIDs, query, max(limit*4, 12))
	if err != nil {
		return nil, err
	}

	vector := []SupportContentChunkSearchResult{}
	if queryEmbedding != "" && r.db.Dialector.Name() == "postgres" {
		vector, err = r.vectorSearch(ctx, workspaceID, agentID, sourceIDs, queryEmbedding, embeddingModel, max(limit*4, 12))
		if err != nil {
			return nil, err
		}
	}

	fused := fuseSupportContentResults(lexical, vector)
	if len(fused) > limit {
		fused = fused[:limit]
	}
	return fused, nil
}

func (r *SupportContentChunkRepository) lexicalSearch(ctx context.Context, workspaceID, agentID string, sourceIDs []string, query string, limit int) ([]SupportContentChunkSearchResult, error) {
	tsQuery := toTSQuery(query)
	if tsQuery == "" {
		return []SupportContentChunkSearchResult{}, nil
	}

	if r.db.Dialector.Name() != "postgres" {
		var results []SupportContentChunkSearchResult
		dbQuery := r.db.WithContext(ctx).Table("support_content_chunks").
			Select("support_content_chunks.id, support_content_chunks.workspace_id, support_content_chunks.content_source_id, support_content_chunks.page_id, support_content_chunks.chunk_index, support_content_chunks.section_key, support_content_chunks.heading_path, support_content_chunks.title, support_content_chunks.url, support_content_chunks.content").
			Where("support_content_chunks.workspace_id = ? AND support_content_chunks.content_source_id IN ?", workspaceID, sourceIDs)
		if agentID != "" {
			dbQuery = dbQuery.Joins("JOIN agent_content_sources acs ON acs.content_source_id = support_content_chunks.content_source_id AND acs.workspace_id = support_content_chunks.workspace_id").
				Joins("JOIN support_content_sources scs ON scs.id = support_content_chunks.content_source_id AND scs.workspace_id = support_content_chunks.workspace_id").
				Joins("JOIN support_content_pages p ON p.id = support_content_chunks.page_id AND p.workspace_id = support_content_chunks.workspace_id").
				Where("acs.agent_id = ?", agentID).
				Where("scs.sync_status <> ? AND p.http_status >= 200 AND p.http_status < 300", model.KnowledgeSourceSyncDisabled)
		}
		if err := dbQuery.
			Order("support_content_chunks.updated_at DESC").
			Limit(limit).
			Scan(&results).Error; err != nil {
			return nil, err
		}
		for idx := range results {
			results[idx].LexicalScore = 1
		}
		return results, nil
	}

	agentJoin := ""
	sourcePredicate := ""
	agentPredicate := ""
	if agentID != "" {
		agentJoin = `JOIN support_content_sources scs ON scs.id = c.content_source_id AND scs.workspace_id = c.workspace_id
		JOIN support_content_pages p ON p.id = c.page_id AND p.workspace_id = c.workspace_id
		JOIN agent_content_sources acs ON acs.content_source_id = c.content_source_id AND acs.workspace_id = c.workspace_id`
		agentPredicate = "AND acs.agent_id = ?"
		sourcePredicate = "AND scs.sync_status <> 'disabled' AND p.http_status >= 200 AND p.http_status < 300"
	}
	sql := fmt.Sprintf(`
		SELECT c.id, c.workspace_id, c.content_source_id, c.page_id, c.chunk_index, c.section_key, c.heading_path, c.title, c.url, c.content,
		       ts_rank(
		         setweight(to_tsvector('english', COALESCE(c.title, '')), 'A') ||
		         setweight(to_tsvector('english', COALESCE(NULLIF(c.search_content, ''), c.content, '')), 'B'),
		         to_tsquery('english', ?)
		       ) AS lexical_score
		FROM support_content_chunks c
		%s
		WHERE c.workspace_id = ?
		  AND c.content_source_id IN ?
		  %s
		  %s
		  AND (
		    setweight(to_tsvector('english', COALESCE(c.title, '')), 'A') ||
		    setweight(to_tsvector('english', COALESCE(NULLIF(c.search_content, ''), c.content, '')), 'B')
		  ) @@ to_tsquery('english', ?)
		ORDER BY lexical_score DESC, c.updated_at DESC
		LIMIT ?
	`, agentJoin, sourcePredicate, agentPredicate)
	var results []SupportContentChunkSearchResult
	args := []any{tsQuery, workspaceID, sourceIDs}
	if agentID != "" {
		args = append(args, agentID)
	}
	args = append(args, tsQuery, limit)
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("lexical content chunk search: %w", err)
	}
	return results, nil
}

func (r *SupportContentChunkRepository) vectorSearch(ctx context.Context, workspaceID, agentID string, sourceIDs []string, queryEmbedding string, embeddingModel string, limit int) ([]SupportContentChunkSearchResult, error) {
	if embeddingModel == "" {
		embeddingModel = defaultChunkEmbeddingModel
	}
	agentJoin := ""
	sourcePredicate := ""
	agentPredicate := ""
	if agentID != "" {
		agentJoin = `JOIN support_content_sources scs ON scs.id = c.content_source_id AND scs.workspace_id = c.workspace_id
		JOIN support_content_pages p ON p.id = c.page_id AND p.workspace_id = c.workspace_id
		JOIN agent_content_sources acs ON acs.content_source_id = c.content_source_id AND acs.workspace_id = c.workspace_id`
		agentPredicate = "AND acs.agent_id = ?"
		sourcePredicate = "AND scs.sync_status <> 'disabled' AND p.http_status >= 200 AND p.http_status < 300"
	}
	sql := fmt.Sprintf(`
		SELECT c.id, c.workspace_id, c.content_source_id, c.page_id, c.chunk_index, c.section_key, c.heading_path, c.title, c.url, c.content,
		       GREATEST(0, 1 - (c.embedding <=> CAST(? AS vector))) AS vector_score
		FROM support_content_chunks c
		%s
		WHERE c.workspace_id = ?
		  AND c.content_source_id IN ?
		  %s
		  %s
		  AND c.embedding_provider = ?
		  AND c.embedding_model = ?
		  AND c.embedding_version IN ?
		  AND c.embedding_dimensions = ?
		ORDER BY c.embedding <=> CAST(? AS vector) ASC
		LIMIT ?
	`, agentJoin, sourcePredicate, agentPredicate)
	var results []SupportContentChunkSearchResult
	args := []any{queryEmbedding, workspaceID, sourceIDs}
	if agentID != "" {
		args = append(args, agentID)
	}
	args = append(args, defaultChunkEmbeddingProvider, embeddingModel, []string{defaultChunkEmbeddingVersion, legacyChunkEmbeddingVersion}, defaultChunkEmbeddingDimensions, queryEmbedding, limit)
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("vector content chunk search: %w", err)
	}
	return results, nil
}

func fuseSupportContentResults(lexical []SupportContentChunkSearchResult, vector []SupportContentChunkSearchResult) []SupportContentChunkSearchResult {
	type scored struct {
		result SupportContentChunkSearchResult
		score  float64
	}

	byID := map[string]*scored{}
	add := func(results []SupportContentChunkSearchResult, field string) {
		for idx, result := range results {
			entry, ok := byID[result.ID]
			if !ok {
				entry = &scored{result: result}
				byID[result.ID] = entry
			}
			switch field {
			case "lexical":
				entry.result.LexicalScore = result.LexicalScore
			case "vector":
				entry.result.VectorScore = result.VectorScore
			}
			entry.score += 1.0 / float64(60+idx+1)
		}
	}

	add(lexical, "lexical")
	add(vector, "vector")

	fused := make([]SupportContentChunkSearchResult, 0, len(byID))
	for _, item := range byID {
		item.result.CombinedScore = item.score
		fused = append(fused, item.result)
	}

	for i := 0; i < len(fused); i++ {
		for j := i + 1; j < len(fused); j++ {
			if fused[j].CombinedScore > fused[i].CombinedScore {
				fused[i], fused[j] = fused[j], fused[i]
			}
		}
	}
	return fused
}
