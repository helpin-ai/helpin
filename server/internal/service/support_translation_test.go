package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/decision"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type translationTestProvider struct {
	calls int
	fail  bool
}

func (p *translationTestProvider) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.calls++
	if p.fail {
		return nil, errors.New("provider unavailable")
	}
	var input map[string]string
	if err := json.Unmarshal([]byte(req.Messages[0].Content), &input); err != nil {
		return nil, err
	}
	translated := strings.ReplaceAll(input["text"], "Hello", "Hallo")
	language := "en"
	if input["target_language"] == "en" {
		translated = strings.ReplaceAll(input["text"], "Hallo", "Hello")
		language = "de"
	}
	raw, err := json.Marshal(map[string]string{"source_language": language, "text": translated})
	if err != nil {
		return nil, err
	}
	return &llm.ChatResponse{Content: string(raw), Provider: req.Provider, Model: req.Model}, nil
}
func translationFixture(t *testing.T) (*emailFallbackTestEnv, *model.SupportConversation, *translationTestProvider) {
	t.Helper()
	env, conv := deliveryModeFixture(t)
	db := env.messageRepo.DB()
	if err := db.AutoMigrate(&model.SupportTranslation{}, &model.SupportTranslationPreference{}, &model.SupportTranslationConversation{}); err != nil {
		t.Fatal(err)
	}
	p := &translationTestProvider{}
	env.service.supportInboxService.SetTranslations(repository.NewSupportTranslationRepository(db), p, nil, AICompletionRoute{Provider: "openrouter", Model: "deepseek/deepseek-v4-flash-0731"}, true)
	return env, conv, p
}
func TestTranslationSendAutomaticallyPreservesDeliveryAndDeduplicates(t *testing.T) {
	for _, mode := range []string{"chat_only", "email_only", "chat_and_email"} {
		t.Run(mode, func(t *testing.T) {
			env, c, p := translationFixture(t)
			s := env.service.supportInboxService
			req := explicitDeliveryRequest(t, mode)
			req.Content = "Hello ORDER-123 https://example.com"
			req.AutoTranslate = true
			req.TranslationTargetLanguage = "de"
			req.ClientMessageID = uuid.NewString()
			actor := "22222222-2222-2222-2222-222222222222"
			first, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", &actor, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if first.Content != "Hallo ORDER-123 https://example.com" {
				t.Fatalf("sent content=%q", first.Content)
			}
			again, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", &actor, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if again.ID != first.ID || p.calls != 1 {
				t.Fatalf("duplicate send=%s/%s calls=%d", first.ID, again.ID, p.calls)
			}
			var artifact model.SupportTranslation
			if err := env.messageRepo.DB().Where("sent_message_id = ?", first.ID).First(&artifact).Error; err != nil {
				t.Fatal(err)
			}
			if artifact.SourceText != req.Content || artifact.ReviewStatus != "unavailable" {
				t.Fatalf("private artifact=%+v", artifact)
			}
			messages, err := s.ListWidgetConversationMessages(context.Background(), c.WorkspaceID, c.ID)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(messages)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "source_text") || strings.Contains(string(raw), "Hello ORDER") {
				t.Fatalf("private source leaked=%s", raw)
			}
			if mode == "email_only" && len(messages) != 0 {
				t.Fatal("email exposed in widget")
			}
			if mode != "chat_only" {
				ids, err := env.redis.LRange(context.Background(), env.service.msgListKey(c.ID), 0, -1).Result()
				if err != nil || len(ids) != 1 || ids[0] != first.ID {
					t.Fatalf("email queue=%v err=%v", ids, err)
				}
			}
		})
	}
}
func TestTranslationFailureNeverSendsOriginal(t *testing.T) {
	env, c, p := translationFixture(t)
	p.fail = true
	req := explicitDeliveryRequest(t, "chat_only")
	req.Content = "Hello"
	req.AutoTranslate = true
	req.TranslationTargetLanguage = "de"
	req.ClientMessageID = uuid.NewString()
	_, err := env.service.supportInboxService.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, nil)
	if err == nil {
		t.Fatal("failed provider allowed sending")
	}
	rows, err := env.messageRepo.ListByConversation(context.Background(), c.WorkspaceID, c.ID, true)
	if err != nil || len(rows) != 0 {
		t.Fatalf("unexpected messages=%v err=%v", rows, err)
	}
}
func TestTranslationIncomingCacheAndSourceRevision(t *testing.T) {
	env, c, p := translationFixture(t)
	s := env.service.supportInboxService
	ctx := context.Background()
	actor := "22222222-2222-2222-2222-222222222222"
	msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Hallo"}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatal(err)
	}
	req := model.SupportTranslateRequest{MessageID: msg.ID, TargetLanguage: "en"}
	first, err := s.TranslateSupport(ctx, c.WorkspaceID, c.ID, actor, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.TranslateSupport(ctx, c.WorkspaceID, c.ID, actor, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.TranslatedText != "Hello" || second.ID != first.ID || p.calls != 1 {
		t.Fatalf("cache failed: %+v calls=%d", second, p.calls)
	}
	if err := env.messageRepo.DB().Model(msg).Update("content", "Hallo again").Error; err != nil {
		t.Fatal(err)
	}
	third, err := s.TranslateSupport(ctx, c.WorkspaceID, c.ID, actor, req)
	if err != nil {
		t.Fatal(err)
	}
	if third.ID == first.ID || third.SourceText != "Hallo again" || p.calls != 2 {
		t.Fatalf("stale source=%+v", third)
	}
}
func TestTranslationRejectsPrivateForeignAndUnavailableInputs(t *testing.T) {
	for _, scenario := range []string{"internal", "foreign_workspace", "foreign_conversation", "deleted", "anonymized", "no_provider", "bad_language"} {
		t.Run(scenario, func(t *testing.T) {
			env, c, p := translationFixture(t)
			s := env.service.supportInboxService
			ctx := context.Background()
			msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Hallo"}
			switch scenario {
			case "internal":
				msg.IsInternal = true
			case "foreign_workspace":
				msg.WorkspaceID = uuid.NewString()
			case "foreign_conversation":
				msg.ConversationID = uuid.NewString()
			case "no_provider":
				s.translations.available = false
			}
			if err := env.messageRepo.Create(ctx, msg); err != nil {
				t.Fatal(err)
			}
			if scenario == "deleted" {
				if err := env.messageRepo.DB().Delete(msg).Error; err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "anonymized" {
				if err := env.messageRepo.DB().Model(c).Update("anonymized_at", "2026-09-18T00:00:00Z").Error; err != nil {
					t.Fatal(err)
				}
			}
			target := "en"
			if scenario == "bad_language" {
				target = "bogus"
			}
			_, err := s.TranslateSupport(ctx, c.WorkspaceID, c.ID, "22222222-2222-2222-2222-222222222222", model.SupportTranslateRequest{MessageID: msg.ID, TargetLanguage: target})
			if err == nil || p.calls != 0 {
				t.Fatalf("rejected input made call: err=%v calls=%d", err, p.calls)
			}
		})
	}
}
func TestTranslationProtectedTextRoundTrip(t *testing.T) {
	for _, source := range []string{"Pay 100.50 for ORDER-123 at https://example.com/a?id=1", "`foo(1)` and ```js\nconst i=2\n```", "HELPIN_KEEP_0_END HELPIN_LITERAL_KEEP_ user@example.com", "مرحبا 123"} {
		t.Run(source, func(t *testing.T) {
			masked, values := protectTranslationText(source)
			restored, err := restoreTranslationText(masked, values)
			if err != nil || restored != source {
				t.Fatalf("restored=%q err=%v", restored, err)
			}
			if len(values) > 0 {
				if _, err := restoreTranslationText(masked+" HELPIN_KEEP_0_END", values); err == nil {
					t.Fatal("duplicate token accepted")
				}
			}
		})
	}
}

func TestTranslationJevPrimaryBlocksAndShadowIsAdvisory(t *testing.T) {
	for _, tc := range []struct {
		name, mode, choice string
		wantError          bool
	}{
		{"accepted", "primary", "yes", false}, {"rejected", "primary", "no", true}, {"uncertain", "primary", "uncertain", true}, {"shadow", "shadow", "no", false}, {"off", "off", "no", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, c, _ := translationFixture(t)
			s := env.service.supportInboxService
			jev, p, _, jevDB := setupJevDecisionTest(t, tc.mode)
			if err := jevDB.Exec("INSERT INTO workspaces(id) VALUES (?)", c.WorkspaceID).Error; err != nil {
				t.Fatal(err)
			}
			jev.policies[JevTranslationReview] = decision.Policy{Mode: tc.mode, Threshold: .95, DailyLimit: 10}
			jev.workspaces = map[string]bool{c.WorkspaceID: true}
			p.choices = map[string]string{"meaning": tc.choice}
			s.translations.jev = jev
			req := explicitDeliveryRequest(t, "chat_only")
			req.Content = "Hello"
			req.AutoTranslate = true
			req.TranslationTargetLanguage = "de"
			req.ClientMessageID = uuid.NewString()
			_, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, nil)
			if (err != nil) != tc.wantError {
				t.Fatalf("send err=%v wantError=%v", err, tc.wantError)
			}
		})
	}
}

func TestTranslationDetectsCustomerLanguageOnSend(t *testing.T) {
	env, c, p := translationFixture(t)
	s := env.service.supportInboxService
	ctx := context.Background()
	msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Hallo"}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatal(err)
	}
	req := explicitDeliveryRequest(t, "chat_only")
	req.Content = "Hello"
	req.AutoTranslate = true
	req.ClientMessageID = uuid.NewString()
	sent, err := s.CreateConversationMessage(ctx, c.WorkspaceID, c.ID, req, "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Content != "Hallo" || p.calls != 2 {
		t.Fatalf("language detection send=%q calls=%d", sent.Content, p.calls)
	}
}

func TestTranslationRetriesUnavailableJevReviewAfterCooldown(t *testing.T) {
	env, c, p := translationFixture(t)
	s := env.service.supportInboxService
	ctx := context.Background()
	jev, reviewer, _, db := setupJevDecisionTest(t, "primary")
	if err := db.Exec("INSERT INTO workspaces(id) VALUES (?)", c.WorkspaceID).Error; err != nil {
		t.Fatal(err)
	}
	jev.policies[JevTranslationReview] = decision.Policy{Mode: "primary", Threshold: .95, DailyLimit: 10}
	jev.workspaces = map[string]bool{c.WorkspaceID: true}
	s.translations.jev = jev
	reviewer.err = errors.New("temporary failure")
	req := explicitDeliveryRequest(t, "chat_only")
	req.Content = "Hello"
	req.AutoTranslate = true
	req.TranslationTargetLanguage = "de"
	req.ClientMessageID = uuid.NewString()
	actor := strPtr("22222222-2222-2222-2222-222222222222")
	if _, err := s.CreateConversationMessage(ctx, c.WorkspaceID, c.ID, req, "user", actor, nil, nil); err == nil {
		t.Fatal("failed Jev allowed send")
	}
	if err := env.messageRepo.DB().Model(&model.SupportTranslation{}).Where("conversation_id = ?", c.ID).Update("updated_at", time.Now().Add(-2*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	reviewer.err = nil
	sent, err := s.CreateConversationMessage(ctx, c.WorkspaceID, c.ID, req, "user", actor, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Content != "Hallo" || reviewer.calls != 2 || p.calls != 2 {
		t.Fatalf("retry result=%q reviewCalls=%d calls=%d", sent.Content, reviewer.calls, p.calls)
	}
}
