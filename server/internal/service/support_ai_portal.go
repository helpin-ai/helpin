package service

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// effectiveSupportAISettings keeps portal AI separate from the existing chat
// and email policy. Callers use the returned copy only for a portal turn.
func effectiveSupportAISettings(settings model.SupportInboxSettings, conv *model.SupportConversation) model.SupportInboxSettings {
	if conv == nil || !strings.EqualFold(conv.Channel, "portal") {
		return settings
	}
	settings.AIEnabled = settings.PortalEnabled && (settings.PortalAIMode == "ai_first" || settings.PortalAIMode == "internal_note")
	settings.AIResponseMode = settings.PortalAIMode
	if settings.PortalAIAgentID != nil && strings.TrimSpace(*settings.PortalAIAgentID) != "" {
		settings.AIAgentID = settings.PortalAIAgentID
	}
	if conv.AssignedAgentID != nil && strings.TrimSpace(*conv.AssignedAgentID) != "" &&
		((conv.AIState != nil && *conv.AIState == "pending") || conv.AIActiveRunID != nil) {
		settings.AIAgentID = conv.AssignedAgentID
	}
	return settings
}
