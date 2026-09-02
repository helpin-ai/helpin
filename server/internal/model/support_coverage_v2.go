package model

import (
	"encoding/json"
	"time"
)

const (
	CoverageBatchQueued        = "queued"
	CoverageBatchRunning       = "running"
	CoverageBatchSucceeded     = "succeeded"
	CoverageBatchPartialFailed = "partial_failed"
	CoverageBatchFailed        = "failed"

	CoverageAttemptQueued              = "queued"
	CoverageAttemptLeased              = "leased"
	CoverageAttemptRetryable           = "retryable"
	CoverageAttemptSucceeded           = "succeeded"
	CoverageAttemptDeadLetter          = "dead_letter"
	CoverageAttemptPausedConfiguration = "paused_configuration"

	CoverageFailureConfiguration      = "configuration"
	CoverageFailureCandidateQuery     = "candidate_query"
	CoverageFailureLLMPreflight       = "llm_preflight"
	CoverageFailureLLMProvider        = "llm_provider"
	CoverageFailureLLMContract        = "llm_contract"
	CoverageFailureKnowledgeRetrieval = "knowledge_retrieval"
	CoverageFailurePersistence        = "persistence"
	CoverageFailureEmbeddingProvider  = "embedding_provider"
	CoverageFailureTopicAssignment    = "topic_assignment"
	CoverageFailureLeaseLost          = "lease_lost"

	CoverageMembershipAutomatic = "automatic"
	CoverageMembershipManual    = "manual"
	CoverageMembershipPromotion = "promotion"
	CoverageMembershipRebuild   = "rebuild"

	CoverageSignalUnreviewed = "unreviewed"
	CoverageSignalPromoted   = "promoted"
	CoverageSignalAttached   = "attached"
	CoverageSignalDismissed  = "dismissed"

	CoverageTopicOpen     = "open"
	CoverageTopicDone     = "done"
	CoverageTopicArchived = "archived"

	CoverageAssignmentAttach = "attach"
	CoverageAssignmentCreate = "create"
	CoverageAssignmentReview = "review"
	CoverageAssignmentFailed = "failed"
)

func IsCoverageBatchStatus(value string) bool {
	return containsCoverageValue(value, CoverageBatchQueued, CoverageBatchRunning, CoverageBatchSucceeded, CoverageBatchPartialFailed, CoverageBatchFailed)
}

func IsCoverageAttemptStatus(value string) bool {
	return containsCoverageValue(value, CoverageAttemptQueued, CoverageAttemptLeased, CoverageAttemptRetryable, CoverageAttemptSucceeded, CoverageAttemptDeadLetter, CoverageAttemptPausedConfiguration)
}

func IsCoverageFailureClass(value string) bool {
	return containsCoverageValue(value, CoverageFailureConfiguration, CoverageFailureCandidateQuery, CoverageFailureLLMPreflight, CoverageFailureLLMProvider, CoverageFailureLLMContract, CoverageFailureKnowledgeRetrieval, CoverageFailurePersistence, CoverageFailureEmbeddingProvider, CoverageFailureTopicAssignment, CoverageFailureLeaseLost)
}

func IsCoverageMembershipSource(value string) bool {
	return containsCoverageValue(value, CoverageMembershipAutomatic, CoverageMembershipManual, CoverageMembershipPromotion, CoverageMembershipRebuild)
}

func IsCoverageSignalStatus(value string) bool {
	return containsCoverageValue(value, CoverageSignalUnreviewed, CoverageSignalPromoted, CoverageSignalAttached, CoverageSignalDismissed)
}

func containsCoverageValue(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

// CoverageBatch is one resumable workspace/window analysis run.
type CoverageBatch struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null"`
	WindowStart     time.Time       `json:"window_start" gorm:"not null"`
	WindowEnd       time.Time       `json:"window_end" gorm:"not null"`
	AnalyzerVersion string          `json:"analyzer_version" gorm:"not null"`
	PolicyVersion   string          `json:"policy_version" gorm:"not null"`
	Status          string          `json:"status" gorm:"not null;default:'queued'"`
	Cursor          string          `json:"cursor" gorm:"not null;default:''"`
	LeaseOwner      string          `json:"lease_owner" gorm:"not null;default:''"`
	LeaseExpiresAt  *time.Time      `json:"lease_expires_at"`
	CandidateCount  int             `json:"candidate_count" gorm:"not null;default:0"`
	SucceededCount  int             `json:"succeeded_count" gorm:"not null;default:0"`
	RetryableCount  int             `json:"retryable_count" gorm:"not null;default:0"`
	DeadLetterCount int             `json:"dead_letter_count" gorm:"not null;default:0"`
	FailureClass    string          `json:"failure_class" gorm:"not null;default:''"`
	FailureMessage  string          `json:"failure_message" gorm:"not null;default:''"`
	CorrelationID   string          `json:"correlation_id" gorm:"not null;default:''"`
	Metadata        json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	StartedAt       *time.Time      `json:"started_at"`
	CompletedAt     *time.Time      `json:"completed_at"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CoverageBatch) TableName() string { return "coverage_batches" }

// CoverageAnalysisAttempt is append-only history for one logical source item.
type CoverageAnalysisAttempt struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null"`
	BatchID         string          `json:"batch_id" gorm:"type:uuid;not null"`
	LogicalWorkKey  string          `json:"logical_work_key" gorm:"not null"`
	SourceKind      string          `json:"source_kind" gorm:"not null"`
	SourceID        string          `json:"source_id" gorm:"not null"`
	SegmentID       string          `json:"segment_id" gorm:"not null;default:''"`
	ContentHash     string          `json:"content_hash" gorm:"not null"`
	AnalyzerVersion string          `json:"analyzer_version" gorm:"not null"`
	PolicyVersion   string          `json:"policy_version" gorm:"not null"`
	Attempt         int             `json:"attempt" gorm:"not null"`
	Status          string          `json:"status" gorm:"not null;default:'queued'"`
	Stage           string          `json:"stage" gorm:"not null;default:'candidate_query'"`
	LeaseOwner      string          `json:"lease_owner" gorm:"not null;default:''"`
	LeaseExpiresAt  *time.Time      `json:"lease_expires_at"`
	RetryAt         *time.Time      `json:"retry_at"`
	RetryBudgetUsed int             `json:"retry_budget_used" gorm:"not null;default:0"`
	FailureClass    string          `json:"failure_class" gorm:"not null;default:''"`
	FailureMessage  string          `json:"failure_message" gorm:"not null;default:''"`
	AIExecutionID   *string         `json:"ai_execution_id" gorm:"type:uuid"`
	CorrelationID   string          `json:"correlation_id" gorm:"not null;default:''"`
	Metadata        json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	StartedAt       *time.Time      `json:"started_at"`
	CompletedAt     *time.Time      `json:"completed_at"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (CoverageAnalysisAttempt) TableName() string { return "coverage_analysis_attempts" }

// CoverageFinding is the durable, actionable analysis result for a source item.
type CoverageFinding struct {
	ID                  string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string          `json:"workspace_id" gorm:"type:uuid;not null"`
	LogicalWorkKey      string          `json:"logical_work_key" gorm:"not null"`
	AnalysisAttemptID   string          `json:"analysis_attempt_id" gorm:"type:uuid;not null"`
	SourceKind          string          `json:"source_kind" gorm:"not null"`
	SourceID            string          `json:"source_id" gorm:"not null"`
	ConversationID      *string         `json:"conversation_id" gorm:"type:uuid"`
	CustomerID          *string         `json:"customer_id" gorm:"type:uuid"`
	CustomerNeed        string          `json:"customer_need" gorm:"type:text;not null"`
	AIAnswer            string          `json:"ai_answer" gorm:"type:text;not null;default:''"`
	AIFailure           string          `json:"ai_failure" gorm:"type:text;not null;default:''"`
	HumanAnswer         string          `json:"human_answer" gorm:"type:text;not null;default:''"`
	FixType             string          `json:"fix_type" gorm:"not null"`
	FixTarget           string          `json:"fix_target" gorm:"type:text;not null"`
	Rationale           string          `json:"rationale" gorm:"type:text;not null"`
	SuggestedChange     string          `json:"suggested_change" gorm:"type:text;not null"`
	Confidence          float64         `json:"confidence" gorm:"not null"`
	IsCurrent           bool            `json:"is_current" gorm:"not null;default:true"`
	EmbeddingStatus     string          `json:"embedding_status" gorm:"not null;default:'pending'"`
	AssignmentStatus    string          `json:"assignment_status" gorm:"not null;default:'pending'"`
	Embedding           string          `json:"-" gorm:"type:vector(1536)"`
	EmbeddingProvider   string          `json:"embedding_provider" gorm:"not null;default:''"`
	EmbeddingModel      string          `json:"embedding_model" gorm:"not null;default:''"`
	EmbeddingVersion    string          `json:"embedding_version" gorm:"not null;default:''"`
	EmbeddingDimensions int             `json:"embedding_dimensions" gorm:"not null;default:0"`
	Metadata            json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	SupersededAt        *time.Time      `json:"superseded_at"`
	CreatedAt           time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CoverageFinding) TableName() string { return "coverage_findings" }

// CoverageTopicV2 is the canonical customer need presented in Coverage.
type CoverageTopicV2 struct {
	ID                string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID       string          `json:"workspace_id" gorm:"type:uuid;not null"`
	CanonicalKey      string          `json:"canonical_key" gorm:"not null"`
	Title             string          `json:"title" gorm:"not null"`
	CustomerNeed      string          `json:"customer_need" gorm:"type:text;not null"`
	Status            string          `json:"status" gorm:"not null;default:'open'"`
	FindingCount      int             `json:"finding_count" gorm:"not null;default:0"`
	ConversationCount int             `json:"conversation_count" gorm:"not null;default:0"`
	CustomerCount     int             `json:"customer_count" gorm:"not null;default:0"`
	AssignmentPolicy  string          `json:"assignment_policy" gorm:"not null"`
	Metadata          json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt         time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CoverageTopicV2) TableName() string { return "coverage_topics" }

// CoverageTopicMembership versions reversible finding-to-topic decisions.
type CoverageTopicMembership struct {
	ID                  string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string          `json:"workspace_id" gorm:"type:uuid;not null"`
	FindingID           string          `json:"finding_id" gorm:"type:uuid;not null"`
	TopicID             string          `json:"topic_id" gorm:"type:uuid;not null"`
	DecisionSource      string          `json:"decision_source" gorm:"not null"`
	Confidence          float64         `json:"confidence" gorm:"not null"`
	PolicyVersion       string          `json:"policy_version" gorm:"not null"`
	AssignmentAttemptID *string         `json:"assignment_attempt_id" gorm:"type:uuid"`
	ActorID             *string         `json:"actor_id" gorm:"type:uuid"`
	ValidFrom           time.Time       `json:"valid_from" gorm:"not null"`
	ValidTo             *time.Time      `json:"valid_to"`
	Metadata            json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt           time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (CoverageTopicMembership) TableName() string { return "coverage_topic_memberships" }

// CoverageAssignmentAttempt records scores, gates, and assignment outcomes.
type CoverageAssignmentAttempt struct {
	ID               string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string          `json:"workspace_id" gorm:"type:uuid;not null"`
	FindingID        string          `json:"finding_id" gorm:"type:uuid;not null"`
	Attempt          int             `json:"attempt" gorm:"not null"`
	PolicyVersion    string          `json:"policy_version" gorm:"not null"`
	CandidateTopicID *string         `json:"candidate_topic_id" gorm:"type:uuid"`
	Similarity       float64         `json:"similarity" gorm:"not null;default:0"`
	Compatibility    float64         `json:"compatibility" gorm:"not null;default:0"`
	Outcome          string          `json:"outcome" gorm:"not null"`
	FailureClass     string          `json:"failure_class" gorm:"not null;default:''"`
	FailureMessage   string          `json:"failure_message" gorm:"not null;default:''"`
	AIExecutionID    *string         `json:"ai_execution_id" gorm:"type:uuid"`
	Metadata         json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt        time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (CoverageAssignmentAttempt) TableName() string { return "coverage_assignment_attempts" }

// CoverageUnreviewedSignal is intentionally excluded from topic metrics.
type CoverageUnreviewedSignal struct {
	ID               string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID      string          `json:"workspace_id" gorm:"type:uuid;not null"`
	SourceKind       string          `json:"source_kind" gorm:"not null"`
	SourceID         string          `json:"source_id" gorm:"not null"`
	SessionID        string          `json:"session_id" gorm:"not null;default:''"`
	NormalizedQuery  string          `json:"normalized_query" gorm:"type:text;not null"`
	SignalKey        string          `json:"signal_key" gorm:"not null"`
	MeaningfulTokens int             `json:"meaningful_tokens" gorm:"not null;default:0"`
	Status           string          `json:"status" gorm:"not null;default:'unreviewed'"`
	Confidence       float64         `json:"confidence" gorm:"not null;default:0"`
	FindingID        *string         `json:"finding_id" gorm:"type:uuid"`
	TopicID          *string         `json:"topic_id" gorm:"type:uuid"`
	ReviewedBy       *string         `json:"reviewed_by" gorm:"type:uuid"`
	ReviewedAt       *time.Time      `json:"reviewed_at"`
	Metadata         json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	ObservedAt       time.Time       `json:"observed_at" gorm:"not null"`
	CreatedAt        time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CoverageUnreviewedSignal) TableName() string { return "coverage_unreviewed_signals" }
