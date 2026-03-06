package model

import "time"

// Organization represents a row in the organizations table.
type Organization struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name      string    `json:"name" gorm:"not null"`
	Slug      string    `json:"slug" gorm:"uniqueIndex;not null"`
	OwnerID   string    `json:"owner_id" gorm:"type:uuid;not null"`
	LogoURL   *string   `json:"logo_url"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Organization) TableName() string { return "organizations" }

// OrganizationMember represents a row in the organization_members table.
type OrganizationMember struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrganizationID string    `json:"organization_id" gorm:"type:uuid;not null;uniqueIndex:idx_org_member_org_user"`
	UserID         string    `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:idx_org_member_org_user"`
	Role           string    `json:"role" gorm:"not null;default:'member'"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (OrganizationMember) TableName() string { return "organization_members" }

// OrganizationWithRole is an organization combined with the requesting user's role.
type OrganizationWithRole struct {
	Organization
	Role string `json:"role"`
}

// CreateOrganizationRequest is the payload for POST /api/organizations.
type CreateOrganizationRequest struct {
	Name    string  `json:"name"`
	Slug    string  `json:"slug"`
	LogoURL *string `json:"logo_url"`
}

// UpdateOrganizationRequest is the payload for PUT /api/organizations/{id}.
type UpdateOrganizationRequest struct {
	Name    *string `json:"name"`
	LogoURL *string `json:"logo_url"`
}

// AddOrgMemberRequest is the payload for POST /api/organizations/{id}/members.
type AddOrgMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// UpdateOrgMemberRequest is the payload for PUT /api/organizations/{id}/members/{userId}.
type UpdateOrgMemberRequest struct {
	Role string `json:"role"`
}
