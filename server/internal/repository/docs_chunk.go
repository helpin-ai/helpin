package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsChunkRepository manages pgvector-backed docs chunks.
type DocsChunkRepository struct {
	db *gorm.DB
}

// DocsChunkSearchResult is a chunk-level retrieval result.
type DocsChunkSearchResult struct {
	ID            string  `json:"id"`
	WorkspaceID   string  `json:"workspace_id"`
	SpaceID       string  `json:"space_id"`
	DocumentID    string  `json:"document_id"`
	BlockID       *string `json:"block_id,omitempty"`
	ChunkIndex    int     `json:"chunk_index"`
	Title         string  `json:"title"`
	Content       string  `json:"content"`
	LexicalScore  float64 `json:"lexical_score"`
	VectorScore   float64 `json:"vector_score"`
	CombinedScore float64 `json:"combined_score"`
}

// NewDocsChunkRepository creates a new DocsChunkRepository.
func NewDocsChunkRepository(db *gorm.DB) *DocsChunkRepository {
	return &DocsChunkRepository{db: db}
}

// ReplaceDocumentChunks overwrites all chunks for a document.
func (r *DocsChunkRepository) ReplaceDocumentChunks(ctx context.Context, documentID string, chunks []model.DocsChunk) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("document_id = ?", documentID).Delete(&model.DocsChunk{}).Error; err != nil {
			return fmt.Errorf("delete document chunks: %w", err)
		}
		if len(chunks) == 0 {
			return nil
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "document_id"}, {Name: "chunk_index"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"block_id", "block_range", "title", "content", "content_hash", "embedding", "updated_at",
			}),
		}).Create(&chunks).Error; err != nil {
			return fmt.Errorf("upsert document chunks: %w", err)
		}
		return nil
	})
}

// DeleteByDocumentID removes all chunks for a document.
func (r *DocsChunkRepository) DeleteByDocumentID(ctx context.Context, documentID string) error {
	return r.db.WithContext(ctx).Where("document_id = ?", documentID).Delete(&model.DocsChunk{}).Error
}

// DeleteByDocumentIDs bulk-deletes chunks for a set of documents in a single
// query. Used by the batch document deletion path.
func (r *DocsChunkRepository) DeleteByDocumentIDs(ctx context.Context, documentIDs []string) error {
	if len(documentIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("document_id IN ?", documentIDs).Delete(&model.DocsChunk{}).Error
}

// DeleteBySpaceExceptDocuments removes stale chunks for docs no longer eligible in a help-center space.
func (r *DocsChunkRepository) DeleteBySpaceExceptDocuments(ctx context.Context, workspaceID, spaceID string, keepDocumentIDs []string) error {
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND space_id = ?", workspaceID, spaceID)
	if len(keepDocumentIDs) > 0 {
		query = query.Where("document_id NOT IN ?", keepDocumentIDs)
	}
	return query.Delete(&model.DocsChunk{}).Error
}

// CountBySpaceID returns the total number of chunks for a docs space.
func (r *DocsChunkRepository) CountBySpaceID(ctx context.Context, workspaceID, spaceID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.DocsChunk{}).
		Where("workspace_id = ? AND space_id = ?", workspaceID, spaceID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// HybridSearch performs lexical + vector retrieval and fuses the results in memory.
func (r *DocsChunkRepository) HybridSearch(
	ctx context.Context,
	workspaceID string,
	spaceIDs []string,
	query string,
	queryEmbedding string,
	limit int,
) ([]DocsChunkSearchResult, error) {
	if limit <= 0 {
		limit = 8
	}
	if len(spaceIDs) == 0 || query == "" {
		return []DocsChunkSearchResult{}, nil
	}

	lexical, err := r.lexicalSearch(ctx, workspaceID, spaceIDs, query, max(limit*4, 12))
	if err != nil {
		return nil, err
	}

	vector := []DocsChunkSearchResult{}
	if queryEmbedding != "" && r.db.Dialector.Name() == "postgres" {
		vector, err = r.vectorSearch(ctx, workspaceID, spaceIDs, queryEmbedding, max(limit*4, 12))
		if err != nil {
			return nil, err
		}
	}

	fused := fuseChunkResults(lexical, vector)
	if len(fused) > limit {
		fused = fused[:limit]
	}
	return fused, nil
}

func (r *DocsChunkRepository) lexicalSearch(ctx context.Context, workspaceID string, spaceIDs []string, query string, limit int) ([]DocsChunkSearchResult, error) {
	tsQuery := toTSQuery(query)
	if tsQuery == "" {
		return []DocsChunkSearchResult{}, nil
	}

	if r.db.Dialector.Name() != "postgres" {
		var chunks []model.DocsChunk
		if err := r.db.WithContext(ctx).
			Where("workspace_id = ? AND space_id IN ?", workspaceID, spaceIDs).
			Order("updated_at DESC").
			Limit(limit).
			Find(&chunks).Error; err != nil {
			return nil, err
		}
		results := make([]DocsChunkSearchResult, 0, len(chunks))
		for _, chunk := range chunks {
			results = append(results, DocsChunkSearchResult{
				ID:           chunk.ID,
				WorkspaceID:  chunk.WorkspaceID,
				SpaceID:      chunk.SpaceID,
				DocumentID:   chunk.DocumentID,
				BlockID:      chunk.BlockID,
				ChunkIndex:   chunk.ChunkIndex,
				Title:        chunk.Title,
				Content:      chunk.Content,
				LexicalScore: 1,
			})
		}
		return results, nil
	}

	sql := `
		SELECT c.id, c.workspace_id, c.space_id, c.document_id, c.block_id, c.chunk_index, c.title, c.content,
		       ts_rank(
		         setweight(to_tsvector('english', COALESCE(c.title, '')), 'A') ||
		         setweight(to_tsvector('english', COALESCE(c.content, '')), 'B'),
		         to_tsquery('english', ?)
		       ) AS lexical_score
		FROM docs_chunks c
		JOIN docs_documents d ON d.id = c.document_id
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE c.workspace_id = ?
		  AND c.space_id IN ?
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND ha.public_published_at IS NOT NULL
		  AND (
		    to_tsvector('english', COALESCE(c.title, '')) ||
		    to_tsvector('english', COALESCE(c.content, ''))
		  ) @@ to_tsquery('english', ?)
		ORDER BY lexical_score DESC, c.updated_at DESC
		LIMIT ?
	`
	var results []DocsChunkSearchResult
	if err := r.db.WithContext(ctx).Raw(sql, tsQuery, workspaceID, spaceIDs, tsQuery, limit).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("lexical chunk search: %w", err)
	}
	return results, nil
}

func (r *DocsChunkRepository) vectorSearch(ctx context.Context, workspaceID string, spaceIDs []string, queryEmbedding string, limit int) ([]DocsChunkSearchResult, error) {
	sql := `
		SELECT c.id, c.workspace_id, c.space_id, c.document_id, c.block_id, c.chunk_index, c.title, c.content,
		       GREATEST(0, 1 - (c.embedding <=> CAST(? AS vector))) AS vector_score
		FROM docs_chunks c
		JOIN docs_documents d ON d.id = c.document_id
		JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE c.workspace_id = ?
		  AND c.space_id IN ?
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND ha.public_published_at IS NOT NULL
		ORDER BY c.embedding <=> CAST(? AS vector) ASC, c.updated_at DESC
		LIMIT ?
	`
	var results []DocsChunkSearchResult
	if err := r.db.WithContext(ctx).Raw(sql, queryEmbedding, workspaceID, spaceIDs, queryEmbedding, limit).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("vector chunk search: %w", err)
	}
	return results, nil
}

func fuseChunkResults(lexical []DocsChunkSearchResult, vector []DocsChunkSearchResult) []DocsChunkSearchResult {
	type scored struct {
		result DocsChunkSearchResult
		score  float64
	}

	byID := map[string]*scored{}
	add := func(results []DocsChunkSearchResult, field string) {
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

	fused := make([]DocsChunkSearchResult, 0, len(byID))
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
