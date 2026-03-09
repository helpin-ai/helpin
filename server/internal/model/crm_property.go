package model

import "time"

// CRM property field types.
const (
	CRMFieldText        = "text"
	CRMFieldNumber      = "number"
	CRMFieldDate        = "date"
	CRMFieldSelect      = "select"
	CRMFieldMultiSelect = "multiselect"
	CRMFieldBoolean     = "boolean"
	CRMFieldURL         = "url"
	CRMFieldEmail       = "email"
	CRMFieldPhone       = "phone"
	CRMFieldCurrency    = "currency"
)

// CRMPropertyDefinition defines a custom property for CRM objects.
type CRMPropertyDefinition struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID  string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ObjectType   string    `json:"object_type" gorm:"not null"` // contact, company, deal
	InternalName string    `json:"internal_name" gorm:"not null"`
	Label        string    `json:"label" gorm:"not null"`
	FieldType    string    `json:"field_type" gorm:"not null;default:'text'"`
	Options      JSONB     `json:"options" gorm:"type:jsonb;default:'[]'"` // for select/multiselect
	GroupName    *string   `json:"group_name"`
	IsRequired   bool      `json:"is_required" gorm:"not null;default:false"`
	IsSystem     bool      `json:"is_system" gorm:"not null;default:false"`
	Position     int       `json:"position" gorm:"not null;default:0"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMPropertyDefinition) TableName() string { return "crm_property_definitions" }

// CRMPropertyGroup groups property definitions together.
type CRMPropertyGroup struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ObjectType  string    `json:"object_type" gorm:"not null"`
	Name        string    `json:"name" gorm:"not null"`
	Position    int       `json:"position" gorm:"not null;default:0"`
	IsSystem    bool      `json:"is_system" gorm:"not null;default:false"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMPropertyGroup) TableName() string { return "crm_property_groups" }

// CreateCRMPropertyDefinitionRequest is the payload for creating a property definition.
type CreateCRMPropertyDefinitionRequest struct {
	WorkspaceID  string                 `json:"workspace_id"`
	ObjectType   string                 `json:"object_type"`
	InternalName string                 `json:"internal_name"`
	Label        string                 `json:"label"`
	FieldType    string                 `json:"field_type"`
	Options      map[string]interface{} `json:"options"`
	GroupName    *string                `json:"group_name"`
	IsRequired   *bool                  `json:"is_required"`
	Position     *int                   `json:"position"`
}

// UpdateCRMPropertyDefinitionRequest is the payload for updating a property definition.
type UpdateCRMPropertyDefinitionRequest struct {
	Label      *string                `json:"label"`
	FieldType  *string                `json:"field_type"`
	Options    map[string]interface{} `json:"options"`
	GroupName  *string                `json:"group_name"`
	IsRequired *bool                  `json:"is_required"`
	Position   *int                   `json:"position"`
}

// CreateCRMPropertyGroupRequest is the payload for creating a property group.
type CreateCRMPropertyGroupRequest struct {
	WorkspaceID string `json:"workspace_id"`
	ObjectType  string `json:"object_type"`
	Name        string `json:"name"`
	Position    *int   `json:"position"`
}

// UpdateCRMPropertyGroupRequest is the payload for updating a property group.
type UpdateCRMPropertyGroupRequest struct {
	Name     *string `json:"name"`
	Position *int    `json:"position"`
}
