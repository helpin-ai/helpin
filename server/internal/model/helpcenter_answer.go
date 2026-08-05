package model

import (
	"encoding/json"
	"time"
)

const (
	// HelpcenterAnswerStatusAnswered marks a grounded, validated answer.
	HelpcenterAnswerStatusAnswered = "answered"
	// HelpcenterAnswerStatusInsufficientEvidence marks a query the published
	// content could not answer confidently; the UI shows articles instead.
	HelpcenterAnswerStatusInsufficientEvidence = "insufficient_evidence"
)

// HelpcenterAnswer is a cached, validated AI answer for a public help center
// query. Rows double as the answer cache (CacheKey embeds a content
// fingerprint, so publishing invalidates naturally) and as query analytics.
type HelpcenterAnswer struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	CacheKey        string          `json:"-" gorm:"uniqueIndex;not null"`
	Locale          string          `json:"locale" gorm:"not null;default:'en'"`
	SpaceSlug       string          `json:"space_slug"`
	Query           string          `json:"query" gorm:"not null"`
	Status          string          `json:"status" gorm:"not null"`
	Answer          string          `json:"answer"`
	Citations       json.RawMessage `json:"citations" gorm:"type:jsonb;not null;default:'[]'"`
	Confidence      float64         `json:"confidence"`
	TokensUsed      int             `json:"tokens_used"`
	HelpfulCount    int             `json:"helpful_count" gorm:"not null;default:0"`
	NotHelpfulCount int             `json:"not_helpful_count" gorm:"not null;default:0"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName names the underlying table for HelpcenterAnswer.
func (HelpcenterAnswer) TableName() string { return "helpcenter_answers" }

// HelpcenterAnswerCitation links an answer to one published article.
type HelpcenterAnswerCitation struct {
	DocumentID     string  `json:"document_id"`
	Title          string  `json:"title"`
	Slug           string  `json:"slug"`
	PublicID       string  `json:"public_id"`
	SpaceSlug      string  `json:"space_slug"`
	CollectionSlug *string `json:"collection_slug,omitempty"`
	Snippet        string  `json:"snippet,omitempty"`
}

// HelpcenterAnswerRequest is the public ask payload.
type HelpcenterAnswerRequest struct {
	Query     string `json:"query"`
	SpaceSlug string `json:"space,omitempty"`
}

// HelpcenterAnswerResponse is the public ask response.
type HelpcenterAnswerResponse struct {
	AnswerID   string                     `json:"answer_id"`
	Status     string                     `json:"status"`
	Answer     string                     `json:"answer,omitempty"`
	Citations  []HelpcenterAnswerCitation `json:"citations"`
	Confidence float64                    `json:"confidence,omitempty"`
	Cached     bool                       `json:"cached"`
}

// HelpcenterAnswerFeedbackRequest records a thumbs vote on an answer.
type HelpcenterAnswerFeedbackRequest struct {
	IsHelpful bool `json:"is_helpful"`
}
