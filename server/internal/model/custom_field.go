package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/customfields"
)

// Objects that can carry custom fields.
const (
	CustomFieldObjectConversation = "conversation"
	CustomFieldObjectCRMContact   = "crm_contact"
	CustomFieldObjectCRMCompany   = "crm_company"
)

// CustomFieldDefinitionLimit bounds the fields one object type keeps.
const CustomFieldDefinitionLimit = 200

// CustomFieldOptions are a select field's choices, stored as JSON.
type CustomFieldOptions []customfields.Option

// Value implements driver.Valuer.
func (o CustomFieldOptions) Value() (driver.Value, error) {
	if o == nil {
		return "[]", nil
	}
	data, err := json.Marshal([]customfields.Option(o))
	return string(data), err
}

// Scan implements sql.Scanner.
func (o *CustomFieldOptions) Scan(src any) error {
	var data []byte
	switch value := src.(type) {
	case nil:
		*o = CustomFieldOptions{}
		return nil
	case []byte:
		data = value
	case string:
		data = []byte(value)
	default:
		return fmt.Errorf("unsupported type for CustomFieldOptions: %T", src)
	}
	var options []customfields.Option
	if err := json.Unmarshal(data, &options); err != nil {
		return err
	}
	*o = options
	return nil
}

// CustomFieldDefinition gives a custom field key of one object type a label,
// a type and, for select fields, options. Keys never change; archived
// definitions keep their values on records.
type CustomFieldDefinition struct {
	ID          string             `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string             `json:"workspace_id" gorm:"type:uuid;not null"`
	ObjectType  string             `json:"object_type" gorm:"not null"`
	Key         string             `json:"key" gorm:"not null"`
	Label       string             `json:"label" gorm:"not null;default:''"`
	Description string             `json:"description" gorm:"not null;default:''"`
	FieldType   string             `json:"field_type" gorm:"not null"`
	Options     CustomFieldOptions `json:"options" gorm:"type:jsonb;not null;default:'[]'"`
	Position    int                `json:"position" gorm:"type:integer;not null;default:0"`
	ArchivedAt  *time.Time         `json:"archived_at,omitempty" gorm:"type:timestamptz"`
	CreatedByID *string            `json:"created_by_id,omitempty" gorm:"type:uuid"`
	CreatedAt   time.Time          `json:"created_at" gorm:"type:timestamptz;not null;default:now();autoCreateTime"`
	UpdatedAt   time.Time          `json:"updated_at" gorm:"type:timestamptz;not null;default:now();autoUpdateTime"`

	// DisplayLabel is the label, or the key written out.
	DisplayLabel string `json:"display_label" gorm:"-"`
	// UsageCount and MismatchCount come from a bounded scan of recent
	// records: how many hold a value, and how many of those do not fit.
	UsageCount    int  `json:"usage_count" gorm:"-"`
	MismatchCount int  `json:"mismatch_count" gorm:"-"`
	UsageSampled  bool `json:"usage_sampled" gorm:"-"`
}

func (CustomFieldDefinition) TableName() string { return "custom_field_definitions" }

// Definition is the validation view of the field.
func (d CustomFieldDefinition) Definition() customfields.Definition {
	return customfields.Definition{Key: d.Key, Label: d.Label, Type: d.FieldType, Options: d.Options}
}

// CustomFieldDefinitionRequest creates (with Key) or edits a field.
type CustomFieldDefinitionRequest struct {
	Key         string                `json:"key"`
	Label       string                `json:"label"`
	Description string                `json:"description"`
	FieldType   string                `json:"field_type"`
	Options     []customfields.Option `json:"options"`
}

// ReorderCustomFieldDefinitionsRequest sets the display order of fields.
type ReorderCustomFieldDefinitionsRequest struct {
	IDs []string `json:"ids"`
}

// SetCustomFieldValuesRequest writes values for defined fields on one record;
// a null value clears the field. Keys not listed are left alone.
type SetCustomFieldValuesRequest struct {
	Values map[string]any `json:"values"`
}
