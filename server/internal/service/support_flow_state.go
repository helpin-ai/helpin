package service

import (
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func defaultConversationFlowState(openedByUserID, assignedUserID, assignedAgentID *string) string {
	if assignedAgentID != nil && *assignedAgentID != "" {
		return model.SupportConversationFlowStateAssignedToHuman
	}
	if assignedUserID != nil && *assignedUserID != "" {
		return model.SupportConversationFlowStateAssignedToHuman
	}
	if openedByUserID != nil && *openedByUserID != "" {
		return model.SupportConversationFlowStateAssignedToHuman
	}
	return model.SupportConversationFlowStateWaitingForHuman
}

func boolPtr(value bool) *bool {
	return &value
}

func escalatedConversationFlowState(settings model.SupportInboxSettings, now time.Time) string {
	if !resolveSupportAvailability(settings, now).IsWithinOfficeHours {
		return model.SupportConversationFlowStateAfterHoursQueue
	}
	return model.SupportConversationFlowStateWaitingForHuman
}
