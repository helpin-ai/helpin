package model

import (
	"encoding/json"
	"time"
)

const (
	SupportCoverageAnalysisRunStatusRunning   = "running"
	SupportCoverageAnalysisRunStatusCompleted = "completed"
	SupportCoverageAnalysisRunStatusFailed    = "failed"

	SupportCoverageConversationAnalysisStatusAnalyzed = "analyzed"
	SupportCoverageConversationAnalysisStatusSkipped  = "skipped"
	SupportCoverageConversationAnalysisStatusFailed   = "failed"

	SupportCoverageFixCreateArticle     = "create_article"
	SupportCoverageFixUpdateArticle     = "update_article"
	SupportCoverageFixUpdateWebsitePage = "update_website_page"
	SupportCoverageFixCreateWebsitePage = "create_website_page"
	SupportCoverageFixAddData           = "add_data"
	SupportCoverageFixAddAction         = "add_action"
	SupportCoverageFixDefinePolicy      = "define_policy"
	SupportCoverageFixImproveWorkflow   = "improve_workflow"
	SupportCoverageFixNoFix             = "no_fix"

	SupportCoverageRecommendationStatusOpen      = "open"
	SupportCoverageRecommendationStatusAccepted  = "accepted"
	SupportCoverageRecommendationStatusDismissed = "dismissed"
	SupportCoverageRecommendationStatusApplied   = "applied"

	SupportCoverageRecommendationPriorityPrimary   = "primary"
	SupportCoverageRecommendationPrioritySecondary = "secondary"
)

// SupportCoverageAnalysisRun tracks one workspace's daily analyzer run.
type SupportCoverageAnalysisRun struct {
	ID              string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID     string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	WindowStart     time.Time       `json:"window_start" gorm:"not null"`
	WindowEnd       time.Time       `json:"window_end" gorm:"not null"`
	CursorStartedAt time.Time       `json:"cursor_started_at" gorm:"not null"`
	CursorEndedAt   time.Time       `json:"cursor_ended_at" gorm:"not null"`
	AnalyzerVersion string          `json:"analyzer_version" gorm:"not null;default:'v1'"`
	Status          string          `json:"status" gorm:"not null;default:'running'"`
	ConversationCnt int             `json:"conversation_count" gorm:"not null;default:0"`
	GapCount        int             `json:"gap_count" gorm:"not null;default:0"`
	ErrorMessage    *string         `json:"error_message"`
	Metadata        json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	StartedAt       time.Time       `json:"started_at" gorm:"not null"`
	CompletedAt     *time.Time      `json:"completed_at"`
	CreatedAt       time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCoverageAnalysisRun) TableName() string { return "support_coverage_analysis_runs" }

// SupportCoverageConversationAnalysis records the analyzer result for a transcript.
type SupportCoverageConversationAnalysis struct {
	ID                        string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID               string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	RunID                     string          `json:"run_id" gorm:"type:uuid;not null;index"`
	ConversationID            string          `json:"conversation_id" gorm:"type:uuid;not null;index"`
	Status                    string          `json:"status" gorm:"not null"`
	HasGap                    bool            `json:"has_gap" gorm:"not null;default:false"`
	GapID                     *string         `json:"gap_id" gorm:"type:uuid"`
	GapKind                   string          `json:"gap_kind" gorm:"not null;default:''"`
	GapCategory               string          `json:"gap_category" gorm:"not null;default:''"`
	PrimaryRecommendationType string          `json:"primary_recommendation_type" gorm:"not null;default:''"`
	TranscriptHash            string          `json:"transcript_hash" gorm:"not null;default:''"`
	AnalyzerVersion           string          `json:"analyzer_version" gorm:"not null;default:'v1'"`
	CustomerNeed              string          `json:"customer_need" gorm:"type:text;not null;default:''"`
	AIFailure                 string          `json:"ai_failure" gorm:"type:text;not null;default:''"`
	HumanResolution           string          `json:"human_resolution" gorm:"type:text;not null;default:''"`
	DecisionReason            string          `json:"decision_reason" gorm:"type:text;not null;default:''"`
	Confidence                float64         `json:"confidence" gorm:"not null;default:0"`
	ErrorMessage              *string         `json:"error_message"`
	RawOutput                 json.RawMessage `json:"raw_output" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt                 time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                 time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCoverageConversationAnalysis) TableName() string {
	return "support_coverage_conversation_analyses"
}

// SupportAIRetrievalTrace stores a compact copy of live inbox AI retrieval.
type SupportAIRetrievalTrace struct {
	ID             string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	ConversationID string          `json:"conversation_id" gorm:"type:uuid;not null;index"`
	MessageID      string          `json:"message_id" gorm:"type:uuid;not null;index"`
	SearchQueries  json.RawMessage `json:"search_queries" gorm:"type:jsonb;not null;default:'[]'"`
	Results        json.RawMessage `json:"results" gorm:"type:jsonb;not null;default:'[]'"`
	CitedSourceIDs json.RawMessage `json:"cited_source_ids" gorm:"type:jsonb;not null;default:'[]'"`
	AIConfidence   float64         `json:"ai_confidence" gorm:"not null;default:0"`
	CanAnswer      *string         `json:"can_answer"`
	CanResolve     *string         `json:"can_resolve"`
	FailureMode    string          `json:"failure_mode" gorm:"not null;default:''"`
	Metadata       json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt      time.Time       `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportAIRetrievalTrace) TableName() string { return "support_ai_retrieval_traces" }

// SupportCoverageRecommendation is a recommended fix for a coverage gap.
type SupportCoverageRecommendation struct {
	ID                  string          `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID         string          `json:"workspace_id" gorm:"type:uuid;not null;index"`
	GapID               string          `json:"gap_id" gorm:"type:uuid;not null;index"`
	AnalysisID          *string         `json:"analysis_id" gorm:"type:uuid"`
	RecommendationType  string          `json:"recommendation_type" gorm:"not null"`
	TargetType          string          `json:"target_type" gorm:"not null;default:''"`
	TargetID            *string         `json:"target_id"`
	TargetTitle         string          `json:"target_title" gorm:"not null;default:''"`
	TargetURL           string          `json:"target_url" gorm:"type:text;not null;default:''"`
	Priority            string          `json:"priority" gorm:"not null;default:'secondary'"`
	Status              string          `json:"status" gorm:"not null;default:'open'"`
	Rationale           string          `json:"rationale" gorm:"type:text;not null;default:''"`
	SuggestedChange     string          `json:"suggested_change" gorm:"type:text;not null;default:''"`
	ImplementationNotes string          `json:"implementation_notes" gorm:"type:text;not null;default:''"`
	SuggestionID        *string         `json:"suggestion_id" gorm:"type:uuid"`
	Metadata            json.RawMessage `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt           time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SupportCoverageRecommendation) TableName() string {
	return "support_coverage_recommendations"
}
