package model

// CRM record upsert object types.
const (
	CRMUpsertObjectCompany = "company"
	CRMUpsertObjectContact = "contact"
)

// CRM record upsert match keys.
const (
	CRMUpsertMatchID         = "id"
	CRMUpsertMatchExternalID = "external_id"
	CRMUpsertMatchDomain     = "domain"
	CRMUpsertMatchEmail      = "email"
)

// CRM record upsert modes.
const (
	CRMUpsertModeUpsert     = "upsert"
	CRMUpsertModeCreateOnly = "create_only"
	CRMUpsertModeUpdateOnly = "update_only"
)

// CRM record upsert per-record outcomes.
const (
	CRMUpsertActionCreated   = "created"
	CRMUpsertActionUpdated   = "updated"
	CRMUpsertActionUnchanged = "unchanged"
	CRMUpsertActionSkipped   = "skipped"
	CRMUpsertActionError     = "error"
)

// CRMRecordUpsertRequest creates or updates a bounded batch of CRM records
// matched on one stable key.
type CRMRecordUpsertRequest struct {
	WorkspaceID string
	ActorUserID string
	ObjectType  string                 `json:"object_type"`
	MatchOn     string                 `json:"match_on"`
	Mode        string                 `json:"mode"`
	FillOnly    bool                   `json:"fill_only"`
	Records     []CRMRecordUpsertInput `json:"records"`
}

// CRMRecordUpsertInput is one record in a CRM upsert batch.
type CRMRecordUpsertInput struct {
	Ref              string               `json:"ref"`
	ID               string               `json:"id"`
	Fields           map[string]any       `json:"fields"`
	CustomProperties map[string]any       `json:"custom_properties"`
	Note             *CRMRecordUpsertNote `json:"note"`
}

// CRMRecordUpsertNote is evidence logged as a note activity on the record.
type CRMRecordUpsertNote struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// CRMRecordUpsertResult reports what happened to one record.
type CRMRecordUpsertResult struct {
	Ref       string   `json:"ref,omitempty"`
	ID        string   `json:"id,omitempty"`
	Action    string   `json:"action"`
	Changed   []string `json:"changed,omitempty"`
	Kept      []string `json:"kept,omitempty"`
	NoteAdded bool     `json:"note_added,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// CRMRecordUpsertResponse summarizes a CRM upsert batch.
type CRMRecordUpsertResponse struct {
	ObjectType string                  `json:"object_type"`
	Counts     map[string]int          `json:"counts"`
	Results    []CRMRecordUpsertResult `json:"results"`
	// NewCustomPropertyKeys lists custom property keys this batch introduced
	// to the workspace, so callers can spot near-duplicates of existing keys.
	NewCustomPropertyKeys []string `json:"new_custom_property_keys,omitempty"`
}
