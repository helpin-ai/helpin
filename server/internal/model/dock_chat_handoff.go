package model

import "time"

// DockChatHandoff is derived task state, not an approval, billing checkpoint,
// or source of product permissions. Only the current revision is retained.
// It is deliberately not embedded in an AgentRun.OutputSummary.
type DockChatHandoff struct {
	WorkspaceID      string `gorm:"type:uuid;primaryKey"`
	DockChatID       string `gorm:"type:uuid;primaryKey"`
	FormatVersion    int    `gorm:"not null"`
	Revision         int64  `gorm:"not null"`
	PreviousRevision int64  `gorm:"not null"`
	CoveredSequence  int64  `gorm:"not null"`
	// AccessScope is an opaque host-owned authorization snapshot identifier.
	// Matching it alone is NOT a substitute for authorizing the source records.
	AccessScope    string   `gorm:"type:text;not null"`
	Payload        JSONBlob `gorm:"type:jsonb"`
	Generator      string   `gorm:"type:text"`
	PromptVersion  string   `gorm:"type:text"`
	LeaseToken     string   `gorm:"type:text;not null"`
	LeaseExpiresAt *time.Time
	FailureCode    string `gorm:"type:text"`
	UpdatedAt      time.Time
	Chat           *DockChat `gorm:"foreignKey:DockChatID;references:ID;constraint:OnDelete:CASCADE"`
}

func (DockChatHandoff) TableName() string { return "dock_chat_handoffs" }

// HandoffClaim keeps model-authored narrative separate from record-derived
// identifiers. Assumptions must be explicit; summaries cannot assert actions.
type HandoffClaim struct {
	Text       string   `json:"text"`
	SourceIDs  []string `json:"source_ids,omitempty"`
	Assumption bool     `json:"assumption,omitempty"`
}

type DockChatHandoffContent struct {
	Objective   []HandoffClaim `json:"objective"`
	Constraints []HandoffClaim `json:"constraints"`
	Corrections []HandoffClaim `json:"corrections"`
	Decisions   []HandoffClaim `json:"decisions"`
	PendingWork []HandoffClaim `json:"pending_work"`
	Progress    []HandoffClaim `json:"progress"`
}
