package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

func TestSupportFirstReplyJoinCommitsWithReply(t *testing.T) {
	env, conv := deliveryModeFixture(t)
	svc := env.service.supportInboxService
	db := env.messageRepo.DB()
	ctx := context.Background()
	actor := "22222222-2222-2222-2222-222222222222"
	req := model.CreateMessageRequest{Content: "Hello", ClientMessageID: "first-reply"}
	if err := db.Callback().Create().Before("gorm:create").Register("fail_public_reply", func(tx *gorm.DB) {
		if msg, ok := tx.Statement.Dest.(*model.SupportMessage); ok && msg.MessageType == "reply" {
			tx.AddError(fmt.Errorf("injected reply failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, req, "user", &actor, nil, strPtr("Teammate")); err == nil {
		t.Fatal("expected reply failure")
	}
	rows, err := env.messageRepo.ListByConversation(ctx, conv.WorkspaceID, conv.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("failed reply left public activity: %+v", rows)
	}
	if err := db.Callback().Create().Remove("fail_public_reply"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := svc.CreateConversationMessage(ctx, conv.WorkspaceID, conv.ID, req, "user", &actor, nil, strPtr("Teammate")); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = env.messageRepo.ListByConversation(ctx, conv.WorkspaceID, conv.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].SystemEventType == nil || *rows[0].SystemEventType != model.SystemEventTeammateJoined || rows[1].MessageType != "reply" {
		t.Fatalf("retry should leave one join followed by one reply: %+v", rows)
	}
}
