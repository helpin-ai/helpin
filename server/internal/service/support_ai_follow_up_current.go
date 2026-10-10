package service

import "github.com/helpin-ai/helpin/server/internal/model"

// currentSupportFollowUp projects only the episode that still describes the
// conversation. Persisted outcomes remain available for audit, not the thread.
func currentSupportFollowUp(conv *model.SupportConversation, episode *model.SupportAIFollowUp) *model.SupportAIFollowUp {
	if conv == nil || episode == nil || conv.AnonymizedAt != nil || supportConversationHumanOwned(conv) ||
		(conv.Status != model.SupportConversationStatusOpen && conv.Status != model.SupportConversationStatusWaitingOnCustomer) ||
		derefString(conv.FlowState) != model.SupportConversationFlowStateAIHandling || derefString(conv.AIState) != "pending" ||
		derefString(conv.LastPublicSenderType) != "ai" || conv.CustomerAwaitingResponse || conv.CustomerRequestedHumanAt != nil || conv.LinkedTaskID != nil {
		return nil
	}
	if conv.AIResumedAt != nil && !episode.CreatedAt.After(*conv.AIResumedAt) {
		return nil
	}
	source := episode.SourceMessageID
	if episode.SentMessageID != nil {
		source = *episode.SentMessageID
	}
	if episode.SecondMessageID != nil {
		source = *episode.SecondMessageID
	}
	if source == "" || derefString(conv.LastPublicMessageID) != source {
		return nil
	}
	switch episode.Status {
	case "scheduled", "assessing", "waiting", "failed":
		return episode
	case "cancelled":
		if episode.Reason == "cancelled_by_teammate" {
			return episode
		}
	}
	return nil
}
