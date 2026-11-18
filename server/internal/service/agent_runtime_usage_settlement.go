package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

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

// subtaskUsageReader reads what a run's subtask workers consumed.
type subtaskUsageReader interface {
	ListToolCalls(ctx context.Context, runtimeRunID string) ([]AgentRuntimeToolCall, error)
	GetWorkJob(ctx context.Context, jobID string) (*agentruntime.WorkJob, error)
}

const agentRuntimeSubtaskUsageSummaryKey = "agent_runtime_subtask_usage"

// subtaskUsage returns the tokens used by a run's subtask workers. Workers
// are hidden runtime runs: their usage never appears in the parent's usage
// events, so without this a run that fans out is billed for the parent only.
// A lookup failure is returned so settlement is retried instead of settling
// short; a job the runtime no longer has is skipped.
func (s *AgentRuntimeProjectionService) subtaskUsage(ctx context.Context, run *model.AgentRun) (agentRuntimeUsagePayload, error) {
	var usage agentRuntimeUsagePayload
	reader, ok := s.agentRuntimeClient.(subtaskUsageReader)
	runtimeRunID := strings.TrimSpace(derefString(run.ExternalRuntimeID))
	if !ok || s.agentRepo == nil || runtimeRunID == "" {
		return usage, nil
	}
	agent, err := s.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID)
	if err != nil {
		return usage, fmt.Errorf("load agent for subtask usage: %w", err)
	}
	// shortcut: only agents that list the tool on their own row can spawn
	// subtasks today; revisit if a preset grants it without storing it there.
	if agent == nil || !bytes.Contains(agent.AllowedTools, []byte(`"spawn_subtasks"`)) {
		return usage, nil
	}
	calls, err := reader.ListToolCalls(ctx, runtimeRunID)
	if err != nil {
		return usage, fmt.Errorf("list tool calls for subtask usage: %w", err)
	}
	for _, jobID := range spawnedSubtaskJobIDs(calls) {
		job, err := reader.GetWorkJob(ctx, jobID)
		var apiErr *agentruntime.HTTPStatusError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			continue
		}
		if err != nil {
			return agentRuntimeUsagePayload{}, fmt.Errorf("get subtask job usage: %w", err)
		}
		if job == nil {
			continue
		}
		usage.InputTokens += int(job.Usage.InputTokens)
		usage.OutputTokens += int(job.Usage.OutputTokens)
		usage.CachedInputTokens += int(job.Usage.CachedInputTokens)
		usage.TotalTokens += int(job.Usage.InputTokens + job.Usage.OutputTokens)
	}
	return usage, nil
}

func addAgentRuntimeUsage(a, b agentRuntimeUsagePayload) agentRuntimeUsagePayload {
	return agentRuntimeUsagePayload{
		TotalTokens:           a.TotalTokens + b.TotalTokens,
		InputTokens:           a.InputTokens + b.InputTokens,
		CachedInputTokens:     a.CachedInputTokens + b.CachedInputTokens,
		OutputTokens:          a.OutputTokens + b.OutputTokens,
		ReasoningOutputTokens: a.ReasoningOutputTokens + b.ReasoningOutputTokens,
	}
}

func storedSubtaskUsage(run *model.AgentRun) agentRuntimeUsagePayload {
	var body map[string]any
	if run == nil || json.Unmarshal(run.OutputSummary, &body) != nil {
		return agentRuntimeUsagePayload{}
	}
	stored, _ := body[agentRuntimeSubtaskUsageSummaryKey].(map[string]any)
	usage, _ := usageFromMap(stored)
	return usage
}

func storeSubtaskUsage(run *model.AgentRun, usage agentRuntimeUsagePayload) {
	body := map[string]any{}
	if len(run.OutputSummary) > 0 {
		_ = json.Unmarshal(run.OutputSummary, &body)
	}
	body[agentRuntimeSubtaskUsageSummaryKey] = map[string]int{
		"total_tokens":        usage.TotalTokens,
		"input_tokens":        usage.InputTokens,
		"cached_input_tokens": usage.CachedInputTokens,
		"output_tokens":       usage.OutputTokens,
	}
	if payload, err := json.Marshal(body); err == nil {
		run.OutputSummary = payload
	}
}

var _subtaskJobIDPattern = regexp.MustCompile(`workjob_[A-Za-z0-9]+`)

// spawnedSubtaskJobIDs returns the job ids created by a run's spawn_subtasks
// calls. The runtime wraps a tool's JSON result in an envelope as a string, so
// the id is matched in the raw output instead of decoded from a fixed shape.
func spawnedSubtaskJobIDs(calls []AgentRuntimeToolCall) []string {
	var ids []string
	for _, call := range calls {
		if call.ToolName != "spawn_subtasks" {
			continue
		}
		if id := _subtaskJobIDPattern.Find(call.Output); id != nil {
			ids = append(ids, string(id))
		}
	}
	return ids
}
