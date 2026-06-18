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
	SupportCoverageV1GapMissingArticle               = "missing_article"
	SupportCoverageV1GapWeakArticle                  = "weak_article"
	SupportCoverageV1GapOutdatedOrConflictingArticle = "outdated_or_conflicting_article"
	SupportCoverageV1GapNeedsReview                  = "needs_review"
)

// ─── Failure modes ─────────────────────────────────────────────────────────

const (
	SupportCoverageFailureMissingContent         = "missing_content"
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
	SupportCoverageSourceAIHandoff                   = "ai_handoff"
	SupportCoverageSourceSelfService                 = "self_service_search"
	SupportCoverageSourceArticleFeedback             = "article_feedback"
	SupportCoverageSourceAgentFeedback               = "agent_feedback"
	SupportCoverageSourceHumanReply                  = "human_reply"
	SupportCoverageSourceRecurrence                  = "recurrence"
	SupportCoverageSourceConversationResolvedByHuman = "conversation_resolved_by_human"
)

// ─── Gap statuses ──────────────────────────────────────────────────────────

const (
	SupportCoverageGapStatusOpen     = "open"
	SupportCoverageGapStatusDone     = "done"
	SupportCoverageGapStatusRejected = "rejected"

	// Deprecated: kept for one release while old callers migrate to open/done/rejected.
	SupportCoverageGapStatusDrafted = "drafted"
	// Deprecated: kept for one release while old callers migrate to open/done/rejected.
	SupportCoverageGapStatusFixed = "fixed"
	// Deprecated: kept for one release while old callers migrate to open/done/rejected.
	SupportCoverageGapStatusIgnored = "ignored"
	// Deprecated: merged/human_only remain readable for legacy rows but should not be written by v2.
	SupportCoverageGapStatusMerged    = "merged"
	SupportCoverageGapStatusHumanOnly = "human_only"
)

// ─── Gap sources ───────────────────────────────────────────────────────────

const (
	SupportCoverageGapSourceEventDetection            = "event_detection"
	SupportCoverageGapSourceDailyConversationAnalysis = "daily_conversation_analysis"
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

// ─── Cluster rebuild statuses ─────────────────────────────────────────────

const (
	SupportCoverageClusterRebuildStatusRunning   = "running"
	SupportCoverageClusterRebuildStatusCompleted = "completed"
	SupportCoverageClusterRebuildStatusFailed    = "failed"

	SupportCoverageMergeSuggestionStatusPending   = "pending"
	SupportCoverageMergeSuggestionStatusApplied   = "applied"
	SupportCoverageMergeSuggestionStatusDismissed = "dismissed"
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
	ID             string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string     `json:"workspace_id" gorm:"type:uuid;not null;uniqueIndex:idx_support_coverage_topics_workspace_issue_key,priority:1"`
	IssueKey       string     `json:"issue_key" gorm:"not null;uniqueIndex:idx_support_coverage_topics_workspace_issue_key,priority:2"`
	Title          string     `json:"title" gorm:"not null;default:''"`
	GapCount       int        `json:"gap_count" gorm:"not null;default:0"`
	ClusterKey     *string    `json:"cluster_key" gorm:"type:text"`
	CanonicalTitle *string    `json:"canonical_title" gorm:"type:text"`
	LastEnrichedAt *time.Time `json:"last_enriched_at"`
	CooldownUntil  *time.Time `json:"cooldown_until"`
	CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCoverageTopic) TableName() string { return "support_coverage_topics" }

// SupportCoverageGap represents a durable blocker preventing
// autonomous resolution. V1 exposes only docs-related gap types.
type SupportCoverageGap struct {
	ID                       string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID              string          `json:"workspace_id" gorm:"type:uuid;not null;index:idx_support_coverage_gaps_workspace_status_seen,priority:1"`
	TopicID                  *string         `json:"topic_id" gorm:"type:uuid"`
	DedupeKey                string          `json:"dedupe_key" gorm:"not null"`
	GapKind                  string          `json:"gap_kind" gorm:"not null;default:'content'"`
	GapCategory              string          `json:"gap_category" gorm:"not null;default:'unknown'"`
	V1GapType                string          `json:"v1_gap_type" gorm:"not null;default:'needs_review'"`
	Title                    string          `json:"title" gorm:"not null;default:''"`
	IssueKey                 string          `json:"issue_key" gorm:"not null;default:''"`
	Status                   string          `json:"status" gorm:"not null;default:'open';index:idx_support_coverage_gaps_workspace_status_seen,priority:2"`
	Confidence               float64         `json:"confidence" gorm:"not null;default:0"`
	EvidenceCount            int             `json:"evidence_count" gorm:"not null;default:0"`
	FailureMode              string          `json:"failure_mode" gorm:"not null;default:''"`
	SourceSignal             string          `json:"source_signal" gorm:"not null;default:''"`
	CanAnswer                *string         `json:"can_answer"`
	CanResolve               *string         `json:"can_resolve"`
	Metadata                 json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	FirstSeenAt              time.Time       `json:"first_seen_at" gorm:"not null"`
	LastSeenAt               time.Time       `json:"last_seen_at" gorm:"not null;index:idx_support_coverage_gaps_workspace_status_seen,priority:3,sort:desc"`
	StatusChangedBy          *string         `json:"status_changed_by" gorm:"type:uuid"`
	StatusChangedAt          *time.Time      `json:"status_changed_at"`
	IssueResolved            *bool           `json:"issue_resolved"`
	ClosedAt                 *time.Time      `json:"closed_at"`
	ClosedEvidenceCount      *int            `json:"closed_evidence_count"`
	ResultDocumentID         *string         `json:"result_document_id" gorm:"type:uuid"`
	RejectionReason          *string         `json:"rejection_reason"`
	Embedding                string          `json:"-" gorm:"type:vector(1536)"`
	EmbeddingProvider        string          `json:"embedding_provider" gorm:"not null;default:''"`
	EmbeddingModel           string          `json:"embedding_model" gorm:"not null;default:''"`
	EmbeddingVersion         string          `json:"embedding_version" gorm:"not null;default:''"`
	EmbeddingDimensions      int             `json:"embedding_dimensions" gorm:"not null;default:0"`
	EmbeddingTextHash        string          `json:"embedding_text_hash" gorm:"not null;default:''"`
	EmbeddingUpdatedAt       *time.Time      `json:"embedding_updated_at"`
	NearestContentScore      float64         `json:"nearest_content_score" gorm:"not null;default:0"`
	NearestContentDocumentID *string         `json:"nearest_content_document_id" gorm:"type:uuid"`
	NearestContentTitle      string          `json:"nearest_content_title" gorm:"not null;default:''"`
	NearestContentCheckedAt  *time.Time      `json:"nearest_content_checked_at"`
	ImpactScore              float64         `json:"impact_score" gorm:"not null;default:0"`
	CreatedAt                time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
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
	SourceKey       string          `json:"source_key" gorm:"not null;default:''"`
	Excerpt         string          `json:"excerpt" gorm:"type:text;not null;default:''"`
	Metadata        json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt       time.Time       `json:"created_at" gorm:"not null;index:idx_support_gap_evidence_gap,priority:2,sort:desc"`
}

func (SupportGapEvidence) TableName() string { return "support_gap_evidence" }

// SupportGapEvidenceView is a read-only projection of SupportGapEvidence
// joined with the originating support_messages.sender_type. Used for the
// gap detail response so the UI can render messages with role-aware layout
// (customer vs agent/ai/user). Excluded from AutoMigrate by virtue of
// having no TableName() — only scanned via explicit .Table()/.Select() in
// repository queries.
type SupportGapEvidenceView struct {
	SupportGapEvidence
	SenderRole string `json:"sender_role" gorm:"column:sender_role"`
}

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
	IsActive           bool            `json:"is_active" gorm:"not null;default:true"`
	SupersededAt       *time.Time      `json:"superseded_at"`
	Metadata           json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt          time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportGapSuggestion) TableName() string { return "support_gap_suggestions" }

// SupportCoverageGapArticle links a gap to a related existing
// docs article (N:N relationship).
type SupportCoverageGapArticle struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	GapID        string    `json:"gap_id" gorm:"type:uuid;not null"`
	DocumentID   string    `json:"document_id" gorm:"type:uuid;not null"`
	WorkspaceID  string    `json:"workspace_id" gorm:"type:uuid;not null"`
	ArticleTitle string    `json:"article_title" gorm:"-"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
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

// SupportCoverageClusterRebuildRun records a workspace-wide gap clustering pass.
type SupportCoverageClusterRebuildRun struct {
	ID                 string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID        string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Status             string          `json:"status" gorm:"not null;default:'running'"`
	GapsScanned        int             `json:"gaps_scanned" gorm:"not null;default:0"`
	ClustersFound      int             `json:"clusters_found" gorm:"not null;default:0"`
	AutoMerged         int             `json:"auto_merged" gorm:"not null;default:0"`
	SuggestionsCreated int             `json:"suggestions_created" gorm:"not null;default:0"`
	Skipped            int             `json:"skipped" gorm:"not null;default:0"`
	ErrorMessage       *string         `json:"error_message"`
	StartedAt          time.Time       `json:"started_at" gorm:"not null"`
	CompletedAt        *time.Time      `json:"completed_at"`
	Metadata           json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt          time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCoverageClusterRebuildRun) TableName() string {
	return "support_coverage_cluster_rebuild_runs"
}

// SupportCoverageGapMergeSuggestion records a reviewed or pending duplicate-gap merge.
type SupportCoverageGapMergeSuggestion struct {
	ID                    string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID           string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	RunID                 *string         `json:"run_id" gorm:"type:uuid"`
	SourceGapID           string          `json:"source_gap_id" gorm:"type:uuid;not null;index"`
	TargetGapID           string          `json:"target_gap_id" gorm:"type:uuid;not null;index"`
	PairKey               string          `json:"pair_key" gorm:"not null;default:'';index"`
	Status                string          `json:"status" gorm:"not null;default:'pending'"`
	SimilarityScore       float64         `json:"similarity_score" gorm:"not null;default:0"`
	Reason                string          `json:"reason" gorm:"type:text;not null;default:''"`
	CombinedEvidenceCount int             `json:"combined_evidence_count" gorm:"not null;default:0"`
	ReviewedBy            *string         `json:"reviewed_by" gorm:"type:uuid"`
	ReviewedAt            *time.Time      `json:"reviewed_at"`
	Metadata              json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt             time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             time.Time       `json:"updated_at" gorm:"autoUpdateTime"`

	SourceGap *SupportCoverageGap `json:"source_gap,omitempty" gorm:"foreignKey:SourceGapID"`
	TargetGap *SupportCoverageGap `json:"target_gap,omitempty" gorm:"foreignKey:TargetGapID"`
}

func (SupportCoverageGapMergeSuggestion) TableName() string {
	return "support_coverage_gap_merge_suggestions"
}

// SupportCoverageGapPairDecision records durable human decisions about whether
// two coverage gaps should be kept apart during future clustering.
type SupportCoverageGapPairDecision struct {
	ID                   string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID          string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	PairKey              string    `json:"pair_key" gorm:"not null;index"`
	GapAID               string    `json:"gap_a_id" gorm:"type:uuid;not null;index"`
	GapBID               string    `json:"gap_b_id" gorm:"type:uuid;not null;index"`
	Decision             string    `json:"decision" gorm:"not null;default:'keep_separate'"`
	DecidedBy            *string   `json:"decided_by" gorm:"type:uuid"`
	DecidedAt            time.Time `json:"decided_at" gorm:"not null"`
	SimilarityAtDecision float64   `json:"similarity_at_decision" gorm:"not null;default:0"`
	GapATextHash         string    `json:"gap_a_text_hash" gorm:"not null;default:''"`
	GapBTextHash         string    `json:"gap_b_text_hash" gorm:"not null;default:''"`
	CreatedAt            time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCoverageGapPairDecision) TableName() string {
	return "support_coverage_gap_pair_decisions"
}

// ─── DTOs ──────────────────────────────────────────────────────────────────

// SupportCoverageGapFilter controls gap list queries.
type SupportCoverageGapFilter struct {
	Status              string `json:"status"`
	GapKind             string `json:"gap_kind"`
	GapCategory         string `json:"gap_category"`
	V1GapType           string `json:"v1_gap_type"`
	IssueKey            string `json:"issue_key"`
	Search              string `json:"search"`
	HasMergeSuggestions bool   `json:"has_merge_suggestions"`
	ShowRaw             bool   `json:"show_raw"`
	Page                int    `json:"page"`
	PerPage             int    `json:"per_page"`
}

// SupportCoverageGapListItem is a row in the gap inbox table.
type SupportCoverageGapListItem struct {
	SupportCoverageGap
	TopicTitle           string  `json:"topic_title"`
	CanonicalTitle       string  `json:"canonical_title"`
	EvidenceText         string  `json:"evidence_text" gorm:"column:evidence_text"`
	CustomerNeedText     string  `json:"customer_need_text" gorm:"column:customer_need_text"`
	SuggestionCount      int     `json:"suggestion_count"`
	RelatedArticleID     *string `json:"related_article_id"`
	Evidence30d          int     `json:"evidence_30d" gorm:"column:evidence_30d"`
	ImpactTier           string  `json:"impact_tier" gorm:"-"`
	DistinctCustomers30d int     `json:"distinct_customers_30d" gorm:"column:distinct_customers_30d"`
	DistinctCustomersAll int     `json:"distinct_customers_all" gorm:"column:distinct_customers_all"`
	EvidenceAll          int     `json:"evidence_all" gorm:"column:evidence_all"`
	ImpactExplanation    string  `json:"impact_explanation" gorm:"-"`
	SplitReviewNeeded    bool    `json:"split_review_needed" gorm:"-"`
	RecurrenceReopened   bool    `json:"recurrence_reopened" gorm:"-"`
}

// SupportCoverageGapDetail is the full gap detail with evidence
// and suggestions.
type SupportCoverageGapDetail struct {
	SupportCoverageGap
	TopicTitle          string                              `json:"topic_title"`
	StatusChangedByName string                              `json:"status_changed_by_name"`
	AnalysisExplanation *SupportCoverageAnalysisExplanation `json:"analysis_explanation,omitempty"`
	Recommendations     []SupportCoverageRecommendation     `json:"recommendations"`
	Evidence            []SupportGapEvidenceView            `json:"evidence"`
	Suggestions         []SupportGapSuggestion              `json:"suggestions"`
	RelatedArticles     []SupportCoverageGapArticle         `json:"related_articles"`
	SplitReviewNeeded   bool                                `json:"split_review_needed"`
	RecurrenceReopened  bool                                `json:"recurrence_reopened"`
}

type SupportCoverageAnalysisExplanation struct {
	CustomerNeed    string `json:"customer_need"`
	AIFailure       string `json:"ai_failure"`
	HumanResolution string `json:"human_resolution"`
	DecisionReason  string `json:"decision_reason"`
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
	NewGapsThisWeek    int        `json:"new_gaps_this_week"`
	TopRecurringGaps   int        `json:"top_recurring_gaps"`
	GapsFixedThisWeek  int        `json:"gaps_fixed_this_week"`
	TotalOpenGaps      int        `json:"total_open_gaps"`
	TotalEvidenceCount int        `json:"total_evidence_count"`
	HandoffsAfterFixes int        `json:"handoffs_after_fixes"`
	LastAnalyzedAt     *time.Time `json:"last_analyzed_at,omitempty"`
}
