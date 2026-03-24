package model

type ApprovalRequest struct {
	Phase   string `json:"phase"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}
