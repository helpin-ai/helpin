package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSenderVerificationUsesConfiguredReplyDomain(t *testing.T) {
	for _, tc := range []struct{ name, replyDomain, wantDomain string }{
		{name: "configured reply domain", replyDomain: " replies.example.test ", wantDomain: "replies.example.test"},
		{name: "route-only installation", wantDomain: "inboxes.example.test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &SupportInboxService{routeDomain: "inboxes.example.test"}
			if tc.replyDomain != "" {
				svc.SetEmailFallbackService(&EmailFallbackService{replyDomain: tc.replyDomain})
			}
			token, address, err := svc.generateEmailSenderForwardingVerification(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if token == "" || address != "verify-"+token+"@"+tc.wantDomain {
				t.Fatalf("unexpected forwarding verification address: %q", address)
			}
		})
	}
}

func TestSenderVerificationRepairsPendingAliasesAndPreservesWorkingSetups(t *testing.T) {
	for _, tc := range []struct {
		name, status, address, wantAddress string
		verifiedAt                         bool
	}{
		{name: "pending route alias", status: "pending", address: "verify-existing@inboxes.example.test", wantAddress: "verify-existing@replies.example.test"},
		{name: "failed route alias", status: "failed", address: "verify-existing@inboxes.example.test", wantAddress: "verify-existing@replies.example.test"},
		{name: "verified route alias", status: "verified", address: "verify-existing@inboxes.example.test", wantAddress: "verify-existing@inboxes.example.test"},
		{name: "previously verified alias", status: "pending", address: "verify-existing@inboxes.example.test", wantAddress: "verify-existing@inboxes.example.test", verifiedAt: true},
		{name: "custom existing address", status: "pending", address: "verify-existing@legacy.example.test", wantAddress: "verify-existing@legacy.example.test"},
		{name: "already uses reply domain", status: "pending", address: "verify-existing@replies.example.test", wantAddress: "verify-existing@replies.example.test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
			svc := env.service.supportInboxService
			svc.SetRouteDomain("inboxes.example.test").SetEmailFallbackService(env.service)
			env.service.replyDomain = "replies.example.test"
			sender := verifiedSender("11111111-1111-1111-1111-111111111111", nil, "support@example.com", "none", "22222222-2222-2222-2222-222222222222")
			sender.ForwardingVerificationToken = "existing"
			sender.ForwardingAddress = tc.address
			sender.ForwardingStatus = tc.status
			if tc.verifiedAt {
				now := time.Now()
				sender.ForwardingVerifiedAt = &now
			}
			if err := env.senderRepo.Create(ctx, sender); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				senders, err := svc.ListEmailSenders(ctx, sender.WorkspaceID)
				if err != nil {
					t.Fatal(err)
				}
				if len(senders) != 1 || senders[0].ForwardingAddress != tc.wantAddress || senders[0].ForwardingVerificationToken != "existing" {
					t.Fatalf("incorrect or unstable verification alias: %+v", senders)
				}
			}
			stored, err := env.senderRepo.GetByID(ctx, sender.WorkspaceID, sender.ID)
			if err != nil || stored == nil || stored.ForwardingAddress != tc.wantAddress {
				t.Fatalf("verification address was not persisted: %+v, %v", stored, err)
			}
			// Verification must accept both old and new recipients with the same token.
			for _, address := range []string{tc.address, tc.wantAddress} {
				payload := model.PostmarkInboundPayload{
					MessageID: "verification-" + strings.ReplaceAll(address, "@", "-"), OriginalRecipient: address,
					To: sender.Email, FromFull: model.PostmarkAddress{Email: "customer@example.com"},
				}
				if err := env.service.ProcessInboundEmail(ctx, payload, "{}"); err != nil {
					t.Fatal(err)
				}
			}
			stored, err = env.senderRepo.GetByID(ctx, sender.WorkspaceID, sender.ID)
			if err != nil || stored == nil || stored.ForwardingStatus != supportEmailSenderForwardingVerified {
				t.Fatalf("verification no longer completes: %+v, %v", stored, err)
			}
		})
	}
}

func TestSenderVerificationRepairPreservesConcurrentVerification(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	svc := env.service.supportInboxService
	svc.SetRouteDomain("inboxes.example.test").SetEmailFallbackService(env.service)
	env.service.replyDomain = "replies.example.test"
	sender := verifiedSender("11111111-1111-1111-1111-111111111111", nil, "support@example.com", "none", "22222222-2222-2222-2222-222222222222")
	sender.ForwardingStatus = supportEmailSenderForwardingPending
	sender.ForwardingVerificationToken = "existing"
	sender.ForwardingAddress = "verify-existing@inboxes.example.test"
	if err := env.senderRepo.Create(ctx, sender); err != nil {
		t.Fatal(err)
	}
	// A webhook verifies the sender after the settings request loaded it.
	_, verified, err := svc.CompleteEmailSenderForwardingVerification(ctx, "existing", model.PostmarkInboundPayload{To: sender.Email})
	if err != nil || !verified {
		t.Fatalf("complete verification: %v, %v", verified, err)
	}
	if err := svc.ensureEmailSenderForwardingVerification(ctx, sender); err != nil {
		t.Fatal(err)
	}
	stored, err := env.senderRepo.GetByID(ctx, sender.WorkspaceID, sender.ID)
	if err != nil || stored == nil || stored.ForwardingStatus != supportEmailSenderForwardingVerified || stored.ForwardingVerifiedAt == nil {
		t.Fatalf("repair overwrote successful verification: %+v, %v", stored, err)
	}
	if sender.ForwardingStatus != supportEmailSenderForwardingVerified || sender.ForwardingAddress != stored.ForwardingAddress {
		t.Fatalf("settings returned stale verification state: %+v", sender)
	}
}
