package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportAIResponseModeAutomation(t *testing.T) {
	agentID := "agent-1"

	tests := []struct {
		name              string
		mode              string
		wantAutoProcess   bool
		wantPublicMessage bool
	}{
		{name: "off disables automatic processing", mode: "off", wantAutoProcess: false, wantPublicMessage: false},
		{name: "internal note runs privately", mode: "internal_note", wantAutoProcess: true, wantPublicMessage: false},
		{name: "ai first runs publicly", mode: "ai_first", wantAutoProcess: true, wantPublicMessage: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			settings := model.DefaultSupportInboxSettings()
			settings.AIEnabled = true
			settings.AIAgentID = &agentID
			settings.AIResponseMode = tc.mode

			if got := shouldAutomaticallyProcessSupportAI(settings); got != tc.wantAutoProcess {
				t.Fatalf("shouldAutomaticallyProcessSupportAI(%q) = %v, want %v", tc.mode, got, tc.wantAutoProcess)
			}
			if got := shouldCreatePublicSupportAIReply(settings); got != tc.wantPublicMessage {
				t.Fatalf("shouldCreatePublicSupportAIReply(%q) = %v, want %v", tc.mode, got, tc.wantPublicMessage)
			}
		})
	}
}

func TestValidateSettingsAllowsInternalNoteResponseMode(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	settings.AIResponseMode = "internal_note"

	if err := (&SupportInboxService{}).validateSettings(context.Background(), "workspace-1", settings); err != nil {
		t.Fatalf("validateSettings returned error: %v", err)
	}
}
