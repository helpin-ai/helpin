package model

import (
	"encoding/json"
	"time"
)

const (
	AgentRunArtifactTypeApprovedPreview        = "approved_preview"
	AgentRunArtifactTypeApprovedPreviewApplied = "approved_preview_applied"
)

// ApprovedRunPreview stores the exact preview payload the human approved.
type ApprovedRunPreview struct {
	Phase           string          `json:"phase,omitempty"`
	ApprovalTitle   string          `json:"approval_title,omitempty"`
	ApprovalSummary string          `json:"approval_summary,omitempty"`
	PanelKey        string          `json:"panel_key"`
	PreviewTitle    string          `json:"preview_title"`
	Format          string          `json:"format"`
	Content         json.RawMessage `json:"content"`
	SourceMessageID string          `json:"source_message_id,omitempty"`
	ApprovedBy      string          `json:"approved_by,omitempty"`
	ApprovedAt      time.Time       `json:"approved_at,omitempty"`
}

// AppliedApprovedRunPreview records that an approved preview artifact was consumed.
type AppliedApprovedRunPreview struct {
	ApprovedArtifactID string    `json:"approved_artifact_id"`
	Phase              string    `json:"phase,omitempty"`
	Action             string    `json:"action,omitempty"`
	AppliedAt          time.Time `json:"applied_at,omitempty"`
}
