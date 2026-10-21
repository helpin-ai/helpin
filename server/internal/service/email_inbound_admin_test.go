package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func adminInboundFixture(t *testing.T) (*emailFallbackTestEnv, *model.SupportConversation, model.SupportInboundJob) {
	t.Helper()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	ctx := context.Background()
	conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: "11111111-1111-1111-1111-111111111111", Subject: "Sign in", Status: "open", Channel: "email", CustomerEmail: strPtr("customer@example.com"), CustomerName: strPtr("Customer")}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	payload := model.PostmarkInboundPayload{MessageID: uuid.NewString(), Subject: "Re: Sign in", FromFull: model.PostmarkAddress{Email: "customer@example.com", Name: "Customer"}, To: "support@example.com", TextBody: "Please help me sign in.", Attachments: []model.PostmarkInboundAttachment{{Name: "error.txt", ContentType: "text/plain", Content: "ZXJyb3I=", ContentLength: 5}}}
	if err := env.service.AcceptInboundEmail(ctx, payload, ""); err != nil {
		t.Fatal(err)
	}
	if err := env.service.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	var job model.SupportInboundJob
	if err := env.convRepo.DB().Where("kind = ?", "email").First(&job).Error; err != nil {
		t.Fatal(err)
	}
	return env, conv, job
}

func TestAdminInboundKnownRetryPreservesReceiptTime(t *testing.T) {
	env, conv, job := adminInboundFixture(t)
	ctx := context.Background()
	var payload model.PostmarkInboundPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	payload.To = "conv-" + conv.ID + "@replies.example.com"
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.convRepo.DB().Model(&job).Updates(map[string]any{"payload": string(raw), "workspace_id": conv.WorkspaceID, "conversation_id": conv.ID, "status": "failed"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := env.service.RecoverAdminInboundEmail(ctx, job.ID, uuid.NewString(), ""); err != nil {
		t.Fatal(err)
	}
	if err := env.service.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	var message model.SupportMessage
	if err := env.convRepo.DB().Where("id = ?", inboundStableID("message:"+payload.MessageID)).First(&message).Error; err != nil {
		t.Fatal(err)
	}
	if !message.CreatedAt.Equal(job.CreatedAt) {
		t.Fatal("retry changed original receipt time")
	}
	// Provider redelivery and another worker pass cannot duplicate recovery.
	if err := env.service.AcceptInboundEmail(ctx, payload, ""); err != nil {
		t.Fatal(err)
	}
	if err := env.service.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := env.convRepo.DB().Model(&model.SupportMessage{}).Where("conversation_id = ?", conv.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("duplicate messages: %d", count)
	}
}

func TestAdminInboundAttachmentsAndErasedContent(t *testing.T) {
	env, conv, job := adminInboundFixture(t)
	ctx := context.Background()
	detail, err := env.service.GetAdminInboundEmail(ctx, job.ID)
	if err != nil || len(detail.Attachments) != 1 {
		t.Fatalf("attachment metadata: %v", err)
	}
	data, name, err := env.service.DownloadAdminInboundAttachment(ctx, job.ID, detail.Attachments[0].ID)
	if err != nil || string(data) != "error" || name != "error.txt" {
		t.Fatalf("original attachment unavailable: %v", err)
	}
	if _, _, err := env.service.DownloadAdminInboundAttachment(ctx, job.ID, uuid.NewString()); !errors.Is(err, ErrInboundRecoveryNotFound) {
		t.Fatal("unrelated attachment was accessible")
	}
	if err := env.service.RecoverAdminInboundEmail(ctx, job.ID, uuid.NewString(), conv.ID); err != nil {
		t.Fatal(err)
	}
	if err := env.service.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	data, _, err = env.service.DownloadAdminInboundAttachment(ctx, job.ID, detail.Attachments[0].ID)
	if err != nil || string(data) != "error" {
		t.Fatal("completed attachment disappeared")
	}
	if err := env.convRepo.DB().Model(conv).Update("anonymized_at", time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := env.service.GetAdminInboundEmail(ctx, job.ID); !errors.Is(err, ErrInboundRecoveryNotFound) {
		t.Fatalf("erased email still visible: %v", err)
	}
	if _, _, err := env.service.DownloadAdminInboundAttachment(ctx, job.ID, detail.Attachments[0].ID); !errors.Is(err, ErrInboundRecoveryNotFound) {
		t.Fatal("erased attachment still visible")
	}
	list, err := env.service.ListAdminInboundEmails(ctx, "all", "", 1)
	if err != nil || list.Total != 0 {
		t.Fatalf("erased receipt listed: %v", err)
	}
}

func TestAdminInboundQueueFiltersAndFailure(t *testing.T) {
	env, conv, job := adminInboundFixture(t)
	ctx := context.Background()
	list, err := env.service.ListAdminInboundEmails(ctx, "attention", "customer@example.com", 1)
	if err != nil || list.Total != 1 || list.AttentionCount != 1 {
		t.Fatalf("attention list: %+v %v", list, err)
	}
	if err := env.convRepo.DB().Model(&job).Update("workspace_id", conv.WorkspaceID).Error; err != nil {
		t.Fatal(err)
	}
	list, err = env.service.ListAdminInboundEmails(ctx, "attention", "", 1)
	if err != nil || list.Total != 1 {
		t.Fatalf("scheduled retry hidden until retries exhausted: %+v %v", list, err)
	}
	if err := env.service.RecoverAdminInboundEmail(ctx, job.ID, uuid.NewString(), conv.ID); err != nil {
		t.Fatal(err)
	}
	list, err = env.service.ListAdminInboundEmails(ctx, "processing", "", 1)
	if err != nil || list.Total != 1 || list.AttentionCount != 0 {
		t.Fatalf("processing list: %+v %v", list, err)
	}
	if err := env.convRepo.DB().Model(conv).Update("customer_email", "changed@example.com").Error; err != nil {
		t.Fatal(err)
	}
	if err := env.service.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	detail, err := env.service.GetAdminInboundEmail(ctx, job.ID)
	if err != nil || detail.Status != "failed" || detail.Body == "" {
		t.Fatalf("recovery failure hidden or content lost: %+v %v", detail, err)
	}
	list, err = env.service.ListAdminInboundEmails(ctx, "attention", "Sign in", 1)
	if err != nil || list.AttentionCount != 1 || list.Total != 1 {
		t.Fatalf("failure not returned to attention: %+v %v", list, err)
	}
}

func TestAdminInboundRecoveryPreservesEmailAndDeduplicates(t *testing.T) {
	env, conv, job := adminInboundFixture(t)
	ctx := context.Background()
	actor := "22222222-2222-2222-2222-222222222222"
	if err := env.service.RecoverAdminInboundEmail(ctx, job.ID, actor, conv.ID); err != nil {
		t.Fatal(err)
	}
	if err := env.service.RecoverAdminInboundEmail(ctx, job.ID, actor, conv.ID); !errors.Is(err, ErrInboundRecoveryConflict) {
		t.Fatalf("duplicate recovery should conflict, got %v", err)
	}
	if err := env.service.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	var saved model.SupportInboundJob
	if err := env.convRepo.DB().First(&saved, "id = ?", job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != "completed" || saved.Payload != "" || derefString(saved.RecoveryRequestedBy) != actor {
		t.Fatalf("recovery not completed and audited: status=%s", saved.Status)
	}
	var messages []model.SupportMessage
	if err := env.convRepo.DB().Where("conversation_id = ? AND sender_type = ?", conv.ID, "customer").Find(&messages).Error; err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].Content != "Please help me sign in." || messages[0].IsInternal {
		t.Fatalf("wrong recovered messages: %d", len(messages))
	}
	if !messages[0].CreatedAt.Equal(job.CreatedAt) {
		t.Fatalf("received time changed: %v != %v", messages[0].CreatedAt, job.CreatedAt)
	}
	var attachments int64
	env.convRepo.DB().Model(&model.SupportAttachment{}).Where("message_id = ?", messages[0].ID).Count(&attachments)
	if attachments != 1 {
		t.Fatalf("attachment lost: %d", attachments)
	}
	if err := env.service.RecoverAdminInboundEmail(ctx, job.ID, actor, conv.ID); !errors.Is(err, ErrInboundRecoveryConflict) {
		t.Fatalf("completed email recovered twice: %v", err)
	}
	list, err := env.service.ListAdminInboundEmails(ctx, "recovered", "Sign in", 1)
	if err != nil || list.Total != 1 || len(list.Entries) != 1 || list.Entries[0].Subject != "Re: Sign in" {
		t.Fatalf("completed receipt not inspectable: %+v %v", list, err)
	}
	detail, err := env.service.GetAdminInboundEmail(ctx, job.ID)
	if err != nil || detail.Body != "Please help me sign in." || detail.Status != "completed" {
		t.Fatalf("recovered body unavailable: %+v %v", detail, err)
	}
}

func TestAdminInboundRejectsUnsafeDestinations(t *testing.T) {
	for _, scenario := range []string{"other sender", "other workspace", "anonymized", "spam", "merged", "active worker", "missing target"} {
		t.Run(scenario, func(t *testing.T) {
			env, conv, job := adminInboundFixture(t)
			db := env.convRepo.DB()
			target := conv.ID
			switch scenario {
			case "other sender":
				db.Model(conv).Update("customer_email", "someone@example.com")
			case "other workspace":
				db.Model(&job).Update("workspace_id", uuid.NewString())
			case "anonymized":
				db.Model(conv).Update("anonymized_at", time.Now().UTC())
			case "spam":
				db.Model(conv).Update("status", "spam")
			case "merged":
				db.Model(conv).Update("merged_into_id", uuid.NewString())
			case "active worker":
				db.Model(&job).Updates(map[string]any{"status": "processing", "lease_token": "active", "available_at": time.Now().Add(time.Minute)})
			case "missing target":
				target = uuid.NewString()
			}
			if err := env.service.RecoverAdminInboundEmail(context.Background(), job.ID, uuid.NewString(), target); err == nil {
				t.Fatal("unsafe recovery accepted")
			}
			var saved model.SupportInboundJob
			db.First(&saved, "id = ?", job.ID)
			if saved.Payload == "" || saved.RecoveryRequestedAt != nil {
				t.Fatal("rejected recovery changed the receipt")
			}
		})
	}
}

func TestAdminInboundSuggestionsRequireSelectionAndHonorWorkspace(t *testing.T) {
	env, conv, job := adminInboundFixture(t)
	ctx := context.Background()
	other := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: uuid.NewString(), Subject: "Other workspace", Status: "open", CustomerEmail: conv.CustomerEmail}
	if err := env.convRepo.Create(ctx, other); err != nil {
		t.Fatal(err)
	}
	choices, err := env.service.FindAdminInboundConversations(ctx, job.ID, "")
	if err != nil || len(choices) != 2 {
		t.Fatalf("want two explicit choices: %d %v", len(choices), err)
	}
	env.convRepo.DB().Model(&job).Update("workspace_id", conv.WorkspaceID)
	choices, err = env.service.FindAdminInboundConversations(ctx, job.ID, "")
	if err != nil || len(choices) != 1 || choices[0].ID != conv.ID {
		t.Fatalf("known workspace not respected: %+v %v", choices, err)
	}
	choices, err = env.service.FindAdminInboundConversations(ctx, job.ID, "https://app.example.com/w/example/support/"+conv.ID+"?view=resolved")
	if err != nil || len(choices) != 1 || choices[0].ID != conv.ID {
		t.Fatalf("conversation URL lookup failed: %+v %v", choices, err)
	}
	var saved model.SupportInboundJob
	env.convRepo.DB().First(&saved, "id = ?", job.ID)
	if saved.RecoveryConversationID != nil {
		t.Fatal("suggestion selected a destination automatically")
	}
}

func TestAdminInboundRevalidatesDestinationInWorker(t *testing.T) {
	env, conv, job := adminInboundFixture(t)
	ctx := context.Background()
	if err := env.service.RecoverAdminInboundEmail(ctx, job.ID, uuid.NewString(), conv.ID); err != nil {
		t.Fatal(err)
	}
	env.convRepo.DB().Model(conv).Update("customer_email", "changed@example.com")
	if err := env.service.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	var saved model.SupportInboundJob
	env.convRepo.DB().First(&saved, "id = ?", job.ID)
	if saved.Payload == "" || saved.Status == "completed" {
		t.Fatal("stale destination discarded or delivered the email")
	}
	var count int64
	env.convRepo.DB().Model(&model.SupportMessage{}).Where("conversation_id = ?", conv.ID).Count(&count)
	if count != 0 {
		t.Fatal("stale selection inserted a message")
	}
}

func TestAdminInboundRetryKeepsKnownRouting(t *testing.T) {
	env, conv, job := adminInboundFixture(t)
	ctx := context.Background()
	if err := env.service.RecoverAdminInboundEmail(ctx, job.ID, uuid.NewString(), ""); err == nil {
		t.Fatal("unrouted email retried without a destination")
	}
	// A failed job may already have a validated route. Retrying must not replace it.
	env.convRepo.DB().Model(&job).Updates(map[string]any{"status": "failed", "workspace_id": conv.WorkspaceID, "conversation_id": conv.ID})
	if err := env.service.RecoverAdminInboundEmail(ctx, job.ID, uuid.NewString(), ""); err != nil {
		t.Fatal(err)
	}
	saved, err := repository.NewSupportInboundJobRepository(env.convRepo.DB()).GetEmail(ctx, job.ID)
	if err != nil || saved.Status != "pending" || saved.RecoveryConversationID != nil || derefString(saved.ConversationID) != conv.ID {
		t.Fatalf("retry changed original routing: %+v %v", saved, err)
	}
}
