package model

type ApprovalRequest struct {
	Phase           string `json:"phase,omitempty"`
	PreviewPanelKey string `json:"preview_panel_key,omitempty"`
	Title           string `json:"title"`
	Summary         string `json:"summary"`
}

type ApprovalResponse struct {
	Decision string `json:"decision,omitempty"`
	Message  string `json:"message,omitempty"`
}

type ReviewCheckpointRequest struct {
	Phase                  string          `json:"phase,omitempty"`
	PreviewPanelKey        string          `json:"preview_panel_key,omitempty"`
	Title                  string          `json:"title"`
	Summary                string          `json:"summary"`
	Findings               []ReviewFinding `json:"findings,omitempty"`
	OverallCorrectness     string          `json:"overall_correctness,omitempty"`
	OverallExplanation     string          `json:"overall_explanation,omitempty"`
	OverallConfidenceScore *float64        `json:"overall_confidence_score,omitempty"`
}
