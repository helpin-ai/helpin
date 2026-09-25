package service

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"testing"
	"time"
)

func TestReportOnlyIdentityCannotClaimAnotherVisitorConversation(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	seedWorkspace(t, db, "ws-identity", "Identity", "identity", "user-123")
	installs := repository.NewSupportInboxInstallationRepository(db)
	sessions := repository.NewSupportInboxSessionRepository(db)
	conversations := repository.NewSupportConversationRepository(db)
	contacts := repository.NewCRMContactRepository(db)
	inst := &model.SupportWidgetInstallation{WorkspaceID: "ws-identity", WidgetKey: "public-key", SecretKey: "signing-secret", Settings: "{}", Active: true, IdentityVerificationMode: model.IdentityVerificationModeReportOnly}
	if err := installs.Create(ctx, inst); err != nil {
		t.Fatal(err)
	}
	svc := svcWithWidgetRepos(installs, conversations, sessions, contacts)
	email := "victim@example.test"
	victim := &model.SupportConversation{WorkspaceID: inst.WorkspaceID, Subject: "private", Status: "open", AnonymousID: strPtr("victim-browser"), CustomerEmail: &email}
	if err := conversations.Create(ctx, victim); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"session", "headless"} {
		t.Run(path, func(t *testing.T) {
			session, err := svc.CreateWidgetSession(ctx, inst.WidgetKey, "attacker-"+path, nil, nil, nil, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			identity := model.WidgetIdentityPayload{Email: email, ExternalUserID: "victim-external", Source: "identify"}
			if path == "session" {
				err = svc.UpgradeWidgetSession(ctx, session.SessionToken, identity)
			} else {
				err = svc.IdentifyByAnonymousID(ctx, inst.WidgetKey, session.AnonymousID, identity)
			}
			if err != nil {
				t.Fatal(err)
			}
			updated, err := sessions.GetByToken(ctx, session.SessionToken)
			if err != nil {
				t.Fatal(err)
			}
			if updated.IdentityTrust != model.IdentityTrustUntrusted || updated.IdentityVerifiedAt != nil {
				t.Fatal("unsigned identity treated as verified")
			}
			if err := svc.SetSessionConversation(ctx, session.SessionToken, victim.ID); err == nil {
				t.Fatal("unsigned claim gained another visitor's history")
			}
		})
	}
	session, err := svc.CreateWidgetSession(ctx, inst.WidgetKey, "signed-browser", nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	identity := model.WidgetIdentityPayload{Email: "signed@example.test", Source: "identify"}
	now := time.Now()
	identity.IdentityVerification = signWidgetIdentityForTest(t, inst, identity, now, now.Add(10*time.Minute))
	if err := svc.UpgradeWidgetSession(ctx, session.SessionToken, identity); err != nil {
		t.Fatal(err)
	}
	signed, err := sessions.GetByToken(ctx, session.SessionToken)
	if err != nil {
		t.Fatal(err)
	}
	if signed.IdentityTrust != model.IdentityTrustVerified || signed.IdentityVerifiedAt == nil {
		t.Fatal("valid proof lost verification")
	}
	identity.Email = "unsigned@example.test"
	identity.IdentityVerification = nil
	if err := svc.IdentifyByAnonymousID(ctx, inst.WidgetKey, session.AnonymousID, identity); err != nil {
		t.Fatal(err)
	}
	unsigned, err := sessions.GetByToken(ctx, session.SessionToken)
	if err != nil {
		t.Fatal(err)
	}
	if unsigned.IdentityTrust != model.IdentityTrustUntrusted || unsigned.IdentityVerifiedAt != nil || unsigned.IdentityVerifierVersion != nil {
		t.Fatal("replacement claim inherited old verification")
	}
}
