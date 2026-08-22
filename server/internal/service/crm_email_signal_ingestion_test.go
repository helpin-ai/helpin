package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/crmsignal"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/sync"
)

type fakeSignalWorkflowStarter struct {
	messageIDs []string
	payloads   [][]model.SignalSourcePayload
}

type fakeSummaryRequester struct {
	contactRefreshes []string
	dealRefreshes    []string
}

func (f *fakeSignalWorkflowStarter) StartEmailSignalDetection(ctx context.Context, messageID string, payloads []model.SignalSourcePayload) error {
	f.messageIDs = append(f.messageIDs, messageID)
	f.payloads = append(f.payloads, payloads)
	return nil
}

func (f *fakeSummaryRequester) RequestContactRefresh(ctx context.Context, workspaceID, contactID string) error {
	f.contactRefreshes = append(f.contactRefreshes, workspaceID+":"+contactID)
	return nil
}

func (f *fakeSummaryRequester) RequestDealRefresh(ctx context.Context, workspaceID, dealID string) error {
	f.dealRefreshes = append(f.dealRefreshes, workspaceID+":"+dealID)
	return nil
}

func TestCRMEmailService_CreateMessageEnqueuesBuyerSignalDetection(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	starter := &fakeSignalWorkflowStarter{}
	summary := &fakeSummaryRequester{}

	ctx := context.Background()
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO workspaces (id, name, slug, owner_id, timezone) VALUES (?, ?, ?, ?, ?)`, "ws-1", "Workspace", "workspace", "owner-1", "UTC")
	account := &model.CRMEmailAccount{
		ID:                     "acct-1",
		WorkspaceID:            "ws-1",
		MemberID:               "member-1",
		Provider:               model.CRMEmailProviderGmail,
		EmailAddress:           "owner@example.com",
		NormalizedEmailAddress: testStringPtr("owner@example.com"),
		IsActive:               true,
		Status:                 model.CRMEmailAccountStatusConnected,
	}
	if err := emailRepo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, email, custom_properties) VALUES (?, ?, ?, ?, ?, CAST(? AS BLOB))`,
		"contact-1", "ws-1", "C-1", "Buyer", "buyer@example.com", `{}`)

	svc := &CRMEmailService{
		emailRepo:       emailRepo,
		contactRepo:     contactRepo,
		workspaceRepo:   workspaceRepo,
		resolver:        crmemail.NewResolver(contactRepo),
		signalIngestion: crmsignal.NewIngestionService(emailRepo, starter),
		summaryRefresh:  summary,
	}

	message, err := svc.CreateMessage(ctx, model.CreateCRMEmailMessageRequest{
		WorkspaceID:    "ws-1",
		EmailAccountID: "acct-1",
		FromAddress:    "buyer@example.com",
		ToAddresses:    []byte(`["owner@example.com"]`),
		Subject:        "Need pricing",
		BodyText:       testStringPtr("Can you send pricing for the enterprise plan?"),
		Direction:      model.CRMEmailDirectionInbound,
		ContactID:      testStringPtr("contact-1"),
		SentAt:         testTimePtr(time.Now()),
	})
	if err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if message == nil {
		t.Fatal("expected message to be returned")
	}
	if len(starter.messageIDs) != 1 {
		t.Fatalf("started workflows = %v, want 1", starter.messageIDs)
	}
	if len(summary.contactRefreshes) != 1 || summary.contactRefreshes[0] != "ws-1:contact-1" {
		t.Fatalf("contact summary refreshes = %v, want ws-1:contact-1", summary.contactRefreshes)
	}
}

func TestCRMEmailService_SendEmailEnqueuesBuyerSignalDetection(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	starter := &fakeSignalWorkflowStarter{}
	summary := &fakeSummaryRequester{}

	ctx := context.Background()
	mustExecCRMEmailLifecycle(t, db, `INSERT INTO workspaces (id, name, slug, owner_id, timezone) VALUES (?, ?, ?, ?, ?)`, "ws-1", "Workspace", "workspace", "owner-1", "UTC")
	account := &model.CRMEmailAccount{
		ID:                     "acct-1",
		WorkspaceID:            "ws-1",
		MemberID:               "member-1",
		Provider:               model.CRMEmailProviderGmail,
		EmailAddress:           "owner@example.com",
		NormalizedEmailAddress: testStringPtr("owner@example.com"),
		IsActive:               true,
		Status:                 model.CRMEmailAccountStatusConnected,
	}
	if err := emailRepo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := &CRMEmailService{
		emailRepo:       emailRepo,
		contactRepo:     contactRepo,
		workspaceRepo:   workspaceRepo,
		gmailSync:       &fakeMailboxClient{profile: &sync.GmailProfile{EmailAddress: "owner@example.com", HistoryID: "hist-1"}},
		resolver:        crmemail.NewResolver(contactRepo),
		signalIngestion: crmsignal.NewIngestionService(emailRepo, starter),
		summaryRefresh:  summary,
	}
	if _, err := svc.SendEmail(ctx, "ws-1", "acct-1", "member-other", false, []string{"buyer@example.com"}, nil, "Follow-up", "<p>Checking in.</p>"); err == nil {
		t.Fatal("expected non-owner send to be rejected")
	}
	if _, err := svc.SendEmail(ctx, "ws-1", "acct-1", "member-other", true, []string{"buyer@example.com"}, nil, "Follow-up", "<p>Checking in.</p>"); err == nil {
		t.Fatal("expected non-owner send to be rejected even when the caller is an administrator")
	}

	message, err := svc.SendEmail(ctx, "ws-1", "acct-1", "member-1", false, []string{"buyer@example.com"}, nil, "Follow-up", "<p>Checking in about pricing.</p>")
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	if message == nil {
		t.Fatal("expected sent message")
	}
	if len(starter.messageIDs) != 1 {
		t.Fatalf("started workflows = %v, want 1", starter.messageIDs)
	}
	if len(starter.payloads) != 1 || len(starter.payloads[0]) != 1 {
		t.Fatalf("payloads = %#v, want one payload", starter.payloads)
	}
	if len(summary.contactRefreshes) != 1 {
		t.Fatalf("contact summary refreshes = %v, want 1", summary.contactRefreshes)
	}
}

func TestCRMEmailService_ReplyToThreadRejectsWorkspaceMemberWhoDoesNotOwnMailbox(t *testing.T) {
	db := setupCRMEmailLifecycleTestDB(t)
	emailRepo := repository.NewCRMEmailRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	account := &model.CRMEmailAccount{
		ID: "acct-owner", WorkspaceID: "ws-1", MemberID: "member-owner",
		Provider: model.CRMEmailProviderGmail, EmailAddress: "owner@example.com",
		IsActive: true, Status: model.CRMEmailAccountStatusConnected,
	}
	if err := emailRepo.CreateAccount(ctx, account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	thread := &model.CRMEmailThread{
		ID: "thread-1", WorkspaceID: "ws-1", EmailAccountID: account.ID,
		ThreadExternalID: "gmail-thread-1", Subject: "Renewal", LastMessageAt: now,
	}
	if err := emailRepo.CreateThread(ctx, thread); err != nil {
		t.Fatalf("create thread: %v", err)
	}

	svc := &CRMEmailService{emailRepo: emailRepo}
	if _, err := svc.ReplyToThread(ctx, "ws-1", thread.ID, "member-colleague", "reply", "<p>Reply</p>"); err == nil || err.Error() != "only the connected mailbox owner can reply to this thread" {
		t.Fatalf("ReplyToThread error = %v, want mailbox-owner authorization error", err)
	}
}
