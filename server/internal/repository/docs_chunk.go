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
	defaultChunkEmbeddingVersion    = "content-chunk-v2"
	legacyChunkEmbeddingVersion     = "content-chunk-v1"
	defaultChunkEmbeddingDimensions = 1536
)

// DocsChunkSearchResult is a chunk-level retrieval result.
type DocsChunkSearchResult struct {
	ID            string  `json:"id"`
	WorkspaceID   string  `json:"workspace_id"`
	SpaceID       string  `json:"space_id"`
	SpaceType     string  `json:"space_type"`
	DocumentID    string  `json:"document_id"`
	BlockID       *string `json:"block_id,omitempty"`
	ChunkIndex    int     `json:"chunk_index"`
	SectionKey    string  `json:"section_key"`
	HeadingPath   string  `json:"heading_path"`
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
				"block_id", "block_range", "section_key", "heading_path", "title", "content", "search_content",
				"previous_chunk_index", "next_chunk_index", "content_hash", "embedding",
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

// DeleteBySpaceExceptDocuments removes stale chunks for docs no longer eligible in a selected space.
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

// ListSearchableExternalSpaceIDs returns workspace external-capable spaces that
// already have chunks belonging to published help-center articles. Public
// knowledge is workspace-wide; agent-to-space links control ingestion state,
// not which agents may retrieve released chunks.
func (r *DocsChunkRepository) ListSearchableExternalSpaceIDs(ctx context.Context, workspaceID string) ([]string, error) {
	ids := []string{}
	if err := r.db.WithContext(ctx).
		Table("docs_chunks AS c").
		Distinct("c.space_id").
		Joins("JOIN docs_spaces s ON s.id = c.space_id AND s.workspace_id = c.workspace_id").
		Joins("JOIN docs_documents d ON d.id = c.document_id AND d.workspace_id = c.workspace_id").
		Joins("JOIN docs_helpcenter_articles ha ON ha.document_id = d.id").
		Where("c.workspace_id = ?", workspaceID).
		Where("s.type = ? AND s.deleted_at IS NULL", model.SpaceTypeExternalCapable).
		Where("d.status = ? AND d.deleted_at IS NULL", model.DocStatusPublished).
		Where("ha.public_published_at IS NOT NULL").
		Pluck("c.space_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list searchable external docs spaces: %w", err)
	}
	return ids, nil
}

// ListDocumentNeighbors returns exact adjacent chunks for an already eligible
// document hit. Workspace and document filters remain in SQL.
func (r *DocsChunkRepository) ListDocumentNeighbors(ctx context.Context, workspaceID, agentID, documentID string, indexes []int) ([]DocsChunkSearchResult, error) {
	if len(indexes) == 0 {
		return []DocsChunkSearchResult{}, nil
	}
	results := []DocsChunkSearchResult{}
	query := r.db.WithContext(ctx).
		Table("docs_chunks AS c").
		Select("c.id, c.workspace_id, c.space_id, s.type AS space_type, c.document_id, c.block_id, c.chunk_index, c.section_key, c.heading_path, c.title, c.content").
		Joins("JOIN docs_documents d ON d.id = c.document_id").
		Joins("JOIN docs_spaces s ON s.id = c.space_id").
		Joins("LEFT JOIN docs_helpcenter_articles ha ON ha.document_id = d.id").
		Where("c.workspace_id = ? AND c.document_id = ? AND c.chunk_index IN ?", workspaceID, documentID, indexes).
		Where("d.status = ? AND d.deleted_at IS NULL", model.DocStatusPublished).
		Where("(s.type = ? OR (s.type = ? AND ha.public_published_at IS NOT NULL))", model.SpaceTypeInternal, model.SpaceTypeExternalCapable)
	if agentID != "" {
		query = query.
			Joins("JOIN agent_knowledge_sources aks ON aks.space_id = c.space_id AND aks.workspace_id = c.workspace_id").
			Where("aks.agent_id = ? AND aks.sync_status <> ?", agentID, model.KnowledgeSourceSyncDisabled)
	}
	if err := query.
		Order("c.chunk_index ASC").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("list docs chunk neighbors: %w", err)
	}
	return results, nil
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
	return r.hybridSearch(ctx, workspaceID, "", spaceIDs, query, queryEmbedding, defaultChunkEmbeddingModel, limit)
}

// HybridSearchForAgent applies the agent-to-space link inside every candidate
// SQL query so unlinking a source cannot leak it through a stale application list.
func (r *DocsChunkRepository) HybridSearchForAgent(ctx context.Context, workspaceID, agentID string, spaceIDs []string, query, queryEmbedding, embeddingModel string, limit int) ([]DocsChunkSearchResult, error) {
	return r.hybridSearch(ctx, workspaceID, agentID, spaceIDs, query, queryEmbedding, embeddingModel, limit)
}

// HybridSearchKnowledgeSources performs hybrid retrieval constrained by an
// agent's selected space, collection, and article knowledge-source scopes.
func (r *DocsChunkRepository) HybridSearchKnowledgeSources(
	ctx context.Context,
	workspaceID string,
	agentID string,
	sources []model.AgentKnowledgeSource,
	query string,
	queryEmbedding string,
	embeddingModel string,
	limit int,
) ([]DocsChunkSearchResult, error) {
	filter := BuildDocsKnowledgeScopeFilter(sources)
	return r.hybridSearchWithScopeFilter(ctx, workspaceID, agentID, filter, query, queryEmbedding, embeddingModel, limit)
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
	return r.hybridSearch(ctx, workspaceID, "", spaceIDs, query, queryEmbedding, embeddingModel, limit)
}

func (r *DocsChunkRepository) hybridSearch(
	ctx context.Context,
	workspaceID string,
	agentID string,
	spaceIDs []string,
	query string,
	queryEmbedding string,
	embeddingModel string,
	limit int,
) ([]DocsChunkSearchResult, error) {
	return r.hybridSearchWithScopeFilter(ctx, workspaceID, agentID, DocsKnowledgeScopeFilter{FullSpaceIDs: spaceIDs}, query, queryEmbedding, embeddingModel, limit)
}

func (r *DocsChunkRepository) hybridSearchWithScopeFilter(
	ctx context.Context,
	workspaceID string,
	agentID string,
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

	lexical, err := r.lexicalSearch(ctx, workspaceID, agentID, filter, query, max(limit*4, 12))
	if err != nil {
		return nil, err
	}

	vector := []DocsChunkSearchResult{}
	if queryEmbedding != "" && r.db.Dialector.Name() == "postgres" {
		vector, err = r.vectorSearch(ctx, workspaceID, agentID, filter, queryEmbedding, embeddingModel, max(limit*4, 12))
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

func (r *DocsChunkRepository) lexicalSearch(ctx context.Context, workspaceID, agentID string, filter DocsKnowledgeScopeFilter, query string, limit int) ([]DocsChunkSearchResult, error) {
	tsQuery := toTSQuery(query)
	if tsQuery == "" {
		return []DocsChunkSearchResult{}, nil
	}

	if r.db.Dialector.Name() != "postgres" {
		var results []DocsChunkSearchResult
		dbQuery := r.db.WithContext(ctx).
			Table("docs_chunks AS c").
			Select("c.id, c.workspace_id, c.space_id, s.type AS space_type, c.document_id, c.block_id, c.chunk_index, c.section_key, c.heading_path, c.title, c.content").
			Joins("JOIN docs_documents d ON d.id = c.document_id").
			Joins("JOIN docs_spaces s ON s.id = c.space_id").
			Joins("LEFT JOIN docs_helpcenter_articles ha ON ha.document_id = d.id").
			Where("c.workspace_id = ?", workspaceID).
			Where("d.status = ? AND d.deleted_at IS NULL", model.DocStatusPublished).
			Where("(s.type = ? OR (s.type = ? AND ha.public_published_at IS NOT NULL))", model.SpaceTypeInternal, model.SpaceTypeExternalCapable)
		if agentID != "" {
			dbQuery = dbQuery.Where(`EXISTS (
				SELECT 1 FROM agent_knowledge_sources aks
				WHERE aks.space_id = c.space_id
				  AND aks.workspace_id = c.workspace_id
				  AND aks.agent_id = ?
				  AND aks.sync_status <> ?
			)`, agentID, model.KnowledgeSourceSyncDisabled)
		}
		dbQuery = applySQLiteDocsScopeFilter(dbQuery, filter)
		if err := dbQuery.
			Order("c.updated_at DESC").
			Limit(limit).
			Scan(&results).Error; err != nil {
			return nil, err
		}
		for idx := range results {
			results[idx].LexicalScore = 1
		}
		return results, nil
	}

	agentPredicate := ""
	if agentID != "" {
		agentPredicate = `AND EXISTS (
		  SELECT 1 FROM agent_knowledge_sources aks
		  WHERE aks.space_id = c.space_id
		    AND aks.workspace_id = c.workspace_id
		    AND aks.agent_id = ?
		    AND aks.sync_status <> 'disabled'
		)`
	}
	scopePredicate, scopeArgs := postgresDocsScopePredicate(workspaceID, filter)
	sql := fmt.Sprintf(`
		SELECT c.id, c.workspace_id, c.space_id, s.type AS space_type, c.document_id, c.block_id, c.chunk_index, c.section_key, c.heading_path, c.title, c.content,
		       ts_rank(
		         setweight(to_tsvector('english', COALESCE(c.title, '')), 'A') ||
		         setweight(to_tsvector('english', COALESCE(NULLIF(c.search_content, ''), c.content, '')), 'B'),
		         to_tsquery('english', ?)
		       ) AS lexical_score
		FROM docs_chunks c
		JOIN docs_documents d ON d.id = c.document_id
		JOIN docs_spaces s ON s.id = c.space_id
		LEFT JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE c.workspace_id = ?
		  AND %s
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND (s.type = 'internal' OR (s.type = 'external_capable' AND ha.public_published_at IS NOT NULL))
		  %s
		  AND (
		    setweight(to_tsvector('english', COALESCE(c.title, '')), 'A') ||
		    setweight(to_tsvector('english', COALESCE(NULLIF(c.search_content, ''), c.content, '')), 'B')
		  ) @@ to_tsquery('english', ?)
		ORDER BY lexical_score DESC, c.updated_at DESC
		LIMIT ?
	`, scopePredicate, agentPredicate)
	var results []DocsChunkSearchResult
	args := []any{tsQuery, workspaceID}
	args = append(args, scopeArgs...)
	if agentID != "" {
		args = append(args, agentID)
	}
	args = append(args, tsQuery, limit)
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("lexical chunk search: %w", err)
	}
	return results, nil
}

func (r *DocsChunkRepository) vectorSearch(ctx context.Context, workspaceID, agentID string, filter DocsKnowledgeScopeFilter, queryEmbedding string, embeddingModel string, limit int) ([]DocsChunkSearchResult, error) {
	if embeddingModel == "" {
		embeddingModel = defaultChunkEmbeddingModel
	}
	agentPredicate := ""
	if agentID != "" {
		agentPredicate = `AND EXISTS (
		  SELECT 1 FROM agent_knowledge_sources aks
		  WHERE aks.space_id = c.space_id
		    AND aks.workspace_id = c.workspace_id
		    AND aks.agent_id = ?
		    AND aks.sync_status <> 'disabled'
		)`
	}
	scopePredicate, scopeArgs := postgresDocsScopePredicate(workspaceID, filter)
	sql := fmt.Sprintf(`
		SELECT c.id, c.workspace_id, c.space_id, s.type AS space_type, c.document_id, c.block_id, c.chunk_index, c.section_key, c.heading_path, c.title, c.content,
		       GREATEST(0, 1 - (c.embedding <=> CAST(? AS vector))) AS vector_score
		FROM docs_chunks c
		JOIN docs_documents d ON d.id = c.document_id
		JOIN docs_spaces s ON s.id = c.space_id
		LEFT JOIN docs_helpcenter_articles ha ON ha.document_id = d.id
		WHERE c.workspace_id = ?
		  AND %s
		  AND d.status = 'published'
		  AND d.deleted_at IS NULL
		  AND (s.type = 'internal' OR (s.type = 'external_capable' AND ha.public_published_at IS NOT NULL))
		  %s
		  AND c.embedding_provider = ?
		  AND c.embedding_model = ?
		  AND c.embedding_version IN ?
		  AND c.embedding_dimensions = ?
		ORDER BY c.embedding <=> CAST(? AS vector) ASC
		LIMIT ?
	`, scopePredicate, agentPredicate)
	var results []DocsChunkSearchResult
	args := []any{queryEmbedding, workspaceID}
	args = append(args, scopeArgs...)
	if agentID != "" {
		args = append(args, agentID)
	}
	args = append(args, defaultChunkEmbeddingProvider, embeddingModel, []string{defaultChunkEmbeddingVersion, legacyChunkEmbeddingVersion}, defaultChunkEmbeddingDimensions, queryEmbedding, limit)
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&results).Error; err != nil {
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
		scopeType := strings.TrimSpace(source.ScopeType)
		if scopeType == "" {
			scopeType = model.KnowledgeSourceScopeSpace
		}
		switch scopeType {
		case model.KnowledgeSourceScopeSpace:
			if source.SpaceID != "" {
				fullSpaces[source.SpaceID] = struct{}{}
			}
		case model.KnowledgeSourceScopeCollection:
			if source.CollectionID != nil && strings.TrimSpace(*source.CollectionID) != "" {
				collections[strings.TrimSpace(*source.CollectionID)] = struct{}{}
			}
		case model.KnowledgeSourceScopeArticle:
			if source.DocumentID != nil && strings.TrimSpace(*source.DocumentID) != "" {
				documents[strings.TrimSpace(*source.DocumentID)] = struct{}{}
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
		conditions = append(conditions, "c.space_id IN ?")
		args = append(args, filter.FullSpaceIDs)
	}
	if len(filter.CollectionIDs) > 0 {
		conditions = append(conditions, "d.collection_id IN ?")
		args = append(args, filter.CollectionIDs)
	}
	if len(filter.DocumentIDs) > 0 {
		conditions = append(conditions, "c.document_id IN ?")
		args = append(args, filter.DocumentIDs)
	}
	if len(conditions) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where("("+strings.Join(conditions, " OR ")+")", args...)
}

func postgresDocsScopePredicate(workspaceID string, filter DocsKnowledgeScopeFilter) (string, []any) {
	conditions := []string{}
	args := []any{}
	if len(filter.FullSpaceIDs) > 0 {
		conditions = append(conditions, "c.space_id IN ?")
		args = append(args, filter.FullSpaceIDs)
	}
	if len(filter.CollectionIDs) > 0 {
		conditions = append(conditions, `d.collection_id IN (
			WITH RECURSIVE scoped_collections AS (
				SELECT id FROM docs_collections WHERE workspace_id = ? AND id IN ?
				UNION ALL
				SELECT child.id
				FROM docs_collections child
				JOIN scoped_collections parent ON child.parent_collection_id = parent.id
				WHERE child.deleted_at IS NULL
			)
			SELECT id FROM scoped_collections
		)`)
		args = append(args, workspaceID, filter.CollectionIDs)
	}
	if len(filter.DocumentIDs) > 0 {
		conditions = append(conditions, "c.document_id IN ?")
		args = append(args, filter.DocumentIDs)
	}
	if len(conditions) == 0 {
		return "1 = 0", nil
	}
	return "(" + strings.Join(conditions, " OR ") + ")", args
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
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
