package model

import "time"

const (
	WorkspaceMemberStatusPending  = "pending"
	WorkspaceMemberStatusActive   = "active"
	WorkspaceMemberStatusRevoked  = "revoked"
	WorkspaceMemberStatusInactive = "inactive"
)

// Workspace represents a row in the workspaces table.
type Workspace struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name           string    `json:"name" gorm:"not null"`
	Slug           string    `json:"slug" gorm:"uniqueIndex;not null"`
	OwnerID        string    `json:"owner_id" gorm:"type:uuid;not null"`
	OrganizationID *string   `json:"organization_id" gorm:"type:uuid"`
	Description    *string   `json:"description"`
	WebsiteURL     *string   `json:"website_url"`
	LogoURL        *string   `json:"logo_url"`
	Timezone       string    `json:"timezone" gorm:"not null;default:'UTC'"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Workspace) TableName() string { return "workspaces" }

// WorkspaceMember represents a row in the workspace_members table.
type WorkspaceMember struct {
	ID          string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string     `json:"workspace_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_ws_member_ws_user,priority:1"`
	UserID      *string    `json:"user_id" gorm:"type:uuid;uniqueIndex:idx_ws_member_ws_user,priority:2"`
	Email       string     `json:"email" gorm:"not null"`
	DisplayName string     `json:"display_name" gorm:"not null"`
	Role        string     `json:"role" gorm:"not null;default:'member'"`
	Status      string     `json:"status" gorm:"not null;default:'active';index"`
	InvitedBy   *string    `json:"invited_by,omitempty" gorm:"type:uuid"`
	InvitedAt   *time.Time `json:"invited_at,omitempty"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspaceMember) TableName() string { return "workspace_members" }

// WorkspaceWithRole is a workspace combined with the requesting user's role.
type WorkspaceWithRole struct {
	Workspace
	Role string `json:"role"`
}

// MemberWithUser is a workspace member with embedded user details.
type MemberWithUser struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	Role      string  `json:"role"`
	Email     string  `json:"email"`
	FullName  string  `json:"full_name"`
	AvatarURL *string `json:"avatar_url"`
}

// AssignableMember is the workspace-level person identity used by PM pickers.
type AssignableMember struct {
	ID          string     `json:"id"`
	UserID      *string    `json:"user_id,omitempty"`
	Role        string     `json:"role"`
	Email       string     `json:"email"`
	DisplayName string     `json:"display_name"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
	Status      string     `json:"status"`
	InvitedBy   *string    `json:"invited_by,omitempty"`
	InvitedAt   *time.Time `json:"invited_at,omitempty"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
}

// CreateWorkspaceRequest is the payload for POST /api/workspaces.
type CreateWorkspaceRequest struct {
	Name           string  `json:"name"`
	Slug           string  `json:"slug"`
	OrganizationID string  `json:"organization_id"`
	Description    *string `json:"description"`
	WebsiteURL     *string `json:"website_url"`
	Timezone       string  `json:"timezone"`
}

// UpdateWorkspaceRequest is the payload for PUT /api/workspaces/{id}.
type UpdateWorkspaceRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	WebsiteURL  *string `json:"website_url"`
	LogoURL     *string `json:"logo_url"`
	Timezone    *string `json:"timezone"`
}

// UpdateWorkspaceMemberRequest is the payload for PUT /api/workspaces/{id}/members/{memberId}.
type UpdateWorkspaceMemberRequest struct {
	Role string `json:"role"`
}
