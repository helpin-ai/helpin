package repository

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// KnowledgeEmbeddingState is the small persisted index signature used to avoid
// regenerating embeddings for an unchanged document or page.
type KnowledgeEmbeddingState struct {
	ChunkIndex  int
	ContentHash string
	SectionKey  string
	HeadingPath string
	BlockID     *string
	Reusable    bool
}

// ListReusableEmbeddingStates reads only metadata, retaining incompatible rows
// so a partial or mixed-version index cannot look like a complete match.
func (r *DocsChunkRepository) ListReusableEmbeddingStates(ctx context.Context, workspaceID, documentID, embeddingModel, version string, dimensions int) ([]KnowledgeEmbeddingState, error) {
	var states []KnowledgeEmbeddingState
	err := r.db.WithContext(ctx).Model(&model.DocsChunk{}).
		Select("chunk_index, content_hash, section_key, heading_path, block_id, (embedding_model = ? AND embedding_version = ? AND embedding_dimensions = ? AND embedding IS NOT NULL AND CAST(embedding AS TEXT) NOT IN ?) AS reusable", embeddingModel, version, dimensions, []string{"", "[]"}).
		Where("workspace_id = ? AND document_id = ?", workspaceID, documentID).
		Order("chunk_index ASC").Scan(&states).Error
	if err != nil {
		return nil, fmt.Errorf("read document embedding states: %w", err)
	}
	return states, nil
}

// ListReusableEmbeddingStates reads only metadata, retaining incompatible rows
// so a partial or mixed-version index cannot look like a complete match.
func (r *SupportContentChunkRepository) ListReusableEmbeddingStates(ctx context.Context, workspaceID, pageID, embeddingModel, version string, dimensions int) ([]KnowledgeEmbeddingState, error) {
	var states []KnowledgeEmbeddingState
	err := r.db.WithContext(ctx).Model(&model.SupportContentChunk{}).
		Select("chunk_index, content_hash, section_key, heading_path, (embedding_model = ? AND embedding_version = ? AND embedding_dimensions = ? AND embedding IS NOT NULL AND CAST(embedding AS TEXT) NOT IN ?) AS reusable", embeddingModel, version, dimensions, []string{"", "[]"}).
		Where("workspace_id = ? AND page_id = ?", workspaceID, pageID).
		Order("chunk_index ASC").Scan(&states).Error
	if err != nil {
		return nil, fmt.Errorf("read page embedding states: %w", err)
	}
	return states, nil
}
