package model

import "encoding/json"

const (
	InvocationModeInteractive = "interactive"
	InvocationModeAutonomous  = "autonomous"
)

// ToolInvocation records a single tool use during an agent run.
type ToolInvocation struct {
	ToolName      string          `json:"tool_name"`
	Input         json.RawMessage `json:"input"`
	OutputSummary string          `json:"output_summary"`
	DurationMs    int64           `json:"duration_ms"`
}

type TaskCompletionFollowupProposal struct {
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	StoryType   string  `json:"story_type,omitempty"`
	Priority    *string `json:"priority,omitempty"`
}

type TaskCompletionAssessment struct {
	Summary   string                          `json:"summary"`
	Followups []TaskCompletionFollowupProposal `json:"followups,omitempty"`
}

type CRMDealReviewActionPlan struct {
	Summary            string  `json:"summary"`
	RecommendedStageID *string `json:"recommended_stage_id,omitempty"`
	Note               *string `json:"note,omitempty"`
}
