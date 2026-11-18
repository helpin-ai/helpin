package service

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const dockChatWorkSummaryMessageType = "status"

func isDockChatDecision(message model.AgentRunMessage) bool {
	switch message.MessageType {
	case "approval", "approval_request_resolution", "review_checkpoint_resolution":
		return true
	default:
		return false
	}
}

func compactDockChatMessagePage(messages []model.AgentRunMessage) []model.AgentRunMessage {
	compact := make([]model.AgentRunMessage, 0, len(messages))
	assistantGroup := make([]model.AgentRunMessage, 0, 4)
	var intervalStartedAt time.Time

	flushAssistants := func() {
		if len(assistantGroup) == 0 {
			return
		}
		// Approval/resume messages are activity, not new human turns or answers.
		assistants := make([]model.AgentRunMessage, 0, len(assistantGroup))
		for _, message := range assistantGroup {
			if message.Role == "assistant" && !isDockChatDecision(message) {
				assistants = append(assistants, message)
			}
		}
		if len(assistants) == 0 {
			compact = append(compact, assistantGroup...)
			assistantGroup = assistantGroup[:0]
			return
		}
		final := finalDockAssistantMessage(assistants)
		workTarget := assistants[len(assistants)-1]
		// In-flight explicit progress is not a completed conversation answer.
		explicitProgress := false
		for _, message := range assistantGroup {
			explicitProgress = explicitProgress || message.MessageType == "assistant_progress"
		}
		if (explicitProgress || isDockChatDecision(assistantGroup[len(assistantGroup)-1])) && final.MessageType != "assistant_final" {
			compact = append(compact, assistantGroup...)
			assistantGroup = assistantGroup[:0]
			return
		}
		activityCount := dockAssistantActivityCount(assistantGroup)
		hasWork := len(assistantGroup) > 1 || activityCount > 0
		if hasWork {
			duration := final.CreatedAt.Sub(intervalStartedAt)
			if intervalStartedAt.IsZero() || duration < 0 {
				duration = 0
			}
			compact = append(compact, model.AgentRunMessage{
				ID:               "work:" + final.ID,
				WorkspaceID:      final.WorkspaceID,
				RunID:            final.RunID,
				DockChatID:       final.DockChatID,
				DockChatSequence: final.DockChatSequence,
				Role:             "assistant",
				MessageType:      dockChatWorkSummaryMessageType,
				CreatedAt:        final.CreatedAt,
				DockWorkSummary: &model.DockChatWorkSummary{
					MessageID:     workTarget.ID,
					DurationMs:    duration.Milliseconds(),
					ActivityCount: activityCount,
					SubtaskJobIDs: dockSubtaskJobIDs(assistantGroup),
				},
			})
		}
		final.TurnSegments = nil
		final.ToolInvocations = nil
		compact = append(compact, final)
		assistantGroup = assistantGroup[:0]
	}

	for _, message := range messages {
		if message.Role == "assistant" || isDockChatDecision(message) {
			assistantGroup = append(assistantGroup, message)
			if intervalStartedAt.IsZero() {
				intervalStartedAt = message.CreatedAt
			}
			continue
		}
		flushAssistants()
		compact = append(compact, message)
		if message.Role == "user" {
			intervalStartedAt = message.CreatedAt
		}
	}
	flushAssistants()
	return compact
}

func finalDockAssistantMessage(messages []model.AgentRunMessage) model.AgentRunMessage {
	// Explicit final answers outrank later-arriving progress/tool metadata.
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].MessageType == "assistant_final" && strings.TrimSpace(messages[i].Content) != "" {
			return messages[i]
		}
	}
	for messageIndex := len(messages) - 1; messageIndex >= 0; messageIndex-- {
		message := messages[messageIndex]
		var segments []model.CodingSessionLiveTurnSegment
		if len(message.TurnSegments) > 0 && json.Unmarshal(message.TurnSegments, &segments) == nil {
			for segmentIndex := len(segments) - 1; segmentIndex >= 0; segmentIndex-- {
				assistant := segments[segmentIndex].AssistantMessage
				if segments[segmentIndex].Kind == "assistant_message" && assistant != nil && strings.TrimSpace(assistant.Content) != "" {
					message.Content = strings.TrimSpace(assistant.Content)
					return message
				}
			}
		}
		if strings.TrimSpace(message.Content) != "" {
			message.Content = strings.TrimSpace(message.Content)
			return message
		}
	}
	return messages[len(messages)-1]
}

func dockAssistantActivityCount(messages []model.AgentRunMessage) int {
	count := 0
	for _, message := range messages {
		if isDockChatDecision(message) {
			count++
			continue
		}
		var segments []model.CodingSessionLiveTurnSegment
		if len(message.TurnSegments) > 0 && json.Unmarshal(message.TurnSegments, &segments) == nil {
			for _, segment := range segments {
				if segment.Kind == "tool_call" || segment.Kind == "reasoning" {
					count++
				}
			}
		}
		if len(message.ToolInvocations) > 0 && len(segments) == 0 {
			var invocations []json.RawMessage
			if json.Unmarshal(message.ToolInvocations, &invocations) == nil {
				count += len(invocations)
			}
		}
	}
	return count
}

// dockSubtaskJobIDs returns the subtask jobs spawned in a turn's messages, in
// order and without repeats. Later calls such as wait_for_subtasks repeat the
// same id, so every tool record of a message that spawned is scanned.
func dockSubtaskJobIDs(messages []model.AgentRunMessage) []string {
	var ids []string
	for _, message := range messages {
		for _, raw := range [][]byte{message.ToolInvocations, message.TurnSegments} {
			if !bytes.Contains(raw, []byte("spawn_subtasks")) {
				continue
			}
			for _, id := range _subtaskJobIDPattern.FindAll(raw, -1) {
				if !slices.Contains(ids, string(id)) {
					ids = append(ids, string(id))
				}
			}
		}
	}
	return ids
}
