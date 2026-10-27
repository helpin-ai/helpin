package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPortalSettingsRejectCustomerFacingAI(t *testing.T) {
	svc := &SupportInboxService{}
	settings := model.DefaultSupportInboxSettings()
	settings.PortalEnabled = true
	for _, mode := range []string{"ai_first", "auto", ""} {
		settings.PortalAIMode = mode
		if err := svc.validatePortalSettings(context.Background(), "ws", settings, model.UpdateInstallationSettingsRequest{}); err == nil {
			t.Fatalf("portal_ai_mode %q accepted", mode)
		}
	}
	settings.PortalAIMode = model.SupportPortalAIModeOff
	if err := svc.validatePortalSettings(context.Background(), "ws", settings, model.UpdateInstallationSettingsRequest{}); err != nil {
		t.Fatalf("off rejected: %v", err)
	}
}

// A workspace still carrying the retired mode before its migration runs gets
// no customer-facing AI and no queued portal turns.
func TestPortalLegacyCustomerReplyModeIsInert(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	settings.PortalEnabled = true
	settings.PortalAIMode = "ai_first"
	settings.AIAgentID = strPtr("agent")
	portal := effectiveSupportAISettings(settings, &model.SupportConversation{Channel: "portal"})
	if portal.AIEnabled || portal.AIResponseMode != model.SupportPortalAIModeInternalNote {
		t.Fatalf("legacy portal mode still enables AI: %+v", portal)
	}
	if portalAIQueueAllowed(settings, nil) || model.SupportAIChannelEnabled(settings, "portal") {
		t.Fatal("legacy portal mode still queues AI work")
	}
	settings.PortalAIMode = model.SupportPortalAIModeInternalNote
	portal = effectiveSupportAISettings(settings, &model.SupportConversation{Channel: "portal"})
	if !portal.AIEnabled || portal.AIResponseMode != model.SupportPortalAIModeInternalNote || !portalAIQueueAllowed(settings, nil) {
		t.Fatalf("private suggestions disabled: %+v", portal)
	}
}

// Whatever was queued or configured, an AI turn on a portal request can only
// ever save a private note.
func TestPortalAIRepliesStayPrivate(t *testing.T) {
	for _, mode := range []string{"ai_first", model.SupportPortalAIModeInternalNote} {
		t.Run(mode, func(t *testing.T) {
			svc, db, _, _ := setupAIControlTest(t)
			ctx := context.Background()
			mustExec(t, db, `UPDATE support_conversations SET channel='portal', source='portal', portal_visible=true`)
			mustExec(t, db, `CREATE TABLE ai_message_processing(id TEXT PRIMARY KEY,workspace_id TEXT,conversation_id TEXT,source_message_id TEXT,reply_message_id TEXT,status TEXT,tokens_used INTEGER,created_at DATETIME,updated_at DATETIME)`)
			mustExec(t, db, `INSERT INTO support_widget_installations (id, workspace_id, widget_key, secret_key, settings, active) VALUES ('install', 'ws', 'key', 'secret', ?, true)`,
				`{"portal_enabled":true,"portal_ai_mode":"`+mode+`","ai_agent_id":"agent"}`)
			customer := &model.SupportMessage{ID: "question", WorkspaceID: "ws", ConversationID: "conv", SenderType: "customer", MessageType: "reply", Content: "Export fails"}
			if err := svc.messageRepo.Create(ctx, customer); err != nil {
				t.Fatal(err)
			}
			mustExec(t, db, `INSERT INTO ai_message_processing(id,workspace_id,conversation_id,source_message_id,status) VALUES ('turn','ws','conv','question','processing')`)
			repo := repository.NewAIMessageProcessingRepository(db)
			public := &model.SupportMessage{ID: "public", WorkspaceID: "ws", ConversationID: "conv", SenderType: "ai", MessageType: "reply", Content: "Try clearing the cache"}
			if created, err := repo.CreateReply(ctx, "turn", public, "old-run"); err != nil || created {
				t.Fatalf("customer-visible AI reply saved: %v/%v", created, err)
			}
			private := &model.SupportMessage{ID: "note", WorkspaceID: "ws", ConversationID: "conv", SenderType: "ai", MessageType: "reply", IsInternal: true, Content: "Suggested reply"}
			created, err := repo.CreateReply(ctx, "turn", private, "old-run")
			if err != nil {
				t.Fatal(err)
			}
			if wantNote := mode == model.SupportPortalAIModeInternalNote; created != wantNote {
				t.Fatalf("private suggestion saved=%v, want %v", created, wantNote)
			}
		})
	}
}

func TestPortalRequestsCannotBeHandedToAI(t *testing.T) {
	svc, db, _, _ := setupAIControlTest(t)
	mustExec(t, db, `UPDATE support_conversations SET channel='portal', source='portal', portal_visible=true, human_takeover=true`)
	ctx := context.Background()
	err := svc.ChangeConversationAIControl(ctx, "ws", "conv", "human", model.SupportAIControlRequest{Action: "return"})
	if !errors.Is(err, ErrPortalRequestsHumanOwned) {
		t.Fatalf("return to AI on a portal request = %v", err)
	}
	if err := svc.ChangeConversationAIControl(ctx, "ws", "conv", "human", model.SupportAIControlRequest{Action: "run_now"}); err == nil || err.Error() != "action must be pause or return" {
		t.Fatalf("run_now = %v", err)
	}
	if readControlConversation(t, db).AIControlVersion != 0 {
		t.Fatal("refused action changed ownership")
	}
}
