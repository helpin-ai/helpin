package service

import (
	"encoding/json"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const agentRunTurnBudgetStartKey = "ai_usage_turn_start"

func agentRunTurnBudgetStart(run *model.AgentRun) agentRunUsageCheckpoint {
	var body struct {
		Start agentRunUsageCheckpoint `json:"ai_usage_turn_start"`
	}
	if run == nil || json.Unmarshal(run.OutputSummary, &body) != nil {
		return agentRunUsageCheckpoint{}
	}
	return body.Start
}

// Reset the turn's cap only when the user actually resumes. Pauses, late usage,
// and duplicate terminal events cannot grant another budget.
func storeAgentRunTurnBudgetStart(run *model.AgentRun) error {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(run.OutputSummary, &body); err != nil {
		return err
	}
	if body == nil {
		body = map[string]json.RawMessage{}
	}
	baseline, err := json.Marshal(agentRunUsageCheckpointFromSummary(run.OutputSummary))
	if err != nil {
		return err
	}
	body[agentRunTurnBudgetStartKey] = baseline
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	run.OutputSummary = encoded
	return nil
}
