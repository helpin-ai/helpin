package model

import "time"

const (
	KnowledgeSourceSyncQueued   = "queued"
	KnowledgeSourceSyncRunning  = "running"
	KnowledgeSourceSyncReady    = "ready"
	KnowledgeSourceSyncFailed   = "failed"
	KnowledgeSourceSyncStale    = "stale"
	KnowledgeSourceSyncDisabled = "disabled"

	KnowledgeSourceScopeSpace      = "space"
	KnowledgeSourceScopeCollection = "collection"
	KnowledgeSourceScopeArticle    = "article"
)

// AgentKnowledgeSource links a support agent to scoped docs knowledge for RAG retrieval.
type AgentKnowledgeSource struct {
	ID                  string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AgentID             string     `json:"agent_id" gorm:"type:uuid;not null;index"`
	ScopeType           string     `json:"scope_type" gorm:"not null;default:'space';index"`
	SpaceID             string     `json:"space_id" gorm:"type:uuid;not null;index"`
	CollectionID        *string    `json:"collection_id,omitempty" gorm:"type:uuid;index"`
	DocumentID          *string    `json:"document_id,omitempty" gorm:"type:uuid;index"`
	WorkspaceID         string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	SyncStatus          string     `json:"sync_status" gorm:"not null;default:'queued';index"`
	SyncProgress        int        `json:"sync_progress" gorm:"not null;default:0"`
	IndexedDocuments    int        `json:"indexed_documents" gorm:"not null;default:0"`
	IndexedChunks       int        `json:"indexed_chunks" gorm:"not null;default:0"`
	LastSyncError       *string    `json:"last_sync_error"`
	LastSyncStartedAt   *time.Time `json:"last_sync_started_at" gorm:"type:timestamptz"`
	LastSyncCompletedAt *time.Time `json:"last_sync_completed_at" gorm:"type:timestamptz"`
	CreatedAt           time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (AgentKnowledgeSource) TableName() string { return "agent_knowledge_sources" }

// IndexedKnowledgeDocument identifies an article present in the knowledge index.
type IndexedKnowledgeDocument struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// KnowledgeSourceScopeRequest identifies a scoped docs source for an agent.
type KnowledgeSourceScopeRequest struct {
	ScopeType    string  `json:"scope_type"`
	SpaceID      string  `json:"space_id"`
	CollectionID *string `json:"collection_id,omitempty"`
	DocumentID   *string `json:"document_id,omitempty"`
}

// UpdateKnowledgeSourcesRequest is the payload for replacing an agent's knowledge sources.
type UpdateKnowledgeSourcesRequest struct {
	SpaceIDs []string                      `json:"space_ids"`
	Sources  []KnowledgeSourceScopeRequest `json:"sources"`
}
