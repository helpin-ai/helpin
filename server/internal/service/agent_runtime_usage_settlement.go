package service

import (
	"context"
	"encoding/json"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const terminalUsageVersionKey = "agent_runtime_terminal_usage_v2"

func markTerminalUsageVersion(run *model.AgentRun) error {
	var body map[string]json.RawMessage
	if len(run.OutputSummary) > 0 {
		if err := json.Unmarshal(run.OutputSummary, &body); err != nil {
			return err
		}
	}
	if body == nil {
		body = map[string]json.RawMessage{}
	}
	body[terminalUsageVersionKey] = json.RawMessage(`true`)
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	run.OutputSummary = encoded
	return nil
}

// Old terminal records lack the new settled-token watermark. Seed their last
// observed totals before applying a late event, never rebill their whole history.
func seedTerminalUsageBaseline(run *model.AgentRun) (bool, error) {
	if run == nil || !runtimeUsageAlreadyConsumed(run.OutputSummary) {
		return false, nil
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(run.OutputSummary, &body); err != nil {
		return false, err
	}
	if string(body[terminalUsageVersionKey]) == "true" {
		return false, nil
	}
	usage, _ := latestAgentRuntimeUsage(run)
	checkpoint := agentRunUsageCheckpointFromSummary(run.OutputSummary)
	checkpoint.Turn = max(checkpoint.Turn, 1)
	checkpoint.InputTokens = max(checkpoint.InputTokens, usage.InputTokens)
	checkpoint.OutputTokens = max(checkpoint.OutputTokens, usage.OutputTokens)
	checkpoint.CachedInputTokens = max(checkpoint.CachedInputTokens, usage.CachedInputTokens)
	checkpoint.ReasoningOutputTokens = max(checkpoint.ReasoningOutputTokens, usage.ReasoningOutputTokens)
	if err := storeAgentRunUsageCheckpoint(run, checkpoint); err != nil {
		return false, err
	}
	return true, markTerminalUsageVersion(run)
}

func maxAgentRuntimeUsage(a, b agentRuntimeUsagePayload) agentRuntimeUsagePayload {
	return agentRuntimeUsagePayload{TotalTokens: max(a.TotalTokens, b.TotalTokens), InputTokens: max(a.InputTokens, b.InputTokens), OutputTokens: max(a.OutputTokens, b.OutputTokens), CachedInputTokens: max(a.CachedInputTokens, b.CachedInputTokens), ReasoningOutputTokens: max(a.ReasoningOutputTokens, b.ReasoningOutputTokens)}
}

func (s *AgentRuntimeProjectionService) settleTerminalUsage(ctx context.Context, run *model.AgentRun, agent *model.Agent, event AgentRuntimeEventEnvelope, usage agentRuntimeUsagePayload) (bool, error) {
	previous := append(json.RawMessage(nil), run.OutputSummary...)
	run.OutputSummary = markRuntimeUsageConsumed(run.OutputSummary, s.eventTime(event))
	if err := markTerminalUsageVersion(run); err != nil {
		run.OutputSummary = previous
		return false, err
	}
	err := s.usageMeter.reconcileAgentRun(ctx, run, usage)
	if err != nil {
		run.OutputSummary = previous
		return false, err
	}
	return true, nil
}
