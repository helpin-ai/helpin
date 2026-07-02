package service

import (
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	supportAIResponseModeOff          = "off"
	supportAIResponseModeInternalNote = "internal_note"
	supportAIResponseModeAIFirst      = "ai_first"
)

func normalizeSupportAIResponseMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case supportAIResponseModeInternalNote:
		return supportAIResponseModeInternalNote
	case supportAIResponseModeAIFirst:
		return supportAIResponseModeAIFirst
	default:
		return supportAIResponseModeOff
	}
}

func shouldAutomaticallyProcessSupportAI(settings model.SupportInboxSettings) bool {
	if !settings.AIEnabled || settings.AIAgentID == nil || strings.TrimSpace(*settings.AIAgentID) == "" {
		return false
	}
	mode := normalizeSupportAIResponseMode(settings.AIResponseMode)
	return mode == supportAIResponseModeInternalNote || mode == supportAIResponseModeAIFirst
}

func shouldCreatePublicSupportAIReply(settings model.SupportInboxSettings) bool {
	return shouldAutomaticallyProcessSupportAI(settings) && normalizeSupportAIResponseMode(settings.AIResponseMode) == supportAIResponseModeAIFirst
}
