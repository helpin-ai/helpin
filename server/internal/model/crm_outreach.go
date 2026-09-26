package model

import "time"

// Email content is reusable; enrollment snapshots never change when its source is edited.
type CRMEmailTemplate struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	OwnerID     string    `json:"owner_id" gorm:"type:uuid;not null"`
	Name        string    `json:"name"`
	Subject     string    `json:"subject"`
	BodyHTML    string    `json:"body_html" gorm:"type:text"`
	Shared      bool      `json:"shared"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName returns the stable persistence table name.
func (CRMEmailTemplate) TableName() string { return "crm_email_templates" }

// CRMSequenceStep defines an email or native task and its preceding delay.
type CRMSequenceStep struct {
	Kind      string `json:"kind"`           // email or task
	Mode      string `json:"mode,omitempty"` // automatic or review
	DelayDays int    `json:"delay_days"`
	Subject   string `json:"subject,omitempty"`
	BodyHTML  string `json:"body_html,omitempty"`
	TaskName  string `json:"task_name,omitempty"`
	TeamID    string `json:"team_id,omitempty"`
}

// CRMEmailSequence stores the editable sequence and optional stage enrollment rule.
type CRMEmailSequence struct {
	DailyNewRecipients int               `json:"daily_new_recipients" gorm:"default:25"`
	ID                 string            `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID        string            `json:"workspace_id" gorm:"type:uuid;not null;index"`
	OwnerID            string            `json:"owner_id" gorm:"type:uuid;not null"`
	Name               string            `json:"name"`
	Status             string            `json:"status" gorm:"index"`
	Version            int               `json:"version"`
	Steps              []CRMSequenceStep `json:"steps" gorm:"serializer:json;type:jsonb"`
	Timezone           string            `json:"timezone"`
	StartHour          int               `json:"start_hour"`
	EndHour            int               `json:"end_hour"`
	Weekdays           bool              `json:"weekdays"`
	IncludeSignature   bool              `json:"include_signature"`
	EntryStageID       string            `json:"entry_stage_id"`
	EntryAccountID     string            `json:"entry_account_id"`
	EntryAfter         *time.Time        `json:"entry_after,omitempty"`
	EntryCursorID      string            `json:"-"`
	EntryError         string            `json:"entry_error,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

// TableName returns the stable persistence table name.
func (CRMEmailSequence) TableName() string { return "crm_email_sequences" }

// CRMSequenceEnrollment stores recipient progress and a per-recipient content snapshot.
type CRMSequenceEnrollment struct {
	ID               string            `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID      string            `json:"workspace_id" gorm:"type:uuid;not null;index"`
	SequenceID       string            `json:"sequence_id" gorm:"type:uuid;not null;uniqueIndex:crm_sequence_recipient"`
	SequenceVersion  int               `json:"sequence_version"`
	SequenceName     string            `json:"sequence_name"`
	ContactID        string            `json:"contact_id" gorm:"type:uuid;index"`
	ContactName      string            `json:"contact_name"`
	Email            string            `json:"email" gorm:"not null;uniqueIndex:crm_sequence_recipient"`
	DealID           string            `json:"deal_id"`
	AccountID        string            `json:"account_id" gorm:"type:uuid;index"`
	OwnerID          string            `json:"owner_id" gorm:"type:uuid;index"`
	Status           string            `json:"status" gorm:"index"`
	StepIndex        int               `json:"step_index"`
	StepCount        int               `json:"step_count"`
	Steps            []CRMSequenceStep `json:"steps" gorm:"serializer:json;type:jsonb"`
	Timezone         string            `json:"timezone"`
	StartHour        int               `json:"start_hour"`
	EndHour          int               `json:"end_hour"`
	Weekdays         bool              `json:"weekdays"`
	NextAt           time.Time         `json:"next_at" gorm:"index"`
	LeaseUntil       *time.Time        `json:"-"`
	LeaseToken       string            `json:"-"`
	UnsubscribeToken string            `json:"-" gorm:"uniqueIndex"`
	Error            string            `json:"error,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// TableName returns the stable persistence table name.
func (CRMSequenceEnrollment) TableName() string { return "crm_sequence_enrollments" }

// CRMSequenceDelivery journals one external operation before execution.
type CRMSequenceDelivery struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey"`
	EnrollmentID string    `json:"enrollment_id" gorm:"type:uuid;not null;uniqueIndex:crm_sequence_delivery_step"`
	StepIndex    int       `json:"step_index" gorm:"not null;uniqueIndex:crm_sequence_delivery_step"`
	WorkspaceID  string    `json:"workspace_id" gorm:"type:uuid;index"`
	AccountID    string    `json:"account_id" gorm:"type:uuid;index"`
	Kind         string    `json:"kind"`
	Status       string    `json:"status"`
	Subject      string    `json:"subject"`
	BodyHTML     string    `json:"body_html" gorm:"type:text"`
	ResultID     string    `json:"result_id,omitempty"`
	Error        string    `json:"error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName returns the stable persistence table name.
func (CRMSequenceDelivery) TableName() string { return "crm_sequence_deliveries" }

// CRMEmailSuppression records a workspace-wide sequence opt-out.
type CRMEmailSuppression struct {
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;primaryKey"`
	Email       string    `json:"email" gorm:"primaryKey"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName returns the stable persistence table name.
func (CRMEmailSuppression) TableName() string { return "crm_email_suppressions" }

// CRMSequenceEnrollRequest identifies the approved sequence version, sender and recipients.
type CRMSequenceEnrollRequest struct {
	AccountID  string   `json:"account_id"`
	ContactIDs []string `json:"contact_ids"`
	DealID     string   `json:"deal_id"`
	Version    int      `json:"version"`
}

// CRMSequencePreview contains personalized steps and any blocking recipient issue.
type CRMSequencePreview struct {
	ContactID   string            `json:"contact_id"`
	ContactName string            `json:"contact_name"`
	Email       string            `json:"email"`
	Steps       []CRMSequenceStep `json:"steps"`
	Error       string            `json:"error,omitempty"`
}

// CRMSequenceEnrollmentFilter limits and filters activity without loading message bodies.
type CRMSequenceEnrollmentFilter struct {
	Page   int
	Search string
	Status string
}
