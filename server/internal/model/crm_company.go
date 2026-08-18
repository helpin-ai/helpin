package model

import "time"

// CRMCompany represents a CRM company.
type CRMCompany struct {
	ID               string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	DisplayID        string    `json:"display_id" gorm:"not null"`
	ExternalID       *string   `json:"external_id,omitempty" gorm:"index"`
	Name             string    `json:"name" gorm:"not null"`
	Domain           *string   `json:"domain" gorm:"index"`
	Industry         *string   `json:"industry"`
	EmployeeCount    *int      `json:"employee_count"`
	AnnualRevenue    *float64  `json:"annual_revenue"`
	Description      *string   `json:"description"`
	LogoURL          *string   `json:"logo_url"`
	LinkedInURL      *string   `json:"linkedin_url" gorm:"column:linkedin_url"`
	Headquarters     *string   `json:"headquarters"`
	OwnerMemberID    *string   `json:"owner_member_id" gorm:"type:uuid;index"`
	CustomProperties JSONB     `json:"custom_properties" gorm:"type:jsonb;default:'{}'"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMCompany) TableName() string { return "crm_companies" }

// CreateCRMCompanyRequest is the payload for creating a company.
type CreateCRMCompanyRequest struct {
	WorkspaceID      string                 `json:"workspace_id"`
	ExternalID       *string                `json:"external_id"`
	Name             string                 `json:"name"`
	Domain           *string                `json:"domain"`
	Industry         *string                `json:"industry"`
	EmployeeCount    *int                   `json:"employee_count"`
	AnnualRevenue    *float64               `json:"annual_revenue"`
	Description      *string                `json:"description"`
	LogoURL          *string                `json:"logo_url"`
	LinkedInURL      *string                `json:"linkedin_url"`
	Headquarters     *string                `json:"headquarters"`
	OwnerMemberID    *string                `json:"owner_member_id"`
	CustomProperties map[string]interface{} `json:"custom_properties"`
}

// UpdateCRMCompanyRequest is the payload for updating a company.
type UpdateCRMCompanyRequest struct {
	Name             *string                `json:"name"`
	ExternalID       *string                `json:"external_id"`
	Domain           *string                `json:"domain"`
	Industry         *string                `json:"industry"`
	EmployeeCount    *int                   `json:"employee_count"`
	AnnualRevenue    *float64               `json:"annual_revenue"`
	Description      *string                `json:"description"`
	LogoURL          *string                `json:"logo_url"`
	LinkedInURL      *string                `json:"linkedin_url"`
	Headquarters     *string                `json:"headquarters"`
	OwnerMemberID    *string                `json:"owner_member_id"`
	ClearOwner       bool                   `json:"clear_owner"`
	CustomProperties map[string]interface{} `json:"custom_properties"`
}

// CRMCompanyListFilters applies filters when listing companies.
type CRMCompanyListFilters struct {
	Industry      *string
	OwnerMemberID *string
	Search        *string
}
