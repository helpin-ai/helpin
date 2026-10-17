//go:build integration

package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/dbmigrate"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAdminInboundRecoveryPostgres(t *testing.T) {
	db := privacyServiceDB(t)
	ctx := context.Background()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	issues, err := dbmigrate.Validate(ctx, sqlDB)
	if err != nil || len(issues) != 0 {
		t.Fatalf("migration ledger validation: %+v %v", issues, err)
	}
	ws, otherWS, conv := uuid.NewString(), uuid.NewString(), uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	seedContactPrivacyWorkspace(t, db, otherWS)
	privacyExec(t, db, `INSERT INTO support_conversations(id,workspace_id,display_id,subject,customer_email,status,channel) VALUES (?,?,1,'Help with _exports','customer@example.com','resolved','email'), (?,?,1,'Another workspace','customer@example.com','open','email')`, conv, ws, uuid.NewString(), otherWS)
	svc := privacyEmail(t, db, ws)
	payload := model.PostmarkInboundPayload{MessageID: uuid.NewString(), FromFull: model.PostmarkAddress{Email: "customer@example.com", Name: "Customer"}, To: "support@example.com", Subject: "Re: _exports", TextBody: "Please help", Attachments: []model.PostmarkInboundAttachment{{Name: "sample.txt", ContentType: "text/plain", Content: "YWJj", ContentLength: 3}}}
	if err := svc.AcceptInboundEmail(ctx, payload, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	id := inboundStableID("email:" + payload.MessageID)
	list, err := svc.ListAdminInboundEmails(ctx, "attention", "_exports", 1)
	if err != nil || list.Total != 1 || list.AttentionCount != 1 {
		t.Fatalf("JSON queue query: %+v %v", list, err)
	}
	choices, err := svc.FindAdminInboundConversations(ctx, id, "")
	if err != nil || len(choices) != 2 {
		t.Fatalf("cross-workspace suggestions: %d %v", len(choices), err)
	}
	var actor string
	if err := db.Raw("SELECT owner_id FROM workspaces WHERE id = ?", ws).Scan(&actor).Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- svc.RecoverAdminInboundEmail(ctx, id, actor, conv) }()
	}
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrInboundRecoveryConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent recovery: success=%d conflict=%d", successes, conflicts)
	}
	if err := svc.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	privacyCheck(t, db, `SELECT count(*)=1 FROM support_messages WHERE conversation_id=? AND sender_type='customer'`, conv)
	privacyCheck(t, db, `SELECT count(*)=1 FROM support_attachments WHERE conversation_id=?`, conv)
	privacyCheck(t, db, `SELECT m.created_at=j.created_at FROM support_messages m JOIN support_inbound_jobs j ON j.message_id=m.id WHERE j.id=?`, id)
	privacyCheck(t, db, `SELECT status='open' FROM support_conversations WHERE id=?`, conv)
	list, err = svc.ListAdminInboundEmails(ctx, "recovered", "_exports", 1)
	if err != nil || list.Total != 1 || list.Entries[0].RecoveryActorName != "Test owner" {
		t.Fatalf("recovered receipt query: %+v %v", list, err)
	}
	detail, err := svc.GetAdminInboundEmail(ctx, id)
	if err != nil || detail.Body != "Please help" || len(detail.Attachments) != 1 {
		t.Fatalf("completed content: %+v %v", detail, err)
	}
	// Real deletion triggers must clear pending attachment bytes and hide the completed receipt.
	privacyExec(t, db, `DELETE FROM support_conversations WHERE id=?`, conv)
	if _, err := svc.GetAdminInboundEmail(ctx, id); !errors.Is(err, ErrInboundRecoveryNotFound) {
		t.Fatalf("deleted receipt accessible: %v", err)
	}
	privacyCheck(t, db, `SELECT count(*)=0 FROM support_inbound_jobs WHERE conversation_id=? AND payload<>''`, conv)
}
