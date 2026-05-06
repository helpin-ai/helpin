package model

import "time"

// PMTaskTemplate represents reusable task templates scoped to workspace or team.
type PMTaskTemplate struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	TeamID          *string   `json:"team_id" gorm:"type:uuid;index"`
	Name            string    `json:"name" gorm:"not null"`
	Description     *string   `json:"description"`
	TaskType        *string   `json:"task_type" gorm:"column:task_type"`
	Priority        *string   `json:"priority"`
	Severity        *string   `json:"severity"`
	Estimate        *int      `json:"estimate"`
	LabelIDs        *string   `json:"label_ids" gorm:"type:text"`
	OwnerMemberID   *string   `json:"owner_member_id" gorm:"type:uuid"`
	EpicID          *string   `json:"epic_id" gorm:"type:uuid"`
	SprintID        *string   `json:"sprint_id" gorm:"type:uuid"`
	WorkflowStateID *string   `json:"workflow_state_id" gorm:"type:uuid;index"`
	Deadline        *string   `json:"deadline" gorm:"type:text"`
	ChecklistItems  *string   `json:"checklist_items" gorm:"type:text"`
	ExternalLinks   *string   `json:"external_links" gorm:"type:text"`
	Archived        bool      `json:"archived" gorm:"not null;default:false"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMTaskTemplate) TableName() string { return "pm_task_templates" }

// CreateTaskTemplateRequest is the payload for creating a task template.
type CreateTaskTemplateRequest struct {
	WorkspaceID     string  `json:"workspace_id"`
	TeamID          *string `json:"team_id"`
	Name            string  `json:"name"`
	Description     *string `json:"description"`
	TaskType        *string `json:"task_type"`
	Priority        *string `json:"priority"`
	Severity        *string `json:"severity"`
	Estimate        *int    `json:"estimate"`
	LabelIDs        *string `json:"label_ids"`
	OwnerMemberID   *string `json:"owner_member_id"`
	EpicID          *string `json:"epic_id"`
	SprintID        *string `json:"sprint_id"`
	WorkflowStateID *string `json:"workflow_state_id"`
	Deadline        *string `json:"deadline"`
	ChecklistItems  *string `json:"checklist_items"`
	ExternalLinks   *string `json:"external_links"`
}

// UpdateTaskTemplateRequest is the payload for updating a task template.
type UpdateTaskTemplateRequest struct {
	TeamID          *string `json:"team_id"`
	Name            *string `json:"name"`
	Description     *string `json:"description"`
	TaskType        *string `json:"task_type"`
	Priority        *string `json:"priority"`
	Severity        *string `json:"severity"`
	Estimate        *int    `json:"estimate"`
	LabelIDs        *string `json:"label_ids"`
	OwnerMemberID   *string `json:"owner_member_id"`
	EpicID          *string `json:"epic_id"`
	SprintID        *string `json:"sprint_id"`
	WorkflowStateID *string `json:"workflow_state_id"`
	Deadline        *string `json:"deadline"`
	ChecklistItems  *string `json:"checklist_items"`
	ExternalLinks   *string `json:"external_links"`
	Archived        *bool   `json:"archived"`
}
