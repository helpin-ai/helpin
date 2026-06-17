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
				"title", "url", "content", "content_hash", "embedding",
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

func (r *SupportContentChunkRepository) HybridSearch(ctx context.Context, workspaceID string, sourceIDs []string, query string, queryEmbedding string, limit int) ([]SupportContentChunkSearchResult, error) {
	if limit <= 0 {
		limit = 8
	}
	if len(sourceIDs) == 0 || query == "" {
		return []SupportContentChunkSearchResult{}, nil
	}

	lexical, err := r.lexicalSearch(ctx, workspaceID, sourceIDs, query, max(limit*4, 12))
	if err != nil {
		return nil, err
	}

	vector := []SupportContentChunkSearchResult{}
	if queryEmbedding != "" && r.db.Dialector.Name() == "postgres" {
		vector, err = r.vectorSearch(ctx, workspaceID, sourceIDs, queryEmbedding, max(limit*4, 12))
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

func (r *SupportContentChunkRepository) lexicalSearch(ctx context.Context, workspaceID string, sourceIDs []string, query string, limit int) ([]SupportContentChunkSearchResult, error) {
	tsQuery := toTSQuery(query)
	if tsQuery == "" {
		return []SupportContentChunkSearchResult{}, nil
	}

	if r.db.Dialector.Name() != "postgres" {
		var chunks []model.SupportContentChunk
		if err := r.db.WithContext(ctx).
			Where("workspace_id = ? AND content_source_id IN ?", workspaceID, sourceIDs).
			Order("updated_at DESC").
			Limit(limit).
			Find(&chunks).Error; err != nil {
			return nil, err
		}
		results := make([]SupportContentChunkSearchResult, 0, len(chunks))
		for _, chunk := range chunks {
			results = append(results, SupportContentChunkSearchResult{
				ID:              chunk.ID,
				WorkspaceID:     chunk.WorkspaceID,
				ContentSourceID: chunk.ContentSourceID,
				PageID:          chunk.PageID,
				ChunkIndex:      chunk.ChunkIndex,
				Title:           chunk.Title,
				URL:             chunk.URL,
				Content:         chunk.Content,
				LexicalScore:    1,
			})
		}
		return results, nil
	}

	sql := `
		SELECT c.id, c.workspace_id, c.content_source_id, c.page_id, c.chunk_index, c.title, c.url, c.content,
		       ts_rank(
		         setweight(to_tsvector('english', COALESCE(c.title, '')), 'A') ||
		         setweight(to_tsvector('english', COALESCE(c.content, '')), 'B'),
		         to_tsquery('english', ?)
		       ) AS lexical_score
		FROM support_content_chunks c
		WHERE c.workspace_id = ?
		  AND c.content_source_id IN ?
		  AND (
		    to_tsvector('english', COALESCE(c.title, '')) ||
		    to_tsvector('english', COALESCE(c.content, ''))
		  ) @@ to_tsquery('english', ?)
		ORDER BY lexical_score DESC, c.updated_at DESC
		LIMIT ?
	`
	var results []SupportContentChunkSearchResult
	if err := r.db.WithContext(ctx).Raw(sql, tsQuery, workspaceID, sourceIDs, tsQuery, limit).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("lexical content chunk search: %w", err)
	}
	return results, nil
}

func (r *SupportContentChunkRepository) vectorSearch(ctx context.Context, workspaceID string, sourceIDs []string, queryEmbedding string, limit int) ([]SupportContentChunkSearchResult, error) {
	sql := `
		SELECT c.id, c.workspace_id, c.content_source_id, c.page_id, c.chunk_index, c.title, c.url, c.content,
		       GREATEST(0, 1 - (c.embedding <=> CAST(? AS vector))) AS vector_score
		FROM support_content_chunks c
		WHERE c.workspace_id = ?
		  AND c.content_source_id IN ?
		  AND c.embedding_provider = ?
		  AND c.embedding_model = ?
		  AND c.embedding_version = ?
		  AND c.embedding_dimensions = ?
		ORDER BY c.embedding <=> CAST(? AS vector) ASC, c.updated_at DESC
		LIMIT ?
	`
	var results []SupportContentChunkSearchResult
	if err := r.db.WithContext(ctx).Raw(sql, queryEmbedding, workspaceID, sourceIDs, defaultChunkEmbeddingProvider, defaultChunkEmbeddingModel, defaultChunkEmbeddingVersion, defaultChunkEmbeddingDimensions, queryEmbedding, limit).Scan(&results).Error; err != nil {
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
