package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DocsChunkRepository manages pgvector-backed docs chunks.
type DocsChunkRepository struct {
	db *gorm.DB
}

const (
	defaultChunkEmbeddingProvider   = "openai"
	defaultChunkEmbeddingModel      = "text-embedding-3-small"
	defaultChunkEmbeddingVersion    = "content-chunk-v1"
	defaultChunkEmbeddingDimensions = 1536
)

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

// DocsKnowledgeScopeFilter is the searchable docs scope derived from selected knowledge sources.
type DocsKnowledgeScopeFilter struct {
	FullSpaceIDs  []string
	CollectionIDs []string
	DocumentIDs   []string
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
				"block_id", "block_range", "title", "content", "content_hash", "embedding",
				"embedding_provider", "embedding_model", "embedding_version", "embedding_dimensions",
				"updated_at",
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
	return r.HybridSearchWithEmbeddingModel(ctx, workspaceID, spaceIDs, query, queryEmbedding, defaultChunkEmbeddingModel, limit)
}

// HybridSearchKnowledgeSources performs hybrid retrieval constrained by selected docs knowledge source scopes.
func (r *DocsChunkRepository) HybridSearchKnowledgeSources(
	ctx context.Context,
	workspaceID string,
	sources []model.AgentKnowledgeSource,
	query string,
	queryEmbedding string,
	limit int,
) ([]DocsChunkSearchResult, error) {
	filter := BuildDocsKnowledgeScopeFilter(sources)
	return r.hybridSearchWithScopeFilter(ctx, workspaceID, filter, query, queryEmbedding, defaultChunkEmbeddingModel, limit)
}

func (r *DocsChunkRepository) HybridSearchWithEmbeddingModel(
	ctx context.Context,
	workspaceID string,
	spaceIDs []string,
	query string,
	queryEmbedding string,
	embeddingModel string,
	limit int,
) ([]DocsChunkSearchResult, error) {
	if limit <= 0 {
		limit = 8
	}
	if len(spaceIDs) == 0 || query == "" {
		return []DocsChunkSearchResult{}, nil
	}
	if embeddingModel == "" {
		embeddingModel = defaultChunkEmbeddingModel
	}

	filter := DocsKnowledgeScopeFilter{FullSpaceIDs: spaceIDs}
	return r.hybridSearchWithScopeFilter(ctx, workspaceID, filter, query, queryEmbedding, embeddingModel, limit)
}

func (r *DocsChunkRepository) hybridSearchWithScopeFilter(
	ctx context.Context,
	workspaceID string,
	filter DocsKnowledgeScopeFilter,
	query string,
	queryEmbedding string,
	embeddingModel string,
	limit int,
) ([]DocsChunkSearchResult, error) {
	if limit <= 0 {
		limit = 8
	}
	if filter.empty() || query == "" {
		return []DocsChunkSearchResult{}, nil
	}
	if embeddingModel == "" {
		embeddingModel = defaultChunkEmbeddingModel
	}

	lexical, err := r.lexicalSearch(ctx, workspaceID, filter, query, max(limit*4, 12))
	if err != nil {
		return nil, err
	}

	vector := []DocsChunkSearchResult{}
	if queryEmbedding != "" && r.db.Dialector.Name() == "postgres" {
		vector, err = r.vectorSearch(ctx, workspaceID, filter, queryEmbedding, embeddingModel, max(limit*4, 12))
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

func (r *DocsChunkRepository) lexicalSearch(ctx context.Context, workspaceID string, filter DocsKnowledgeScopeFilter, query string, limit int) ([]DocsChunkSearchResult, error) {
	tsQuery := toTSQuery(query)
	if tsQuery == "" {
		return []DocsChunkSearchResult{}, nil
	}

	if r.db.Dialector.Name() != "postgres" {
		query := r.db.WithContext(ctx).
			Model(&model.DocsChunk{}).
			Joins("JOIN docs_documents d ON d.id = docs_chunks.document_id").
			Where("docs_chunks.workspace_id = ?", workspaceID)
		query = applySQLiteDocsScopeFilter(query, filter)
		var chunks []model.DocsChunk
		if err := query.
			Order("docs_chunks.updated_at DESC").
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
		WITH RECURSIVE scoped_collections AS (
		  SELECT id
		  FROM docs_collections
		  WHERE workspace_id = ? AND id IN ?
		  UNION ALL
		  SELECT c.id
		  FROM docs_collections c
		  JOIN scoped_collections sc ON c.parent_collection_id = sc.id
		  WHERE c.deleted_at IS NULL
		)
		SELECT c.id, c.workspace_id, c.space_id, c.document_id, c.block_id, c.chunk_index, c.title, c.content,
		       ts_rank(
		         setweight(to_tsvector('english', COALESCE(c.title, '')), 'A') ||
		         setweight(to_tsvector('english', COALESCE(c.content, '')), 'B'),
		         to_tsquery('english', ?)
		       ) AS lexical_score
		FROM docs_chunks c
		JOIN docs_documents d ON d.id = c.document_id
		LEFT JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE c.workspace_id = ?
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND (
		    (? AND c.space_id IN ?)
		    OR (? AND d.collection_id IN (SELECT id FROM scoped_collections))
		    OR (? AND c.document_id IN ?)
		  )
		  AND (
		    to_tsvector('english', COALESCE(c.title, '')) ||
		    to_tsvector('english', COALESCE(c.content, ''))
		  ) @@ to_tsquery('english', ?)
		ORDER BY lexical_score DESC, c.updated_at DESC
		LIMIT ?
	`
	var results []DocsChunkSearchResult
	if err := r.db.WithContext(ctx).Raw(
		sql,
		workspaceID,
		nonEmptyStrings(filter.CollectionIDs),
		tsQuery,
		workspaceID,
		len(filter.FullSpaceIDs) > 0,
		nonEmptyStrings(filter.FullSpaceIDs),
		len(filter.CollectionIDs) > 0,
		len(filter.DocumentIDs) > 0,
		nonEmptyStrings(filter.DocumentIDs),
		tsQuery,
		limit,
	).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("lexical chunk search: %w", err)
	}
	return results, nil
}

func (r *DocsChunkRepository) vectorSearch(ctx context.Context, workspaceID string, filter DocsKnowledgeScopeFilter, queryEmbedding string, embeddingModel string, limit int) ([]DocsChunkSearchResult, error) {
	if embeddingModel == "" {
		embeddingModel = defaultChunkEmbeddingModel
	}
	sql := `
		WITH RECURSIVE scoped_collections AS (
		  SELECT id
		  FROM docs_collections
		  WHERE workspace_id = ? AND id IN ?
		  UNION ALL
		  SELECT c.id
		  FROM docs_collections c
		  JOIN scoped_collections sc ON c.parent_collection_id = sc.id
		  WHERE c.deleted_at IS NULL
		)
		SELECT c.id, c.workspace_id, c.space_id, c.document_id, c.block_id, c.chunk_index, c.title, c.content,
		       GREATEST(0, 1 - (c.embedding <=> CAST(? AS vector))) AS vector_score
		FROM docs_chunks c
		JOIN docs_documents d ON d.id = c.document_id
		LEFT JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE c.workspace_id = ?
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND (
		    (? AND c.space_id IN ?)
		    OR (? AND d.collection_id IN (SELECT id FROM scoped_collections))
		    OR (? AND c.document_id IN ?)
		  )
		  AND c.embedding_provider = ?
		  AND c.embedding_model = ?
		  AND c.embedding_version = ?
		  AND c.embedding_dimensions = ?
		ORDER BY c.embedding <=> CAST(? AS vector) ASC, c.updated_at DESC
		LIMIT ?
	`
	var results []DocsChunkSearchResult
	if err := r.db.WithContext(ctx).Raw(
		sql,
		workspaceID,
		nonEmptyStrings(filter.CollectionIDs),
		queryEmbedding,
		workspaceID,
		len(filter.FullSpaceIDs) > 0,
		nonEmptyStrings(filter.FullSpaceIDs),
		len(filter.CollectionIDs) > 0,
		len(filter.DocumentIDs) > 0,
		nonEmptyStrings(filter.DocumentIDs),
		defaultChunkEmbeddingProvider,
		embeddingModel,
		defaultChunkEmbeddingVersion,
		defaultChunkEmbeddingDimensions,
		queryEmbedding,
		limit,
	).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("vector chunk search: %w", err)
	}
	return results, nil
}

// BuildDocsKnowledgeScopeFilter turns selected source rows into a compact retrieval filter.
func BuildDocsKnowledgeScopeFilter(sources []model.AgentKnowledgeSource) DocsKnowledgeScopeFilter {
	fullSpaces := map[string]struct{}{}
	collections := map[string]struct{}{}
	documents := map[string]struct{}{}

	for _, source := range sources {
		scopeType := source.ScopeType
		if scopeType == "" {
			scopeType = model.KnowledgeSourceScopeSpace
		}
		switch scopeType {
		case model.KnowledgeSourceScopeSpace:
			if source.SpaceID != "" {
				fullSpaces[source.SpaceID] = struct{}{}
			}
		case model.KnowledgeSourceScopeCollection:
			if source.CollectionID != nil && *source.CollectionID != "" {
				collections[*source.CollectionID] = struct{}{}
			}
		case model.KnowledgeSourceScopeArticle:
			if source.DocumentID != nil && *source.DocumentID != "" {
				documents[*source.DocumentID] = struct{}{}
			}
		}
	}

	return DocsKnowledgeScopeFilter{
		FullSpaceIDs:  mapKeys(fullSpaces),
		CollectionIDs: mapKeys(collections),
		DocumentIDs:   mapKeys(documents),
	}
}

func (f DocsKnowledgeScopeFilter) empty() bool {
	return len(f.FullSpaceIDs) == 0 && len(f.CollectionIDs) == 0 && len(f.DocumentIDs) == 0
}

func applySQLiteDocsScopeFilter(query *gorm.DB, filter DocsKnowledgeScopeFilter) *gorm.DB {
	conditions := []string{}
	args := []any{}
	if len(filter.FullSpaceIDs) > 0 {
		conditions = append(conditions, "docs_chunks.space_id IN ?")
		args = append(args, filter.FullSpaceIDs)
	}
	if len(filter.CollectionIDs) > 0 {
		conditions = append(conditions, "d.collection_id IN ?")
		args = append(args, filter.CollectionIDs)
	}
	if len(filter.DocumentIDs) > 0 {
		conditions = append(conditions, "docs_chunks.document_id IN ?")
		args = append(args, filter.DocumentIDs)
	}
	if len(conditions) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where("("+strings.Join(conditions, " OR ")+")", args...)
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func nonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return []string{""}
	}
	return values
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
