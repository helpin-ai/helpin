package service

import "github.com/helpin-ai/helpin/server/internal/repository"

// matchingKnowledgeIndex also checks chunk order and navigation metadata. A
// partial index or changed chunk boundaries must be rebuilt, even when some
// individual chunks have matching text.
func matchingKnowledgeIndex(title string, chunks []structuredKnowledgeChunk, states []repository.KnowledgeEmbeddingState) bool {
	if len(chunks) == 0 || len(chunks) != len(states) {
		return false
	}
	for i, chunk := range chunks {
		state := states[i]
		if !state.Reusable || state.ChunkIndex != i || state.ContentHash != hashChunk(title, chunk.SearchContent) ||
			state.SectionKey != chunk.SectionKey || state.HeadingPath != chunk.HeadingPath {
			return false
		}
		if (state.BlockID == nil) != (chunk.BlockID == nil) {
			return false
		}
		if state.BlockID != nil && *state.BlockID != *chunk.BlockID {
			return false
		}
	}
	return true
}

// fileKnowledgeChunks preserves the existing uploaded-file chunk format in
// both initial indexing and later reindexing.
func fileKnowledgeChunks(text string) []structuredKnowledgeChunk {
	raw := chunkDocumentText(text)
	chunks := make([]structuredKnowledgeChunk, len(raw))
	for i, content := range raw {
		chunks[i] = structuredKnowledgeChunk{Content: content, SearchContent: content}
	}
	return chunks
}
