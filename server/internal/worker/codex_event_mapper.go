package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

type codexEventMapper struct {
	execCtx *ExecutionContext
	run     *appmodel.AgentRun
	writer  *codexArtifactWriter

	result         ExecutionResult
	pendingRequest *codexPendingRequest
	completedTurn  *codexTurn
	latestDiff     string

	assistantText      strings.Builder
	assistantMessageID string
	assistantStarted   bool
	assistantCompleted bool
	liveTools          map[string]codexLiveToolCall

	stdout strings.Builder
	stderr strings.Builder
}

type codexLiveToolCall struct {
	Name    string
	Input   string
	Started time.Time
}

func newCodexEventMapper(execCtx *ExecutionContext, run *appmodel.AgentRun, writer *codexArtifactWriter) *codexEventMapper {
	return &codexEventMapper{
		execCtx:   execCtx,
		run:       run,
		writer:    writer,
		liveTools: map[string]codexLiveToolCall{},
	}
}

func (m *codexEventMapper) HandleNotification(ctx context.Context, method string, params json.RawMessage) error {
	switch strings.TrimSpace(method) {
	case "thread/started":
		var payload codexThreadStartedNotification
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		m.appendStdout(ctx, "Codex session started.\n", true)
	case "turn/started":
		var payload codexTurnStartedNotification
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		m.appendStdout(ctx, "Codex started the turn.\n", true)
	case "thread/tokenUsage/updated":
		var payload codexThreadTokenUsageUpdatedNotification
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		m.result.Usage = ExecutionUsage{
			CachedInputTokens: int(payload.TokenUsage.Total.CachedInputTokens),
			InputTokens:       int(payload.TokenUsage.Total.InputTokens),
			OutputTokens:      int(payload.TokenUsage.Total.OutputTokens),
		}
	case "turn/diff/updated":
		var payload codexTurnDiffUpdatedNotification
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		m.latestDiff = strings.TrimSpace(normalizeCodexUnifiedDiff(m.execCtx, payload.Diff))
	case "turn/plan/updated":
		var payload codexTurnPlanUpdatedNotification
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		plan := RunPlanArtifact{
			Plan: make([]RunPlanStep, 0, len(payload.Plan)),
		}
		if payload.Explanation != nil {
			plan.Note = strings.TrimSpace(*payload.Explanation)
		}
		for _, step := range payload.Plan {
			plan.Plan = append(plan.Plan, RunPlanStep{
				Step:   strings.TrimSpace(step.Step),
				Status: codexPlanStepStatus(step.Status),
			})
		}
		if err := ValidateRunPlanArtifactForContext(&plan); err == nil {
			input, _ := json.Marshal(plan)
			metadata, _ := json.Marshal(map[string]any{
				"runtime_kind":       "codex",
				"codex_request_kind": "turn_plan",
				"codex_turn_id":      strings.TrimSpace(payload.TurnID),
			})
			m.result.ToolInvocations = append(m.result.ToolInvocations, appmodel.ToolInvocation{
				ToolName:      ToolUpdatePlan,
				Input:         input,
				OutputSummary: "plan updated",
			})
			m.result.RunPlanMetadata = metadata
			// Emit a live event so the plan panel updates immediately during streaming.
			m.emitEvent(ExecutionEvent{
				Type:    "plan_updated",
				Content: string(input),
			})
		}
	case "item/agentMessage/delta":
		var payload codexAgentMessageDeltaNotification
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		m.appendAssistantDelta(payload.Delta)
	case "item/commandExecution/outputDelta":
		var payload codexCommandExecutionOutputDeltaNotification
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		m.appendStdout(ctx, payload.Delta, true)
	case "item/started":
		var payload codexItemStartedNotification
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		m.handleItemStarted(payload.Item)
	case "item/completed":
		var payload codexItemCompletedNotification
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		m.handleItemCompleted(payload.Item)
	case "error":
		var payload codexErrorNotification
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		if payload.Error != nil && strings.TrimSpace(payload.Error.Message) != "" {
			m.appendStderr(ctx, strings.TrimSpace(payload.Error.Message)+"\n", true)
		}
	case "turn/completed":
		var payload codexTurnCompletedNotification
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		m.completedTurn = &payload.Turn
		if payload.Turn.Error != nil && strings.TrimSpace(payload.Turn.Error.Message) != "" {
			m.appendStderr(ctx, strings.TrimSpace(payload.Turn.Error.Message)+"\n", true)
		}
		m.completeAssistantStream()
		m.appendStdout(ctx, "Codex finished the turn.\n", true)
	case "serverRequest/resolved":
		return nil
	default:
		return nil
	}
	return nil
}

func (m *codexEventMapper) HandleRequest(method string, id json.RawMessage, params json.RawMessage) error {
	switch strings.TrimSpace(method) {
	case "item/tool/requestUserInput":
		if m.execCtx != nil && !RequestUserInputUsesRuntimeBridge(m.execCtx.SkillPolicy, "codex") {
			return nil
		}
		var payload codexToolRequestUserInputParams
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		invocation, metadata, pending := codexHumanInputPause(payload, id)
		m.result.ToolInvocations = append(m.result.ToolInvocations, invocation)
		m.result.HumanInputMetadata = metadata
		m.pendingRequest = pending
		m.ensurePauseAssistantText("Codex needs input to continue.")
		return nil
	case "item/commandExecution/requestApproval":
		var payload codexCommandExecutionRequestApprovalParams
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		invocation, metadata, pending := codexHumanApprovalPause(
			codexPendingRequestKindCommandApproval,
			"Approve command execution",
			codexCommandApprovalSummary(payload),
			id,
			payload.TurnID,
			payload.ItemID,
			params,
		)
		m.result.ToolInvocations = append(m.result.ToolInvocations, invocation)
		m.result.HumanApprovalMetadata = metadata
		m.pendingRequest = pending
		m.ensurePauseAssistantText("Codex needs approval to execute a command.")
		return nil
	case "item/fileChange/requestApproval":
		var payload codexFileChangeRequestApprovalParams
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		invocation, metadata, pending := codexHumanApprovalPause(
			codexPendingRequestKindFileApproval,
			"Approve file changes",
			codexFileChangeApprovalSummary(payload, m.latestDiff),
			id,
			payload.TurnID,
			payload.ItemID,
			params,
		)
		m.result.ToolInvocations = append(m.result.ToolInvocations, invocation)
		m.result.HumanApprovalMetadata = metadata
		m.pendingRequest = pending
		m.ensurePauseAssistantText("Codex needs approval to apply file changes.")
		return nil
	case "item/permissions/requestApproval":
		var payload codexPermissionsRequestApprovalParams
		if err := json.Unmarshal(params, &payload); err != nil {
			return err
		}
		invocation, metadata, pending := codexHumanApprovalPause(
			codexPendingRequestKindPermissions,
			"Approve additional permissions",
			codexPermissionsApprovalSummary(payload),
			id,
			payload.TurnID,
			payload.ItemID,
			params,
		)
		m.result.ToolInvocations = append(m.result.ToolInvocations, invocation)
		m.result.HumanApprovalMetadata = metadata
		m.pendingRequest = pending
		m.ensurePauseAssistantText("Codex needs approval for additional permissions.")
		return nil
	default:
		return nil
	}
}

func (m *codexEventMapper) ensurePauseAssistantText(text string) {
	if strings.TrimSpace(m.assistantText.String()) == "" {
		m.appendAssistantDelta(strings.TrimSpace(text))
	}
	m.completeAssistantStream()
}

func (m *codexEventMapper) ensureAssistantMessageID() string {
	if strings.TrimSpace(m.assistantMessageID) == "" {
		m.assistantMessageID = uuid.NewString()
	}
	return strings.TrimSpace(m.assistantMessageID)
}

func (m *codexEventMapper) appendAssistantDelta(text string) {
	if text == "" {
		return
	}
	if !m.assistantStarted {
		m.assistantStarted = true
		m.emitEvent(ExecutionEvent{
			Type:      "assistant_message_started",
			MessageID: m.ensureAssistantMessageID(),
		})
	}
	m.assistantText.WriteString(text)
	m.emitEvent(ExecutionEvent{
		Type:      "assistant_message_delta",
		MessageID: m.ensureAssistantMessageID(),
		Text:      text,
		Content:   text,
	})
}

func (m *codexEventMapper) completeAssistantStream() {
	if !m.assistantStarted || m.assistantCompleted {
		return
	}
	m.assistantCompleted = true
	finalText := strings.TrimSpace(m.assistantText.String())
	m.emitEvent(ExecutionEvent{
		Type:      "assistant_message_completed",
		MessageID: m.ensureAssistantMessageID(),
		Text:      finalText,
		Content:   finalText,
	})
}

func (m *codexEventMapper) handleItemStarted(item codexThreadItem) {
	toolName, input := codexToolEventDetails(m.execCtx, item)
	if toolName == "" {
		return
	}
	m.liveTools[item.ID] = codexLiveToolCall{
		Name:    toolName,
		Input:   input,
		Started: time.Now(),
	}
	parentMessageID := m.ensureAssistantMessageID()
	argsText := strings.TrimSpace(input)
	m.emitEvent(ExecutionEvent{
		Type:            "tool_call_started",
		ToolCallID:      strings.TrimSpace(item.ID),
		ToolName:        toolName,
		ToolInput:       input,
		ParentMessageID: parentMessageID,
		ArgsText:        argsText,
	})
	if argsText != "" {
		m.emitEvent(ExecutionEvent{
			Type:            "tool_call_args_delta",
			ToolCallID:      strings.TrimSpace(item.ID),
			ToolName:        toolName,
			ParentMessageID: parentMessageID,
			ArgsDelta:       argsText,
			ArgsText:        argsText,
		})
	}
}

func (m *codexEventMapper) handleItemCompleted(item codexThreadItem) {
	switch strings.TrimSpace(item.Type) {
	case "agentMessage":
		text := strings.TrimSpace(item.Text)
		if text != "" && strings.TrimSpace(m.assistantText.String()) == "" {
			m.appendAssistantDelta(text)
		}
		m.completeAssistantStream()
		return
	}

	toolName, _ := codexToolEventDetails(m.execCtx, item)
	if toolName == "" {
		return
	}

	live := m.liveTools[item.ID]
	delete(m.liveTools, item.ID)
	durationMs := item.DurationMs
	if durationMs == nil && !live.Started.IsZero() {
		derived := time.Since(live.Started).Milliseconds()
		durationMs = &derived
	}
	outputSummary := codexToolOutputSummary(m.execCtx, item)
	parentMessageID := m.ensureAssistantMessageID()
	resultMessageID := uuid.NewString()
	errorText := ""
	if codexItemFailed(item) {
		errorText = outputSummary
	}
	m.emitEvent(ExecutionEvent{
		Type:            "tool_call_result",
		ToolCallID:      strings.TrimSpace(item.ID),
		ToolName:        toolName,
		ParentMessageID: parentMessageID,
		ResultMessageID: resultMessageID,
		Content:         outputSummary,
		OutputSummary:   outputSummary,
		Error:           errorText,
	})
	m.emitEvent(ExecutionEvent{
		Type:            "tool_call_finished",
		ToolCallID:      strings.TrimSpace(item.ID),
		ToolName:        toolName,
		ParentMessageID: parentMessageID,
		ResultMessageID: resultMessageID,
		OutputSummary:   outputSummary,
		Content:         outputSummary,
		DurationMs:      derefInt64(durationMs),
		Error:           errorText,
	})

	if strings.TrimSpace(item.Type) == "fileChange" && m.latestDiff == "" {
		m.latestDiff = codexDiffFromFileChange(m.execCtx, item)
	}
	if summary := strings.TrimSpace(outputSummary); summary != "" {
		m.result.Messages = append(m.result.Messages, ExecutionMessage{
			Role:    "tool",
			Content: summary,
			Blocks: []ExecutionBlock{{
				Type:     ExecutionBlockTypeToolResult,
				ToolName: toolName,
				Output:   summary,
				IsError:  codexItemFailed(item),
			}},
		})
	}
}

func (m *codexEventMapper) appendStdout(ctx context.Context, text string, notify bool) {
	if text == "" {
		return
	}
	m.stdout.WriteString(text)
	if m.writer != nil {
		m.writer.Save(ctx, "codex_stdout_chunk", "text", text, notify)
	}
}

func (m *codexEventMapper) appendStderr(ctx context.Context, text string, notify bool) {
	if text == "" {
		return
	}
	m.stderr.WriteString(text)
	if m.writer != nil {
		m.writer.Save(ctx, "codex_stderr_chunk", "text", text, notify)
	}
}

func (m *codexEventMapper) emitEvent(event ExecutionEvent) {
	if m == nil || m.execCtx == nil || m.execCtx.OnExecutionEvent == nil {
		return
	}
	m.execCtx.OnExecutionEvent(event)
}

func (m *codexEventMapper) Result() *ExecutionResult {
	result := m.result
	result.AssistantText = strings.TrimSpace(m.assistantText.String())
	if result.AssistantText != "" {
		result.AssistantBlocks = []ExecutionBlock{{
			Type: ExecutionBlockTypeText,
			Text: result.AssistantText,
		}}
	}
	return &result
}

func (m *codexEventMapper) PendingRequest() *codexPendingRequest {
	return m.pendingRequest
}

func (m *codexEventMapper) CompletedTurn() *codexTurn {
	return m.completedTurn
}

func (m *codexEventMapper) Stdout() string {
	return m.stdout.String()
}

func (m *codexEventMapper) Stderr() string {
	return m.stderr.String()
}

func (m *codexEventMapper) LatestDiff() string {
	return strings.TrimSpace(m.latestDiff)
}

func codexPlanStepStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "completed":
		return PlanStepCompleted
	case "inprogress":
		return PlanStepInProgress
	default:
		return PlanStepPending
	}
}

func codexToolEventDetails(execCtx *ExecutionContext, item codexThreadItem) (string, string) {
	switch strings.TrimSpace(item.Type) {
	case "commandExecution":
		command := strings.TrimSpace(item.Command)
		if command == "" {
			command = "command"
		}
		return "run_command", command
	case "fileChange":
		return "apply_patch", codexDiffFromFileChange(execCtx, item)
	case "mcpToolCall":
		name := strings.TrimSpace(item.Tool)
		if server := strings.TrimSpace(item.Server); server != "" && name != "" {
			name = server + "/" + name
		}
		return firstNonEmptyText(name, "mcp_tool_call"), strings.TrimSpace(string(item.Arguments))
	case "dynamicToolCall":
		return firstNonEmptyText(strings.TrimSpace(item.Tool), "dynamic_tool_call"), strings.TrimSpace(string(item.Arguments))
	default:
		return "", ""
	}
}

func codexToolOutputSummary(execCtx *ExecutionContext, item codexThreadItem) string {
	switch strings.TrimSpace(item.Type) {
	case "commandExecution":
		if item.AggregatedOutput != nil && strings.TrimSpace(*item.AggregatedOutput) != "" {
			return truncate(strings.TrimSpace(*item.AggregatedOutput), 4000)
		}
		switch strings.ToLower(strings.TrimSpace(item.Status)) {
		case "completed":
			if item.ExitCode != nil {
				return fmt.Sprintf("command completed with exit code %d", *item.ExitCode)
			}
			return "command completed"
		case "declined":
			return "command approval was declined"
		case "failed":
			if item.ExitCode != nil {
				return fmt.Sprintf("command failed with exit code %d", *item.ExitCode)
			}
			return "command failed"
		default:
			return strings.TrimSpace(item.Status)
		}
	case "fileChange":
		diff := codexDiffFromFileChange(execCtx, item)
		if diff != "" {
			return truncate(diff, 4000)
		}
		if count := len(item.Changes); count > 0 {
			return fmt.Sprintf("%d file change(s)", count)
		}
		return firstNonEmptyText(strings.TrimSpace(item.Status), "file changes completed")
	case "mcpToolCall":
		if item.Error != nil && strings.TrimSpace(item.Error.Message) != "" {
			return strings.TrimSpace(item.Error.Message)
		}
		if strings.TrimSpace(string(item.Result)) != "" {
			return truncate(strings.TrimSpace(string(item.Result)), 4000)
		}
		return firstNonEmptyText(strings.TrimSpace(item.Status), "mcp tool call completed")
	case "dynamicToolCall":
		if item.Error != nil && strings.TrimSpace(item.Error.Message) != "" {
			return strings.TrimSpace(item.Error.Message)
		}
		if strings.TrimSpace(string(item.Result)) != "" {
			return truncate(strings.TrimSpace(string(item.Result)), 4000)
		}
		if item.Success != nil {
			return fmt.Sprintf("dynamic tool success=%t", *item.Success)
		}
		return firstNonEmptyText(strings.TrimSpace(item.Status), "dynamic tool call completed")
	default:
		return ""
	}
}

func codexDiffFromFileChange(execCtx *ExecutionContext, item codexThreadItem) string {
	if len(item.Changes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(item.Changes))
	for _, change := range item.Changes {
		diff := strings.TrimSpace(change.Diff)
		if diff == "" {
			continue
		}
		// Prepend standard unified diff file headers so the frontend can identify
		// which file each hunk belongs to when rendering the diff.
		normalizedPath := normalizeCodexDiffPath(execCtx, change.Path, item.Cwd)
		if normalizedPath != "" && !strings.HasPrefix(diff, "---") && !strings.HasPrefix(diff, "diff ") {
			diff = "--- a/" + normalizedPath + "\n+++ b/" + normalizedPath + "\n" + diff
		}
		diff = normalizeCodexUnifiedDiff(execCtx, diff, item.Cwd)
		parts = append(parts, diff)
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func normalizeCodexDiffPath(execCtx *ExecutionContext, rawPath string, extraRoots ...string) string {
	cleaned := strings.TrimSpace(rawPath)
	if cleaned == "" {
		return ""
	}

	for _, root := range append([]string{workDirForCodexDiff(execCtx)}, extraRoots...) {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		rel, ok := codexRelativePathWithinRoot(root, cleaned)
		if ok {
			return rel
		}
	}

	return codexDiffPathToSlashes(cleaned)
}

func workDirForCodexDiff(execCtx *ExecutionContext) string {
	if execCtx == nil {
		return ""
	}
	return strings.TrimSpace(execCtx.WorkDir)
}

func codexRelativePathWithinRoot(root, target string) (string, bool) {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return "", false
	}
	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "." {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return codexDiffPathToSlashes(rel), true
}

func codexDiffPathToSlashes(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(value))
}

func normalizeCodexUnifiedDiff(execCtx *ExecutionContext, diff string, extraRoots ...string) string {
	if strings.TrimSpace(diff) == "" {
		return ""
	}
	lines := strings.Split(diff, "\n")
	for index, line := range lines {
		switch {
		case strings.HasPrefix(line, "--- a/"):
			lines[index] = "--- a/" + normalizeCodexDiffPath(execCtx, strings.TrimPrefix(line, "--- a/"), extraRoots...)
		case strings.HasPrefix(line, "+++ b/"):
			lines[index] = "+++ b/" + normalizeCodexDiffPath(execCtx, strings.TrimPrefix(line, "+++ b/"), extraRoots...)
		case strings.HasPrefix(line, "rename from "):
			lines[index] = "rename from " + normalizeCodexDiffPath(execCtx, strings.TrimPrefix(line, "rename from "), extraRoots...)
		case strings.HasPrefix(line, "rename to "):
			lines[index] = "rename to " + normalizeCodexDiffPath(execCtx, strings.TrimPrefix(line, "rename to "), extraRoots...)
		case strings.HasPrefix(line, "copy from "):
			lines[index] = "copy from " + normalizeCodexDiffPath(execCtx, strings.TrimPrefix(line, "copy from "), extraRoots...)
		case strings.HasPrefix(line, "copy to "):
			lines[index] = "copy to " + normalizeCodexDiffPath(execCtx, strings.TrimPrefix(line, "copy to "), extraRoots...)
		case strings.HasPrefix(line, "diff --git a/"):
			lines[index] = normalizeCodexDiffGitHeader(execCtx, line, extraRoots...)
		}
	}
	return strings.Join(lines, "\n")
}

func normalizeCodexDiffGitHeader(execCtx *ExecutionContext, line string, extraRoots ...string) string {
	rest := strings.TrimPrefix(line, "diff --git a/")
	separator := " b/"
	idx := strings.Index(rest, separator)
	if idx <= 0 {
		return line
	}
	left := normalizeCodexDiffPath(execCtx, rest[:idx], extraRoots...)
	right := normalizeCodexDiffPath(execCtx, rest[idx+len(separator):], extraRoots...)
	return "diff --git a/" + left + " b/" + right
}

func codexItemFailed(item codexThreadItem) bool {
	switch strings.ToLower(strings.TrimSpace(item.Status)) {
	case "failed", "declined":
		return true
	default:
		return item.Error != nil && strings.TrimSpace(item.Error.Message) != ""
	}
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
