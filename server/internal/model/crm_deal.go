package model

import "time"

// CRM pipeline stage types.
const (
	CRMStageTypeOpen = "open"
	CRMStageTypeWon  = "won"
	CRMStageTypeLost = "lost"
)

// CRMPipeline represents a sales pipeline.
type CRMPipeline struct {
	ID          string             `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string             `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name        string             `json:"name" gorm:"not null"`
	IsDefault   bool               `json:"is_default" gorm:"not null;default:false"`
	Position    int                `json:"position" gorm:"not null;default:0"`
	Stages      []CRMPipelineStage `json:"stages,omitempty" gorm:"foreignKey:PipelineID"`
	CreatedAt   time.Time          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time          `json:"updated_at" gorm:"autoUpdateTime"`
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
	WorkspaceID string                       `json:"workspace_id"`
	Name        string                       `json:"name"`
	IsDefault   *bool                        `json:"is_default"`
	Stages      []CreateCRMPipelineStageItem `json:"stages"`
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
	Name      *string                       `json:"name"`
	IsDefault *bool                         `json:"is_default"`
	Stages    []UpdateCRMPipelineStageItem  `json:"stages"`
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
	PipelineID       string                 `json:"pipeline_id"`
	StageID          string                 `json:"stage_id"`
	Amount           *float64               `json:"amount"`
	Currency         *string                `json:"currency"`
	CloseDate        *time.Time             `json:"close_date"`
	OwnerMemberID    *string                `json:"owner_member_id"`
	Probability      *int                   `json:"probability"`
	CustomProperties map[string]interface{} `json:"custom_properties"`
}

// UpdateCRMDealRequest is the payload for updating a deal.
type UpdateCRMDealRequest struct {
	Name             *string                `json:"name"`
	PipelineID       *string                `json:"pipeline_id"`
	StageID          *string                `json:"stage_id"`
	Amount           *float64               `json:"amount"`
	Currency         *string                `json:"currency"`
	CloseDate        *time.Time             `json:"close_date"`
	OwnerMemberID    *string                `json:"owner_member_id"`
	Probability      *int                   `json:"probability"`
	CustomProperties map[string]interface{} `json:"custom_properties"`
}

// CRMDealListFilters applies filters when listing deals.
type CRMDealListFilters struct {
	PipelineID    *string
	StageID       *string
	OwnerMemberID *string
	Search        *string
}
