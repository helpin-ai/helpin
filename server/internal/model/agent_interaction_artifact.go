package model

const (
	AgentRunArtifactTypeHumanInputRequest    = "human_input_request"
	AgentRunArtifactTypeHumanApprovalRequest = "human_approval_request"
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
