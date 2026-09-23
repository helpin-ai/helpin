package service

import (
	"context"
	"fmt"
	"github.com/helpin-ai/helpin/server/internal/model"
	"testing"
	"time"
)

func TestLiveTranslateConversationOffStillAllowsManualTranslation(t *testing.T) {
	env, c, p := translationFixture(t)
	s := env.service.supportInboxService
	ctx := context.Background()
	if _, err := s.SetLiveTranslate(ctx, c.WorkspaceID, c.ID, false, ""); err != nil {
		t.Fatal(err)
	}
	options, err := s.TranslationOptions(ctx, c.WorkspaceID, c.ID, "reader")
	if err != nil {
		t.Fatal(err)
	}
	if !options.Available || options.Preference.AutoTranslateIncoming || options.Preference.AutoTranslateOutgoing {
		t.Fatalf("options=%+v", options)
	}
	msg := &model.SupportMessage{ID: "manual", WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Hallo"}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatal(err)
	}
	got, err := s.TranslateSupport(ctx, c.WorkspaceID, c.ID, "reader", model.SupportTranslateRequest{MessageID: msg.ID, TargetLanguage: "en"})
	if err != nil || got.Status != "ready" || p.calls != 1 {
		t.Fatalf("manual result=%+v err=%v calls=%d", got, err, p.calls)
	}
}

func TestLiveTranslateNormalizesLanguageHints(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"en-US", "en"}, {"pt-BR", "pt-BR"}, {"zh-Hant-TW", "zh-TW"}, {"fa-IR", "fa"}, {"xx", ""}} {
		if got := normalizeLiveLanguage(tc.in); got != tc.want {
			t.Errorf("%s: %s != %s", tc.in, got, tc.want)
		}
	}
}

func TestLiveTranslateQueuesPrivateReplyWithoutCallingProvider(t *testing.T) {
	env, c, p := translationFixture(t)
	s := env.service.supportInboxService
	ctx := context.Background()
	if err := env.messageRepo.DB().AutoMigrate(&model.SupportPendingSend{}); err != nil {
		t.Fatal(err)
	}
	req := explicitDeliveryRequest(t, "chat_only")
	req.Content = "Hello, please check the invoice."
	req.ClientMessageID = "12345678-1234-1234-1234-123456789012"
	first, err := s.QueueSupportSend(ctx, c.WorkspaceID, c.ID, "22222222-2222-2222-2222-222222222222", req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.QueueSupportSend(ctx, c.WorkspaceID, c.ID, "22222222-2222-2222-2222-222222222222", req)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.PendingSend != "queued" || p.calls != 0 {
		t.Fatalf("queue=%+v calls=%d", first, p.calls)
	}
	messages, err := env.messageRepo.ListByConversation(ctx, c.WorkspaceID, c.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatal("pending draft was published as a public message")
	}
	other, err := s.PendingSupportSends(ctx, c.WorkspaceID, c.ID, "another-user")
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Fatal("another teammate received pending draft")
	}
	req.Content = "different intent"
	if _, err := s.QueueSupportSend(ctx, c.WorkspaceID, c.ID, "22222222-2222-2222-2222-222222222222", req); err == nil {
		t.Fatal("identity reused for changed intent")
	}
}

func TestLiveTranslateSameLanguageSkipsTranslatingProgress(t *testing.T) {
	env, c, p := translationFixture(t)
	s := env.service.supportInboxService
	if _, err := s.SetLiveTranslate(context.Background(), c.WorkspaceID, c.ID, true, "en"); err != nil {
		t.Fatal(err)
	}
	p.responseLanguage = "en"
	p.responseText = ""
	var statuses []string
	ctx := context.WithValue(context.Background(), supportSendProgressKey{}, func(status string) { statuses = append(statuses, status) })
	req := explicitDeliveryRequest(t, "chat_only")
	req.Content = "Hello, we will check your invoice."
	sent, err := s.CreateConversationMessage(ctx, c.WorkspaceID, c.ID, req, "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Content != req.Content {
		t.Fatal("same-language original changed")
	}
	for _, status := range statuses {
		if status == "translating" {
			t.Fatal("English no-op announced translation")
		}
	}
}

func TestLiveTranslateLanguageEvidenceDoesNotTreatIdentifiersAsLanguage(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{{"OK", false}, {"Thank you", false}, {"ORDER-ABC12345", false}, {"https://example.com/invoices/123", false}, {"John Smith", false}, {"你好我需要帮助", true}, {"この請求書を確認してください", true}, {"Please check my invoice", true}, {"```const order = 123456```", false}} {
		if got := meaningfulLanguageText(tc.text); got != tc.want {
			t.Errorf("%q: %v", tc.text, got)
		}
	}
}

// A local attempt deadline must leave the parent usable for fallback.
func TestLiveTranslateAttemptDeadlineIsIndependent(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	first, stop := completionAttemptContext(parent, BillingFeatureSupportTranslation, AICompletionRoute{})
	firstDeadline, _ := first.Deadline()
	parentDeadline, _ := parent.Deadline()
	if !firstDeadline.Before(parentDeadline) {
		t.Fatal("primary consumed the whole fallback budget")
	}
	stop()
	second, stopSecond := completionAttemptContext(parent, BillingFeatureSupportTranslation, AICompletionRoute{})
	defer stopSecond()
	if second.Err() != nil || parent.Err() != nil {
		t.Fatal("cancelled primary poisoned fallback")
	}
}

func TestLiveTranslateEvidenceUsesChronologyAndPrimarySender(t *testing.T) {
	env, c, _ := translationFixture(t)
	s := env.service.supportInboxService
	ctx := context.Background()
	if _, err := s.SetLiveTranslate(ctx, c.WorkspaceID, c.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for _, sample := range []struct {
		id, text, language, purpose, metadata string
		age                                   time.Duration
	}{
		{"newer", "Por favor revise esta factura", "es", "message_display", "{}", -time.Minute},
		{"older", "Please check the invoice", "en", "message_display", "{}", -time.Hour},
		{"cc", "Bonjour pouvez vous vérifier", "fr", "message_display", `{"email_participant_sender":true,"email_sender":"cc@example.invalid"}`, 0},
		{"manual", "Bitte prüfen Sie die Rechnung", "de", "manual_display", "{}", time.Minute},
	} {
		msg := &model.SupportMessage{ID: sample.id, WorkspaceID: c.WorkspaceID, ConversationID: c.ID, Content: sample.text, SenderType: "customer", MessageType: "reply", Metadata: sample.metadata, CreatedAt: now.Add(sample.age)}
		if err := env.messageRepo.Create(ctx, msg); err != nil {
			t.Fatal(err)
		}
		artifact := &model.SupportTranslation{ID: sample.id, WorkspaceID: c.WorkspaceID, ConversationID: c.ID, Purpose: sample.purpose, SourceMessageID: &msg.ID, SourceText: msg.Content, SourceHash: translationHash(msg.Content), SourceLanguage: sample.language, TargetLanguage: "en", Status: "ready", CacheKey: sample.id}
		if err := env.messageRepo.DB().Create(artifact).Error; err != nil {
			t.Fatal(err)
		}
	}
	options, err := s.TranslationOptions(ctx, c.WorkspaceID, c.ID, "reader")
	if err != nil {
		t.Fatal(err)
	}
	if options.DetectedCustomerLanguage != "es" {
		t.Fatalf("reply language %q", options.DetectedCustomerLanguage)
	}
}

func TestLiveTranslateManualBudgetCannotBlockOutgoingReply(t *testing.T) {
	env, c, _ := translationFixture(t)
	s := env.service.supportInboxService
	ctx := context.Background()
	msg := &model.SupportMessage{ID: "history", WorkspaceID: c.WorkspaceID, ConversationID: c.ID, Content: "Hallo", SenderType: "customer", MessageType: "reply"}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatal(err)
	}
	rows := make([]model.SupportTranslation, 1000)
	for i := range rows {
		id := fmt.Sprintf("history-%d", i)
		rows[i] = model.SupportTranslation{ID: id, WorkspaceID: c.WorkspaceID, ConversationID: c.ID, Purpose: "manual_display", SourceMessageID: &msg.ID, SourceText: msg.Content, SourceHash: translationHash(msg.Content), Status: "ready", CacheKey: id}
	}
	if err := env.messageRepo.DB().CreateInBatches(rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.TranslateSupport(ctx, c.WorkspaceID, c.ID, "reader", model.SupportTranslateRequest{MessageID: msg.ID}); err == nil {
		t.Fatal("manual budget was not enforced")
	}
	req := explicitDeliveryRequest(t, "chat_only")
	req.Content = "Hello"
	if _, err := s.CreateConversationMessage(ctx, c.WorkspaceID, c.ID, req, "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, nil); err != nil {
		t.Fatalf("reading activity blocked a reply: %v", err)
	}
}
