package service

import (
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCompactAgentFleetRunInputsKeepsOnlyTriggerIdentity(t *testing.T) {
	runs := []model.AgentRun{{
		Input: json.RawMessage(`{
			"trigger":{"source":"automation_rule","trigger_type":"task.state_entered","context":{"large":"payload"}},
			"workspace_context":{"company_product_context":"must not be sent"},
			"additional_context":"must not be sent"
		}`),
	}}

	compactAgentFleetRunInputs(runs)

	if got, want := string(runs[0].Input), `{"trigger":{"source":"automation_rule","trigger_type":"task.state_entered"}}`; got != want {
		t.Fatalf("compact input = %s, want %s", got, want)
	}
}

func TestFleetAttentionPriority(t *testing.T) {
	input := model.AgentRun{Status: model.AgentRunStatusPaused, PauseReason: model.AgentRunPauseReasonHumanInput}
	auth := model.AgentRun{Status: model.AgentRunStatusPaused, PauseReason: model.AgentRunPauseReasonAuthentication}
	approval := model.AgentRun{Status: model.AgentRunStatusPaused, PauseReason: model.AgentRunPauseReasonHumanApproval}
	chat := model.AgentRun{Status: model.AgentRunStatusPaused, PauseReason: model.AgentRunPauseReasonUserMessage}

	if fleetAttentionPriority(approval) <= fleetAttentionPriority(auth) || fleetAttentionPriority(auth) <= fleetAttentionPriority(input) {
		t.Fatalf("attention priorities are not ordered approval > auth > input")
	}
	if isFleetAttentionRun(chat) {
		t.Fatal("chat reply pause must not require fleet attention")
	}
}
