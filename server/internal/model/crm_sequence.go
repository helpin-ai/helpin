package model

import "time"

// CRM sequence statuses.
const (
	CRMSequenceStatusDraft  = "draft"
	CRMSequenceStatusActive = "active"
	CRMSequenceStatusPaused = "paused"
)

// CRM sequence step types.
const (
	CRMSequenceStepEmail = "email"
	CRMSequenceStepDelay = "delay"
	CRMSequenceStepTask  = "task"
)

// CRM enrollment statuses.
const (
	CRMEnrollmentStatusActive       = "active"
	CRMEnrollmentStatusCompleted    = "completed"
	CRMEnrollmentStatusPaused       = "paused"
	CRMEnrollmentStatusBounced      = "bounced"
	CRMEnrollmentStatusUnsubscribed = "unsubscribed"
	CRMEnrollmentStatusExited       = "exited"
)

// CRMSequence represents an outbound sequence of steps.
type CRMSequence struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Name            string    `json:"name" gorm:"not null"`
	Description     *string   `json:"description"`
	Status          string    `json:"status" gorm:"not null;default:'draft'"` // draft, active, paused
	Steps           JSONB     `json:"steps" gorm:"type:jsonb;default:'[]'"`   // array of step objects
	EnrollmentCount int       `json:"enrollment_count" gorm:"not null;default:0"`
	CreatedBy       *string   `json:"created_by" gorm:"type:uuid"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMSequence) TableName() string { return "crm_sequences" }

// CRMSequenceEnrollment represents a contact's enrollment in a sequence.
type CRMSequenceEnrollment struct {
	ID                 string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID        string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	SequenceID         string     `json:"sequence_id" gorm:"type:uuid;not null;index"`
	ContactID          string     `json:"contact_id" gorm:"type:uuid;not null;index"`
	CurrentStep        int        `json:"current_step" gorm:"not null;default:0"`
	Status             string     `json:"status" gorm:"not null;default:'active'"` // active, completed, paused, bounced, unsubscribed, exited
	EnrolledAt         time.Time  `json:"enrolled_at" gorm:"not null"`
	CompletedAt        *time.Time `json:"completed_at"`
	ExitReason         *string    `json:"exit_reason"`
	LastStepExecutedAt *time.Time `json:"last_step_executed_at"`
	CreatedAt          time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMSequenceEnrollment) TableName() string { return "crm_sequence_enrollments" }

// CreateCRMSequenceRequest is the payload for creating a sequence.
type CreateCRMSequenceRequest struct {
	WorkspaceID string                 `json:"workspace_id"`
	Name        string                 `json:"name"`
	Description *string                `json:"description"`
	Status      *string                `json:"status"`
	Steps       map[string]interface{} `json:"steps"`
	CreatedBy   *string                `json:"created_by"`
}

// UpdateCRMSequenceRequest is the payload for updating a sequence.
type UpdateCRMSequenceRequest struct {
	Name        *string                `json:"name"`
	Description *string                `json:"description"`
	Status      *string                `json:"status"`
	Steps       map[string]interface{} `json:"steps"`
}

// CreateCRMSequenceEnrollmentRequest is the payload for enrolling a contact.
type CreateCRMSequenceEnrollmentRequest struct {
	WorkspaceID string `json:"workspace_id"`
	SequenceID  string `json:"sequence_id"`
	ContactID   string `json:"contact_id"`
}

// UpdateCRMSequenceEnrollmentRequest is the payload for updating an enrollment.
type UpdateCRMSequenceEnrollmentRequest struct {
	Status     *string `json:"status"`
	ExitReason *string `json:"exit_reason"`
}

// CRMSequenceListFilters applies filters when listing sequences.
type CRMSequenceListFilters struct {
	Status *string
	Search *string
}

// CRMSequenceEnrollmentListFilters applies filters when listing enrollments.
type CRMSequenceEnrollmentListFilters struct {
	SequenceID *string
	ContactID  *string
	Status     *string
}
