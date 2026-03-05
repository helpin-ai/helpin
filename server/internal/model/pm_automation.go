package model

import "time"

const (
	PMAutomationTypeEpicAutoStart          = "epic_auto_start"
	PMAutomationTypeEpicAutoComplete       = "epic_auto_complete"
	PMAutomationTypeSprintAutoCreate    = "sprint_auto_create"
	PMAutomationTypeSprintMoveUnfinished = "sprint_move_unfinished"
)

// PMAutomation stores a single automation rule per workspace (epic) or workspace+team (sprint).
type PMAutomation struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	AutomationType string    `json:"automation_type" gorm:"not null"`
	Enabled        bool      `json:"enabled" gorm:"not null;default:false"`
	TeamID         *string   `json:"team_id" gorm:"type:uuid;index"`
	ConfigStateID  *string   `json:"config_state_id" gorm:"type:uuid"`
	ConfigInt      *int      `json:"config_int"`
	ConfigInt2     *int      `json:"config_int2"`
	ConfigInt3     *int      `json:"config_int3"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PMAutomation) TableName() string { return "pm_automations" }

// UpsertAutomationRequest is the payload for creating or updating an automation.
type UpsertAutomationRequest struct {
	WorkspaceID    string  `json:"workspace_id"`
	AutomationType string  `json:"automation_type"`
	Enabled        bool    `json:"enabled"`
	TeamID         *string `json:"team_id"`
	ConfigStateID  *string `json:"config_state_id"`
	ConfigInt      *int    `json:"config_int"`
	ConfigInt2     *int    `json:"config_int2"`
	ConfigInt3     *int    `json:"config_int3"`
}

// DeleteAutomationRequest identifies an automation to remove.
type DeleteAutomationRequest struct {
	WorkspaceID    string  `json:"workspace_id"`
	AutomationType string  `json:"automation_type"`
	TeamID         *string `json:"team_id"`
}
