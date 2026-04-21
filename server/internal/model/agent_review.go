package model

import "time"

const (
	AgentRunArtifactTypeReviewFindings = "review_findings"
	AgentRunArtifactTypeReviewDecision = "review_decision"
)

type ReviewFinding struct {
	ID           string   `json:"id,omitempty"`
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	Priority     string   `json:"priority,omitempty"`
	Confidence   *float64 `json:"confidence,omitempty"`
	CodeLocation string   `json:"code_location,omitempty"`
}

type ReviewFindingsArtifact struct {
	Phase                      string          `json:"phase,omitempty"`
	Title                      string          `json:"title,omitempty"`
	Summary                    string          `json:"summary,omitempty"`
	Findings                   []ReviewFinding `json:"findings,omitempty"`
	OverallCorrectness         string          `json:"overall_correctness,omitempty"`
	OverallExplanation         string          `json:"overall_explanation,omitempty"`
	OverallConfidenceScore     *float64        `json:"overall_confidence_score,omitempty"`
	AssistantMessageSequenceNo int             `json:"assistant_message_sequence_no,omitempty"`
	RecordedAt                 time.Time       `json:"recorded_at,omitempty"`
}

type ReviewCheckpointResponse struct {
	Decision           string   `json:"decision,omitempty"`
	Message            string   `json:"message,omitempty"`
	SelectionMode      string   `json:"selection_mode,omitempty"`
	SelectedFindingIDs []string `json:"selected_finding_ids,omitempty"`
}

type ReviewDecisionFinding struct {
	ID           string `json:"id"`
	Title        string `json:"title,omitempty"`
	CodeLocation string `json:"code_location,omitempty"`
	Status       string `json:"status"`
}

type ReviewDecisionArtifact struct {
	Phase                      string                  `json:"phase,omitempty"`
	Title                      string                  `json:"title,omitempty"`
	Summary                    string                  `json:"summary,omitempty"`
	Decision                   string                  `json:"decision,omitempty"`
	Message                    string                  `json:"message,omitempty"`
	SelectionMode              string                  `json:"selection_mode,omitempty"`
	Findings                   []ReviewDecisionFinding `json:"findings,omitempty"`
	AssistantMessageSequenceNo int                     `json:"assistant_message_sequence_no,omitempty"`
	ResolvedBy                 string                  `json:"resolved_by,omitempty"`
	ResolvedAt                 time.Time               `json:"resolved_at,omitempty"`
}
