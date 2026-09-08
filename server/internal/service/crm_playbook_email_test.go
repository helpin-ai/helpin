package service

import (
	"context"
	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/sync"
	"testing"
)

type playbookMailbox struct {
	fakeMailboxClient
	sends    int
	identity string
	message  *sync.GmailMessage
}

func (c *playbookMailbox) SendMessageWithMessageID(_ context.Context, _, from string, to, cc []string, subject, html, identity string) (*sync.GmailSendResult, error) {
	c.sends++
	c.identity = identity
	c.message = &sync.GmailMessage{ID: "confirmed-send", From: from, To: to, CC: cc, Subject: subject, BodyHTML: html}
	return &sync.GmailSendResult{ID: c.message.ID}, nil
}
func (c *playbookMailbox) FindSentMessageByMessageID(_ context.Context, _, identity string) (*sync.GmailMessage, error) {
	if identity != c.identity {
		return nil, nil
	}
	return c.message, nil
}

func TestCRMPlaybookEmailUsesExistingServiceWithInspectableExactSendIdentity(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	ctx := context.Background()
	emails := repository.NewCRMEmailRepository(db)
	contacts := repository.NewCRMContactRepository(db)
	ws, user, accountID, contactID, intentID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	account := &model.CRMEmailAccount{ID: accountID, WorkspaceID: ws, MemberID: user, Provider: "gmail", EmailAddress: "sales@example.test", IsActive: true, SyncState: model.JSONB{"status": model.CRMEmailAccountStatusConnected}}
	if err := emails.CreateAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO crm_contacts (id,workspace_id,display_id,first_name,email) VALUES (?,?,?,?,?)", contactID, ws, "C-1", "Customer", "customer@example.test").Error; err != nil {
		t.Fatal(err)
	}
	client := &playbookMailbox{}
	svc := &CRMEmailService{emailRepo: emails, contactRepo: contacts, gmailSync: client, resolver: crmemail.NewResolver(contacts)}
	action := model.CRMPlaybookEmailAction{AccountID: accountID, ContactID: contactID, To: "customer@example.test", Subject: "Your agreed next step", BodyHTML: "<p>Here is the agreed evaluation plan.</p>"}
	if _, err := svc.SendActionEmail(ctx, ws, uuid.NewString(), intentID, action); err == nil || client.sends != 0 {
		t.Fatal("another member's mailbox was used")
	}
	message, err := svc.SendActionEmail(ctx, ws, user, intentID, action)
	if err != nil || message.MessageExternalID != "confirmed-send" || client.sends != 1 {
		t.Fatalf("send failed: %#v %v", message, err)
	}
	if message.RFCMessageID == nil || *message.RFCMessageID != crmActionMessageID(intentID) || client.identity != *message.RFCMessageID {
		t.Fatal("durable send identity was not preserved")
	}
	for range 2 {
		confirmed, _, err := svc.ReconcileActionEmail(ctx, ws, user, intentID, action)
		if err != nil || !confirmed || client.sends != 1 {
			t.Fatalf("inspection resent or lost the confirmed result: %v", err)
		}
	}
	changed := action
	changed.Subject = "Not the approved message"
	if confirmed, _, err := svc.ReconcileActionEmail(ctx, ws, user, intentID, changed); err == nil || confirmed || client.sends != 1 {
		t.Fatal("different message was presented as success")
	}
}
