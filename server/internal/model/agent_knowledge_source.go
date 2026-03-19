package model

import "time"

// AgentKnowledgeSource links a support agent to a docs space for RAG retrieval.
type AgentKnowledgeSource struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AgentID     string    `json:"agent_id" gorm:"type:uuid;not null;uniqueIndex:idx_aks_agent_space,priority:1"`
	SpaceID     string    `json:"space_id" gorm:"type:uuid;not null;uniqueIndex:idx_aks_agent_space,priority:2"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (AgentKnowledgeSource) TableName() string { return "agent_knowledge_sources" }

// UpdateKnowledgeSourcesRequest is the payload for replacing an agent's knowledge sources.
type UpdateKnowledgeSourcesRequest struct {
	SpaceIDs []string `json:"space_ids"`
}
