package model

import "time"

// CRM pipeline stage types.
const (
	CRMStageTypeOpen = "open"
	CRMStageTypeWon  = "won"
	CRMStageTypeLost = "lost"

	CRMDealMotionNewBusiness = "new_business"
	CRMDealMotionExpansion   = "expansion"
	CRMDealMotionRenewal     = "renewal"
)

// CRMPipeline represents a sales pipeline.
type CRMPipeline struct {
	ID                      string             `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID             string             `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name                    string             `json:"name" gorm:"not null"`
	IsDefault               bool               `json:"is_default" gorm:"not null;default:false"`
	DefaultCommercialMotion string             `json:"default_commercial_motion" gorm:"not null;default:'new_business';index"`
	Position                int                `json:"position" gorm:"not null;default:0"`
	DealCount               int64              `json:"deal_count" gorm:"-"`
	Stages                  []CRMPipelineStage `json:"stages,omitempty" gorm:"foreignKey:PipelineID"`
	CreatedAt               time.Time          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt               time.Time          `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMPipeline) TableName() string { return "crm_pipelines" }

// CRMPipelineStage represents a stage within a pipeline.
type CRMPipelineStage struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PipelineID  string    `json:"pipeline_id" gorm:"type:uuid;not null;index"`
	Name        string    `json:"name" gorm:"not null"`
	StageType   string    `json:"stage_type" gorm:"not null;default:'open'"` // open, won, lost
	Position    int       `json:"position" gorm:"not null;default:0"`
	Probability int       `json:"probability" gorm:"not null;default:0"` // 0-100
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMPipelineStage) TableName() string { return "crm_pipeline_stages" }

// CRMDeal represents a sales deal/opportunity.
type CRMDeal struct {
	ID               string            `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string            `json:"workspace_id" gorm:"type:uuid;not null;index"`
	DisplayID        string            `json:"display_id" gorm:"not null"`
	Name             string            `json:"name" gorm:"not null"`
	PipelineID       string            `json:"pipeline_id" gorm:"type:uuid;not null;index"`
	StageID          string            `json:"stage_id" gorm:"type:uuid;not null;index"`
	Amount           *float64          `json:"amount"`
	Currency         string            `json:"currency" gorm:"not null;default:'USD'"`
	CloseDate        *time.Time        `json:"close_date" gorm:"type:date"`
	OwnerMemberID    *string           `json:"owner_member_id" gorm:"type:uuid;index"`
	CommercialMotion *string           `json:"commercial_motion,omitempty" gorm:"index"`
	Probability      *int              `json:"probability"`
	CustomProperties JSONB             `json:"custom_properties" gorm:"type:jsonb;default:'{}'"`
	Pipeline         *CRMPipeline      `json:"pipeline,omitempty" gorm:"foreignKey:PipelineID"`
	Stage            *CRMPipelineStage `json:"stage,omitempty" gorm:"foreignKey:StageID"`
	CreatedAt        time.Time         `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time         `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMDeal) TableName() string { return "crm_deals" }

// CreateCRMPipelineRequest is the payload for creating a pipeline.
type CreateCRMPipelineRequest struct {
	WorkspaceID             string                       `json:"workspace_id"`
	Name                    string                       `json:"name"`
	IsDefault               *bool                        `json:"is_default"`
	DefaultCommercialMotion *string                      `json:"default_commercial_motion"`
	Stages                  []CreateCRMPipelineStageItem `json:"stages"`
}

// CreateCRMPipelineStageItem is a stage within a pipeline create request.
type CreateCRMPipelineStageItem struct {
	Name        string `json:"name"`
	StageType   string `json:"stage_type"`
	Position    int    `json:"position"`
	Probability int    `json:"probability"`
}

// UpdateCRMPipelineRequest is the payload for updating a pipeline.
type UpdateCRMPipelineRequest struct {
	Name                    *string                      `json:"name"`
	IsDefault               *bool                        `json:"is_default"`
	DefaultCommercialMotion *string                      `json:"default_commercial_motion"`
	Stages                  []UpdateCRMPipelineStageItem `json:"stages"`
}

// UpdateCRMPipelineStageItem is a stage within a pipeline update request.
type UpdateCRMPipelineStageItem struct {
	ID          *string `json:"id"`
	Name        string  `json:"name"`
	StageType   string  `json:"stage_type"`
	Position    int     `json:"position"`
	Probability int     `json:"probability"`
}

// CreateCRMDealRequest is the payload for creating a deal.
type CreateCRMDealRequest struct {
	WorkspaceID      string                 `json:"workspace_id"`
	Name             string                 `json:"name"`
	ContactID        string                 `json:"contact_id"`
	CompanyID        string                 `json:"company_id"`
	PipelineID       string                 `json:"pipeline_id"`
	StageID          string                 `json:"stage_id"`
	Amount           *float64               `json:"amount"`
	Currency         *string                `json:"currency"`
	CloseDate        *time.Time             `json:"close_date"`
	OwnerMemberID    *string                `json:"owner_member_id"`
	CommercialMotion *string                `json:"commercial_motion"`
	Probability      *int                   `json:"probability"`
	CustomProperties map[string]interface{} `json:"custom_properties"`
}

// SetCRMDealCustomerRequest changes the canonical customer for a deal. A
// company customer may optionally include a primary contact; an independent
// contact customer must not include a company.
type SetCRMDealCustomerRequest struct {
	WorkspaceID string `json:"workspace_id"`
	ContactID   string `json:"contact_id"`
	CompanyID   string `json:"company_id"`
}

// CRMDealCustomer is the resolved customer relationship for a deal.
type CRMDealCustomer struct {
	CustomerType     string `json:"customer_type"`
	CustomerID       string `json:"customer_id"`
	PrimaryContactID string `json:"primary_contact_id,omitempty"`
}

// UpdateCRMDealRequest is the payload for updating a deal.
type UpdateCRMDealRequest struct {
	Name                  *string                `json:"name"`
	PipelineID            *string                `json:"pipeline_id"`
	StageID               *string                `json:"stage_id"`
	Amount                *float64               `json:"amount"`
	ClearAmount           bool                   `json:"clear_amount"`
	Currency              *string                `json:"currency"`
	CloseDate             *time.Time             `json:"close_date"`
	ClearCloseDate        bool                   `json:"clear_close_date"`
	OwnerMemberID         *string                `json:"owner_member_id"`
	ClearOwner            bool                   `json:"clear_owner"`
	CommercialMotion      *string                `json:"commercial_motion"`
	ClearCommercialMotion bool                   `json:"clear_commercial_motion"`
	Probability           *int                   `json:"probability"`
	ClearProbability      bool                   `json:"clear_probability"`
	CustomProperties      map[string]interface{} `json:"custom_properties"`
}

// CRMDealListFilters applies filters when listing deals.
type CRMDealListFilters struct {
	PipelineID    *string
	StageID       *string
	OwnerMemberID *string
	ContactID     *string
	Search        *string
}
