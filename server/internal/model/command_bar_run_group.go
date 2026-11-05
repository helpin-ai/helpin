package model

import "time"

// CommandBarPlanRunGroup links a command-bar plan to its Agent Runtime run
// group. The plan's child runs join the group, and the runtime records each
// child's terminal transition with a per-group completion sequence; Helpin
// reads completions after CompletionCursor instead of inferring settlement
// from events or a sweep.
type CommandBarPlanRunGroup struct {
	PlanID      string `json:"plan_id" gorm:"type:uuid;primaryKey"`
	WorkspaceID string `json:"workspace_id" gorm:"type:uuid;not null;index"`
	GroupID     string `json:"group_id" gorm:"not null"`
	// CompletionCursor is the highest completion sequence already applied.
	CompletionCursor int64     `json:"completion_cursor" gorm:"not null;default:0"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the plan run group table.
func (CommandBarPlanRunGroup) TableName() string { return "command_bar_plan_run_groups" }
