package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPortalAIPolicyDoesNotChangeChatSettings(t *testing.T) {
	shared := "chat-agent"
	portal := "portal-agent"
	settings := model.DefaultSupportInboxSettings()
	settings.AIEnabled = true
	settings.AIAgentID = &shared
	settings.AIResponseMode = "ai_first"
	settings.AIReplyChannels = "chat"
	settings.PortalEnabled = true
	settings.PortalAIMode = "internal_note"
	settings.PortalAIAgentID = &portal

	chat := effectiveSupportAISettings(settings, &model.SupportConversation{Channel: "widget"})
	if !chat.AIEnabled || chat.AIResponseMode != "ai_first" || *chat.AIAgentID != shared || !model.SupportAIChannelEnabled(chat, "chat") {
		t.Fatalf("chat policy changed: %+v", chat)
	}
	portalSettings := effectiveSupportAISettings(settings, &model.SupportConversation{Channel: "portal", PortalVisible: true})
	if !portalSettings.AIEnabled || portalSettings.AIResponseMode != "internal_note" || *portalSettings.AIAgentID != portal {
		t.Fatalf("portal policy not applied: %+v", portalSettings)
	}
	pinned := effectiveSupportAISettings(settings, &model.SupportConversation{Channel: "portal", AIActiveRunID: strPtr("run"), AssignedAgentID: strPtr("original-agent")})
	if pinned.AIAgentID == nil || *pinned.AIAgentID != "original-agent" {
		t.Fatalf("active portal agent was replaced: %+v", pinned)
	}
	if model.SupportAIChannelEnabled(settings, "email") || !model.SupportAIChannelEnabled(settings, "chat") {
		t.Fatal("portal setting changed the shared reply-channel selection")
	}
	settings.PortalAIMode = "off"
	if effectiveSupportAISettings(settings, &model.SupportConversation{Channel: "portal"}).AIEnabled {
		t.Fatal("portal AI must default to off independently of chat")
	}
}

func TestStoppingPortalAITurnDoesNotTouchChat(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := repository.NewSupportConversationRepository(db)
	for _, channel := range []string{"portal", "widget"} {
		conv := &model.SupportConversation{ID: channel, WorkspaceID: "ws", Subject: "Question", Channel: channel, Source: channel, Status: "open", AIState: strPtr("pending"), AssignedAgentID: strPtr("agent")}
		if err := repo.Create(ctx, conv); err != nil {
			t.Fatal(err)
		}
	}
	ai := &SupportAIService{conversationRepo: repo}
	ai.stopPortalAITurn(ctx, "ws", "portal")
	for _, tc := range []struct {
		id      string
		blocked bool
	}{{"portal", true}, {"widget", false}} {
		conv, err := repo.GetByID(ctx, "ws", tc.id, "", model.RoleOwner)
		if err != nil {
			t.Fatal(err)
		}
		if got := model.SupportAIConversationBlocked(conv); got != tc.blocked {
			t.Fatalf("%s blocked=%v", tc.id, got)
		}
	}
}

func TestPortalAIQueueKeepsAssignedTeammateOwnership(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	settings.PortalEnabled = true
	settings.PortalAIMode = "internal_note"
	if portalAIQueueAllowed(settings, strPtr("teammate")) {
		t.Fatal("assigned teammate request must not be moved into an AI run")
	}
	if !portalAIQueueAllowed(settings, nil) {
		t.Fatal("unassigned request should receive a private suggestion")
	}
}
