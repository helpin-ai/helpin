package model

import (
	"time"

	"github.com/helpin-ai/helpin/server/internal/pmtriage"
)

// PMTriageAssessment records a decision attempt without source text. SourceHash
// binds the source revision; ContextHash also binds candidates, taxonomy and mode.
// SQL owns this table so its constraints apply before the API starts.
type PMTriageAssessment struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null"`
	ActorID     string    `json:"actor_id" gorm:"type:uuid;not null"`
	SourceKind  string    `json:"source_kind" gorm:"not null"`
	SourceID    string    `json:"source_id" gorm:"type:uuid;not null"`
	SourceHash  string    `json:"source_hash" gorm:"not null"`
	ContextHash string    `json:"context_hash" gorm:"not null"`
	Mode        string    `json:"mode" gorm:"not null"`
	Status      string    `json:"status" gorm:"not null"`
	Outcome     JSONB     `json:"outcome" gorm:"type:jsonb;not null"`
	Reviewed    JSONB     `json:"reviewed" gorm:"type:jsonb;not null"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName returns the PM decision audit table.
func (PMTriageAssessment) TableName() string { return "pm_triage_assessments" }

// PMTriageLabelSuppression preserves a person's explicit removal of a label
// across future automatic triage attempts.
type PMTriageLabelSuppression struct {
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;primaryKey"`
	TaskID      string    `json:"task_id" gorm:"type:uuid;primaryKey"`
	LabelID     string    `json:"label_id" gorm:"type:uuid;primaryKey"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName returns the persistent automatic label exclusion table.
func (PMTriageLabelSuppression) TableName() string { return "pm_triage_label_suppressions" }

// PMTriageView returns a decision together with freshly authorized display data.
type PMTriageView struct {
	Status     string                  `json:"status"`
	ID         string                  `json:"id,omitempty"`
	SourceKind string                  `json:"source_kind"`
	SourceID   string                  `json:"source_id"`
	SourceHash string                  `json:"source_hash,omitempty"`
	Assessment *pmtriage.Assessment    `json:"assessment,omitempty"`
	Teams      []pmtriage.Option       `json:"teams"`
	Labels     []pmtriage.Option       `json:"labels"`
	Candidates []PMTriageCandidateView `json:"candidates"`
	Reviewed   JSONB                   `json:"reviewed,omitempty"`
}

// PMTriageCandidateView exposes only candidate navigation metadata, not bodies.
type PMTriageCandidateView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	DisplayID int    `json:"display_id"`
}
