package service

import (
	"context"
	"encoding/json"
	"github.com/helpin-ai/helpin/server/internal/sync"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMEmailSignatureOwnership(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	repo := repository.NewCRMEmailRepository(db)
	account := &model.CRMEmailAccount{ID: "account", WorkspaceID: "workspace", MemberID: "owner", EmailAddress: "owner@example.com", Provider: "gmail", Status: "connected", IsActive: true, SyncState: model.JSONB{"phase": "idle"}}
	if err := repo.CreateAccount(context.Background(), account); err != nil {
		t.Fatal(err)
	}
	svc := &CRMEmailService{emailRepo: repo}
	for _, scope := range [][2]string{{"workspace", "other"}, {"other", "owner"}} {
		if err := svc.UpdateSignature(context.Background(), scope[0], "account", scope[1], "Forbidden"); err == nil {
			t.Fatal("unauthorized signature update succeeded")
		}
	}
	if err := svc.UpdateSignature(context.Background(), "workspace", "account", "owner", "Waqar\nContentStudio"); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.GetAccountByID(context.Background(), "account")
	if err != nil || stored.Signature != "Waqar\nContentStudio" || stored.SyncState["phase"] != "idle" {
		t.Fatalf("signature or sync state changed unexpectedly: %#v, %v", stored, err)
	}
	if err := svc.UpdateSignature(context.Background(), "workspace", "account", "owner", ""); err != nil {
		t.Fatal(err)
	}
	stored, _ = repo.GetAccountByID(context.Background(), "account")
	if stored.Signature != "" {
		t.Fatal("signature could not be removed")
	}
}

func TestCRMEmailComposeDealAssociation(t *testing.T) {
	for _, test := range []struct {
		name, dealWorkspace string
		failLink            bool
		wantCalls           int
	}{
		{"linked", "ws-1", false, 1}, {"foreign deal", "other", false, 0}, {"link failure after delivery", "ws-1", true, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := setupCRMEmailLifecycleTestDB(t)
			mustExecCRMEmailLifecycle(t, db, `CREATE TABLE crm_deals (id TEXT PRIMARY KEY, workspace_id TEXT)`)
			mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_deals VALUES ('deal-1', ?)`, test.dealWorkspace)
			mustExecCRMEmailLifecycle(t, db, `INSERT INTO workspaces (id,name,slug,owner_id) VALUES ('ws-1','Workspace','workspace','owner')`)
			if test.failLink {
				mustExecCRMEmailLifecycle(t, db, `CREATE TRIGGER fail_deal_link BEFORE UPDATE OF deal_id ON crm_email_threads BEGIN SELECT RAISE(FAIL, 'link failed'); END`)
			}
			repo := repository.NewCRMEmailRepository(db)
			contacts := repository.NewCRMContactRepository(db)
			account := &model.CRMEmailAccount{ID: "account", WorkspaceID: "ws-1", MemberID: "owner", EmailAddress: "owner@example.com", Provider: "gmail", Status: "connected", IsActive: true}
			if err := repo.CreateAccount(context.Background(), account); err != nil {
				t.Fatal(err)
			}
			gmail := &fakeMailboxClient{}
			svc := &CRMEmailService{emailRepo: repo, contactRepo: contacts, workspaceRepo: repository.NewWorkspaceRepository(db), gmailSync: gmail, resolver: crmemail.NewResolver(contacts)}
			message, err := svc.SendEmailWithAttachments(context.Background(), "ws-1", "account", "owner", []string{"buyer@example.com"}, nil, "Proposal", "<p>Hello</p>", "", nil, "deal-1")
			if gmail.sendCalls != test.wantCalls {
				t.Fatalf("sent %d emails, want %d", gmail.sendCalls, test.wantCalls)
			}
			if test.wantCalls == 0 {
				if err == nil {
					t.Fatal("foreign deal accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.failLink {
				if message.AssociationWarning == "" {
					t.Fatal("missing delivery warning")
				}
				return
			}
			thread, err := repo.GetThreadByID(context.Background(), *message.ThreadID)
			if err != nil || thread.DealID == nil || *thread.DealID != "deal-1" || message.DealID == nil || *message.DealID != "deal-1" {
				t.Fatalf("missing deal association: %#v %#v %v", thread, message, err)
			}
		})
	}
}

type compositionThreadClient struct {
	fakeMailboxClient
	to, cc                          []string
	threadID, inReplyTo, references string
}

func (f *compositionThreadClient) GetMessageDetail(context.Context, string, string) (*sync.GmailMessage, error) {
	return &sync.GmailMessage{RFCMessageID: "<original@example.com>", ReferencesHeader: "<first@example.com>"}, nil
}
func (f *compositionThreadClient) SendThreadMessage(_ context.Context, _, _ string, to, cc []string, _, _, threadID, inReplyTo, references string) (*sync.GmailSendResult, error) {
	f.sendCalls++
	f.to = to
	f.cc = cc
	f.threadID = threadID
	f.inReplyTo = inReplyTo
	f.references = references
	return &sync.GmailSendResult{ID: "reply-1", ThreadID: threadID}, nil
}
func TestCRMEmailReplyKeepsRecipientsHeadersAndDeal(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	repo := repository.NewCRMEmailRepository(db)
	contacts := repository.NewCRMContactRepository(db)
	ctx := context.Background()
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO workspaces (id,name,slug,owner_id) VALUES ('ws-1','Workspace','workspace','owner')`)
	account := &model.CRMEmailAccount{ID: "account", WorkspaceID: "ws-1", MemberID: "owner", EmailAddress: "owner@example.com", Provider: "gmail", Status: "connected", IsActive: true}
	if err := repo.CreateAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	thread := &model.CRMEmailThread{ID: "thread", WorkspaceID: "ws-1", EmailAccountID: "account", ThreadExternalID: "gmail-thread", Subject: "Proposal", LastMessageAt: time.Now(), DealID: testStringPtr("deal-1")}
	if err := repo.CreateThread(ctx, thread); err != nil {
		t.Fatal(err)
	}
	message := &model.CRMEmailMessage{ID: "message", WorkspaceID: "ws-1", EmailAccountID: "account", ThreadID: &thread.ID, MessageExternalID: "gmail-message", FromAddress: "buyer@example.com", ToAddresses: json.RawMessage(`["owner@example.com"]`), CCAddresses: json.RawMessage(`["finance@example.com","owner@example.com"]`), Direction: "inbound", SentAt: time.Now()}
	if err := repo.CreateMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	gmail := &compositionThreadClient{}
	svc := &CRMEmailService{emailRepo: repo, contactRepo: contacts, workspaceRepo: repository.NewWorkspaceRepository(db), gmailSync: gmail, resolver: crmemail.NewResolver(contacts)}
	sent, err := svc.ReplyToThread(ctx, "ws-1", "thread", "owner", "reply_all", "<p>Thank you</p>")
	if err != nil {
		t.Fatal(err)
	}
	if gmail.sendCalls != 1 || strings.Join(gmail.to, ",") != "buyer@example.com" || strings.Join(gmail.cc, ",") != "finance@example.com" {
		t.Fatalf("incorrect recipients: %#v", gmail)
	}
	if gmail.threadID != "gmail-thread" || gmail.inReplyTo != "<original@example.com>" || gmail.references != "<first@example.com> <original@example.com>" {
		t.Fatalf("lost reply headers: %#v", gmail)
	}
	if sent.DealID == nil || *sent.DealID != "deal-1" {
		t.Fatal("reply lost deal context")
	}
}
