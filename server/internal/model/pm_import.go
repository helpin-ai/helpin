package model

import "time"

const (
	PMImportStatusPending    = "pending"
	PMImportStatusScanning   = "scanning"
	PMImportStatusReady      = "ready"
	PMImportStatusProcessing = "processing"
	PMImportStatusCompleted  = "completed"
	PMImportStatusFailed     = "failed"
	PMImportStatusCanceled   = "canceled"

	PMImportSourceShortcut = "shortcut"
)

type PMImportJob struct {
	ID                string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Source            string     `json:"source" gorm:"not null;index"`
	Status            string     `json:"status" gorm:"not null;default:'pending';index"`
	FileName          string     `json:"file_name" gorm:"not null"`
	TotalRows         int        `json:"total_rows"`
	Progress          int        `json:"progress" gorm:"not null;default:0"`
	CurrentStep       string     `json:"current_step"`
	StepsCompleted    int        `json:"steps_completed" gorm:"not null;default:0"`
	StepsTotal        int        `json:"steps_total" gorm:"not null;default:0"`
	EntitiesProcessed int        `json:"entities_processed" gorm:"not null;default:0"`
	EntitiesTotal     int        `json:"entities_total" gorm:"not null;default:0"`
	Result            *string    `json:"result"`
	Error             *string    `json:"error"`
	PayloadEncrypted  *string    `json:"-" gorm:"type:text"`
	WorkflowID        *string    `json:"workflow_id,omitempty" gorm:"index"`
	StartedBy         string     `json:"started_by" gorm:"type:uuid;not null"`
	CreatedAt         time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	CompletedAt       *time.Time `json:"completed_at"`
}

func (PMImportJob) TableName() string { return "pm_import_jobs" }

type ShortcutWorkflowPreview struct {
	ID        string                         `json:"id,omitempty"`
	Name      string                         `json:"name"`
	TaskCount int                            `json:"task_count"`
	States    []ShortcutWorkflowStatePreview `json:"states"`
}

type ShortcutWorkflowStatePreview struct {
	Name          string `json:"name"`
	SuggestedType string `json:"suggested_type"`
	TaskCount     int    `json:"task_count"`
}

type ShortcutTeamPreview struct {
	Name      string `json:"name"`
	TaskCount int    `json:"task_count"`
}

type ShortcutUserMatch struct {
	Email         string  `json:"email"`
	MatchedUserID *string `json:"matched_user_id"`
	MatchedName   *string `json:"matched_name"`
	ShortcutName  *string `json:"shortcut_name,omitempty"`
}

type ShortcutImportPreviewSummary struct {
	TotalTasks          int            `json:"total_tasks"`
	TasksByType         map[string]int `json:"tasks_by_type"`
	EpicsCount          int            `json:"epics_count"`
	ObjectivesCount     int            `json:"objectives_count"`
	SprintsCount        int            `json:"sprints_count"`
	LabelsCount         int            `json:"labels_count"`
	TeamsCount          int            `json:"teams_count"`
	WorkflowsCount      int            `json:"workflows_count"`
	WorkflowStatesCount int            `json:"workflow_states_count"`
	ChecklistItemsCount int            `json:"checklist_items_count"`
	DuplicateTasks      int            `json:"duplicate_tasks"`
}

type ShortcutImportPreviewResponse struct {
	Summary   ShortcutImportPreviewSummary `json:"summary"`
	Users     []ShortcutUserMatch          `json:"users"`
	Teams     []ShortcutTeamPreview        `json:"teams"`
	Workflows []ShortcutWorkflowPreview    `json:"workflows"`
	Warnings  []string                     `json:"warnings"`
}

type ShortcutStateCreateMapping struct {
	ShortcutState string `json:"shortcut_state"`
	NewStateName  string `json:"new_state_name"`
	StateType     string `json:"state_type"`
	Position      int    `json:"position"`
}

type ShortcutStateExistingMapping struct {
	ShortcutState   string `json:"shortcut_state"`
	ExistingStateID string `json:"existing_state_id"`
}

type ShortcutWorkflowStateMapping struct {
	ShortcutWorkflowID   string                         `json:"shortcut_workflow_id,omitempty"`
	ShortcutWorkflowName string                         `json:"shortcut_workflow_name"`
	Mode                 string                         `json:"mode"`
	NewWorkflowName      string                         `json:"new_workflow_name,omitempty"`
	ExistingWorkflowID   string                         `json:"existing_workflow_id,omitempty"`
	States               []ShortcutStateCreateMapping   `json:"states,omitempty"`
	ExistingStates       []ShortcutStateExistingMapping `json:"existing_states,omitempty"`
}

type ShortcutWorkflowStateMappingPayload struct {
	ShortcutWorkflowID   string `json:"shortcut_workflow_id,omitempty"`
	ShortcutWorkflowName string `json:"shortcut_workflow_name"`
	Mode                 string `json:"mode"`
	NewWorkflowName      string `json:"new_workflow_name,omitempty"`
	ExistingWorkflowID   string `json:"existing_workflow_id,omitempty"`
	States               []struct {
		ShortcutState   string `json:"shortcut_state"`
		NewStateName    string `json:"new_state_name,omitempty"`
		StateType       string `json:"state_type,omitempty"`
		Position        int    `json:"position,omitempty"`
		ExistingStateID string `json:"existing_state_id,omitempty"`
	} `json:"states"`
}

type ShortcutImportOptions struct {
	ImportArchived          bool   `json:"import_archived"`
	ImportCompleted         bool   `json:"import_completed"`
	StoryDateField          string `json:"story_date_field,omitempty"`
	StoryLookbackMonths     int    `json:"story_lookback_months,omitempty"`
	EpicLookbackMonths      int    `json:"epic_lookback_months,omitempty"`
	ObjectiveLookbackMonths int    `json:"objective_lookback_months,omitempty"`
	MaxStories              int    `json:"max_stories,omitempty"`
}

type ShortcutImportExecuteRequest struct {
	UserMappings          map[string]string                     `json:"user_mappings"`
	TeamMappings          map[string]string                     `json:"team_mappings,omitempty"`
	WorkflowStateMappings []ShortcutWorkflowStateMappingPayload `json:"workflow_state_mappings"`
	Options               ShortcutImportOptions                 `json:"options"`
	APIToken              string                                `json:"api_token,omitempty"`
}

type ShortcutImportExecuteResponse struct {
	ImportID string `json:"import_id"`
	Status   string `json:"status"`
}

type ShortcutImportResult struct {
	TeamsCreated          int      `json:"teams_created"`
	WorkflowsCreated      int      `json:"workflows_created"`
	WorkflowStatesCreated int      `json:"workflow_states_created"`
	LabelsCreated         int      `json:"labels_created"`
	ObjectivesCreated     int      `json:"objectives_created"`
	EpicsCreated          int      `json:"epics_created"`
	SprintsCreated        int      `json:"sprints_created"`
	TasksCreated          int      `json:"tasks_created"`
	TasksSkipped          int      `json:"tasks_skipped"`
	ChecklistItemsCreated int      `json:"checklist_items_created"`
	OwnerLinksCreated     int      `json:"owner_links_created"`
	LabelLinksCreated     int      `json:"label_links_created"`
	ExternalLinksCreated  int      `json:"external_links_created"`
	TaskLinksCreated      int      `json:"task_links_created"`
	AttachmentsCreated    int      `json:"attachments_created"`
	CommentsCreated       int      `json:"comments_created"`
	Warnings              []string `json:"warnings"`
}

type ShortcutImportStatusProgress struct {
	CurrentStep       string `json:"current_step"`
	StepsCompleted    int    `json:"steps_completed"`
	StepsTotal        int    `json:"steps_total"`
	EntitiesProcessed int    `json:"entities_processed"`
	EntitiesTotal     int    `json:"entities_total"`
}

type ShortcutImportStatusResponse struct {
	ImportID    string                       `json:"import_id"`
	Status      string                       `json:"status"`
	FileName    string                       `json:"file_name,omitempty"`
	TotalRows   int                          `json:"total_rows,omitempty"`
	Progress    ShortcutImportStatusProgress `json:"progress"`
	Result      *ShortcutImportResult        `json:"result,omitempty"`
	Error       *string                      `json:"error,omitempty"`
	CreatedAt   *time.Time                   `json:"created_at,omitempty"`
	UpdatedAt   *time.Time                   `json:"updated_at,omitempty"`
	CompletedAt *time.Time                   `json:"completed_at,omitempty"`
}

type ShortcutAPIImportPreviewRequest struct {
	APIToken string                `json:"api_token"`
	Options  ShortcutImportOptions `json:"options"`
	ScanID   string                `json:"scan_id,omitempty"`
}

type ShortcutAPIImportExecuteRequest = ShortcutImportExecuteRequest
