package model

import "time"

const (
	PMStoryTypeFeature = "feature"
	PMStoryTypeBug     = "bug"
	PMStoryTypeChore   = "chore"

	PMStoryPriorityNone   = "none"
	PMStoryPriorityLow    = "low"
	PMStoryPriorityMedium = "medium"
	PMStoryPriorityHigh   = "high"
	PMStoryPriorityUrgent = "urgent"

	PMStorySeverityNone     = "none"
	PMStorySeverityMinor    = "minor"
	PMStorySeverityMajor    = "major"
	PMStorySeverityCritical = "critical"
)

// PMStory represents a single work item.
type PMStory struct {
	ID              string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	DisplayID       int        `json:"display_id" gorm:"not null;index"`
	Name            string     `json:"name" gorm:"not null"`
	Description     *string    `json:"description"`
	StoryType       string     `json:"story_type" gorm:"not null;default:'feature'"`
	WorkflowID      string     `json:"workflow_id" gorm:"type:uuid;not null;index"`
	WorkflowStateID string     `json:"workflow_state_id" gorm:"type:uuid;not null;index"`
	EpicID          *string    `json:"epic_id" gorm:"type:uuid;index"`
	IterationID     *string    `json:"iteration_id" gorm:"type:uuid;index"`
	TeamID          *string    `json:"team_id" gorm:"type:uuid;index"`
	OwnerID         *string    `json:"owner_id" gorm:"type:uuid;index"`
	RequesterID     *string    `json:"requester_id" gorm:"type:uuid"`
	Estimate        *int       `json:"estimate"`
	Priority        string     `json:"priority" gorm:"not null;default:'none'"`
	Severity        string     `json:"severity" gorm:"not null;default:'none'"`
	Deadline        *time.Time `json:"deadline" gorm:"type:date"`
	Position        int        `json:"position" gorm:"not null;default:0"`
	Started         bool       `json:"started" gorm:"not null;default:false"`
	StartedAt       *time.Time `json:"started_at"`
	Completed       bool       `json:"completed" gorm:"not null;default:false"`
	CompletedAt     *time.Time `json:"completed_at"`
	MovedAt         *time.Time `json:"moved_at"`
	Blocked         bool       `json:"blocked" gorm:"not null;default:false"`
	Blocker         *string    `json:"blocker"`
	Archived        bool       `json:"archived" gorm:"not null;default:false"`
	TemplateID      *string    `json:"template_id"`
	ExternalID      *string    `json:"external_id"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMStory) TableName() string { return "pm_stories" }

// PMStoryOwner is the join table for many owners per story.
type PMStoryOwner struct {
	StoryID   string    `json:"story_id" gorm:"type:uuid;primaryKey"`
	UserID    string    `json:"user_id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMStoryOwner) TableName() string { return "pm_story_owners" }

// PMStoryFollower is the join table for story followers.
type PMStoryFollower struct {
	StoryID   string    `json:"story_id" gorm:"type:uuid;primaryKey"`
	UserID    string    `json:"user_id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMStoryFollower) TableName() string { return "pm_story_followers" }

// PMStoryLabel is the join table for story labels.
type PMStoryLabel struct {
	StoryID   string    `json:"story_id" gorm:"type:uuid;primaryKey"`
	LabelID   string    `json:"label_id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMStoryLabel) TableName() string { return "pm_story_labels" }

// PMStoryFilters applies filter options when listing stories.
type PMStoryFilters struct {
	TeamID          *string
	EpicID          *string
	IterationID     *string
	WorkflowID      *string
	WorkflowStateID *string
	StoryType       *string
	OwnerID         *string
	RequesterID     *string
	LabelID         *string
	Priority        *string
	Severity        *string
	Blocked         *string
	UpdatedAfter    *string
	Archived        *bool
}

// PMPagination is common pagination input.
type PMPagination struct {
	Page    int
	PerPage int
}

// CreateStoryRequest is the payload for creating a story.
type CreateStoryRequest struct {
	WorkspaceID     string     `json:"workspace_id"`
	Name            string     `json:"name"`
	Description     *string    `json:"description"`
	StoryType       string     `json:"story_type"`
	WorkflowID      string     `json:"workflow_id"`
	WorkflowStateID string     `json:"workflow_state_id"`
	EpicID          *string    `json:"epic_id"`
	IterationID     *string    `json:"iteration_id"`
	TeamID          *string    `json:"team_id"`
	OwnerID         *string    `json:"owner_id"`
	RequesterID     *string    `json:"requester_id"`
	Estimate        *int       `json:"estimate"`
	Priority        *string    `json:"priority"`
	Severity        *string    `json:"severity"`
	Deadline        *time.Time `json:"deadline"`
	Position        *int       `json:"position"`
	Blocked         *bool      `json:"blocked"`
	Blocker         *string    `json:"blocker"`
	TemplateID      *string    `json:"template_id"`
	ExternalID      *string    `json:"external_id"`
	OwnerIDs        []string   `json:"owner_ids"`
	FollowerIDs     []string   `json:"follower_ids"`
	LabelIDs        []string   `json:"label_ids"`
}

// UpdateStoryRequest is the payload for updating a story.
type UpdateStoryRequest struct {
	Name            *string    `json:"name"`
	Description     *string    `json:"description"`
	StoryType       *string    `json:"story_type"`
	WorkflowID      *string    `json:"workflow_id"`
	WorkflowStateID *string    `json:"workflow_state_id"`
	EpicID          *string    `json:"epic_id"`
	IterationID     *string    `json:"iteration_id"`
	TeamID          *string    `json:"team_id"`
	OwnerID         *string    `json:"owner_id"`
	RequesterID     *string    `json:"requester_id"`
	Estimate        *int       `json:"estimate"`
	Priority        *string    `json:"priority"`
	Severity        *string    `json:"severity"`
	Deadline        *time.Time `json:"deadline"`
	Position        *int       `json:"position"`
	Blocked         *bool      `json:"blocked"`
	Blocker         *string    `json:"blocker"`
	Archived        *bool      `json:"archived"`
	TemplateID      *string    `json:"template_id"`
	ExternalID      *string    `json:"external_id"`
	OwnerIDs        []string   `json:"owner_ids"`
	FollowerIDs     []string   `json:"follower_ids"`
	LabelIDs        []string   `json:"label_ids"`
}

// MoveStoryRequest moves a story to a new state and optionally position.
type MoveStoryRequest struct {
	StateID  string `json:"state_id"`
	Position *int   `json:"position"`
}

// ReorderStoryRequest reorders a story in its current state.
type ReorderStoryRequest struct {
	Position int `json:"position"`
}

// StoryUserLinkRequest links a user to a story as owner/follower.
type StoryUserLinkRequest struct {
	UserID string `json:"user_id"`
}

// StoryLabelLinkRequest links a label to a story.
type StoryLabelLinkRequest struct {
	LabelID string `json:"label_id"`
}

// StoryDetail is a story enriched with relation data.
type StoryDetail struct {
	Story         PMStory          `json:"story"`
	Owners        []User           `json:"owners"`
	Followers     []User           `json:"followers"`
	Labels        []PMLabel        `json:"labels"`
	EpicName      *string          `json:"epic_name"`
	IterationName *string          `json:"iteration_name"`
	State         *PMWorkflowState `json:"state"`
}

// BoardStory is a story enriched with relation names for board display.
type BoardStory struct {
	PMStory
	EpicName  *string `json:"epic_name,omitempty"`
	OwnerName *string `json:"owner_name,omitempty"`
}

// StoryStateColumn is the data shape used for board columns.
type StoryStateColumn struct {
	State      PMWorkflowState `json:"state"`
	Stories    []BoardStory    `json:"stories"`
	StoryCount int             `json:"story_count"`
	PointTotal int             `json:"point_total"`
}

// StoryStateCount stores aggregate count per state.
type StoryStateCount struct {
	StateID    string `json:"state_id"`
	StateName  string `json:"state_name"`
	StateType  string `json:"state_type"`
	StoryCount int    `json:"story_count"`
}
