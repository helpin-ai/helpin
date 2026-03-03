package model

import "time"

const (
	PMStateTypeBacklog   = "backlog"
	PMStateTypeUnstarted = "unstarted"
	PMStateTypeStarted   = "started"
	PMStateTypeDone      = "done"
)

// PMWorkflow represents a workflow configuration for a workspace/team.
type PMWorkflow struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name            string    `json:"name" gorm:"not null"`
	Description     *string   `json:"description"`
	TeamID          *string   `json:"team_id" gorm:"type:uuid;index"`
	DefaultStateID  *string   `json:"default_state_id" gorm:"type:uuid"`
	AutoAssignOwner bool      `json:"auto_assign_owner" gorm:"not null;default:false"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMWorkflow) TableName() string { return "pm_workflows" }

// PMWorkflowState represents an individual state column in a workflow.
type PMWorkflowState struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkflowID  string    `json:"workflow_id" gorm:"type:uuid;not null;index"`
	Name        string    `json:"name" gorm:"not null"`
	StateType   string    `json:"state_type" gorm:"not null"`
	Position    int       `json:"position" gorm:"not null;default:0"`
	Color       *string   `json:"color"`
	Description *string   `json:"description"`
	WIPLimit    *int      `json:"wip_limit"`
	IsDefault   bool      `json:"is_default" gorm:"not null;default:false"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMWorkflowState) TableName() string { return "pm_workflow_states" }

// PMEpicWorkflowState defines the workflow states used by epics.
type PMEpicWorkflowState struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name        string    `json:"name" gorm:"not null"`
	StateType   string    `json:"state_type" gorm:"not null"`
	Position    int       `json:"position" gorm:"not null;default:0"`
	Color       *string   `json:"color"`
	IsDefault   bool      `json:"is_default" gorm:"not null;default:false"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMEpicWorkflowState) TableName() string { return "pm_epic_workflow_states" }

// CreateWorkflowRequest is the payload for creating a workflow.
type CreateWorkflowRequest struct {
	WorkspaceID     string  `json:"workspace_id"`
	Name            string  `json:"name"`
	Description     *string `json:"description"`
	TeamID          *string `json:"team_id"`
	AutoAssignOwner bool    `json:"auto_assign_owner"`
}

// UpdateWorkflowRequest is the payload for updating a workflow.
type UpdateWorkflowRequest struct {
	Name            *string `json:"name"`
	Description     *string `json:"description"`
	TeamID          *string `json:"team_id"`
	DefaultStateID  *string `json:"default_state_id"`
	AutoAssignOwner *bool   `json:"auto_assign_owner"`
}

// CreateWorkflowStateRequest is the payload for adding a state.
type CreateWorkflowStateRequest struct {
	WorkflowID  string  `json:"workflow_id"`
	Name        string  `json:"name"`
	StateType   string  `json:"state_type"`
	Position    *int    `json:"position"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
	WIPLimit    *int    `json:"wip_limit"`
	IsDefault   bool    `json:"is_default"`
}

// UpdateWorkflowStateRequest is the payload for updating a state.
type UpdateWorkflowStateRequest struct {
	Name        *string `json:"name"`
	StateType   *string `json:"state_type"`
	Position    *int    `json:"position"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
	WIPLimit    *int    `json:"wip_limit"`
	IsDefault   *bool   `json:"is_default"`
}

// ReorderStatesRequest is the payload for state reordering.
type ReorderStatesRequest struct {
	StateIDs []string `json:"state_ids"`
}

// WorkflowWithStates is a workflow with all associated states.
type WorkflowWithStates struct {
	Workflow PMWorkflow        `json:"workflow"`
	States   []PMWorkflowState `json:"states"`
}
