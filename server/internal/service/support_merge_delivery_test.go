package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestMergeDeliveryUsesReviewedAudienceAndOldEmailRoutes(t *testing.T) {
	for _, sourceKind := range []string{"email", "widget"} {
		for _, targetKind := range []string{"email", "widget", "anonymous_widget"} {
			t.Run(sourceKind+"_into_"+targetKind, func(t *testing.T) {
				ctx := context.Background()
				settings := model.DefaultSupportInboxSettings()
				settings.EmailFallbackEnabled = true
				env := setupEmailFallbackTestEnv(t, settings)
				defer env.redisServer.Close()
				db := env.convRepo.DB()
				if err := db.AutoMigrate(&model.SupportPendingSend{}, &model.SupportAIFollowUp{}); err != nil {
					t.Fatal(err)
				}
				ws := "11111111-1111-1111-1111-111111111111"
				sourceEmail, targetEmail := "source@example.test", "target@example.test"
				a := model.SupportConversation{ID: uuid.NewString(), WorkspaceID: ws, Subject: "Original A", Status: "open", Source: sourceKind, CustomerEmail: &sourceEmail}
				b := model.SupportConversation{ID: uuid.NewString(), WorkspaceID: ws, Subject: "Original B", Status: "open", Source: targetKind, CustomerEmail: &targetEmail}
				if targetKind == "anonymous_widget" {
					b.Source = "widget"
					b.CustomerEmail = nil
				}
				for _, c := range []*model.SupportConversation{&a, &b} {
					if err := env.convRepo.Create(ctx, c); err != nil {
						t.Fatal(err)
					}
				}
				repo := repository.NewSupportMergeRepository(db)
				edges, err := repo.Merge(ctx, ws, b.ID, []string{a.ID}, uuid.NewString(), uuid.NewString(), true)
				if err != nil {
					t.Fatal(err)
				}
				current, err := env.convRepo.GetByID(ctx, ws, b.ID, "", model.RoleOwner)
				if err != nil {
					t.Fatal(err)
				}
				reply := model.SupportMessage{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: b.ID, SenderType: "user", MessageType: "reply", Content: "Only the new shared reply", Metadata: `{"reply_delivery_mode":"email_only"}`, ExpectedMergeRevision: &current.MergeRevision}
				if err := env.messageRepo.Create(ctx, &reply); err != nil {
					t.Fatal(err)
				}
				var captured struct{ To, Cc, Bcc, TextBody string }
				calls := 0
				env.service.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
						return nil, err
					}
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ErrorCode":0,"MessageID":"merge-test-delivery","Message":"OK"}`))}, nil
				})})
				if err := env.service.fireEmail(ctx, b.ID, []string{reply.ID}); err != nil {
					t.Fatal(err)
				}
				wantTo, wantCC := targetEmail, sourceEmail
				if targetKind == "anonymous_widget" {
					wantTo, wantCC = sourceEmail, ""
				}
				if calls != 1 || captured.To != wantTo || captured.Cc != wantCC || captured.Bcc != "" {
					t.Fatalf("audience: calls=%d, %+v", calls, captured)
				}
				if !strings.Contains(captured.TextBody, reply.Content) {
					t.Fatal("new shared reply missing")
				}
				incoming := func(route, sender, id string) {
					t.Helper()
					payload := model.PostmarkInboundPayload{FromFull: model.PostmarkAddress{Email: sender}, To: "conv-" + route + "@replies.helpin.ai", OriginalRecipient: "conv-" + route + "@replies.helpin.ai", Subject: "Re: Original", MessageID: id, StrippedTextReply: "Shared reply " + id, TextBody: "Shared reply " + id}
					raw, _ := json.Marshal(payload)
					if err := env.service.ProcessInboundEmail(ctx, payload, string(raw)); err != nil {
						t.Fatal(err)
					}
				}
				incoming(a.ID, sourceEmail, "original-alias")
				incoming(b.ID, sourceEmail, "new-alias")
				originals, err := env.messageRepo.ListByConversation(ctx, ws, a.ID, true)
				if err != nil || len(originals) != 0 {
					t.Fatalf("source received routed writes: %d %v", len(originals), err)
				}
				messages, err := env.messageRepo.ListByConversation(ctx, ws, b.ID, true)
				if err != nil || len(messages) != 3 {
					t.Fatalf("root messages=%d %v", len(messages), err)
				}
				logs, err := env.emailLogRepo.ListByConversation(ctx, ws, a.ID)
				if err != nil || len(logs) != 1 {
					t.Fatalf("old RFC route lost: %d %v", len(logs), err)
				}
				if err := repo.Unmerge(ctx, ws, edges[0].ID, uuid.NewString(), uuid.NewString()); err != nil {
					t.Fatal(err)
				}
				incoming(a.ID, sourceEmail, "restored-alias")
				originals, err = env.messageRepo.ListByConversation(ctx, ws, a.ID, true)
				if err != nil || len(originals) != 1 {
					t.Fatalf("restored route=%d %v", len(originals), err)
				}
				incoming(b.ID, sourceEmail, "removed-participant")
				messages, err = env.messageRepo.ListByConversation(ctx, ws, b.ID, true)
				if err != nil {
					t.Fatal(err)
				}
				last := messages[len(messages)-1]
				if !last.IsInternal {
					t.Fatal("former merge participant remained in the public reply audience")
				}
			})
		}
	}
}

func TestMergeWidgetReplyKeepsOriginalSessionAndAttachment(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackTestEnv(t, model.DefaultSupportInboxSettings())
	defer env.redisServer.Close()
	db := env.convRepo.DB()
	if err := db.AutoMigrate(&model.SupportPendingSend{}, &model.SupportAIFollowUp{}); err != nil {
		t.Fatal(err)
	}
	ws := "11111111-1111-1111-1111-111111111111"
	anon := "merged-widget-visitor"
	a := model.SupportConversation{ID: uuid.NewString(), WorkspaceID: ws, Subject: "New conversation", Status: "open", Source: "widget", AnonymousID: &anon}
	b := a
	b.ID = uuid.NewString()
	b.Subject = "Primary"
	for _, c := range []*model.SupportConversation{&a, &b} {
		if err := env.convRepo.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	session := &model.SupportWidgetSession{WorkspaceID: ws, SessionToken: "merged-widget-token", AnonymousID: anon, ConversationID: &a.ID, IsAnonymous: true, ExpiresAt: time.Now().Add(time.Hour)}
	if err := env.sessionRepo.Create(ctx, session); err != nil {
		t.Fatal(err)
	}
	attachmentRepo := repository.NewSupportAttachmentRepository(db)
	attachment := &model.SupportAttachment{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: &a.ID, SessionID: &session.ID, FileName: "receipt.pdf", FileSize: 10, ContentType: "application/pdf", StorageKey: "merge-test.pdf", IsUploaded: true, UploadedByType: "customer"}
	if err := attachmentRepo.Create(ctx, attachment); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewSupportMergeRepository(db)
	if _, err := repo.Merge(ctx, ws, b.ID, []string{a.ID}, uuid.NewString(), uuid.NewString(), false); err != nil {
		t.Fatal(err)
	}
	svc := env.service.supportInboxService.SetAttachmentService(NewSupportAttachmentService(attachmentRepo, nil))
	reply, err := svc.WidgetCreateMessage(ctx, session.SessionToken, "Here is the receipt", []string{attachment.ID})
	if err != nil {
		t.Fatal(err)
	}
	if reply.ConversationID != a.ID || len(reply.Attachments) != 1 {
		t.Fatalf("session/attachment changed: %+v", reply)
	}
	saved, err := env.messageRepo.GetByID(ctx, reply.ID)
	if err != nil || saved.ConversationID != b.ID || derefString(saved.OriginConversationID) != a.ID {
		t.Fatalf("canonical route lost: %+v %v", saved, err)
	}
	shared, _, err := repo.SharedMessages(ctx, ws, a.ID, "", true, nil, 100)
	if err != nil || len(shared) != 1 || shared[0].ID != reply.ID {
		t.Fatalf("original widget lost its message: %+v %v", shared, err)
	}
}
