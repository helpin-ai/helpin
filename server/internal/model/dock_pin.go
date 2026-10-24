package model

import "time"

// DockPin is a personal roster shortcut, never an access grant.
type DockPin struct {
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;primaryKey"`
	UserID      string    `json:"user_id" gorm:"type:uuid;primaryKey"`
	Kind        string    `json:"kind" gorm:"primaryKey"`
	ResourceID  string    `json:"resource_id" gorm:"type:uuid;primaryKey"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the personal dock pin table.
func (DockPin) TableName() string { return "dock_pins" }

// DockPinsResponse includes only currently accessible, unarchived resources.
type DockPinsResponse struct {
	Chats []DockChat       `json:"chats"`
	Runs  []DockRunSummary `json:"runs"`
}
