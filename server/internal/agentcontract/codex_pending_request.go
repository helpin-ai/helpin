package agentcontract

import (
	"encoding/json"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

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

const QuestionTypeSingleSelect = "single_select"

type HumanInputOption struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Freetext bool   `json:"freetext,omitempty"`
}

type HumanInputQuestion struct {
	ID      string             `json:"id"`
	Type    string             `json:"type,omitempty"`
	Text    string             `json:"text"`
	Options []HumanInputOption `json:"options"`
}

type HumanInputRequest struct {
	Questions []HumanInputQuestion `json:"questions"`
}

type HumanApprovalRequest = model.ApprovalRequest

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func truncate(value string, maxLen int) string {
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}
