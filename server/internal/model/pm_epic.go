package model

import (
	"encoding/json"
	"time"
)

const (
	PMEpicHealthNone     = "no_health"
	PMEpicHealthOnTrack  = "on_track"
	PMEpicHealthAtRisk   = "at_risk"
	PMEpicHealthOffTrack = "off_track"
)

// PMEpic represents an epic.
type PMEpic struct {
	ID                    string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID           string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name                  string          `json:"name" gorm:"not null"`
	Description           *string         `json:"description"`
	ExternalID            *string         `json:"external_id" gorm:"index"`
	EpicStateID           *string         `json:"epic_state_id" gorm:"type:uuid;index"`
	OwnerID               *string         `json:"owner_id" gorm:"type:uuid;index"`
	OwnerMemberID         *string         `json:"owner_member_id" gorm:"type:uuid;index"`
	TeamID                *string         `json:"team_id" gorm:"type:uuid;index"`
	PlannedStartDate      *time.Time      `json:"planned_start_date" gorm:"type:date"`
	Deadline              *time.Time      `json:"deadline" gorm:"type:date"`
	Started               bool            `json:"started" gorm:"not null;default:false"`
	StartedAt             *time.Time      `json:"started_at"`
	Completed             bool            `json:"completed" gorm:"not null;default:false"`
	CompletedAt           *time.Time      `json:"completed_at"`
	Position              int             `json:"position" gorm:"not null;default:0"`
	Color                 *string         `json:"color"`
	Health                string          `json:"health" gorm:"not null;default:'no_health'"`
	HealthComment         *string         `json:"health_comment"`
	Archived              bool            `json:"archived" gorm:"not null;default:false"`
	AssignedAgentID       *string         `json:"assigned_agent_id" gorm:"type:uuid;index"`
	SpecDocumentID        *string         `json:"spec_document_id" gorm:"type:uuid;index"`
	PlanningRepositoryID  *string         `json:"planning_repository_id" gorm:"type:uuid;index"`
	PlanningState         string          `json:"planning_state" gorm:"not null;default:'not_started'"`
	SpecClarifications    json.RawMessage `json:"spec_clarifications" gorm:"type:jsonb;not null;default:'[]'"`
	SpecClarifiedAt       *time.Time      `json:"spec_clarified_at"`
	SpecClarifiedBy       *string         `json:"spec_clarified_by" gorm:"type:uuid"`
	ApprovedSpecVersionID *string         `json:"approved_spec_version_id" gorm:"type:uuid;index"`
	LastPlanningRunID     *string         `json:"last_planning_run_id" gorm:"type:uuid;index"`
	CreatedBy             *string         `json:"created_by" gorm:"type:uuid"`
	CreatedAt             time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMEpic) TableName() string { return "pm_epics" }

// PMEpicObjective is the many-to-many join between epics and objectives.
type PMEpicObjective struct {
	EpicID      string    `json:"epic_id" gorm:"type:uuid;primaryKey"`
	ObjectiveID string    `json:"objective_id" gorm:"type:uuid;primaryKey"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMEpicObjective) TableName() string { return "pm_epic_objectives" }

// PMEpicLabel is the many-to-many join between epics and labels.
type PMEpicLabel struct {
	EpicID    string    `json:"epic_id" gorm:"type:uuid;primaryKey"`
	LabelID   string    `json:"label_id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMEpicLabel) TableName() string { return "pm_epic_labels" }

// PMEpicListFilters applies filters when listing epics.
type PMEpicListFilters struct {
	Search   *string
	TeamID   *string
	StateID  *string
	LabelID  *string
	Archived *bool
	// AccessibleTeamIDs enforces team-based access boundaries.
	// nil = no filtering (admin/owner), [] = no access, [ids] = filter to these teams.
	AccessibleTeamIDs []string
}

// CreateEpicRequest is the payload for creating an epic.
type CreateEpicRequest struct {
	WorkspaceID          string     `json:"workspace_id"`
	Name                 string     `json:"name"`
	Description          *string    `json:"description"`
	EpicStateID          *string    `json:"epic_state_id"`
	OwnerID              *string    `json:"owner_id"`
	OwnerMemberID        *string    `json:"owner_member_id"`
	TeamID               *string    `json:"team_id"`
	PlannedStartDate     *time.Time `json:"planned_start_date"`
	Deadline             *time.Time `json:"deadline"`
	Position             *int       `json:"position"`
	Color                *string    `json:"color"`
	Health               *string    `json:"health"`
	HealthComment        *string    `json:"health_comment"`
	LabelIDs             []string   `json:"label_ids"`
	AttachmentIDs        []string   `json:"attachment_ids,omitempty"`
	PlanningRepositoryID *string    `json:"planning_repository_id"`
	AssignedAgentID      *string    `json:"assigned_agent_id"`
	RunOnCreate          bool       `json:"run_on_create"`
}

// UpdateEpicRequest is the payload for updating an epic.
type UpdateEpicRequest struct {
	Name                 *string    `json:"name"`
	Description          *string    `json:"description"`
	EpicStateID          *string    `json:"epic_state_id"`
	OwnerID              *string    `json:"owner_id"`
	OwnerMemberID        *string    `json:"owner_member_id"`
	TeamID               *string    `json:"team_id"`
	PlannedStartDate     *time.Time `json:"planned_start_date"`
	Deadline             *time.Time `json:"deadline"`
	Position             *int       `json:"position"`
	Color                *string    `json:"color"`
	Archived             *bool      `json:"archived"`
	Health               *string    `json:"health"`
	HealthComment        *string    `json:"health_comment"`
	LabelIDs             []string   `json:"label_ids"`
	PlanningRepositoryID *string    `json:"planning_repository_id"`
	AssignedAgentID      *string    `json:"assigned_agent_id"`
	// Presence flags are used by non-HTTP callers that must distinguish an
	// omitted nullable field from an explicitly requested clear. Existing API
	// callers retain the pointer-based behavior above.
	EpicStateIDSet          bool `json:"-"`
	OwnerSet                bool `json:"-"`
	TeamIDSet               bool `json:"-"`
	PlannedStartDateSet     bool `json:"-"`
	DeadlineSet             bool `json:"-"`
	PlanningRepositoryIDSet bool `json:"-"`
}

// UpdateEpicHealthRequest updates epic health fields.
type UpdateEpicHealthRequest struct {
	Health  string  `json:"health"`
	Comment *string `json:"comment"`
}

// PMEpicStats contains derived epic progress metrics.
type PMEpicStats struct {
	TaskCount       int `json:"task_count"`
	DoneTaskCount   int `json:"done_task_count"`
	TotalPoints     int `json:"total_points"`
	DonePoints      int `json:"done_points"`
	InProgressCount int `json:"in_progress_count"`
	UnstartedCount  int `json:"unstarted_count"`
}

// EpicWithStats is an epic with computed progress metrics.
type EpicWithStats struct {
	Epic            PMEpic                `json:"epic"`
	Labels          []PMLabel             `json:"labels"`
	Objectives      []RoadmapObjectiveRef `json:"objectives"`
	Stats           PMEpicStats           `json:"stats"`
	SuggestedHealth string                `json:"suggested_health"`
}

// CreateEpicResponse returns the created epic and any best-effort run result.
type CreateEpicResponse struct {
	Epic          EpicWithStats `json:"epic"`
	AgentRun      *AgentRun     `json:"agent_run"`
	AgentRunError *string       `json:"agent_run_error,omitempty"`
}
