package model

import (
	"encoding/json"
	"time"
)

// ─── Gap categories (broad taxonomy, backend-only in v1) ───────────────────

const (
	SupportCoverageGapCategoryKnowledge  = "knowledge"
	SupportCoverageGapCategoryStructure  = "structure"
	SupportCoverageGapCategoryConflict   = "conflict"
	SupportCoverageGapCategoryContext    = "context"
	SupportCoverageGapCategoryAction     = "action"
	SupportCoverageGapCategoryWorkflow   = "workflow"
	SupportCoverageGapCategoryPolicy     = "policy"
	SupportCoverageGapCategoryEvaluation = "evaluation"
	SupportCoverageGapCategoryUnknown    = "unknown"
)

// ─── V1 gap types (docs-focused, exposed in UI) ───────────────────────────

const (
	SupportCoverageV1GapMissingArticle              = "missing_article"
	SupportCoverageV1GapWeakArticle                 = "weak_article"
	SupportCoverageV1GapOutdatedOrConflictingArticle = "outdated_or_conflicting_article"
	SupportCoverageV1GapNeedsReview                 = "needs_review"
)

// ─── Failure modes ─────────────────────────────────────────────────────────

const (
	SupportCoverageFailureNoRetrieval            = "no_retrieval"
	SupportCoverageFailureWeakRetrieval          = "weak_retrieval"
	SupportCoverageFailureLowConfidence          = "low_confidence"
	SupportCoverageFailureStuck                  = "stuck"
	SupportCoverageFailureCustomerRequestedHuman = "customer_requested_human"
	SupportCoverageFailureActionUnavailable      = "action_unavailable"
	SupportCoverageFailureContextUnavailable     = "context_unavailable"
	SupportCoverageFailurePolicyBlocked          = "policy_blocked"
	SupportCoverageFailureUnknown                = "unknown"
)

// ─── Source signals ────────────────────────────────────────────────────────

const (
	SupportCoverageSourceAIHandoff       = "ai_handoff"
	SupportCoverageSourceSelfService     = "self_service_search"
	SupportCoverageSourceArticleFeedback = "article_feedback"
	SupportCoverageSourceAgentFeedback   = "agent_feedback"
	SupportCoverageSourceHumanReply      = "human_reply"
	SupportCoverageSourceRecurrence      = "recurrence"
)

// ─── Gap statuses ──────────────────────────────────────────────────────────

const (
	SupportCoverageGapStatusOpen      = "open"
	SupportCoverageGapStatusDrafted   = "drafted"
	SupportCoverageGapStatusFixed     = "fixed"
	SupportCoverageGapStatusIgnored   = "ignored"
	SupportCoverageGapStatusMerged    = "merged"
	SupportCoverageGapStatusHumanOnly = "human_only"
)

// ─── Suggestion types ──────────────────────────────────────────────────────

const (
	SupportCoverageSuggestionCreateArticle = "create_article"
	SupportCoverageSuggestionUpdateArticle = "update_article"
	SupportCoverageSuggestionMergeArticle  = "merge_article"
)

// ─── Suggestion statuses ───────────────────────────────────────────────────

const (
	SupportCoverageSuggestionStatusDraft    = "draft"
	SupportCoverageSuggestionStatusApplied  = "applied"
	SupportCoverageSuggestionStatusRejected = "rejected"
)

// ─── Can-answer / can-resolve tristate ─────────────────────────────────────

const (
	SupportCoverageTriYes     = "yes"
	SupportCoverageTriNo      = "no"
	SupportCoverageTriUnknown = "unknown"
)

// ─── Coverage Models ───────────────────────────────────────────────────────

// SupportCoverageTopic is a stable cluster of related customer issues.
type SupportCoverageTopic struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_support_coverage_topics_workspace_issue_key,priority:1"`
	IssueKey    string    `json:"issue_key" gorm:"not null;uniqueIndex:idx_support_coverage_topics_workspace_issue_key,priority:2"`
	Title       string    `json:"title" gorm:"not null;default:''"`
	GapCount    int       `json:"gap_count" gorm:"not null;default:0"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCoverageTopic) TableName() string { return "support_coverage_topics" }

// SupportCoverageGap represents a durable blocker preventing
// autonomous resolution. V1 exposes only docs-related gap types.
type SupportCoverageGap struct {
	ID            string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID   string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_support_coverage_gaps_workspace_status_seen,priority:1"`
	TopicID       *string         `json:"topic_id" gorm:"type:uuid"`
	DedupeKey     string          `json:"dedupe_key" gorm:"not null"`
	GapCategory   string          `json:"gap_category" gorm:"not null;default:'unknown'"`
	V1GapType     string          `json:"v1_gap_type" gorm:"not null;default:'needs_review'"`
	Title         string          `json:"title" gorm:"not null;default:''"`
	IssueKey      string          `json:"issue_key" gorm:"not null;default:''"`
	Status        string          `json:"status" gorm:"not null;default:'open';index:idx_support_coverage_gaps_workspace_status_seen,priority:2"`
	Confidence    float64         `json:"confidence" gorm:"not null;default:0"`
	EvidenceCount int             `json:"evidence_count" gorm:"not null;default:0"`
	FailureMode   string          `json:"failure_mode" gorm:"not null;default:''"`
	SourceSignal  string          `json:"source_signal" gorm:"not null;default:''"`
	CanAnswer     *string         `json:"can_answer"`
	CanResolve    *string         `json:"can_resolve"`
	Metadata      json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	FirstSeenAt   time.Time       `json:"first_seen_at" gorm:"not null"`
	LastSeenAt    time.Time       `json:"last_seen_at" gorm:"not null;index:idx_support_coverage_gaps_workspace_status_seen,priority:3,sort:desc"`
	CreatedAt     time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCoverageGap) TableName() string { return "support_coverage_gaps" }

// SupportGapEvidence links a gap to a specific conversation, message,
// search, or article event that contributed to the gap's detection.
type SupportGapEvidence struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	GapID           string          `json:"gap_id" gorm:"type:uuid;not null;index:idx_support_gap_evidence_gap,priority:1"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null"`
	EvidenceType    string          `json:"evidence_type" gorm:"not null"`
	ConversationID  *string         `json:"conversation_id" gorm:"type:uuid"`
	MessageID       *string         `json:"message_id" gorm:"type:uuid"`
	WidgetSessionID *string         `json:"widget_session_id" gorm:"type:uuid"`
	DocumentID      *string         `json:"document_id" gorm:"type:uuid"`
	ArticlePublicID *string         `json:"article_public_id"`
	SourceSignal    string          `json:"source_signal" gorm:"not null;default:''"`
	Excerpt         string          `json:"excerpt" gorm:"type:text;not null;default:''"`
	Metadata        json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt       time.Time       `json:"created_at" gorm:"not null;index:idx_support_gap_evidence_gap,priority:2,sort:desc"`
}

func (SupportGapEvidence) TableName() string { return "support_gap_evidence" }

// SupportGapSuggestion is a proposed fix linked to a gap.
type SupportGapSuggestion struct {
	ID                 string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	GapID              string          `json:"gap_id" gorm:"type:uuid;not null;index"`
	WorkspaceID        string          `json:"workspace_id" gorm:"type:uuid;not null"`
	SuggestionType     string          `json:"suggestion_type" gorm:"not null"`
	Status             string          `json:"status" gorm:"not null;default:'draft'"`
	Title              string          `json:"title" gorm:"not null;default:''"`
	Content            json.RawMessage `json:"content" gorm:"type:jsonb"`
	EvidenceSummary    string          `json:"evidence_summary" gorm:"type:text;not null;default:''"`
	TargetSpaceID      *string         `json:"target_space_id" gorm:"type:uuid"`
	TargetCollectionID *string         `json:"target_collection_id" gorm:"type:uuid"`
	TargetDocumentID   *string         `json:"target_document_id" gorm:"type:uuid"`
	ResultDocumentID   *string         `json:"result_document_id" gorm:"type:uuid"`
	ResultArticleID    *string         `json:"result_article_id" gorm:"type:uuid"`
	AppliedAt          *time.Time      `json:"applied_at"`
	Metadata           json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt          time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportGapSuggestion) TableName() string { return "support_gap_suggestions" }

// SupportCoverageGapArticle links a gap to a related existing
// docs article (N:N relationship).
type SupportCoverageGapArticle struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	GapID       string    `json:"gap_id" gorm:"type:uuid;not null"`
	DocumentID  string    `json:"document_id" gorm:"type:uuid;not null"`
	WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportCoverageGapArticle) TableName() string { return "support_coverage_gap_articles" }

// SupportCoverageSnapshot stores pre-computed aggregate metrics.
// Read path for UI — avoids scanning raw events on every page load.
// Retention: keep 90 days per workspace, clean up during refresh.
type SupportCoverageSnapshot struct {
	ID          string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_support_coverage_snapshots_workspace,priority:1"`
	SnapshotAt  time.Time       `json:"snapshot_at" gorm:"not null;index:idx_support_coverage_snapshots_workspace,priority:2,sort:desc"`
	Metrics     json.RawMessage `json:"metrics" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt   time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportCoverageSnapshot) TableName() string { return "support_coverage_snapshots" }

// SupportCoverageDigestDelivery tracks weekly digest sends to
// prevent duplicate emails after restarts or repeated sweeps.
type SupportCoverageDigestDelivery struct {
	ID              string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string    `json:"workspace_id" gorm:"type:uuid;not null"`
	WeekStart       time.Time `json:"week_start" gorm:"not null"`
	RecipientUserID string    `json:"recipient_user_id" gorm:"type:uuid;not null"`
	SentAt          time.Time `json:"sent_at" gorm:"not null"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportCoverageDigestDelivery) TableName() string {
	return "support_coverage_digest_deliveries"
}

// ─── DTOs ──────────────────────────────────────────────────────────────────

// SupportCoverageGapFilter controls gap list queries.
type SupportCoverageGapFilter struct {
	Status    string `json:"status"`
	V1GapType string `json:"v1_gap_type"`
	IssueKey  string `json:"issue_key"`
	Search    string `json:"search"`
	Page      int    `json:"page"`
	PerPage   int    `json:"per_page"`
}

// SupportCoverageGapListItem is a row in the gap inbox table.
type SupportCoverageGapListItem struct {
	SupportCoverageGap
	TopicTitle      string  `json:"topic_title"`
	SuggestionCount int     `json:"suggestion_count"`
	RelatedArticleID *string `json:"related_article_id"`
}

// SupportCoverageGapDetail is the full gap detail with evidence
// and suggestions.
type SupportCoverageGapDetail struct {
	SupportCoverageGap
	TopicTitle      string                      `json:"topic_title"`
	Evidence        []SupportGapEvidence        `json:"evidence"`
	Suggestions     []SupportGapSuggestion      `json:"suggestions"`
	RelatedArticles []SupportCoverageGapArticle `json:"related_articles"`
}

// SupportConversationCoverageState tells the frontend whether
// docs-issue feedback was already submitted for a conversation.
type SupportConversationCoverageState struct {
	DocsIssueFeedbackSubmitted bool    `json:"docs_issue_feedback_submitted"`
	DocsIssueValue             *bool   `json:"docs_issue_value"`
	GapID                      *string `json:"gap_id"`
}

// SupportCoverageSummary is returned by the summary endpoint.
type SupportCoverageSummary struct {
	NewGapsThisWeek    int `json:"new_gaps_this_week"`
	TopRecurringGaps   int `json:"top_recurring_gaps"`
	GapsFixedThisWeek  int `json:"gaps_fixed_this_week"`
	TotalOpenGaps      int `json:"total_open_gaps"`
	TotalEvidenceCount int `json:"total_evidence_count"`
	HandoffsAfterFixes int `json:"handoffs_after_fixes"`
}
