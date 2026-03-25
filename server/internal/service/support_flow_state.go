package service

import (
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func defaultConversationFlowState(openedByUserID, assignedAgentID *string) string {
	if assignedAgentID != nil && *assignedAgentID != "" {
		return model.SupportConversationFlowStateAssignedToHuman
	}
	if openedByUserID != nil && *openedByUserID != "" {
		return model.SupportConversationFlowStateAssignedToHuman
	}
	return model.SupportConversationFlowStateWaitingForHuman
}

func escalatedConversationFlowState(settings model.SupportInboxSettings, now time.Time) string {
	if !resolveSupportAvailability(settings, now).IsWithinOfficeHours {
		return model.SupportConversationFlowStateAfterHoursQueue
	}
	return model.SupportConversationFlowStateWaitingForHuman
}
