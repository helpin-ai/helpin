package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestLanguageEvidenceExcludesLegalFooter(t *testing.T) {
	text := "Yes! Please send the analytics report.\n\nThis email contains confidential information.\n" + strings.Repeat("Diese E-Mail enthält vertrauliche Informationen. ", 100)
	if got := languageEvidence(text); got != "Yes! Please send the analytics report." {
		t.Fatalf("included footer: %q", got)
	}
}

func TestTranslationUnknownCustomerSendsOriginal(t *testing.T) {
	for _, result := range []string{"und", "mul", "unavailable"} {
		t.Run(result, func(t *testing.T) {
			env, c, provider := translationFixture(t)
			setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationCustomerLanguage = "" })
			provider.responseLanguage = result
			provider.fail = result == "unavailable"
			msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Yes, please help with my report."}
			if err := env.messageRepo.Create(context.Background(), msg); err != nil {
				t.Fatal(err)
			}
			req := explicitDeliveryRequest(t, "chat_only")
			req.Content = "Hello, we are checking your report."
			req.ClientMessageID = uuid.NewString()
			actor := "22222222-2222-2222-2222-222222222222"
			s := env.service.supportInboxService
			sent, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", &actor, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if sent.Content != req.Content || sent.TranslationID != "" {
				t.Fatalf("original changed: %+v", sent)
			}
			again, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", &actor, nil, nil)
			if err != nil || again.ID != sent.ID {
				t.Fatalf("retry duplicated: %v", err)
			}
		})
	}
}

func TestCustomerLanguageUncertainResultCanRecover(t *testing.T) {
	env, c, provider := translationFixture(t)
	provider.responseLanguage = "mul"
	msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Please help with the report."}
	if err := env.messageRepo.Create(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	s := env.service.supportInboxService
	if _, err := s.detectCustomerLanguage(context.Background(), c.WorkspaceID, c.ID, msg); err != errCustomerLanguageUnknown {
		t.Fatalf("expected unknown: %v", err)
	}
	if err := env.messageRepo.DB().Model(&model.SupportTranslation{}).Where("source_message_id = ?", msg.ID).Update("updated_at", time.Now().Add(-2*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	provider.responseLanguage = "en"
	language, err := s.detectCustomerLanguage(context.Background(), c.WorkspaceID, c.ID, msg)
	if err != nil || language != "en" {
		t.Fatalf("uncertain result poisoned cache: %q %v", language, err)
	}
}
