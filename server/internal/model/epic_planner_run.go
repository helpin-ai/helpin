package model

type ApprovalRequest struct {
	Phase                  string          `json:"phase"`
	Title                  string          `json:"title"`
	Summary                string          `json:"summary"`
	Findings               []ReviewFinding `json:"findings,omitempty"`
	OverallCorrectness     string          `json:"overall_correctness,omitempty"`
	OverallExplanation     string          `json:"overall_explanation,omitempty"`
	OverallConfidenceScore *float64        `json:"overall_confidence_score,omitempty"`
}
