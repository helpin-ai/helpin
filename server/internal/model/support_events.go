package model

import (
	"encoding/json"
	"time"
)

// ─── Support Event Types ───────────────────────────────────────────────────
// These are shared operational support events written to the support_events
// table. Coverage, analytics, and future consumers all read from this table.
// Values are stable lowercase snake_case — do not rename without migration.

const (
	SupportEventConversationCreated       = "conversation_created"
	SupportEventCustomerMessageCreated    = "customer_message_created"
	SupportEventHumanReplySent            = "human_reply_sent"
	SupportEventConversationStatusChanged = "conversation_status_changed"
	SupportEventConversationAssigned      = "conversation_assigned"
	SupportEventAIAttemptStarted          = "ai_attempt_started"
	SupportEventAIRetrievalCompleted      = "ai_retrieval_completed"
	SupportEventAIPreRouterDecision       = "ai_pre_router_decision"
	SupportEventAIAnswerSent              = "ai_answer_sent"
	SupportEventAIHandoffTriggered        = "ai_handoff_triggered"
	SupportEventConversationResolved      = "conversation_resolved"
	SupportEventWidgetSearchPerformed     = "widget_search_performed"
	SupportEventWidgetArticleOpened       = "widget_article_opened"
	SupportEventAIAnswerFeedback          = "ai_answer_feedback"
	SupportEventArticleFeedback           = "article_feedback_submitted"
	SupportEventHumanReplyAfterAI         = "human_reply_after_ai"
	SupportEventDocsIssueFeedback         = "docs_issue_feedback"
)

// ─── Actor types ───────────────────────────────────────────────────────────

const (
	SupportEventActorCustomer = "customer"
	SupportEventActorAgent    = "agent"
	SupportEventActorAI       = "ai"
	SupportEventActorSystem   = "system"
)

// ─── Channels ──────────────────────────────────────────────────────────────

const (
	SupportEventChannelWidget     = "widget"
	SupportEventChannelEmail      = "email"
	SupportEventChannelHelpCenter = "help_center"
	SupportEventChannelInternal   = "internal"
)

// SupportEvent is an append-only operational event capturing signals
// from support conversations, AI interactions, widget/help center
// usage, and feedback. This table is the shared foundation for
// Coverage, analytics, and future intelligence features.
type SupportEvent struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_support_events_workspace_time,priority:1;index:idx_support_events_workspace_type_time,priority:1"`
	EventType       string          `json:"event_type" gorm:"not null;index:idx_support_events_workspace_type_time,priority:2"`
	ConversationID  *string         `json:"conversation_id" gorm:"type:uuid;index:idx_support_events_conversation_time,priority:1"`
	MessageID       *string         `json:"message_id" gorm:"type:uuid"`
	WidgetSessionID *string         `json:"widget_session_id" gorm:"type:uuid"`
	AnonymousID     *string         `json:"anonymous_id"`
	DocumentID      *string         `json:"document_id" gorm:"type:uuid"`
	ArticleID       *string         `json:"article_id" gorm:"type:uuid"`
	ArticlePublicID *string         `json:"article_public_id"`
	ActorType       string          `json:"actor_type" gorm:"not null;default:''"`
	Channel         string          `json:"channel" gorm:"not null;default:''"`
	Source          string          `json:"source" gorm:"not null;default:''"`
	IssueKey        string          `json:"issue_key" gorm:"not null;default:''"`
	IssueSummary    string          `json:"issue_summary" gorm:"not null;default:''"`
	FailureMode     string          `json:"failure_mode" gorm:"not null;default:''"`
	SourceSignal    string          `json:"source_signal" gorm:"not null;default:''"`
	CanAnswer       *string         `json:"can_answer"`
	CanResolve      *string         `json:"can_resolve"`
	Metadata        json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	OccurredAt      time.Time       `json:"occurred_at" gorm:"not null;index:idx_support_events_workspace_time,priority:2,sort:desc;index:idx_support_events_workspace_type_time,priority:3,sort:desc;index:idx_support_events_conversation_time,priority:2,sort:desc"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportEvent) TableName() string { return "support_events" }

// SupportEventFilter controls event list queries.
type SupportEventFilter struct {
	EventType      string `json:"event_type"`
	ConversationID string `json:"conversation_id"`
	Limit          int    `json:"limit"`
}
