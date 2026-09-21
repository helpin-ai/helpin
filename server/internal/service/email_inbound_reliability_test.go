package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestInboundParticipantIdentity(t *testing.T) {
	for _, known := range []bool{true, false} {
		t.Run(map[bool]string{true: "cc", false: "unknown"}[known], func(t *testing.T) {
			env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
			ctx := context.Background()
			conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: "11111111-1111-1111-1111-111111111111", Subject: "Help", Status: "resolved", CustomerEmail: strPtr("customer@example.com"), CustomerName: strPtr("Original")}
			if known {
				conv.EmailCC = model.DocsStringArray{"colleague@example.com"}
			}
			if err := env.convRepo.Create(ctx, conv); err != nil {
				t.Fatal(err)
			}
			p := model.PostmarkInboundPayload{MessageID: uuid.NewString(), FromFull: model.PostmarkAddress{Email: "colleague@example.com", Name: "Colleague"}, To: "conv-" + conv.ID + "@replies.helpin.ai", StrippedTextReply: "Here is the answer"}
			if err := env.service.ProcessInboundEmail(ctx, p, ""); err != nil {
				t.Fatal(err)
			}
			var msgs []model.SupportMessage
			if err := env.convRepo.DB().Where("conversation_id = ? AND sender_type = ?", conv.ID, "customer").Find(&msgs).Error; err != nil {
				t.Fatal(err)
			}
			if len(msgs) != 1 {
				t.Fatalf("want preserved message, got %d", len(msgs))
			}
			if derefString(msgs[0].SenderDisplayName) != "Colleague" || msgs[0].IsInternal == known {
				t.Fatalf("wrong attribution/privacy: %+v", msgs[0])
			}
			var updated model.SupportConversation
			env.convRepo.DB().First(&updated, "id = ?", conv.ID)
			if derefString(updated.CustomerEmail) != "customer@example.com" {
				t.Fatal("primary customer changed")
			}
			if updated.Status != "open" {
				t.Fatal("reply did not bring conversation to attention")
			}
		})
	}
}

func TestInboundReceiptDurability(t *testing.T) {
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	ctx := context.Background()
	p := model.PostmarkInboundPayload{MessageID: "durable-test", FromFull: model.PostmarkAddress{Email: "person@example.com"}, To: "invalid@example.com", TextBody: "Retain this email"}
	if err := env.service.AcceptInboundEmail(ctx, p, ""); err != nil {
		t.Fatal(err)
	}
	if err := env.service.AcceptInboundEmail(ctx, p, ""); err != nil {
		t.Fatal(err)
	}
	var count int64
	env.convRepo.DB().Table("support_inbound_jobs").Count(&count)
	if count != 1 {
		t.Fatalf("duplicate receipt: %d", count)
	}
	if err := env.service.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	var row struct {
		Status   string
		Payload  string
		Attempts int
	}
	env.convRepo.DB().Table("support_inbound_jobs").First(&row)
	if row.Status != "pending" || row.Payload == "" || row.Attempts != 1 {
		t.Fatalf("failed routing must remain retryable: %+v", row)
	}
}

func TestInboundAttachmentFailureDoesNotLoseMessage(t *testing.T) {
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	ctx := context.Background()
	conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: "11111111-1111-1111-1111-111111111111", Subject: "File", Status: "open", CustomerEmail: strPtr("customer@example.com")}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	p := model.PostmarkInboundPayload{MessageID: uuid.NewString(), FromFull: model.PostmarkAddress{Email: "customer@example.com"}, To: "conv-" + conv.ID + "@replies.helpin.ai"}
	// No body: an attachment alone is still a message.
	p.Attachments = append(p.Attachments, model.PostmarkInboundAttachment{Name: "report.pdf", ContentType: "application/pdf", Content: "YWJj", ContentLength: 3})
	if err := env.service.ProcessInboundEmail(ctx, p, ""); err != nil {
		t.Fatal(err)
	}
	var msg model.SupportMessage
	if err := env.convRepo.DB().Where("conversation_id = ?", conv.ID).First(&msg).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		env.convRepo.DB().Model(&model.SupportInboundJob{}).Where("kind = ?", "attachment").Update("available_at", time.Now().Add(-time.Second))
		if err := env.service.processNextInboundJob(ctx, "attachment"); err != nil {
			t.Fatal(err)
		}
	}
	var a model.SupportAttachment
	env.convRepo.DB().First(&a, "message_id = ?", msg.ID)
	if a.ProcessingStatus != "failed" {
		t.Fatalf("attachment status %q", a.ProcessingStatus)
	}
	var job model.SupportInboundJob
	env.convRepo.DB().First(&job, "id = ?", a.ID)
	if job.Status != "failed" || job.Payload == "" {
		t.Fatal("failed attachment lost retry data")
	}
	if err := env.service.supportInboxService.RetryInboundAttachment(ctx, uuid.NewString(), msg.ID, a.ID); err == nil {
		t.Fatal("another workspace could retry the attachment")
	}
	if err := env.service.supportInboxService.RetryInboundAttachment(ctx, conv.WorkspaceID, msg.ID, uuid.NewString()); err == nil {
		t.Fatal("an unrelated attachment could be retried")
	}
	if err := env.service.supportInboxService.RetryInboundAttachment(ctx, conv.WorkspaceID, msg.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := env.service.ProcessInboundEmail(ctx, p, ""); err != nil {
		t.Fatal(err)
	}
	var count int64
	env.convRepo.DB().Model(&model.SupportMessage{}).Where("conversation_id = ?", conv.ID).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate created %d", count)
	}
}

func TestInboundLeaseRecoveryFencesOldWorker(t *testing.T) {
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	ctx := context.Background()
	now := time.Now().UTC()
	repo := repository.NewSupportInboundJobRepository(env.convRepo.DB())
	job := &model.SupportInboundJob{ID: uuid.NewString(), Kind: "email", Payload: `{"MessageID":"lease-test"}`, Status: "pending", AvailableAt: now}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	first, err := repo.Claim(ctx, "email", now)
	if err != nil || first == nil {
		t.Fatalf("first claim %v", err)
	}
	second, err := repo.Claim(ctx, "email", now)
	if err != nil || second != nil {
		t.Fatal("leased job claimed twice")
	}
	second, err = repo.Claim(ctx, "email", now.Add(3*time.Minute))
	if err != nil || second == nil {
		t.Fatal("abandoned lease was not recovered")
	}
	if err := repo.Finish(ctx, first, nil, now); err != nil {
		t.Fatal(err)
	}
	var saved model.SupportInboundJob
	env.convRepo.DB().First(&saved, "id = ?", job.ID)
	if saved.Status != "processing" || saved.Payload == "" || saved.LeaseToken != second.LeaseToken {
		t.Fatal("stale worker completed current lease")
	}
	if err := repo.Finish(ctx, second, nil, now); err != nil {
		t.Fatal(err)
	}
	env.convRepo.DB().First(&saved, "id = ?", job.ID)
	if saved.Status != "completed" || saved.Payload != "" {
		t.Fatal("completed receipt retained raw payload")
	}
}

func TestInboundParticipantAddedOnLaterReply(t *testing.T) {
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	ctx := context.Background()
	conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: "11111111-1111-1111-1111-111111111111", Subject: "Help", Status: "open", CustomerEmail: strPtr("customer@example.com")}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	p := model.PostmarkInboundPayload{MessageID: uuid.NewString(), FromFull: model.PostmarkAddress{Email: "customer@example.com"}, To: "conv-" + conv.ID + "@replies.helpin.ai", Cc: "colleague@example.com", TextBody: "Adding my colleague"}
	if err := env.service.ProcessInboundEmail(ctx, p, ""); err != nil {
		t.Fatal(err)
	}
	p.MessageID = uuid.NewString()
	p.FromFull = model.PostmarkAddress{Email: "colleague@example.com", Name: "Colleague"}
	p.Cc = ""
	p.TextBody = "I can help"
	if err := env.service.ProcessInboundEmail(ctx, p, ""); err != nil {
		t.Fatal(err)
	}
	var msg model.SupportMessage
	if err := env.convRepo.DB().First(&msg, "id = ?", inboundStableID("message:"+p.MessageID)).Error; err != nil {
		t.Fatal(err)
	}
	if msg.IsInternal {
		t.Fatal("a CC added by the customer was treated as an unknown sender")
	}
}

func TestInboundConstraintFailureWithoutReceiptRemainsRetryable(t *testing.T) {
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	ctx := context.Background()
	conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: "11111111-1111-1111-1111-111111111111", Subject: "Help", Status: "open", CustomerEmail: strPtr("customer@example.com")}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	p := model.PostmarkInboundPayload{MessageID: uuid.NewString(), FromFull: model.PostmarkAddress{Email: "customer@example.com"}, To: "conv-" + conv.ID + "@replies.helpin.ai", TextBody: "Do not discard"}
	conflict := &model.SupportMessage{ID: inboundStableID("message:" + p.MessageID), WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "user", Content: "Unrelated row", MessageType: "reply"}
	if err := env.convRepo.DB().Create(conflict).Error; err != nil {
		t.Fatal(err)
	}
	if err := env.service.ProcessInboundEmail(ctx, p, ""); err == nil {
		t.Fatal("constraint failure acknowledged without a saved email receipt")
	}
}
