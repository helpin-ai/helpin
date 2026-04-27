package temporalapp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

type nativeRepairInstruction struct {
	Source       string
	Class        string
	Instructions string
}

func latestUnresolvedNativeToolFailure(messages []model.AgentRunMessage) *workerpkg.ExecutionBlock {
	if len(messages) == 0 {
		return nil
	}

	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if strings.TrimSpace(message.Role) == "assistant" && shouldIncludeRunMessageInExecutionHistory(message) {
			break
		}
		if strings.TrimSpace(message.Role) != "tool" || strings.TrimSpace(message.MessageType) != "tool_result" {
			continue
		}
		for _, block := range parsePersistedExecutionBlocks(message.ContentBlocks) {
			if block.Type != workerpkg.ExecutionBlockTypeToolResult || !block.IsError || strings.TrimSpace(block.ToolName) == "" {
				continue
			}
			copied := block
			if strings.TrimSpace(copied.Output) == "" {
				copied.Output = strings.TrimSpace(message.Content)
			}
			return &copied
		}
	}
	return nil
}

func latestExecutionToolFailure(messages []workerpkg.ExecutionMessage) *workerpkg.ExecutionBlock {
	toolMessages := finalRoundToolMessages(messages)
	for i := len(toolMessages) - 1; i >= 0; i-- {
		for j := len(toolMessages[i].Blocks) - 1; j >= 0; j-- {
			block := toolMessages[i].Blocks[j]
			if block.Type != workerpkg.ExecutionBlockTypeToolResult || !block.IsError {
				continue
			}
			copied := block
			return &copied
		}
	}
	return nil
}

func parsePersistedExecutionBlocks(raw json.RawMessage) []workerpkg.ExecutionBlock {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var blocks []workerpkg.ExecutionBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil
	}
	return workerpkg.NormalizeExecutionBlocks(blocks)
}

func classifyNativeToolFailureRepair(state *resolvedRunState, failure *workerpkg.ExecutionBlock) nativeRepairInstruction {
	if state == nil || !state.nativeSelectivePathEnabled || failure == nil {
		return nativeRepairInstruction{}
	}
	toolName := strings.TrimSpace(failure.ToolName)
	output := strings.TrimSpace(failure.Output)
	if toolName == "" || output == "" {
		return nativeRepairInstruction{}
	}

	switch toolName {
	case workerpkg.ToolPublishTaskPlan:
		return classifyPublishTaskPlanRepair(output)
	case workerpkg.ToolPublishTaskPlanDoc, workerpkg.ToolPublishPRDDraft:
		return classifyMarkdownPreviewRepair(toolName, output)
	default:
		return nativeRepairInstruction{}
	}
}

func classifyPublishTaskPlanRepair(output string) nativeRepairInstruction {
	switch {
	case strings.Contains(output, "publish_task_plan content must be a JSON object with summary and proposed_tasks"):
		return nativeRepairInstruction{
			Class: "publish_task_plan_object_shape",
			Instructions: strings.Join([]string{
				"Your previous publish_task_plan call failed validation because content was not a structured JSON object.",
				"Retry publish_task_plan with one complete JSON object in content.",
				`The content object must include a non-empty "summary" string and a "proposed_tasks" array of task objects.`,
				"Do not send markdown, prose wrappers, or stringified JSON blobs inside content.",
			}, "\n"),
		}
	case strings.Contains(output, "publish_task_plan requires content.proposed_tasks to be an array of task objects"):
		return nativeRepairInstruction{
			Class: "publish_task_plan_task_array_shape",
			Instructions: strings.Join([]string{
				"Your previous publish_task_plan call failed validation because proposed_tasks was not an array of task objects.",
				`Retry publish_task_plan with content.proposed_tasks as an array of full task objects, not strings, refs, placeholders, or partial fragments.`,
				"Each task entry should include the normal structured task fields expected by the task-plan contract.",
			}, "\n"),
		}
	case strings.Contains(output, `publish_task_plan is missing content; include the task plan JSON object in "content"`):
		return nativeRepairInstruction{
			Class: "publish_task_plan_missing_content",
			Instructions: strings.Join([]string{
				"Your previous publish_task_plan call failed validation because the content field was missing.",
				`Retry publish_task_plan with the full task-plan JSON object under "content".`,
				`Do not send title-only payloads; include content.summary and content.proposed_tasks in the same tool call.`,
			}, "\n"),
		}
	case strings.Contains(output, "publish_task_plan input must be a JSON object with structured fields; do not send a raw string wrapper"):
		return nativeRepairInstruction{
			Class: "publish_task_plan_raw_wrapper",
			Instructions: strings.Join([]string{
				"Your previous publish_task_plan call failed validation because the tool arguments arrived as raw text instead of a complete structured JSON object.",
				"This usually means the tool-call JSON was stringified, malformed, or cut off before the required content object was complete.",
				"Retry publish_task_plan with one compact, complete JSON object. Do not send a raw wrapper string, markdown, prose, or partial JSON.",
				`The top-level object must include "content", and content must include a non-empty "summary" string plus a "proposed_tasks" array of task objects.`,
				"Keep each task concise enough for one tool call while preserving name, description, task_type, acceptance_criteria, and dependency_refs.",
			}, "\n"),
		}
	default:
		return nativeRepairInstruction{}
	}
}

func classifyMarkdownPreviewRepair(toolName, output string) nativeRepairInstruction {
	toolName = strings.TrimSpace(toolName)
	contentLabel := "full markdown draft"
	switch toolName {
	case workerpkg.ToolPublishPRDDraft:
		contentLabel = "full PRD markdown draft"
	case workerpkg.ToolPublishTaskPlanDoc:
		contentLabel = "full task planning markdown draft"
	}

	switch {
	case strings.Contains(output, fmt.Sprintf(`%s is missing content; include markdown in "content"`, toolName)):
		return nativeRepairInstruction{
			Class: toolName + "_missing_content",
			Instructions: strings.Join([]string{
				fmt.Sprintf("Your previous %s call failed validation because the content field was missing.", toolName),
				fmt.Sprintf(`Retry %s with the %s under "content".`, toolName, contentLabel),
				"Do not send title-only payloads when publishing a markdown preview for review.",
			}, "\n"),
		}
	case strings.Contains(output, fmt.Sprintf(`%s content must be a markdown string in "content"`, toolName)):
		return nativeRepairInstruction{
			Class: toolName + "_markdown_type",
			Instructions: strings.Join([]string{
				fmt.Sprintf("Your previous %s call failed validation because content was not a markdown string.", toolName),
				fmt.Sprintf(`Retry %s with the %s as a plain markdown string in "content".`, toolName, contentLabel),
				"Do not send JSON objects, arrays, or other non-string content to markdown preview tools.",
			}, "\n"),
		}
	default:
		return nativeRepairInstruction{}
	}
}

func latestUnresolvedPolicyRetryMessage(messages []model.AgentRunMessage) *model.AgentRunMessage {
	sawLaterAssistant := false
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if strings.TrimSpace(message.Role) == "assistant" && shouldIncludeRunMessageInExecutionHistory(message) {
			sawLaterAssistant = true
		}
		if strings.TrimSpace(message.MessageType) != "policy_retry" {
			continue
		}
		if sawLaterAssistant {
			return nil
		}
		copied := message
		return &copied
	}
	return nil
}

func classifyNativeRepairInstruction(state *resolvedRunState, message *model.AgentRunMessage) nativeRepairInstruction {
	if state == nil || !state.nativeSelectivePathEnabled || message == nil {
		return nativeRepairInstruction{}
	}
	content := strings.TrimSpace(message.Content)
	if content == "" {
		return nativeRepairInstruction{}
	}

	switch {
	case strings.Contains(content, "multiple same-turn previews") && strings.Contains(content, "preview_panel_key"):
		return nativeRepairInstruction{
			Source: "policy_retry",
			Class:  "approval_preview_panel_key_required",
			Instructions: strings.Join([]string{
				"Continue from your last assistant turn instead of restarting the run.",
				"If this turn requests approval or a review checkpoint after publishing multiple previews, include preview_panel_key so the handoff binds to the intended preview.",
				"Publish the target preview in the same turn before the approval handoff, then treat request_approval or request_review_checkpoint as the final action in that turn.",
			}, "\n"),
		}
	case strings.Contains(content, "required same-turn ") && strings.Contains(content, " preview"):
		requiredPreviewKey := extractRequiredSameTurnPreviewKey(content)
		lines := []string{
			"Continue from your last assistant turn instead of restarting the run.",
			"If this turn requests approval or a review checkpoint, first publish the required preview in the same turn before the handoff.",
		}
		if requiredPreviewKey != "" {
			lines = append(lines,
				fmt.Sprintf("Use preview_panel_key=%q so the approval request binds to the %s preview.", requiredPreviewKey, requiredPreviewKey),
				fmt.Sprintf("If that %s preview is missing or stale, republish it in the same turn before requesting approval.", requiredPreviewKey),
			)
		} else {
			lines = append(lines, "Include preview_panel_key when needed so the approval request binds to the intended preview.")
		}
		lines = append(lines, "Treat request_approval or request_review_checkpoint as the final action in that turn.")
		return nativeRepairInstruction{
			Source:       "policy_retry",
			Class:        "approval_specific_preview_required",
			Instructions: strings.Join(lines, "\n"),
		}
	case strings.Contains(content, "same-turn preview") || strings.Contains(content, "preview_panel_key"):
		return nativeRepairInstruction{
			Source: "policy_retry",
			Class:  "approval_preview_binding",
			Instructions: strings.Join([]string{
				"Continue from your last assistant turn instead of restarting the run.",
				"If this turn requests approval or a review checkpoint, first publish the preview in the same turn before the approval handoff.",
				"When multiple same-turn previews exist, include preview_panel_key so the approval request binds to the correct preview.",
				"Treat request_approval or request_review_checkpoint as the final action in that turn.",
			}, "\n"),
		}
	case strings.Contains(content, "review_checkpoint handoff"):
		return nativeRepairInstruction{
			Source: "policy_retry",
			Class:  "review_checkpoint_handoff",
			Instructions: strings.Join([]string{
				"Continue from your last assistant turn instead of restarting the review.",
				"Before the run stops, emit a review_checkpoint handoff using the runtime-appropriate mechanism.",
				"Only emit request_user_input instead if the human explicitly closed the review or asked a blocking follow-up question.",
				"Do not end the turn with prose only.",
			}, "\n"),
		}
	default:
		requiredKinds := sortedCompletionInteractionKinds(completionRequiredInteractionKinds(state.skillPolicy))
		requiredKindsText := "the required interaction handoff"
		if len(requiredKinds) > 0 {
			requiredKindsText = fmt.Sprintf("one of the required interaction handoffs [%s]", strings.Join(requiredKinds, ", "))
		}
		return nativeRepairInstruction{
			Source: "policy_retry",
			Class:  "required_interaction_handoff",
			Instructions: strings.Join([]string{
				"Continue from your last assistant turn instead of restarting the run.",
				fmt.Sprintf("Before the run stops, emit %s declared by the active skill policy.", requiredKindsText),
				"Do not end the turn with prose only.",
			}, "\n"),
		}
	}
}

func extractRequiredSameTurnPreviewKey(content string) string {
	return extractPreviewKeyAfterMarker(content, "required same-turn ", " preview")
}

func latestNativeRepairInstruction(state *resolvedRunState, messages []model.AgentRunMessage) nativeRepairInstruction {
	if state == nil || !state.nativeSelectivePathEnabled {
		return nativeRepairInstruction{}
	}
	if failure := latestUnresolvedNativeToolFailure(messages); failure != nil {
		if instruction := classifyNativeToolFailureRepair(state, failure); strings.TrimSpace(instruction.Instructions) != "" {
			if strings.TrimSpace(instruction.Source) == "" {
				instruction.Source = "tool_result_history"
			}
			return instruction
		}
	}
	message := latestUnresolvedPolicyRetryMessage(messages)
	if message == nil {
		return nativeRepairInstruction{}
	}
	instruction := classifyNativeRepairInstruction(state, message)
	if strings.TrimSpace(instruction.Instructions) != "" {
		if strings.TrimSpace(instruction.Source) == "" {
			instruction.Source = "policy_retry"
		}
		return instruction
	}
	return nativeRepairInstruction{
		Source:       "policy_retry",
		Class:        "raw_policy_retry",
		Instructions: strings.TrimSpace(message.Content),
	}
}

func latestNativeRepairInstructionFromArtifacts(messages []model.AgentRunMessage, artifacts []model.AgentRunArtifact) nativeRepairInstruction {
	if len(messages) == 0 || len(artifacts) == 0 {
		return nativeRepairInstruction{}
	}
	latestAssistantSeq := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.TrimSpace(messages[i].Role) != "assistant" {
			continue
		}
		latestAssistantSeq = messages[i].SequenceNo
		break
	}
	if latestAssistantSeq <= 0 {
		return nativeRepairInstruction{}
	}
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeNativeRepairState || artifact.InlineContent == nil {
			continue
		}
		if artifactAssistantMessageSequenceNo(artifact) != latestAssistantSeq {
			continue
		}
		var payload model.NativeRepairState
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
			continue
		}
		hint := strings.TrimSpace(payload.RepairHint)
		if hint == "" {
			continue
		}
		return nativeRepairInstruction{
			Source:       nativeRepairArtifactSource(payload.Source),
			Class:        strings.TrimSpace(payload.RepairClass),
			Instructions: hint,
		}
	}
	return nativeRepairInstruction{}
}

func nativeRepairArtifactSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "native_repair_state"
	}
	return "native_repair_state:" + source
}

func resolveLatestNativeRepairInstruction(state *resolvedRunState, messages []model.AgentRunMessage, artifacts []model.AgentRunArtifact) nativeRepairInstruction {
	if instruction := latestNativeRepairInstructionFromArtifacts(messages, artifacts); strings.TrimSpace(instruction.Instructions) != "" {
		return instruction
	}
	return latestNativeRepairInstruction(state, messages)
}

func normalizedCompletionRetryInstruction(state *resolvedRunState, cause error) nativeRepairInstruction {
	if cause == nil || strings.TrimSpace(cause.Error()) == "" {
		return nativeRepairInstruction{}
	}
	causeText := strings.TrimSpace(cause.Error())
	if state != nil && state.agent != nil && strings.TrimSpace(state.agent.EffectivePresetKey()) == model.AgentPresetReviewAgent {
		return nativeRepairInstruction{
			Source:       "completion_retry",
			Class:        "review_checkpoint_handoff",
			Instructions: "System correction: the previous review turn ended without the required interaction. Continue from your last assistant message instead of restarting. Do not end with prose only. In this next turn, emit a review_checkpoint handoff using the runtime-appropriate mechanism, or emit request_user_input only if the human explicitly closed the review or asked a blocking follow-up.",
		}
	}
	switch {
	case strings.Contains(causeText, "requires preview_panel_key when multiple same-turn previews exist"):
		return nativeRepairInstruction{
			Source:       "completion_retry",
			Class:        "approval_preview_panel_key_required",
			Instructions: approvalPreviewRetryInstruction(causeText),
		}
	case strings.Contains(causeText, "requires a same-turn ") && strings.Contains(causeText, " preview before requesting approval"):
		return nativeRepairInstruction{
			Source:       "completion_retry",
			Class:        "approval_specific_preview_required",
			Instructions: approvalPreviewRetryInstruction(causeText),
		}
	case strings.Contains(causeText, "same-turn") || strings.Contains(causeText, "preview_panel_key"):
		return nativeRepairInstruction{
			Source:       "completion_retry",
			Class:        "approval_preview_binding",
			Instructions: "System correction: the previous turn requested approval without binding it to a same-turn preview. Continue from your last assistant message instead of restarting. Do not end with prose only. If you emit request_approval or request_review_checkpoint, first publish the preview in the same turn. When multiple previews exist in that turn, include preview_panel_key so it binds to the correct preview.",
		}
	default:
		policy := workerpkg.SkillPolicy{}
		if state != nil {
			policy = state.skillPolicy
		}
		requiredKinds := sortedCompletionInteractionKinds(completionRequiredInteractionKinds(policy))
		instruction := "System correction: the previous turn ended without creating the required interaction. Continue from your last assistant message instead of restarting. Do not end with prose only. Before this run stops, emit one of the required interaction handoffs declared by the active skill policy."
		if len(requiredKinds) > 0 {
			instruction = instruction + " Required interaction kinds for this turn: " + strings.Join(requiredKinds, ", ") + "."
		}
		return nativeRepairInstruction{
			Source:       "completion_retry",
			Class:        "required_interaction_handoff",
			Instructions: instruction,
		}
	}
}
