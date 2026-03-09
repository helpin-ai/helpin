package model

import "time"

// CRMWritingProfile represents a user's AI writing style profile.
type CRMWritingProfile struct {
	ID              string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
	MemberID        string     `json:"member_id" gorm:"type:uuid;not null;index"`
	StyleAttributes JSONB      `json:"style_attributes" gorm:"type:jsonb;default:'{}'"` // tone, formality, avg_length, etc.
	SampleCount     int        `json:"sample_count" gorm:"not null;default:0"`
	LastAnalyzedAt  *time.Time `json:"last_analyzed_at"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CRMWritingProfile) TableName() string { return "crm_writing_profiles" }

// CreateCRMWritingProfileRequest is the payload for creating a writing profile.
type CreateCRMWritingProfileRequest struct {
	WorkspaceID     string                 `json:"workspace_id"`
	MemberID        string                 `json:"member_id"`
	StyleAttributes map[string]interface{} `json:"style_attributes"`
}

// UpdateCRMWritingProfileRequest is the payload for updating a writing profile.
type UpdateCRMWritingProfileRequest struct {
	StyleAttributes map[string]interface{} `json:"style_attributes"`
	SampleCount     *int                   `json:"sample_count"`
}
