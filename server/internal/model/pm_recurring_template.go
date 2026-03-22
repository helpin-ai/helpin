package model

import (
	"encoding/json"
	"time"
)

const (
	PMRecurringTemplateStatusActive  = "active"
	PMRecurringTemplateStatusPaused  = "paused"
	PMRecurringTemplateStatusStopped = "stopped"
	PMRecurringTemplateStatusFailed  = "failed"

	PMRecurringScheduleTypeTime       = "time"
	PMRecurringScheduleTypeCompletion = "completion"

	PMRecurringFrequencyDaily   = "daily"
	PMRecurringFrequencyWeekly  = "weekly"
	PMRecurringFrequencyMonthly = "monthly"
	PMRecurringFrequencyYearly  = "yearly"

	PMRecurringCompletionEventCompleted = "completed"
	PMRecurringCompletionEventDoneState = "done_state"

	PMRecurringDueDateModeNone         = "none"
	PMRecurringDueDateModeScheduled    = "scheduled_date"
	PMRecurringDueDateModeOffsetDays   = "offset_days"

	PMRecurringSprintAssignmentNone     = "none"
	PMRecurringSprintAssignmentCurrent  = "current_sprint"
	PMRecurringSprintAssignmentDueDate  = "by_due_date"

	PMRecurringRunStatusSucceeded = "succeeded"
	PMRecurringRunStatusFailed    = "failed"
	PMRecurringRunStatusSkipped   = "skipped"

	PMRecurringRunTriggerManualSeed = "manual_seed"
	PMRecurringRunTriggerSchedule   = "schedule"
	PMRecurringRunTriggerCompletion = "completion"
	PMRecurringRunTriggerManualNow  = "generate_now"
)

// PMRecurringTemplate stores a recurring work template that generates normal stories.
type PMRecurringTemplate struct {
	ID                   string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID          string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	TeamID               *string         `json:"team_id" gorm:"type:uuid;index"`
	Title                string          `json:"title" gorm:"not null"`
	Description          *string         `json:"description"`
	Status               string          `json:"status" gorm:"not null;default:'active';index"`
	OwnerMemberID        *string         `json:"owner_member_id" gorm:"type:uuid;index"`
	CreatedFromStoryID   *string         `json:"created_from_story_id" gorm:"type:uuid;index"`
	SeedPayload          json.RawMessage `json:"seed_payload" gorm:"type:jsonb;not null;default:'{}'"`
	Config               json.RawMessage `json:"config" gorm:"type:jsonb;not null;default:'{}'"`
	StartDate            *time.Time      `json:"start_date" gorm:"type:date"`
	EndDate              *time.Time      `json:"end_date" gorm:"type:date"`
	EndsAfterOccurrences *int            `json:"ends_after_occurrences"`
	NextRunAt            *time.Time      `json:"next_run_at" gorm:"index"`
	LastRunAt            *time.Time      `json:"last_run_at"`
	LastGeneratedStoryID *string         `json:"last_generated_story_id" gorm:"type:uuid;index"`
	LastError            *string         `json:"last_error"`
	FailureCount         int             `json:"failure_count" gorm:"not null;default:0"`
	GeneratedCount       int             `json:"generated_count" gorm:"not null;default:0"`
	SkipNextRun          bool            `json:"skip_next_run" gorm:"not null;default:false"`
	CreatedByID          *string         `json:"created_by_id" gorm:"type:uuid"`
	UpdatedByID          *string         `json:"updated_by_id" gorm:"type:uuid"`
	CreatedAt            time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMRecurringTemplate) TableName() string { return "pm_recurring_templates" }

// PMRecurringRun captures every recurring execution attempt.
type PMRecurringRun struct {
	ID               string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	TemplateID       string     `json:"template_id" gorm:"type:uuid;not null;index"`
	OccurrenceNumber int        `json:"occurrence_number" gorm:"not null"`
	TriggerType      string     `json:"trigger_type" gorm:"not null"`
	ScheduledFor     *time.Time `json:"scheduled_for"`
	StartedAt        *time.Time `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at"`
	Status           string     `json:"status" gorm:"not null;index"`
	GeneratedStoryID *string    `json:"generated_story_id" gorm:"type:uuid;index"`
	DedupeKey        string     `json:"dedupe_key" gorm:"not null;uniqueIndex"`
	ErrorMessage     *string    `json:"error_message"`
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMRecurringRun) TableName() string { return "pm_recurring_runs" }

// PMRecurringTemplateConfig is the structured schedule and generation config.
type PMRecurringTemplateConfig struct {
	ScheduleType         string     `json:"schedule_type"`
	Frequency            string     `json:"frequency,omitempty"`
	Interval             int        `json:"interval,omitempty"`
	Weekdays             []int      `json:"weekdays,omitempty"`
	DayOfMonth           *int       `json:"day_of_month,omitempty"`
	CompletionEvent      string     `json:"completion_event,omitempty"`
	CompletionStateIDs   []string   `json:"completion_state_ids,omitempty"`
	DueDateMode          string     `json:"due_date_mode,omitempty"`
	DueOffsetDays        *int       `json:"due_offset_days,omitempty"`
	StartsOn             *time.Time `json:"starts_on,omitempty"`
	EndsOn               *time.Time `json:"ends_on,omitempty"`
	EndsAfterOccurrences *int       `json:"ends_after_occurrences,omitempty"`
	SprintAssignmentMode string     `json:"sprint_assignment_mode,omitempty"`
}

// PMRecurringStorySeed stores the story defaults used for generation.
type PMRecurringStorySeed struct {
	Name              string                      `json:"name"`
	Description       *string                     `json:"description,omitempty"`
	StoryType         string                      `json:"story_type,omitempty"`
	WorkflowID        string                      `json:"workflow_id"`
	WorkflowStateID   string                      `json:"workflow_state_id"`
	EpicID            *string                     `json:"epic_id,omitempty"`
	TeamID            *string                     `json:"team_id,omitempty"`
	OwnerMemberID     *string                     `json:"owner_member_id,omitempty"`
	RequesterMemberID *string                     `json:"requester_member_id,omitempty"`
	Estimate          *int                        `json:"estimate,omitempty"`
	Priority          *string                     `json:"priority,omitempty"`
	Severity          *string                     `json:"severity,omitempty"`
	OwnerIDs          []string                    `json:"owner_ids,omitempty"`
	FollowerIDs       []string                    `json:"follower_ids,omitempty"`
	LabelIDs          []string                    `json:"label_ids,omitempty"`
	ChecklistItems    []CreateChecklistItemRequest `json:"checklist_items,omitempty"`
	ExternalLinks     []CreateExternalLinkRequest `json:"external_links,omitempty"`
}

type CreateRecurringTemplateRequest struct {
	WorkspaceID string                   `json:"workspace_id"`
	Title       string                   `json:"title"`
	Description *string                  `json:"description,omitempty"`
	StoryID     string                   `json:"story_id"`
	Config      PMRecurringTemplateConfig `json:"config"`
}

type UpdateRecurringTemplateRequest struct {
	Title       *string                    `json:"title,omitempty"`
	Description *string                    `json:"description,omitempty"`
	StoryID     *string                    `json:"story_id,omitempty"`
	Config      *PMRecurringTemplateConfig `json:"config,omitempty"`
}

type RecurringTemplateActionResponse struct {
	Template RecurringTemplateDetail `json:"template"`
}

type RecurringTemplateDetail struct {
	Template           PMRecurringTemplate      `json:"template"`
	Config             PMRecurringTemplateConfig `json:"config"`
	Seed               PMRecurringStorySeed      `json:"seed"`
	RuleSummary        string                    `json:"rule_summary"`
	LastGeneratedStory *PMStory                  `json:"last_generated_story,omitempty"`
	Runs               []PMRecurringRun          `json:"runs,omitempty"`
}

type StoryRecurringSummary struct {
	TemplateID         string                    `json:"template_id"`
	TemplateTitle      string                    `json:"template_title"`
	Status             string                    `json:"status"`
	OccurrenceNumber   int                       `json:"occurrence_number"`
	GeneratedCount     int                       `json:"generated_count"`
	RuleSummary        string                    `json:"rule_summary"`
	NextRunAt          *time.Time                `json:"next_run_at,omitempty"`
	LastError          *string                   `json:"last_error,omitempty"`
	LastGeneratedStory *PMStory                  `json:"last_generated_story,omitempty"`
	Config             PMRecurringTemplateConfig `json:"config"`
}
