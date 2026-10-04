package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestTranslationDisplayDetectsOlderMessagesAndCaches(t *testing.T) {
	for _, language := range []string{"en", "de"} {
		t.Run(language, func(t *testing.T) {
			env, c, provider := translationFixture(t)
			ctx := context.Background()
			s := env.service.supportInboxService
			setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationCustomerLanguage = "" })
			provider.responseLanguage = language
			older := time.Now().Add(-30 * 24 * time.Hour)
			msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Please help me with my report.", CreatedAt: older}
			if err := env.messageRepo.Create(ctx, msg); err != nil {
				t.Fatal(err)
			}
			// Ignore acknowledgments, teammate replies, notes and other email participants.
			for i, sample := range []struct {
				sender, text, metadata string
				internal               bool
			}{
				{"customer", "OK", "{}", false},
				{"user", "Je vais examiner votre rapport.", "{}", false},
				{"customer", "Dies ist eine interne Notiz.", "{}", true},
				{"customer", "Dies ist eine Antwort eines anderen Teilnehmers.", `{"email_participant_sender":true,"email_sender":"someone-else@example.com"}`, false},
			} {
				other := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: sample.sender, MessageType: "reply", Content: sample.text, Metadata: sample.metadata, IsInternal: sample.internal, CreatedAt: older.Add(time.Duration(i+1) * time.Hour)}
				if err := env.messageRepo.Create(ctx, other); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.TranslationOptions(ctx, c.WorkspaceID, c.ID, "reader"); err != nil {
				t.Fatal(err)
			}
			if provider.calls != 0 {
				t.Fatal("internal settings reads must not run detection")
			}
			for i := 0; i < 2; i++ {
				options, err := s.TranslationOptionsForDisplay(ctx, c.WorkspaceID, c.ID, "reader")
				if err != nil {
					t.Fatal(err)
				}
				if options.DetectedCustomerLanguage != language {
					t.Fatalf("language=%q", options.DetectedCustomerLanguage)
				}
				if options.Conversation.CustomerLanguage != "" || options.Conversation.TranslationMode != "on" || options.Conversation.Revision != 1 {
					t.Fatalf("policy changed: %+v", options.Conversation)
				}
			}
			if provider.calls != 1 {
				t.Fatalf("detection calls=%d", provider.calls)
			}
			var artifacts []model.SupportTranslation
			if err := env.messageRepo.DB().Find(&artifacts).Error; err != nil {
				t.Fatal(err)
			}
			if len(artifacts) != 1 || artifacts[0].Purpose != "language_detection" || artifacts[0].SourceMessageID == nil || *artifacts[0].SourceMessageID != msg.ID {
				t.Fatalf("unexpected artifacts: %+v", artifacts)
			}
		})
	}
}

func TestTranslationDisplayPreservesSettingsAndHandlesUnknown(t *testing.T) {
	for _, scenario := range []string{"paused", "manual", "unavailable", "no-message", "acknowledgment", "uncertain", "provider-failure", "paused-during-detection"} {
		t.Run(scenario, func(t *testing.T) {
			env, c, provider := translationFixture(t)
			ctx := context.Background()
			s := env.service.supportInboxService
			setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationCustomerLanguage = "" })
			before, err := s.TranslationOptions(ctx, c.WorkspaceID, c.ID, "reader")
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "no-message" {
				content := "Please help me with my report."
				if scenario == "acknowledgment" {
					content = "OK"
				}
				msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: content}
				if err := env.messageRepo.Create(ctx, msg); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "paused":
				before, err = s.SetLiveTranslate(ctx, c.WorkspaceID, c.ID, false, "")
			case "manual":
				before, err = s.SetLiveTranslate(ctx, c.WorkspaceID, c.ID, true, "fr")
			case "unavailable":
				s.translations.available = false
			case "uncertain":
				provider.responseLanguage = "und"
			case "provider-failure":
				provider.fail = true
			case "paused-during-detection":
				provider.onCall = func() { before, err = s.SetLiveTranslate(ctx, c.WorkspaceID, c.ID, false, "fr") }
			}
			if err != nil {
				t.Fatal(err)
			}
			options, err := s.TranslationOptionsForDisplay(ctx, c.WorkspaceID, c.ID, "reader")
			if err != nil {
				t.Fatal(err)
			}
			if options.Conversation != before.Conversation {
				t.Fatalf("policy changed: before=%+v after=%+v", before.Conversation, options.Conversation)
			}
			switch scenario {
			case "uncertain", "provider-failure":
				if provider.calls == 0 || options.DetectedCustomerLanguage != "" {
					t.Fatalf("calls=%d language=%q", provider.calls, options.DetectedCustomerLanguage)
				}
				calls := provider.calls
				if _, err := s.TranslationOptionsForDisplay(ctx, c.WorkspaceID, c.ID, "reader"); err != nil {
					t.Fatal(err)
				}
				if provider.calls != calls {
					t.Fatal("reopening bypassed detection retry backoff")
				}
			case "paused-during-detection":
				if options.Conversation.TranslationMode != "off" || options.Conversation.CustomerLanguage != "fr" {
					t.Fatal("returned stale policy")
				}
			default:
				if provider.calls != 0 {
					t.Fatalf("unexpected detection calls=%d", provider.calls)
				}
			}
		})
	}
}
