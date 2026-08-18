package model

import "time"

const (
	CuratedGuidanceStatusActive   = "active"
	CuratedGuidanceStatusDisabled = "disabled"
)

// CuratedGuidance is a short administrator-authored canonical answer. It has
// maximum authority only within its workspace, agent, audience, brand,
// language, and validity scope.
type CuratedGuidance struct {
	ID                  string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_curated_guidance_scope,priority:1"`
	AgentID             string          `json:"agent_id" gorm:"type:uuid;not null;index:idx_curated_guidance_scope,priority:2"`
	Title               string          `json:"title" gorm:"not null"`
	QuestionPatterns    DocsStringArray `json:"question_patterns" gorm:"type:text[]"`
	Answer              string          `json:"answer" gorm:"type:text;not null"`
	Intent              string          `json:"intent" gorm:"not null;default:'unknown';index"`
	Topics              DocsStringArray `json:"topics" gorm:"type:text[]"`
	Language            string          `json:"language" gorm:"not null;default:'';index"`
	AudiencePolicyID    *string         `json:"audience_policy_id,omitempty" gorm:"type:uuid;index"`
	BrandID             *string         `json:"brand_id,omitempty" gorm:"type:uuid;index"`
	Status              string          `json:"status" gorm:"not null;default:'active';index:idx_curated_guidance_scope,priority:3"`
	ValidFrom           *time.Time      `json:"valid_from,omitempty" gorm:"type:timestamptz;index"`
	ValidUntil          *time.Time      `json:"valid_until,omitempty" gorm:"type:timestamptz;index"`
	Embedding           *string         `json:"-" gorm:"type:vector(1536)"`
	EmbeddingProvider   string          `json:"embedding_provider,omitempty"`
	EmbeddingModel      string          `json:"embedding_model,omitempty"`
	EmbeddingVersion    string          `json:"embedding_version,omitempty"`
	EmbeddingDimensions int             `json:"embedding_dimensions,omitempty"`
	CreatedByID         string          `json:"created_by_id" gorm:"type:uuid;not null"`
	CreatedAt           time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CuratedGuidance) TableName() string { return "curated_guidance" }

type CreateCuratedGuidanceRequest struct {
	Title            string     `json:"title"`
	QuestionPatterns []string   `json:"question_patterns"`
	Answer           string     `json:"answer"`
	Intent           string     `json:"intent"`
	Topics           []string   `json:"topics"`
	Language         string     `json:"language"`
	AudiencePolicyID *string    `json:"audience_policy_id"`
	BrandID          *string    `json:"brand_id"`
	ValidFrom        *time.Time `json:"valid_from"`
	ValidUntil       *time.Time `json:"valid_until"`
}

type UpdateCuratedGuidanceRequest struct {
	Title            *string    `json:"title"`
	QuestionPatterns []string   `json:"question_patterns"`
	Answer           *string    `json:"answer"`
	Intent           *string    `json:"intent"`
	Topics           []string   `json:"topics"`
	Language         *string    `json:"language"`
	AudiencePolicyID *string    `json:"audience_policy_id"`
	BrandID          *string    `json:"brand_id"`
	Status           *string    `json:"status"`
	ValidFrom        *time.Time `json:"valid_from"`
	ValidUntil       *time.Time `json:"valid_until"`
}
