package model

import (
	"encoding/json"
	"time"
)

// DocsChunk stores chunked help-center content and its vector embedding.
type DocsChunk struct {
	ID          string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_docs_chunk_ws_space_doc,priority:1"`
	SpaceID     string          `json:"space_id" gorm:"type:uuid;not null;index:idx_docs_chunk_ws_space_doc,priority:2;index"`
	DocumentID  string          `json:"document_id" gorm:"type:uuid;not null;index:idx_docs_chunk_ws_space_doc,priority:3;uniqueIndex:idx_docs_chunk_doc_order,priority:1"`
	BlockID     *string         `json:"block_id,omitempty" gorm:"type:uuid;index"`
	ChunkIndex  int             `json:"chunk_index" gorm:"not null;uniqueIndex:idx_docs_chunk_doc_order,priority:2"`
	BlockRange  json.RawMessage `json:"block_range,omitempty" gorm:"type:jsonb"`
	Title       string          `json:"title" gorm:"not null"`
	Content     string          `json:"content" gorm:"type:text;not null"`
	ContentHash string          `json:"content_hash" gorm:"size:64;not null;index"`
	Embedding   string          `json:"-" gorm:"type:vector(1536);not null"`
	CreatedAt   time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DocsChunk) TableName() string { return "docs_chunks" }
