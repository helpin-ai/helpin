package model

import "time"

const (
	PMObjectiveTypeTactical  = "tactical"
	PMObjectiveTypeStrategic = "strategic"

	PMObjectiveStateNotStarted = "not_started"
	PMObjectiveStateActive     = "active"
	PMObjectiveStateClosed     = "closed"

	PMObjectiveHealthOnTrack  = "on_track"
	PMObjectiveHealthAtRisk   = "at_risk"
	PMObjectiveHealthOffTrack = "off_track"

	PMKeyResultTypeBoolean = "boolean"
	PMKeyResultTypePercent = "percent"
	PMKeyResultTypeNumeric = "numeric"
)

// PMObjective represents an OKR objective.
type PMObjective struct {
	ID               string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name             string     `json:"name" gorm:"not null"`
	Description      *string    `json:"description"`
	ExternalID       *string    `json:"external_id" gorm:"index"`
	ObjectiveType    string     `json:"objective_type" gorm:"not null;default:'tactical'"`
	State            string     `json:"state" gorm:"not null;default:'not_started'"`
	PlannedStartDate *time.Time `json:"planned_start_date" gorm:"type:date"`
	Deadline         *time.Time `json:"deadline" gorm:"type:date"`
	Health           string     `json:"health" gorm:"not null;default:'on_track'"`
	HealthComment    *string    `json:"health_comment"`
	Position         int        `json:"position" gorm:"not null;default:0"`
	Archived         bool       `json:"archived" gorm:"not null;default:false"`
	CreatedBy        *string    `json:"created_by" gorm:"type:uuid"`
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMObjective) TableName() string { return "pm_objectives" }

// PMKeyResult represents a key result belonging to an objective.
type PMKeyResult struct {
	ID            string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ObjectiveID   string     `json:"objective_id" gorm:"type:uuid;not null;index"`
	Name          string     `json:"name" gorm:"not null"`
	ResultType    string     `json:"result_type" gorm:"not null;default:'boolean'"`
	InitialValue  float64    `json:"initial_value" gorm:"not null;default:0"`
	CurrentValue  float64    `json:"current_value" gorm:"not null;default:0"`
	TargetValue   float64    `json:"target_value" gorm:"not null;default:100"`
	Progress      float64    `json:"progress" gorm:"not null;default:0"`
	Note          *string    `json:"note"`
	NoteUpdatedBy *string    `json:"note_updated_by" gorm:"type:uuid"`
	NoteUpdatedAt *time.Time `json:"note_updated_at"`
	Position      int        `json:"position" gorm:"not null;default:0"`
	UpdatedBy     *string    `json:"updated_by" gorm:"type:uuid"`
	CreatedAt     time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMKeyResult) TableName() string { return "pm_key_results" }

// PMObjectiveTeam links an objective to a team.
type PMObjectiveTeam struct {
	ObjectiveID string    `json:"objective_id" gorm:"type:uuid;primaryKey"`
	TeamID      string    `json:"team_id" gorm:"type:uuid;primaryKey"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMObjectiveTeam) TableName() string { return "pm_objective_teams" }

// PMObjectiveOwner links an objective to a user/person owner.
type PMObjectiveOwner struct {
	ObjectiveID string    `json:"objective_id" gorm:"type:uuid;primaryKey"`
	UserID      string    `json:"user_id" gorm:"type:uuid;primaryKey"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMObjectiveOwner) TableName() string { return "pm_objective_owners" }

// PMObjectiveLabel links an objective to a label.
type PMObjectiveLabel struct {
	ObjectiveID string    `json:"objective_id" gorm:"type:uuid;primaryKey"`
	LabelID     string    `json:"label_id" gorm:"type:uuid;primaryKey"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMObjectiveLabel) TableName() string { return "pm_objective_labels" }

// ── Filters & Request DTOs ─────────────────────────────────────────

type PMObjectiveListFilters struct {
	TeamID        *string
	LabelID       *string
	ObjectiveType *string
	State         *string
	Archived      *bool
}

type CreateObjectiveRequest struct {
	WorkspaceID      string     `json:"workspace_id"`
	Name             string     `json:"name"`
	Description      *string    `json:"description"`
	ObjectiveType    string     `json:"objective_type"`
	State            *string    `json:"state"`
	PlannedStartDate *time.Time `json:"planned_start_date"`
	Deadline         *time.Time `json:"deadline"`
	Health           *string    `json:"health"`
	HealthComment    *string    `json:"health_comment"`
	Position         *int       `json:"position"`
	TeamIDs          []string   `json:"team_ids"`
	OwnerIDs         []string   `json:"owner_ids"`
	LabelIDs         []string   `json:"label_ids"`
	EpicIDs          []string   `json:"epic_ids"`
}

type UpdateObjectiveRequest struct {
	Name             *string    `json:"name"`
	Description      *string    `json:"description"`
	ObjectiveType    *string    `json:"objective_type"`
	State            *string    `json:"state"`
	PlannedStartDate *time.Time `json:"planned_start_date"`
	Deadline         *time.Time `json:"deadline"`
	Health           *string    `json:"health"`
	HealthComment    *string    `json:"health_comment"`
	Position         *int       `json:"position"`
	Archived         *bool      `json:"archived"`
	TeamIDs          []string   `json:"team_ids"`
	OwnerIDs         []string   `json:"owner_ids"`
	LabelIDs         []string   `json:"label_ids"`
	EpicIDs          []string   `json:"epic_ids"`
}

type CreateKeyResultRequest struct {
	Name         string  `json:"name"`
	ResultType   string  `json:"result_type"`
	InitialValue float64 `json:"initial_value"`
	CurrentValue float64 `json:"current_value"`
	TargetValue  float64 `json:"target_value"`
	Note         *string `json:"note"`
	Position     *int    `json:"position"`
}

type UpdateKeyResultRequest struct {
	Name         *string  `json:"name"`
	ResultType   *string  `json:"result_type"`
	InitialValue *float64 `json:"initial_value"`
	CurrentValue *float64 `json:"current_value"`
	TargetValue  *float64 `json:"target_value"`
	Note         *string  `json:"note"`
	Position     *int     `json:"position"`
}

// ── Response DTOs ──────────────────────────────────────────────────

type PMObjectiveStats struct {
	KeyResultCount  int     `json:"key_result_count"`
	KeyResultAvgPct float64 `json:"key_result_avg_pct"`
	EpicCount       int     `json:"epic_count"`
	EpicDoneCount   int     `json:"epic_done_count"`
	EpicStoryCount  int     `json:"epic_story_count"`
	EpicDoneStories int     `json:"epic_done_stories"`
	EpicProgressPct float64 `json:"epic_progress_pct"`
}

type ObjectiveWithDetails struct {
	Objective       PMObjective      `json:"objective"`
	Teams           []string         `json:"teams"`
	Owners          []string         `json:"owners"`
	Labels          []PMLabel        `json:"labels"`
	KeyResults      []PMKeyResult    `json:"key_results"`
	Epics           []EpicWithStats  `json:"epics"`
	Stats           PMObjectiveStats `json:"stats"`
	SuggestedHealth string           `json:"suggested_health"`
}
