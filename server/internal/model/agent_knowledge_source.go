package model

import "time"

const (
	KnowledgeSourceSyncQueued   = "queued"
	KnowledgeSourceSyncRunning  = "running"
	KnowledgeSourceSyncReady    = "ready"
	KnowledgeSourceSyncFailed   = "failed"
	KnowledgeSourceSyncStale    = "stale"
	KnowledgeSourceSyncDisabled = "disabled"
)

// AgentKnowledgeSource links a support agent to a docs space for RAG retrieval.
type AgentKnowledgeSource struct {
	ID                  string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AgentID             string     `json:"agent_id" gorm:"type:uuid;not null;uniqueIndex:idx_aks_agent_space,priority:1"`
	SpaceID             string     `json:"space_id" gorm:"type:uuid;not null;uniqueIndex:idx_aks_agent_space,priority:2"`
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

// UpdateKnowledgeSourcesRequest is the payload for replacing an agent's knowledge sources.
type UpdateKnowledgeSourcesRequest struct {
	SpaceIDs []string `json:"space_ids"`
}
