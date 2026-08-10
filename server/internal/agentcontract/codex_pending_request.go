package agentcontract

import "encoding/json"

const (
	codexPendingRequestKindHumanInput      = "human_input"
	codexPendingRequestKindCommandApproval = "command_execution"
	codexPendingRequestKindFileApproval    = "file_change"
	codexPendingRequestKindPermissions     = "permissions"
)

type codexPendingRequest struct {
	Kind         string          `json:"kind,omitempty"`
	RequestID    string          `json:"request_id,omitempty"`
	RequestIDRaw json.RawMessage `json:"request_id_raw,omitempty"`
	TurnID       string          `json:"turn_id,omitempty"`
	ItemID       string          `json:"item_id,omitempty"`
	QuestionIDs  []string        `json:"question_ids,omitempty"`
	Payload      json.RawMessage `json:"payload,omitempty"`
}
