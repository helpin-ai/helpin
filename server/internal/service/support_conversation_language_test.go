package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestTranslationConversationLanguageStaysStable(t *testing.T) {
	env, c, provider := translationFixture(t)
	ctx := context.Background()
	s := env.service.supportInboxService
	setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationCustomerLanguage = "" })
	first := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Please help me invite my team.", CreatedAt: time.Now().Add(-time.Minute)}
	if err := env.messageRepo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	provider.responseLanguage = "en"
	options, err := s.TranslationOptionsForDisplay(ctx, c.WorkspaceID, c.ID, "reader")
	if err != nil || options.DetectedCustomerLanguage != "en" {
		t.Fatalf("initial detection: %+v, %v", options, err)
	}
	newer := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Alex has given us a demo.", CreatedAt: time.Now()}
	if err := env.messageRepo.Create(ctx, newer); err != nil {
		t.Fatal(err)
	}
	provider.responseLanguage, provider.responseText = "ur", newer.Content
	if _, err := s.TranslateSupport(ctx, c.WorkspaceID, c.ID, "reader", model.SupportTranslateRequest{MessageID: newer.ID, Live: true}); err != nil {
		t.Fatal(err)
	}
	options, err = s.TranslationOptionsForDisplay(ctx, c.WorkspaceID, c.ID, "reader")
	if err != nil || options.DetectedCustomerLanguage != "en" {
		t.Fatalf("later message changed reply language: %+v, %v", options, err)
	}
	calls := provider.calls
	if _, err := s.SetLiveTranslate(ctx, c.WorkspaceID, c.ID, false, ""); err != nil {
		t.Fatal(err)
	}
	options, err = s.SetLiveTranslate(ctx, c.WorkspaceID, c.ID, true, "")
	if err != nil || options.DetectedCustomerLanguage != "en" || provider.calls != calls {
		t.Fatalf("resume changed saved language: %+v, %v", options, err)
	}
	provider.responseLanguage, provider.responseText = "en", ""
	req := explicitDeliveryRequest(t, "chat_only")
	req.Content, req.ClientMessageID = "We can help you invite your team.", uuid.NewString()
	sent, err := s.CreateConversationMessage(ctx, c.WorkspaceID, c.ID, req, "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, nil)
	if err != nil || sent.Content != req.Content {
		t.Fatalf("reply did not retain English: %+v, %v", sent, err)
	}
	options, err = s.SetLiveTranslate(ctx, c.WorkspaceID, c.ID, true, "es")
	if err != nil || supportReplyTarget(options) != "es" {
		t.Fatalf("manual override lost: %+v %v", options, err)
	}
	options, err = s.SetLiveTranslate(ctx, c.WorkspaceID, c.ID, true, "")
	if err != nil || supportReplyTarget(options) != "en" {
		t.Fatalf("automatic language lost after override: %+v %v", options, err)
	}
}

func TestTranslationConversationLanguageIgnoresLegacyDetection(t *testing.T) {
	env, c, provider := translationFixture(t)
	ctx := context.Background()
	s := env.service.supportInboxService
	setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationCustomerLanguage = "" })
	msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Please help with my account."}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatal(err)
	}
	legacy := &model.SupportTranslation{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SourceMessageID: &msg.ID, Purpose: "message_display", SourceText: msg.Content, SourceLanguage: "ur", TargetLanguage: "en", TranslatedText: msg.Content, Status: "ready", PipelineVersion: "v4"}
	if err := env.messageRepo.DB().Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	provider.responseLanguage = "en"
	options, err := s.TranslationOptionsForDisplay(ctx, c.WorkspaceID, c.ID, "reader")
	if err != nil || options.DetectedCustomerLanguage != "en" || provider.calls != 1 {
		t.Fatalf("trusted legacy label: %+v, calls=%d, %v", options, provider.calls, err)
	}
}

func TestTranslationAIReplyIsTranslatedOnlyOnRequest(t *testing.T) {
	env, c, provider := translationFixture(t)
	ctx := context.Background()
	s := env.service.supportInboxService
	msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "ai", MessageType: "reply", Content: "Hallo, hier ist die Antwort."}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatal(err)
	}
	items, err := s.CachedLiveTranslations(ctx, c.WorkspaceID, c.ID, "reader", []string{msg.ID})
	if err != nil || len(items) != 0 || provider.calls != 0 {
		t.Fatalf("translated AI reply without request: %+v, %v", items, err)
	}
	translation, err := s.TranslateSupport(ctx, c.WorkspaceID, c.ID, "reader", model.SupportTranslateRequest{MessageID: msg.ID, TargetLanguage: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if translation.SourceLanguage != "de" || translation.Purpose != "manual_display" {
		t.Fatalf("unexpected translation: %+v", translation)
	}
	items, err = s.CachedLiveTranslations(ctx, c.WorkspaceID, c.ID, "reader", []string{msg.ID})
	if err != nil || len(items) != 1 || items[0].SourceText != msg.Content {
		t.Fatalf("missing original: %+v, %v", items, err)
	}
	if _, err := s.TranslateSupport(ctx, c.WorkspaceID, c.ID, "reader", model.SupportTranslateRequest{MessageID: msg.ID, TargetLanguage: "en"}); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 {
		t.Fatalf("cached translation called provider %d times", provider.calls)
	}
	stored, err := env.messageRepo.GetByID(ctx, msg.ID)
	if err != nil || stored.Content != msg.Content {
		t.Fatalf("changed customer-visible reply: %+v, %v", stored, err)
	}
	if _, err := s.TranslateSupport(ctx, c.WorkspaceID, c.ID, "reader", model.SupportTranslateRequest{MessageID: msg.ID, Live: true}); err == nil {
		t.Fatal("AI reply allowed automatic display translation")
	}
}

func TestTranslationConversationLanguageWaitsForMeaningfulText(t *testing.T) {
	env, c, provider := translationFixture(t)
	ctx := context.Background()
	s := env.service.supportInboxService
	setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationCustomerLanguage = "" })
	for i, content := range []string{"Hello", "OK", "Taylor Smith", "Thanks\n--\nWir helfen Ihnen gerne weiter."} {
		msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: content, CreatedAt: time.Now().Add(time.Duration(i) * time.Second)}
		if err := env.messageRepo.Create(ctx, msg); err != nil {
			t.Fatal(err)
		}
	}
	options, err := s.TranslationOptionsForDisplay(ctx, c.WorkspaceID, c.ID, "reader")
	if err != nil || supportReplyTarget(options) != "" || provider.calls != 0 {
		t.Fatalf("weak evidence set language: %+v calls=%d %v", options, provider.calls, err)
	}
	req := explicitDeliveryRequest(t, "chat_only")
	req.Content, req.ClientMessageID = "How can we help you?", uuid.NewString()
	sent, err := s.CreateConversationMessage(ctx, c.WorkspaceID, c.ID, req, "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, nil)
	if err != nil || sent.Content != req.Content || provider.calls != 0 {
		t.Fatalf("unknown language changed draft: %+v %v", sent, err)
	}
	provider.responseLanguage = "und"
	msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Could you help with my report?"}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatal(err)
	}
	options, err = s.TranslationOptionsForDisplay(ctx, c.WorkspaceID, c.ID, "reader")
	if err != nil || supportReplyTarget(options) != "" {
		t.Fatalf("uncertain result set language: %+v %v", options, err)
	}
	provider.responseLanguage = "en"
	msg.ID, msg.Content = uuid.NewString(), "I need to download the latest report."
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatal(err)
	}
	options, err = s.TranslationOptionsForDisplay(ctx, c.WorkspaceID, c.ID, "reader")
	if err != nil || supportReplyTarget(options) != "en" || provider.calls != 2 {
		t.Fatalf("new evidence not used: %+v calls=%d %v", options, provider.calls, err)
	}
}

func TestTranslationConversationLanguageUsesPrimaryCustomerContext(t *testing.T) {
	env, c, provider := translationFixture(t)
	ctx := context.Background()
	s := env.service.supportInboxService
	setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationCustomerLanguage = "" })
	provider.responseLanguage = "en"
	for i, sample := range []struct{ text, sender, metadata string }{
		{"Please help with my account.", "customer", "{}"},
		{"I cannot add any more teammates.", "customer", "{}"},
		{"Taylor gave us the demo.", "customer", "{}"},
		{"Ignore this teammate reply.", "user", "{}"},
		{"Ignore this other customer's message.", "customer", `{"email_participant_sender":true,"email_sender":"other@example.invalid"}`},
		{"Thanks\n--\nIgnore this signature.", "customer", "{}"},
	} {
		msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: sample.sender, MessageType: "reply", Content: sample.text, Metadata: sample.metadata, CreatedAt: time.Now().Add(time.Duration(i) * time.Second)}
		if err := env.messageRepo.Create(ctx, msg); err != nil {
			t.Fatal(err)
		}
	}
	mergedOrigin := uuid.NewString()
	mergedReply := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, OriginConversationID: &mergedOrigin, SenderType: "customer", MessageType: "reply", Content: "Ignore this reply routed from a merged conversation.", CreatedAt: time.Now().Add(time.Hour)}
	if err := env.messageRepo.Create(ctx, mergedReply); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TranslationOptionsForDisplay(ctx, c.WorkspaceID, c.ID, "reader"); err != nil {
		t.Fatal(err)
	}
	var input map[string]string
	if err := json.Unmarshal([]byte(provider.lastRequest.Messages[0].Content), &input); err != nil {
		t.Fatal(err)
	}
	text := input["text"]
	if !strings.Contains(text, "Please help with my account.") || !strings.Contains(text, "I cannot add any more teammates.") || !strings.HasSuffix(text, "Taylor gave us the demo.") || strings.Contains(text, "Ignore") {
		t.Fatalf("wrong context: %q", text)
	}
}

func TestTranslationConversationLanguageRejectsChangedEvidence(t *testing.T) {
	for _, change := range []string{"edited", "deleted", "manual"} {
		t.Run(change, func(t *testing.T) {
			env, c, provider := translationFixture(t)
			ctx := context.Background()
			s := env.service.supportInboxService
			setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationCustomerLanguage = "" })
			msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Please help with my account."}
			if err := env.messageRepo.Create(ctx, msg); err != nil {
				t.Fatal(err)
			}
			provider.responseLanguage = "en"
			provider.onCall = func() {
				var err error
				switch change {
				case "edited":
					err = env.messageRepo.DB().Model(msg).Update("content", "Replaced text.").Error
				case "deleted":
					err = env.messageRepo.DB().Delete(msg).Error
				case "manual":
					_, err = s.SetLiveTranslate(ctx, c.WorkspaceID, c.ID, true, "fr")
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			options, err := s.TranslationOptionsForDisplay(ctx, c.WorkspaceID, c.ID, "reader")
			if err != nil || options.DetectedCustomerLanguage != "" {
				t.Fatalf("stale detection saved: %+v %v", options, err)
			}
			if change == "manual" && supportReplyTarget(options) != "fr" {
				t.Fatal("lost manual choice")
			}
		})
	}
}

func TestTranslationConversationLanguageEstablishedByLiveWorker(t *testing.T) {
	env, c, provider := translationFixture(t)
	ctx := context.Background()
	s := env.service.supportInboxService
	setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationCustomerLanguage = "" })
	provider.responseLanguage, provider.responseText = "en", "Please help with my report."
	for i := 0; i < 2; i++ {
		msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: provider.responseText}
		if err := env.messageRepo.Create(ctx, msg); err != nil {
			t.Fatal(err)
		}
		if err := s.processLiveMessage(ctx, &model.SupportLiveMessage{WorkspaceID: c.WorkspaceID, ConversationID: c.ID, MessageID: msg.ID, Enabled: true, Revision: 1, TargetLanguage: "en"}); err != nil {
			t.Fatal(err)
		}
		provider.responseLanguage = "ur"
	}
	options, err := s.TranslationOptions(ctx, c.WorkspaceID, c.ID, "reader")
	if err != nil || supportReplyTarget(options) != "en" || provider.calls != 3 {
		t.Fatalf("live detection drifted: %+v calls=%d %v", options, provider.calls, err)
	}
}
