package model

import "time"

// WorkspaceInvitation represents a row in the workspace_invitations table.
type WorkspaceInvitation struct {
	ID          string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Email       string     `json:"email" gorm:"not null"`
	Role        string     `json:"role" gorm:"not null;default:'member'"`
	Token       string     `json:"-" gorm:"uniqueIndex;not null"`
	InvitedBy   string     `json:"invited_by" gorm:"type:uuid;not null"`
	Status      string     `json:"status" gorm:"not null;default:'pending'"`
	ExpiresAt   time.Time  `json:"expires_at" gorm:"not null"`
	AcceptedAt  *time.Time `json:"accepted_at"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspaceInvitation) TableName() string { return "workspace_invitations" }

// CreateInvitationRequest is the payload for creating an invitation.
type CreateInvitationRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}

// InvitationResponse is the API response for an invitation.
type InvitationResponse struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Email       string     `json:"email"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	InvitedBy   string     `json:"invited_by"`
	ExpiresAt   time.Time  `json:"expires_at"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	JoinURL     string     `json:"join_url,omitempty"`
}

// AcceptInvitationRequest is the payload for accepting an invitation.
type AcceptInvitationRequest struct {
	Token string `json:"token"`
}

// InviteInfoResponse is the public info returned for a join page.
type InviteInfoResponse struct {
	WorkspaceName string `json:"workspace_name"`
	WorkspaceSlug string `json:"workspace_slug"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	InvitedByName string `json:"invited_by_name"`
	Status        string `json:"status"`
	Expired       bool   `json:"expired"`
}

// InvitationWithDetails holds invitation data joined with workspace and inviter info.
type InvitationWithDetails struct {
	WorkspaceInvitation
	WorkspaceName string `json:"workspace_name"`
	WorkspaceSlug string `json:"workspace_slug"`
	InviterName   string `json:"inviter_name"`
}
