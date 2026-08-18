package model

import (
	"encoding/json"
	"time"
)

const (
	TaskUpdateKindComment  = "comment"
	TaskUpdateKindChange   = "change"
	TaskUpdateKindAgentRun = "agent_run"
	TaskUpdateKindGit      = "git"

	TaskUpdateFilterAll        = "all"
	TaskUpdateFilterDiscussion = "discussion"
	TaskUpdateFilterChanges    = "changes"

	TaskStandingBriefPending = "pending_refresh"
	TaskStandingBriefReady   = "ready"
	TaskStandingBriefStale   = "stale"
	TaskStandingBriefError   = "error"
)

// PMTaskUpdateRead stores a per-user high-water mark for a task's unified updates feed.
type PMTaskUpdateRead struct {
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;primaryKey"`
	TaskID      string    `json:"task_id" gorm:"type:uuid;primaryKey"`
	UserID      string    `json:"user_id" gorm:"type:uuid;primaryKey"`
	SeenThrough time.Time `json:"seen_through" gorm:"not null;index"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMTaskUpdateRead) TableName() string { return "pm_task_update_reads" }

// PMTaskStandingBrief is the durable, system-owned "Where this stands" artifact.
type PMTaskStandingBrief struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_pm_task_standing_brief,priority:1"`
	TaskID          string          `json:"task_id" gorm:"type:uuid;not null;uniqueIndex:idx_pm_task_standing_brief,priority:2"`
	Narrative       string          `json:"narrative" gorm:"not null;default:''"`
	Suggestions     json.RawMessage `json:"suggestions" gorm:"type:jsonb;not null;default:'[]'"`
	Evidence        json.RawMessage `json:"evidence" gorm:"type:jsonb;not null;default:'[]'"`
	Status          string          `json:"status" gorm:"not null;default:'pending_refresh';index"`
	SourceUpdatedAt *time.Time      `json:"source_updated_at"`
	ComputedAt      *time.Time      `json:"computed_at"`
	LastTriggeredAt *time.Time      `json:"last_triggered_at"`
	LastError       *string         `json:"last_error,omitempty"`
	Version         string          `json:"version" gorm:"not null;default:'v1'"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMTaskStandingBrief) TableName() string { return "pm_task_standing_briefs" }

// PMTaskBriefSuggestionDismissal suppresses a stable suggestion for the whole task.
type PMTaskBriefSuggestionDismissal struct {
	WorkspaceID   string    `json:"workspace_id" gorm:"type:uuid;primaryKey"`
	TaskID        string    `json:"task_id" gorm:"type:uuid;primaryKey"`
	SuggestionKey string    `json:"suggestion_key" gorm:"primaryKey"`
	DismissedBy   string    `json:"dismissed_by" gorm:"type:uuid;not null"`
	DismissedAt   time.Time `json:"dismissed_at" gorm:"autoCreateTime"`
}

func (PMTaskBriefSuggestionDismissal) TableName() string {
	return "pm_task_brief_suggestion_dismissals"
}

type TaskUpdateEntry struct {
	ID         string             `json:"id"`
	Kind       string             `json:"kind"`
	OccurredAt time.Time          `json:"occurred_at"`
	Actor      *User              `json:"actor,omitempty"`
	Comment    *CommentWithAuthor `json:"comment,omitempty"`
	Activity   *PMActivityLog     `json:"activity,omitempty"`
	AgentRun   *AgentRun          `json:"agent_run,omitempty"`
	AgentName  string             `json:"agent_name,omitempty"`
	GitLink    *TaskGitLink       `json:"git_link,omitempty"`
}

type TaskUpdatesResponse struct {
	Data            []TaskUpdateEntry `json:"data"`
	NextCursor      string            `json:"next_cursor,omitempty"`
	HighWater       time.Time         `json:"high_water"`
	UnreadCount     int               `json:"unread_count"`
	ReadInitialized bool              `json:"read_initialized"`
}

type UpdateTaskReadStateRequest struct {
	InitializeOnly bool       `json:"initialize_only"`
	SeenThrough    *time.Time `json:"seen_through,omitempty"`
}

type TaskStandingBriefSuggestionAction struct {
	Type      string `json:"type"`
	TargetID  string `json:"target_id,omitempty"`
	RunID     string `json:"run_id,omitempty"`
	CommentID string `json:"comment_id,omitempty"`
	Text      string `json:"text,omitempty"`
}

type TaskStandingBriefSuggestion struct {
	Key      string                            `json:"key"`
	Label    string                            `json:"label"`
	Action   TaskStandingBriefSuggestionAction `json:"action"`
	Evidence []string                          `json:"evidence,omitempty"`
}

type TaskStandingBriefEvidence struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Label     string `json:"label"`
	Timestamp string `json:"timestamp,omitempty"`
}

type TaskStandingBriefResponse struct {
	Narrative       string                        `json:"narrative"`
	Suggestions     []TaskStandingBriefSuggestion `json:"suggestions"`
	Evidence        []TaskStandingBriefEvidence   `json:"evidence"`
	Status          string                        `json:"status"`
	IsStale         bool                          `json:"is_stale"`
	ComputedAt      *time.Time                    `json:"computed_at,omitempty"`
	SourceUpdatedAt *time.Time                    `json:"source_updated_at,omitempty"`
}
