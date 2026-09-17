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
	CandidateHashes map[string]string       `json:"-"`
	Status          string                  `json:"status"`
	ID              string                  `json:"id,omitempty"`
	SourceKind      string                  `json:"source_kind"`
	SourceID        string                  `json:"source_id"`
	SourceHash      string                  `json:"source_hash,omitempty"`
	Assessment      *pmtriage.Assessment    `json:"assessment,omitempty"`
	Teams           []pmtriage.Option       `json:"teams"`
	Labels          []pmtriage.Option       `json:"labels"`
	Candidates      []PMTriageCandidateView `json:"candidates"`
	Reviewed        JSONB                   `json:"reviewed,omitempty"`
}

// PMTriageCandidateView exposes only candidate navigation metadata, not bodies.
type PMTriageCandidateView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	DisplayID int    `json:"display_id"`
}

// PMTriageReviewRequest identifies a displayed suggestion and a human decision.
// Action is task_type, team, or match. Value is the suggested type/team/task ID.
type PMTriageReviewRequest struct {
	AssessmentID string `json:"assessment_id"`
	Action       string `json:"action"`
	Value        string `json:"value"`
	Dismiss      bool   `json:"dismiss"`
}

// PMTriageReviewResult identifies a persisted human decision.
type PMTriageReviewResult struct {
	Key    string `json:"key"`
	Status string `json:"status"`
}

// SupportTaskDraftPreview is editable text generated before reviewed task creation.
type SupportTaskDraftPreview struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TaskType    string `json:"task_type"`
	Priority    string `json:"priority"`
	SourceHash  string `json:"source_hash"`
}

// PMTriageDraftRequest previews unsaved task work without creating a task.
type PMTriageDraftRequest struct {
	DraftID     string  `json:"draft_id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	TeamID      *string `json:"team_id"`
}
