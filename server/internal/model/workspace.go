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
	WorkspaceKey   string    `json:"workspace_key" gorm:"type:varchar(5);uniqueIndex:idx_ws_key_org"`
	OwnerID        string    `json:"owner_id" gorm:"type:uuid;not null"`
	OrganizationID *string   `json:"organization_id" gorm:"type:uuid;uniqueIndex:idx_ws_key_org"`
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
	ID                         string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID                string     `json:"workspace_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_ws_member_ws_user,priority:1"`
	UserID                     *string    `json:"user_id" gorm:"type:uuid;uniqueIndex:idx_ws_member_ws_user,priority:2"`
	Email                      string     `json:"email" gorm:"not null"`
	DisplayName                string     `json:"display_name" gorm:"not null"`
	Role                       string     `json:"role" gorm:"not null;default:'member'"`
	Status                     string     `json:"status" gorm:"not null;default:'active';index"`
	InvitedBy                  *string    `json:"invited_by,omitempty" gorm:"type:uuid"`
	InvitedAt                  *time.Time `json:"invited_at,omitempty"`
	AcceptedAt                 *time.Time `json:"accepted_at,omitempty"`
	SupportDefaultTeamID       *string    `json:"support_default_team_id,omitempty" gorm:"type:uuid"`
	SupportTaskDialogDismissed bool       `json:"support_task_dialog_dismissed" gorm:"default:false"`
	CreatedAt                  time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                  time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspaceMember) TableName() string { return "workspace_members" }

// WorkspaceWithRole is a workspace combined with the requesting user's role.
type WorkspaceWithRole struct {
	Workspace
	Role    string                          `json:"role"`
	Billing *WorkspaceBillingSummaryForList `json:"billing,omitempty" gorm:"-"`
}

// WorkspaceBillingSummaryForList is the non-sensitive billing state shown on
// cross-workspace selectors.
type WorkspaceBillingSummaryForList struct {
	WorkspaceID            string     `json:"workspace_id"`
	Plan                   string     `json:"plan"`
	Status                 string     `json:"status"`
	BillingInterval        string     `json:"billing_interval"`
	Trialing               bool       `json:"trialing"`
	TrialEndsAt            *time.Time `json:"trial_ends_at,omitempty"`
	CurrentPeriodStart     time.Time  `json:"current_period_start"`
	CurrentPeriodEnd       time.Time  `json:"current_period_end"`
	IncludedCredits        int        `json:"included_credits"`
	CreditsUsed            int        `json:"credits_used"`
	CreditsRemaining       int        `json:"credits_remaining"`
	OnDemandEnabled        bool       `json:"on_demand_enabled"`
	OnDemandAvailable      bool       `json:"on_demand_available"`
	Locked                 bool       `json:"locked"`
	ManageBillingEnabled   bool       `json:"manage_billing_enabled"`
	OnDemandBlocksInvoiced int        `json:"on_demand_blocks_invoiced"`
}

// MemberWithUser is a workspace member with embedded user details.
type MemberWithUser struct {
	ID                    string  `json:"id"`
	UserID                string  `json:"user_id"`
	Role                  string  `json:"role"`
	Email                 string  `json:"email"`
	FullName              string  `json:"full_name"`
	TwoFAEnabled          bool    `json:"two_fa_enabled"`
	AvatarURL             *string `json:"avatar_url"`
	AvatarStyle           *string `json:"avatar_style,omitempty"`
	AvatarSeed            *string `json:"avatar_seed,omitempty"`
	AvatarBackgroundMode  *string `json:"avatar_background_mode,omitempty"`
	AvatarBackgroundColor *string `json:"avatar_background_color,omitempty"`
}

// WorkspaceMemberPresenceStatus represents live presence for a workspace member.
type WorkspaceMemberPresenceStatus struct {
	UserID       string     `json:"user_id"`
	Status       string     `json:"status"`
	Source       string     `json:"source"` // auto | manual
	ManualStatus *string    `json:"manual_status,omitempty"`
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
}

// AssignableMember is the workspace-level person identity used by PM pickers.
type AssignableMember struct {
	ID                    string     `json:"id"`
	UserID                *string    `json:"user_id,omitempty"`
	Role                  string     `json:"role"`
	Email                 string     `json:"email"`
	DisplayName           string     `json:"display_name"`
	AvatarURL             *string    `json:"avatar_url,omitempty"`
	AvatarStyle           *string    `json:"avatar_style,omitempty"`
	AvatarSeed            *string    `json:"avatar_seed,omitempty"`
	AvatarBackgroundMode  *string    `json:"avatar_background_mode,omitempty"`
	AvatarBackgroundColor *string    `json:"avatar_background_color,omitempty"`
	Status                string     `json:"status"`
	InvitedBy             *string    `json:"invited_by,omitempty"`
	InvitedAt             *time.Time `json:"invited_at,omitempty"`
	AcceptedAt            *time.Time `json:"accepted_at,omitempty"`
}

// WorkspaceMFAPolicy captures the workspace MFA policy and the current user's
// ability to satisfy it.
type WorkspaceMFAPolicy struct {
	EnforceTwoFactor bool `json:"enforce_two_factor"`
	MFARequired      bool `json:"mfa_required"`
	MFAEnabled       bool `json:"mfa_enabled"`
	MFASatisfied     bool `json:"mfa_satisfied"`
}

// CreateWorkspaceRequest is the payload for POST /api/workspaces.
type CreateWorkspaceRequest struct {
	Name           string  `json:"name"`
	Slug           string  `json:"slug"`
	WorkspaceKey   string  `json:"workspace_key"`
	OrganizationID string  `json:"organization_id"`
	Description    *string `json:"description"`
	WebsiteURL     *string `json:"website_url"`
	Timezone       string  `json:"timezone"`
}

// UpdateWorkspaceRequest is the payload for PUT /api/workspaces/{id}.
type UpdateWorkspaceRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	WebsiteURL   *string `json:"website_url"`
	LogoURL      *string `json:"logo_url"`
	Timezone     *string `json:"timezone"`
	WorkspaceKey *string `json:"workspace_key,omitempty"`
}

// WorkspaceKeyHistory tracks workspace key changes for alias resolution.
type WorkspaceKeyHistory struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	OldKey      string    `json:"old_key" gorm:"type:varchar(5);not null;uniqueIndex"`
	NewKey      string    `json:"new_key" gorm:"type:varchar(5);not null"`
	ChangedAt   time.Time `json:"changed_at" gorm:"autoCreateTime"`
	ChangedBy   string    `json:"changed_by" gorm:"type:uuid"`
}

func (WorkspaceKeyHistory) TableName() string { return "workspace_key_history" }

// UpdateWorkspaceMemberRequest is the payload for PUT /api/workspaces/{id}/members/{memberId}.
type UpdateWorkspaceMemberRequest struct {
	Role string `json:"role"`
}

// UpdateSupportTaskPreferencesRequest is the payload for updating support task creation preferences.
type UpdateSupportTaskPreferencesRequest struct {
	SupportDefaultTeamID       *string `json:"support_default_team_id"`
	SupportTaskDialogDismissed *bool   `json:"support_task_dialog_dismissed"`
}
