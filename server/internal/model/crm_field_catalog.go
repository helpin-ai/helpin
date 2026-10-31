package model

// CRM object types the field catalog describes.
const (
	CRMFieldObjectContact = "contact"
	CRMFieldObjectCompany = "company"
)

// CRMContactFilterOptions lists the filterable values in use on contacts.
type CRMContactFilterOptions struct {
	CustomProperties []CRMCustomPropertySummary `json:"custom_properties"`
	ScannedContacts  int                        `json:"scanned_contacts"`
	// Truncated reports that only the most recently updated contacts were scanned.
	Truncated bool `json:"truncated"`
}

// CRMFieldDescriptor describes one native field a CRM list filter accepts.
type CRMFieldDescriptor struct {
	Field     string                `json:"field"`
	Type      string                `json:"type"`
	Operators []QueryFilterOperator `json:"operators"`
	// Values lists the allowed values of an enum field.
	Values      []string `json:"values,omitempty"`
	Description string   `json:"description,omitempty"`
}

// CRMCustomPropertyDescriptor is a custom property summary plus the filter
// field and operators that apply to it.
type CRMCustomPropertyDescriptor struct {
	CRMCustomPropertySummary
	Field     string                `json:"field"`
	Operators []QueryFilterOperator `json:"operators"`
	// Definition is the workspace's field definition for the key: its label,
	// type and options. Absent for keys that are only in use.
	Definition *CustomFieldDefinition `json:"definition,omitempty"`
}

// CRMFieldCatalog describes every field a contact or company list filter
// accepts, with the custom property keys and values currently in use.
type CRMFieldCatalog struct {
	ObjectType       string                        `json:"object_type"`
	Fields           []CRMFieldDescriptor          `json:"fields"`
	CustomProperties []CRMCustomPropertyDescriptor `json:"custom_properties"`
	// ScannedRecords is how many records custom properties were read from.
	ScannedRecords int `json:"scanned_records"`
	// Truncated reports that only the most recently updated records were scanned.
	Truncated bool `json:"truncated"`
	// Behavior reports how many records have synced product-usage data.
	Behavior *CRMBehaviorCoverage `json:"behavior,omitempty"`
}

// CRMCustomPropertyValues lists the values in use for one custom property.
type CRMCustomPropertyValues struct {
	ObjectType string                `json:"object_type"`
	Key        string                `json:"key"`
	Field      string                `json:"field"`
	Found      bool                  `json:"found"`
	ValueType  string                `json:"value_type,omitempty"`
	Count      int                   `json:"count"`
	Values     []CRMFilterValueCount `json:"values"`
	HasMore    bool                  `json:"has_more_values"`
	// ValueQuery echoes the case-insensitive substring values were matched on.
	ValueQuery     string                `json:"value_query,omitempty"`
	Operators      []QueryFilterOperator `json:"operators,omitempty"`
	ScannedRecords int                   `json:"scanned_records"`
	Truncated      bool                  `json:"truncated"`
}

// CRMFieldCatalogRequest selects a catalog or one custom property's values.
type CRMFieldCatalogRequest struct {
	ObjectType string
	// Key, when set, returns the values of custom_properties.<Key>.
	Key        string
	ValueQuery string
	Limit      int
}
