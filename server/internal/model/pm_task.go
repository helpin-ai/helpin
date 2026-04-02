package model

import (
	"encoding/json"
	"time"
)

const (
	PMTaskTypeFeature = "feature"
	PMTaskTypeBug     = "bug"
	PMTaskTypeChore   = "chore"

	PMTaskPriorityNone   = "none"
	PMTaskPriorityLow    = "low"
	PMTaskPriorityMedium = "medium"
	PMTaskPriorityHigh   = "high"
	PMTaskPriorityUrgent = "urgent"

	PMTaskSeverityNone     = "none"
	PMTaskSeverityMinor    = "minor"
	PMTaskSeverityMajor    = "major"
	PMTaskSeverityCritical = "critical"
)

// PMTask represents a single work item.
type PMTask struct {
	ID                        string                 `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID               string                 `json:"workspace_id" gorm:"type:uuid;not null;index"`
	DisplayID                 int                    `json:"display_id" gorm:"not null;index"`
	Name                      string                 `json:"name" gorm:"not null"`
	Description               *string                `json:"description"`
	TaskType                  string                 `json:"task_type" gorm:"column:task_type;not null;default:'feature'"`
	WorkflowID                string                 `json:"workflow_id" gorm:"type:uuid;not null;index"`
	WorkflowStateID           string                 `json:"workflow_state_id" gorm:"type:uuid;not null;index"`
	EpicID                    *string                `json:"epic_id" gorm:"type:uuid;index"`
	SprintID                  *string                `json:"sprint_id" gorm:"type:uuid;index"`
	TeamID                    *string                `json:"team_id" gorm:"type:uuid;index"`
	OwnerID                   *string                `json:"owner_id" gorm:"type:uuid;index"`
	OwnerMemberID             *string                `json:"owner_member_id" gorm:"type:uuid;index"`
	RequesterID               *string                `json:"requester_id" gorm:"type:uuid"`
	RequesterMemberID         *string                `json:"requester_member_id" gorm:"type:uuid;index"`
	Estimate                  *int                   `json:"estimate"`
	Priority                  string                 `json:"priority" gorm:"not null;default:'none'"`
	Severity                  string                 `json:"severity" gorm:"not null;default:'none'"`
	Deadline                  *time.Time             `json:"deadline" gorm:"type:date"`
	Position                  int                    `json:"position" gorm:"not null;default:0"`
	Started                   bool                   `json:"started" gorm:"not null;default:false"`
	StartedAt                 *time.Time             `json:"started_at"`
	Completed                 bool                   `json:"completed" gorm:"not null;default:false"`
	CompletedAt               *time.Time             `json:"completed_at"`
	MovedAt                   *time.Time             `json:"moved_at"`
	Blocked                   bool                   `json:"blocked" gorm:"not null;default:false"`
	Blocker                   *string                `json:"blocker"`
	Archived                  bool                   `json:"archived" gorm:"not null;default:false"`
	AssignedAgentID           *string                `json:"assigned_agent_id" gorm:"type:uuid;index"`
	PlanDocumentID            *string                `json:"plan_document_id" gorm:"type:uuid;index"`
	TemplateID                *string                `json:"template_id"`
	RecurringTemplateID       *string                `json:"recurring_template_id" gorm:"type:uuid;index"`
	RecurringRunID            *string                `json:"recurring_run_id" gorm:"type:uuid;index"`
	RecurringOccurrenceNumber *int                   `json:"recurring_occurrence_number"`
	ExternalID                *string                `json:"external_id"`
	SliceType                 *string                `json:"slice_type,omitempty" gorm:"type:text"`
	ImplementationBrief       json.RawMessage        `json:"implementation_brief,omitempty" gorm:"type:jsonb"`
	IsBlockedByTask           bool                   `json:"is_blocked_by_task" gorm:"-"`
	BlockedByCount            int                    `json:"blocked_by_count" gorm:"-"`
	IsBlockingOtherTask       bool                   `json:"is_blocking_other_task" gorm:"-"`
	BlockingCount             int                    `json:"blocking_count" gorm:"-"`
	BlockedByTasks            []TaskDependencyTask   `json:"blocked_by_tasks,omitempty" gorm:"-"`
	BlockingTasks             []TaskDependencyTask   `json:"blocking_tasks,omitempty" gorm:"-"`
	CreatedAt                 time.Time              `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                 time.Time              `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMTask) TableName() string { return "pm_tasks" }

// PMTaskOwner is the join table for many owners per task.
type PMTaskOwner struct {
	TaskID    string    `json:"task_id" gorm:"column:task_id;type:uuid;primaryKey"`
	UserID    string    `json:"user_id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMTaskOwner) TableName() string { return "pm_task_owners" }

// PMTaskFollower is the join table for task followers.
type PMTaskFollower struct {
	TaskID    string    `json:"task_id" gorm:"column:task_id;type:uuid;primaryKey"`
	UserID    string    `json:"user_id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMTaskFollower) TableName() string { return "pm_task_followers" }

// PMTaskLabel is the join table for task labels.
type PMTaskLabel struct {
	TaskID    string    `json:"task_id" gorm:"column:task_id;type:uuid;primaryKey"`
	LabelID   string    `json:"label_id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PMTaskLabel) TableName() string { return "pm_task_labels" }

// PMTaskFilters applies filter options when listing tasks.
type PMTaskFilters struct {
	TeamID                *string
	EpicID                *string
	SprintID              *string
	ContactID             *string
	CompanyID             *string
	DealID                *string
	SupportConversationID *string
	IncludeContacts       bool
	IncludeCompanies      bool
	IncludeDeals          bool
	IncludeSupport        bool
	WorkflowID            *string
	WorkflowStateID       *string
	TaskType              *string
	OwnerID               *string
	OwnerMemberID         *string
	RequesterID           *string
	RequesterMemberID     *string
	LabelID               *string
	Priority              *string
	Severity              *string
	Blocked               *string
	Blocking              *string
	UpdatedAfter          *string
	Archived              *bool
	// AccessibleTeamIDs enforces team-based access boundaries.
	// nil = no filtering (admin/owner), [] = no access, [ids] = filter to these teams.
	AccessibleTeamIDs []string
}

// PMPagination is common pagination input.
type PMPagination struct {
	Page    int
	PerPage int
}

// CreateTaskRequest is the payload for creating a task.
type CreateTaskRequest struct {
	WorkspaceID       string                       `json:"workspace_id"`
	Name              string                       `json:"name"`
	Description       *string                      `json:"description"`
	TaskType          string                       `json:"task_type"`
	WorkflowID        string                       `json:"workflow_id"`
	WorkflowStateID   string                       `json:"workflow_state_id"`
	EpicID            *string                      `json:"epic_id"`
	SprintID          *string                      `json:"sprint_id"`
	TeamID            *string                      `json:"team_id"`
	OwnerID           *string                      `json:"owner_id"`
	OwnerMemberID     *string                      `json:"owner_member_id"`
	RequesterID       *string                      `json:"requester_id"`
	RequesterMemberID *string                      `json:"requester_member_id"`
	Estimate          *int                         `json:"estimate"`
	Priority          *string                      `json:"priority"`
	Severity          *string                      `json:"severity"`
	Deadline          *time.Time                   `json:"deadline"`
	Position          *int                         `json:"position"`
	Blocked           *bool                        `json:"blocked"`
	Blocker           *string                      `json:"blocker"`
	TemplateID        *string                      `json:"template_id"`
	ExternalID        *string                      `json:"external_id"`
	OwnerIDs          []string                     `json:"owner_ids"`
	FollowerIDs       []string                     `json:"follower_ids"`
	LabelIDs          []string                     `json:"label_ids"`
	AttachmentIDs     []string                     `json:"attachment_ids,omitempty"`
	ChecklistItems    []CreateChecklistItemRequest `json:"checklist_items,omitempty"`
	ExternalLinks     []CreateExternalLinkRequest  `json:"external_links,omitempty"`
}

// UpdateTaskRequest is the payload for updating a task.
type UpdateTaskRequest struct {
	Name              *string    `json:"name"`
	Description       *string    `json:"description"`
	TaskType          *string    `json:"task_type"`
	WorkflowID        *string    `json:"workflow_id"`
	WorkflowStateID   *string    `json:"workflow_state_id"`
	EpicID            *string    `json:"epic_id"`
	SprintID          *string    `json:"sprint_id"`
	TeamID            *string    `json:"team_id"`
	OwnerID           *string    `json:"owner_id"`
	OwnerMemberID     *string    `json:"owner_member_id"`
	RequesterID       *string    `json:"requester_id"`
	RequesterMemberID *string    `json:"requester_member_id"`
	Estimate          *int       `json:"estimate"`
	Priority          *string    `json:"priority"`
	Severity          *string    `json:"severity"`
	Deadline          *time.Time `json:"deadline"`
	Position          *int       `json:"position"`
	Blocked           *bool      `json:"blocked"`
	Blocker           *string    `json:"blocker"`
	Archived          *bool      `json:"archived"`
	TemplateID        *string    `json:"template_id"`
	ExternalID        *string    `json:"external_id"`
	OwnerIDs          []string   `json:"owner_ids"`
	FollowerIDs       []string   `json:"follower_ids"`
	LabelIDs          []string   `json:"label_ids"`
}

// MoveTaskRequest moves a task to a new state and optionally position.
type MoveTaskRequest struct {
	StateID      string `json:"state_id"`
	Position     *int   `json:"position"`
	DebugTraceID string `json:"debug_trace_id"`
}

// ReorderTaskRequest reorders a task in its current state.
type ReorderTaskRequest struct {
	Position     int    `json:"position"`
	DebugTraceID string `json:"debug_trace_id"`
}

// TaskUserLinkRequest links a user to a task as owner/follower.
type TaskUserLinkRequest struct {
	UserID string `json:"user_id"`
}

// TaskLabelLinkRequest links a label to a task.
type TaskLabelLinkRequest struct {
	LabelID string `json:"label_id"`
}

// TaskDetail is a task enriched with relation data.
type TaskDetail struct {
	Story           PMTask            `json:"task"`
	Owners          []User            `json:"owners"`
	Followers       []User            `json:"followers"`
	OwnerMember     *AssignableMember `json:"owner_member,omitempty"`
	RequesterMember *AssignableMember `json:"requester_member,omitempty"`
	Labels          []PMLabel         `json:"labels"`
	EpicName        *string           `json:"epic_name"`
	SprintName      *string           `json:"sprint_name"`
	ObjectiveName   *string           `json:"objective_name"`
	ObjectiveID     *string           `json:"objective_id"`
	State           *PMWorkflowState  `json:"state"`
}

// TaskDependencyTask is the lightweight task payload used in dependency read models.
type TaskDependencyTask struct {
	ID              string `json:"id"`
	DisplayID       int    `json:"display_id"`
	Name            string `json:"name"`
	WorkflowStateID string `json:"workflow_state_id"`
	Completed       bool   `json:"completed"`
}

// BoardTask is a task enriched with relation names for board display.
// NOTE: embeds PMTask for board display.
type BoardTask struct {
	PMTask
	EpicName             *string                    `json:"epic_name,omitempty"`
	SprintName           *string                    `json:"sprint_name,omitempty"`
	OwnerName            *string                    `json:"owner_name,omitempty"`
	StateName            *string                    `json:"state_name,omitempty"`
	StateType            *string                    `json:"state_type,omitempty"`
	StateColor           *string                    `json:"state_color,omitempty"`
	Labels               []PMLabel                  `json:"labels"`
	Contacts             []AssociationObjectSummary `json:"contacts,omitempty"`
	Companies            []AssociationObjectSummary `json:"companies,omitempty"`
	Deals                []AssociationObjectSummary `json:"deals,omitempty"`
	SupportConversations []AssociationObjectSummary `json:"support_conversations,omitempty"`
}

// TaskGroup is a labeled bucket of tasks inside a board column.
type TaskGroup struct {
	Key     string      `json:"key"`
	Label   string      `json:"label"`
	Stories []BoardTask `json:"stories"`
}

// TaskStateColumn is the data shape used for board columns.
type TaskStateColumn struct {
	State       PMWorkflowState `json:"state"`
	Stories     []BoardTask     `json:"stories"`
	TaskGroups  []TaskGroup     `json:"task_groups,omitempty"`
	TaskCount   int             `json:"task_count"`
	PointTotal  int             `json:"point_total"`
	HasMore     bool            `json:"has_more"`
}

// TaskMemberColumn is the data shape for member-grouped board columns.
type TaskMemberColumn struct {
	Member     *AssignableMember `json:"member"`
	Stories    []BoardTask       `json:"stories"`
	TaskCount  int               `json:"task_count"`
	PointTotal int               `json:"point_total"`
	HasMore    bool              `json:"has_more"`
}

// ColumnTasksResponse is the paginated payload for a single board column.
type ColumnTasksResponse struct {
	Stories     []BoardTask `json:"stories"`
	TaskGroups  []TaskGroup `json:"task_groups,omitempty"`
	Total       int         `json:"total"`
}

// TaskStateCount stores aggregate count per state.
type TaskStateCount struct {
	StateID   string `json:"state_id"`
	StateName string `json:"state_name"`
	StateType string `json:"state_type"`
	TaskCount int    `json:"task_count"`
}
