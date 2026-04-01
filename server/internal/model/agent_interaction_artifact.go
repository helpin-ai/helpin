package model

import "time"

const (
	AgentRunArtifactTypeHumanInputRequest    = "human_input_request"
	AgentRunArtifactTypeHumanApprovalRequest = "human_approval_request"
	AgentRunArtifactTypeCodexAuthState       = "codex_auth_state"
)

const (
	CodexAuthStateRequired  = "required"
	CodexAuthStatePending   = "pending"
	CodexAuthStateConnected = "connected"
	CodexAuthStateFailed    = "failed"
	CodexAuthStateCancelled = "cancelled"
)

type HumanInputArtifact struct {
	Questions []HumanInputArtifactQuestion `json:"questions"`
}

type HumanInputArtifactQuestion struct {
	ID      string                     `json:"id"`
	Type    string                     `json:"type,omitempty"`
	Text    string                     `json:"text"`
	Options []HumanInputArtifactOption `json:"options"`
}

type HumanInputArtifactOption struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Freetext bool   `json:"freetext,omitempty"`
}

type CodexAuthState struct {
	Provider        string    `json:"provider,omitempty"`
	AuthMode        string    `json:"auth_mode,omitempty"`
	State           string    `json:"state"`
	LoginID         *string   `json:"login_id,omitempty"`
	AuthURL         *string   `json:"auth_url,omitempty"`
	VerificationURL *string   `json:"verification_url,omitempty"`
	UserCode        *string   `json:"user_code,omitempty"`
	PlanType        *string   `json:"plan_type,omitempty"`
	Error           *string   `json:"error,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}
