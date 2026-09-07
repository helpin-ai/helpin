package service

import (
	"context"
	"encoding/json"
	"fmt"

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
	settled := runtimeUsageAlreadyConsumed(previous)
	run.OutputSummary = markRuntimeUsageConsumed(run.OutputSummary, s.eventTime(event))
	if err := markTerminalUsageVersion(run); err != nil {
		run.OutputSummary = previous
		return false, err
	}
	var err error
	if s.usageMeter.usage != nil {
		err = s.usageMeter.reconcileAgentRun(ctx, run, usage)
	} else {
		err = s.settleLegacyTerminalUsage(ctx, run, agent, event, usage, settled)
	}
	if err != nil {
		run.OutputSummary = previous
		return false, err
	}
	return true, nil
}

func (s *AgentRuntimeProjectionService) settleLegacyTerminalUsage(ctx context.Context, run *model.AgentRun, agent *model.Agent, event AgentRuntimeEventEnvelope, usage agentRuntimeUsagePayload, settled bool) error {
	meter := s.usageMeter
	if meter.consumer == nil {
		return fmt.Errorf("ai usage meter billing consumer is required")
	}
	feature := AgentRunAIUsageFeature(agent)
	checkpoint := agentRunUsageCheckpointFromSummary(run.OutputSummary)
	units := func(u agentRuntimeUsagePayload) int {
		if agentRunUsageIsZero(u) {
			return 0
		}
		return CalculateAIUsageUnits(AIUsageCalculation{FeatureKey: feature, InputTokens: u.InputTokens, OutputTokens: int(agentRunTokenTelemetry(run, u).OutputTokens), ReasoningTokens: u.ReasoningOutputTokens, CachedInputTokens: u.CachedInputTokens})
	}
	previousUnits := 0
	if settled {
		previousUnits = units(agentRuntimeUsagePayload{InputTokens: checkpoint.InputTokens, OutputTokens: checkpoint.OutputTokens, CachedInputTokens: checkpoint.CachedInputTokens, ReasoningOutputTokens: checkpoint.ReasoningOutputTokens})
	}
	credits := max(units(usage)-previousUnits, 0)
	key := aiUsageIdempotencyKey(run.WorkspaceID, "agent-runtime", run.ID, "terminal-usage")
	if settled {
		key = aiUsageIdempotencyKey(key, "late", fmt.Sprint(checkpoint.Turn+1))
	}
	if credits > 0 {
		metadata := map[string]any{"run_id": run.ID, "runtime_run_id": derefString(run.ExternalRuntimeID), "agent_id": run.AgentID, "terminal_event": event.Type, "delegated": true}
		var err error
		if !settled {
			// Preserve the existing first-settlement metadata and feature labeling.
			_, err = meter.Consume(ctx, AIUsageMeterInput{WorkspaceID: run.WorkspaceID, FeatureKey: feature, IdempotencyKey: key, InputTokens: usage.InputTokens, OutputTokens: int(agentRunTokenTelemetry(run, usage).OutputTokens), ReasoningTokens: usage.ReasoningOutputTokens, CachedInputTokens: usage.CachedInputTokens, AllowOverage: true, Metadata: metadata})
		} else {
			metadata["late_usage"] = true
			metadata["cumulative_usage"] = usage
			metadata["previous_units"] = previousUnits
			_, err = meter.consumer.ConsumeCredits(ctx, BillingCreditConsumption{WorkspaceID: run.WorkspaceID, FeatureKey: feature, Credits: credits, IdempotencyKey: key, AllowOverage: true, Metadata: metadata})
		}
		if err != nil {
			return err
		}
	}
	return storeAgentRunUsageCheckpoint(run, agentRunUsageCheckpoint{Turn: checkpoint.Turn + 1, InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens, CachedInputTokens: usage.CachedInputTokens, ReasoningOutputTokens: usage.ReasoningOutputTokens})
}
