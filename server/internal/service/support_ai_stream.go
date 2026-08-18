package service

import (
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

const (
	supportAIProgressLooking    = "looking"
	supportAIProgressChecking   = "checking"
	supportAIProgressComposing  = "composing"
	supportAIProgressFinalizing = "finalizing"
)

var supportAIProgressLabels = map[string]string{
	supportAIProgressLooking:    "Looking into this…",
	supportAIProgressChecking:   "Checking the details…",
	supportAIProgressComposing:  "Preparing your answer…",
	supportAIProgressFinalizing: "Finishing your answer…",
}

func (s *SupportAIService) publishProgress(workspaceID, conversationID, stage string) {
	if s == nil || s.wsPublisher == nil || workspaceID == "" || conversationID == "" {
		return
	}
	label, ok := supportAIProgressLabels[stage]
	if !ok {
		return
	}
	s.wsPublisher.Publish(websocket.SupportAIProgressEvent(
		workspaceID,
		conversationID,
		"ai",
		stage,
		label,
	))
}

func publishSupportAIMessageStream(
	publisher websocket.EventPublisher,
	workspaceID string,
	message *model.SupportMessage,
	actorID string,
) {
	if publisher == nil || message == nil {
		return
	}
	for _, event := range websocket.SupportAIResponseStartEvents(workspaceID, message, actorID) {
		publisher.Publish(event)
	}
	publisher.Publish(websocket.SupportMessageEvent(workspaceID, message, actorID))
	complete := websocket.SupportAIResponseCompleteEvent(workspaceID, message, actorID)
	if complete.Entity != "" {
		publisher.Publish(complete)
	}
}
