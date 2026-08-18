package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const dockChatWorkSummaryMessageType = "status"

func compactDockChatMessagePage(messages []model.AgentRunMessage) []model.AgentRunMessage {
	compact := make([]model.AgentRunMessage, 0, len(messages))
	assistantGroup := make([]model.AgentRunMessage, 0, 4)
	var intervalStartedAt time.Time

	flushAssistants := func() {
		if len(assistantGroup) == 0 {
			return
		}
		final := assistantGroup[len(assistantGroup)-1]
		final.Content = finalDockAssistantContent(assistantGroup)
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
					MessageID:     final.ID,
					DurationMs:    duration.Milliseconds(),
					ActivityCount: activityCount,
				},
			})
		}
		final.TurnSegments = nil
		final.ToolInvocations = nil
		compact = append(compact, final)
		assistantGroup = assistantGroup[:0]
	}

	for _, message := range messages {
		if message.Role == "assistant" {
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

func finalDockAssistantContent(messages []model.AgentRunMessage) string {
	for messageIndex := len(messages) - 1; messageIndex >= 0; messageIndex-- {
		message := messages[messageIndex]
		var segments []model.CodingSessionLiveTurnSegment
		if len(message.TurnSegments) > 0 && json.Unmarshal(message.TurnSegments, &segments) == nil {
			for segmentIndex := len(segments) - 1; segmentIndex >= 0; segmentIndex-- {
				assistant := segments[segmentIndex].AssistantMessage
				if segments[segmentIndex].Kind == "assistant_message" && assistant != nil && strings.TrimSpace(assistant.Content) != "" {
					return strings.TrimSpace(assistant.Content)
				}
			}
		}
		if strings.TrimSpace(message.Content) != "" {
			return strings.TrimSpace(message.Content)
		}
	}
	return ""
}

func dockAssistantActivityCount(messages []model.AgentRunMessage) int {
	count := 0
	for _, message := range messages {
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
