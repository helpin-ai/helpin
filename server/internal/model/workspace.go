package model

import "time"

// Workspace represents a row in the workspaces table.
type Workspace struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name        string    `json:"name" gorm:"not null"`
	Slug        string    `json:"slug" gorm:"uniqueIndex;not null"`
	OwnerID     string    `json:"owner_id" gorm:"type:uuid;not null"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Workspace) TableName() string { return "workspaces" }

// WorkspaceMember represents a row in the workspace_members table.
type WorkspaceMember struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_ws_member_ws_user"`
	UserID      string    `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:idx_ws_member_ws_user"`
	Role        string    `json:"role" gorm:"not null;default:'member'"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspaceMember) TableName() string { return "workspace_members" }

// WorkspaceWithRole is a workspace combined with the requesting user's role.
type WorkspaceWithRole struct {
	Workspace
	Role string `json:"role"`
}

// CreateWorkspaceRequest is the payload for POST /api/workspaces.
type CreateWorkspaceRequest struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description"`
}

// UpdateWorkspaceRequest is the payload for PUT /api/workspaces/{id}.
type UpdateWorkspaceRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}
