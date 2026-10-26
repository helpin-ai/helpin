package model

import "time"

// CSAT channels record where the customer submitted a rating.
const (
	SupportCSATChannelWidget = "widget"
	SupportCSATChannelPortal = "portal"
)

// Who resolved the conversation when the customer rated it.
const (
	SupportCSATResolvedByAI    = "ai"
	SupportCSATResolvedByHuman = "human"
)

// SupportCSATCommentMaxRunes bounds the optional customer comment.
const SupportCSATCommentMaxRunes = 1000

// SupportCSATResponse is the customer's satisfaction rating for one resolved
// conversation. A customer can change the rating while the conversation stays
// resolved; the row is updated rather than duplicated.
type SupportCSATResponse struct {
	ID             string  `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string  `json:"workspace_id" gorm:"type:uuid;not null;index:idx_support_csat_responses_workspace_submitted,priority:1"`
	ConversationID string  `json:"conversation_id" gorm:"type:uuid;not null;uniqueIndex"`
	Rating         int     `json:"rating" gorm:"type:integer;not null"`
	Comment        *string `json:"comment,omitempty"`
	Channel        string  `json:"channel" gorm:"not null"`
	ResolvedBy     string  `json:"resolved_by" gorm:"not null"`
	// AssignedUserID is the teammate who owned the conversation when it was rated.
	AssignedUserID *string   `json:"assigned_user_id,omitempty" gorm:"type:uuid"`
	SubmittedAt    time.Time `json:"submitted_at" gorm:"type:timestamptz;not null;index:idx_support_csat_responses_workspace_submitted,priority:2"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCSATResponse) TableName() string { return "support_csat_responses" }

// SupportCSATSubmission is a customer rating from a public surface.
type SupportCSATSubmission struct {
	WorkspaceID    string
	ConversationID string
	Channel        string
	Rating         int
	Comment        string
}

// SupportCSATSummary is the rating shown to the customer and on the conversation.
type SupportCSATSummary struct {
	Rating      int       `json:"rating"`
	Comment     *string   `json:"comment,omitempty"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// WidgetCSATRequest is the widget's session-authenticated rating submission.
type WidgetCSATRequest struct {
	SessionToken string `json:"session_token"`
	Rating       int    `json:"rating"`
	Comment      string `json:"comment"`
}

// SupportCSATRatingLabel names a 1-5 rating for timeline entries.
func SupportCSATRatingLabel(rating int) string {
	switch rating {
	case 1:
		return "Very unsatisfied"
	case 2:
		return "Unsatisfied"
	case 3:
		return "Neutral"
	case 4:
		return "Satisfied"
	case 5:
		return "Very satisfied"
	default:
		return ""
	}
}
