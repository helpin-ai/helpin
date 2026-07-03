package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// TestSendWidgetConversationTranscriptPersistsCapturedEmail verifies that a
// transcript request carrying an email address, made against a conversation
// with no customer_email on file, persists that email onto the conversation
// so later flows (e.g. reply-by-email) have something to key off.
func TestSendWidgetConversationTranscriptPersistsCapturedEmail(t *testing.T) {
	db := newTestDB(t)

	const workspaceID = "ws-widget-transcript-capture"
	seedWorkspace(t, db, workspaceID, "Widget Transcript WS", "widget-transcript-ws", "user-123")

	ctx := context.Background()
	conversationRepo := repository.NewSupportConversationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)

	conversation := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Anonymous visitor question",
		Status:      "open",
		AnonymousID: strPtr("anon-transcript-1"),
	}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	// IsAnonymous is false so that the pre-existing UpgradeWidgetSession
	// backfill path (which only fires for `session.IsAnonymous && email != ""`)
	// does NOT run — isolating this test to the new persistence added in
	// SendWidgetConversationTranscript itself, rather than incidentally
	// passing because of that unrelated backfill.
	session := &model.SupportWidgetSession{
		WorkspaceID:  workspaceID,
		SessionToken: "widget-transcript-token",
		AnonymousID:  "anon-transcript-1",
		IsAnonymous:  false,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	svc := NewSupportInboxService(
		conversationRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		nil,
		nil,
		repository.NewSupportInboxInstallationRepository(db),
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	emailClient := email.NewClient("postmark-token", "noreply@example.com")
	emailClient.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"ErrorCode": 0,
					"Message": "OK",
					"MessageID": "pm-transcript-1",
					"SubmittedAt": "2026-07-02T12:30:00Z",
					"To": "visitor@example.com"
				}`)),
			}, nil
		}),
	})
	svc.SetEmailFallbackService(NewEmailFallbackService(
		nil, nil, nil, emailClient, nil, nil, nil, nil, nil, nil, nil, "", "", "",
	))

	resp, err := svc.SendWidgetConversationTranscript(ctx, session.SessionToken, conversation.ID, "visitor@example.com")
	if err != nil {
		t.Fatalf("SendWidgetConversationTranscript: %v", err)
	}
	if !resp.Success {
		t.Fatalf("resp.Success = false, want true (message: %s)", resp.Message)
	}

	updated, err := conversationRepo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.CustomerEmail == nil || *updated.CustomerEmail != "visitor@example.com" {
		t.Fatalf("customer_email = %v, want %q", updated.CustomerEmail, "visitor@example.com")
	}
}

// TestSendWidgetConversationTranscriptDoesNotOverwriteExistingEmail verifies
// that a transcript request does not clobber a customer_email the
// conversation already has.
func TestSendWidgetConversationTranscriptDoesNotOverwriteExistingEmail(t *testing.T) {
	db := newTestDB(t)

	const workspaceID = "ws-widget-transcript-existing"
	seedWorkspace(t, db, workspaceID, "Widget Transcript Existing WS", "widget-transcript-existing-ws", "user-123")

	ctx := context.Background()
	conversationRepo := repository.NewSupportConversationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)

	conversation := &model.SupportConversation{
		WorkspaceID:   workspaceID,
		Subject:       "Known visitor question",
		Status:        "open",
		AnonymousID:   strPtr("anon-transcript-2"),
		CustomerEmail: strPtr("original@example.com"),
	}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	session := &model.SupportWidgetSession{
		WorkspaceID:  workspaceID,
		SessionToken: "widget-transcript-existing-token",
		AnonymousID:  "anon-transcript-2",
		IsAnonymous:  true,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	if err := sessionRepo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	svc := NewSupportInboxService(
		conversationRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		nil,
		nil,
		repository.NewSupportInboxInstallationRepository(db),
		sessionRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	emailClient := email.NewClient("postmark-token", "noreply@example.com")
	emailClient.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"ErrorCode": 0,
					"Message": "OK",
					"MessageID": "pm-transcript-2",
					"SubmittedAt": "2026-07-02T12:30:00Z",
					"To": "original@example.com"
				}`)),
			}, nil
		}),
	})
	svc.SetEmailFallbackService(NewEmailFallbackService(
		nil, nil, nil, emailClient, nil, nil, nil, nil, nil, nil, nil, "", "", "",
	))

	if _, err := svc.SendWidgetConversationTranscript(ctx, session.SessionToken, conversation.ID, ""); err != nil {
		t.Fatalf("SendWidgetConversationTranscript: %v", err)
	}

	updated, err := conversationRepo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.CustomerEmail == nil || *updated.CustomerEmail != "original@example.com" {
		t.Fatalf("customer_email = %v, want unchanged %q", updated.CustomerEmail, "original@example.com")
	}
}
