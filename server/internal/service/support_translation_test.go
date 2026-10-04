package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/decision"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type translationTestProvider struct {
	calls            int
	fail             bool
	failure          error
	onCall           func()
	responseLanguage string
	responseText     string
}

func (p *translationTestProvider) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.calls++
	if p.onCall != nil {
		p.onCall()
	}
	if p.failure != nil {
		return nil, p.failure
	}
	if p.fail {
		return nil, errors.New("provider unavailable")
	}
	var input map[string]string
	if err := json.Unmarshal([]byte(req.Messages[0].Content), &input); err != nil {
		return nil, err
	}
	translated := strings.ReplaceAll(input["text"], "Hello", "Hallo")
	language := "en"
	if input["target_language"] == "en" || (input["target_language"] == "" && strings.Contains(input["text"], "Hallo")) {
		translated = strings.ReplaceAll(input["text"], "Hallo", "Hello")
		language = "de"
	}
	if p.responseLanguage != "" {
		language = p.responseLanguage
		translated = p.responseText
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
	setTranslationWorkspaceSettings(t, env, conv.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationCustomerLanguage = "de" })
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
			if artifact.SourceText != req.Content || artifact.ReviewStatus != "not_requested" {
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

func TestTranslatedPortalReplyUsesPortalSurfaceWithoutWidgetSession(t *testing.T) {
	env, conv, _ := translationFixture(t)
	conv.Source, conv.Channel, conv.PortalVisible, conv.AnonymousID = "portal", "portal", true, nil
	if err := env.convRepo.Update(context.Background(), conv); err != nil {
		t.Fatal(err)
	}
	req := explicitDeliveryRequest(t, "chat_only")
	req.Content = "Hello, we are looking into it."
	req.AutoTranslate = true
	req.TranslationTargetLanguage = "de"
	req.ClientMessageID = uuid.NewString()
	actor := "22222222-2222-2222-2222-222222222222"
	msg, err := env.service.supportInboxService.CreateConversationMessage(context.Background(), conv.WorkspaceID, conv.ID, req, "user", &actor, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || !portalMessageVisible(msg) {
		t.Fatalf("portal message=%+v", msg)
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
	for _, scenario := range []string{"internal", "foreign_workspace", "foreign_conversation", "deleted", "anonymized", "no_provider"} {
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

func TestTranslationNeverUsesJevAsSendGate(t *testing.T) {
	for _, tc := range []struct {
		name, mode, choice string
		wantError          bool
	}{
		{"accepted", "primary", "yes", false}, {"rejected", "primary", "no", false}, {"uncertain", "primary", "uncertain", false}, {"shadow", "shadow", "no", false}, {"off", "off", "no", false},
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
	setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) {
		settings.TranslationCustomerLanguage = ""
		settings.TranslationIncomingEnabled = true
	})
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

func TestTranslationUnavailableJevDoesNotBlockOrDuplicateSend(t *testing.T) {
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
	if _, err := s.CreateConversationMessage(ctx, c.WorkspaceID, c.ID, req, "user", actor, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := env.messageRepo.DB().Model(&model.SupportTranslation{}).Where("conversation_id = ?", c.ID).Update("updated_at", time.Now().Add(-2*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	reviewer.err = nil
	sent, err := s.CreateConversationMessage(ctx, c.WorkspaceID, c.ID, req, "user", actor, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Content != "Hallo" || reviewer.calls != 0 || p.calls != 1 {
		t.Fatalf("retry result=%q reviewCalls=%d calls=%d", sent.Content, reviewer.calls, p.calls)
	}
}

func setTranslationWorkspaceSettings(t *testing.T, env *emailFallbackTestEnv, workspaceID string, change func(*model.SupportInboxSettings)) {
	t.Helper()
	var installation model.SupportWidgetInstallation
	db := env.messageRepo.DB()
	if err := db.Where("workspace_id = ?", workspaceID).First(&installation).Error; err != nil {
		t.Fatal(err)
	}
	settings := parseSettings(installation.Settings)
	change(&settings)
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&installation).Update("settings", string(raw)).Error; err != nil {
		t.Fatal(err)
	}
}

func TestTranslationPreservesConversationOverrides(t *testing.T) {
	env, c, _ := translationFixture(t)
	db := env.messageRepo.DB()
	actor := "22222222-2222-2222-2222-222222222222"
	preference := model.SupportTranslationPreference{WorkspaceID: c.WorkspaceID, UserID: actor, ReadingLanguage: "fr", AutoTranslateIncoming: false, AutoTranslateOutgoing: false}
	conversation := model.SupportTranslationConversation{WorkspaceID: c.WorkspaceID, ConversationID: c.ID, CustomerLanguage: "es", TranslationMode: "off"}
	if err := db.Create(&preference).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	for _, userID := range []string{actor, uuid.NewString()} {
		options, err := env.service.supportInboxService.TranslationOptions(context.Background(), c.WorkspaceID, c.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		if !options.Available || options.Preference.AutoTranslateIncoming || options.Preference.AutoTranslateOutgoing || options.Preference.ReadingLanguage != "en" || options.Conversation.CustomerLanguage != "es" || options.Conversation.TranslationMode != "off" {
			t.Fatalf("workspace policy not applied: %+v", options)
		}
	}
}

func TestTranslationSendUsesWorkspacePolicy(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		master, outgoing, provider, internal bool
		want                                 string
		calls                                int
	}{
		{"enabled ignores client opt out", true, true, true, false, "Hallo", 1},
		{"master disabled", false, true, true, false, "Hello", 0},
		{"outgoing disabled", true, false, true, false, "Hello", 0},
		{"provider missing", true, true, false, false, "Hello", 0},
		{"internal note", true, true, true, true, "Hello", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, c, p := translationFixture(t)
			s := env.service.supportInboxService
			s.translations.available = tc.provider
			setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) {
				settings.TranslationEnabled = tc.master
				settings.TranslationOutgoingEnabled = tc.outgoing
			})
			req := explicitDeliveryRequest(t, "chat_only")
			req.Content = "Hello"
			req.IsInternal = tc.internal
			if tc.internal {
				req.DeliveryMode = ""
				req.Channels = nil
			}
			req.AutoTranslate = !tc.master || !tc.outgoing
			req.TranslationTargetLanguage = "fr"
			sent, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, nil)
			if !tc.provider && tc.master && tc.outgoing && !tc.internal {
				if err == nil || sent != nil || p.calls != 0 {
					t.Fatal("provider outage silently sent original")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if sent.Content != tc.want || p.calls != tc.calls {
				t.Fatalf("content=%q calls=%d", sent.Content, p.calls)
			}
		})
	}
}

// A server without a translation provider (a Community install with no
// OpenRouter key) must never block teammate replies; only an outage of a
// configured provider does.
func TestTranslationUnconfiguredServerSendsReplies(t *testing.T) {
	env, c, p := translationFixture(t)
	s := env.service.supportInboxService
	s.translations.available = false
	s.SetTranslationProviderConfigured(false)
	setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) {
		settings.TranslationEnabled = true
		settings.TranslationOutgoingEnabled = true
	})

	options, err := s.TranslationOptions(context.Background(), c.WorkspaceID, c.ID, "22222222-2222-2222-2222-222222222222")
	if err != nil {
		t.Fatalf("TranslationOptions: %v", err)
	}
	if options.Available || options.Preference.AutoTranslateOutgoing || options.Conversation.TranslationMode != "off" || !strings.Contains(options.UnavailableReason, "isn’t set up") {
		t.Fatalf("unconfigured options = %+v", options)
	}

	req := explicitDeliveryRequest(t, "chat_only")
	req.Content = "Hello"
	sent, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, nil)
	if err != nil {
		t.Fatalf("reply blocked on a server without translation: %v", err)
	}
	if sent.Content != "Hello" || p.calls != 0 {
		t.Fatalf("content=%q calls=%d", sent.Content, p.calls)
	}
}

// queuedReplyMembers makes every requested user an active workspace member.
type queuedReplyMembers struct{}

func (queuedReplyMembers) GetMembership(_ context.Context, _, user string) (*authorization.MemberInfo, error) {
	return &authorization.MemberInfo{ID: user, Status: "active", Role: "member"}, nil
}

func (queuedReplyMembers) GetTeamMemberships(context.Context, string) ([]authorization.TeamRole, error) {
	return nil, nil
}

// The dashboard queues replies and a worker delivers them behind a fence on
// the conversation's stored translation policy revision. Without a
// translation provider the queued reply must still carry that revision, or
// every teammate reply on a Community install fails as "Not sent".
func TestTranslationUnconfiguredServerDeliversQueuedReply(t *testing.T) {
	env, c, p := translationFixture(t)
	s := env.service.supportInboxService
	ctx := context.Background()
	user := "22222222-2222-2222-2222-222222222222"
	if err := env.messageRepo.DB().AutoMigrate(&model.SupportPendingSend{}); err != nil {
		t.Fatal(err)
	}
	// The visitor's first message stores the conversation policy at revision 1.
	if _, err := s.translations.repo.LiveConversation(ctx, c.WorkspaceID, c.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	s.translations.available = false
	s.SetTranslationProviderConfigured(false)
	// The delivery fence re-checks the replying teammate's permissions.
	s.SetAuthzService(authorization.NewAuthzService(env.messageRepo.DB(), queuedReplyMembers{}, nil))

	req := explicitDeliveryRequest(t, "chat_only")
	req.Content = "Reply without a translation provider"
	req.ClientMessageID = "87654321-4321-4321-4321-210987654321"
	if _, err := s.QueueSupportSend(ctx, c.WorkspaceID, c.ID, user, req); err != nil {
		t.Fatalf("queue reply: %v", err)
	}
	var job model.SupportPendingSend
	db := env.messageRepo.DB()
	if err := db.Where("conversation_id = ? AND user_id = ?", c.ID, user).First(&job).Error; err != nil {
		t.Fatal(err)
	}
	if job.Revision != 1 {
		t.Fatalf("queued reply revision = %d, want the stored policy revision 1", job.Revision)
	}
	// Deliver it as the send worker does, through the revision fence.
	job.Status, job.Attempts = "sending", 1
	if err := db.Model(&job).Updates(map[string]any{"status": job.Status, "attempts": job.Attempts}).Error; err != nil {
		t.Fatal(err)
	}
	options, err := s.translationOptions(ctx, c.WorkspaceID, c.ID, user, true)
	if err != nil || options.Conversation.Revision != job.Revision {
		t.Fatalf("worker options = %+v, err = %v", options, err)
	}
	work := context.WithValue(ctx, supportSendPolicyKey{}, supportSendPolicySnapshot{job.WorkspaceID, job.ConversationID, job.UserID, options})
	work = context.WithValue(work, supportSendGuardKey{}, &job)
	sent, err := s.CreateConversationMessage(work, c.WorkspaceID, c.ID, req, "user", &job.UserID, nil, nil)
	if err != nil {
		t.Fatalf("queued reply blocked on a server without translation: %v", err)
	}
	if sent.Content != req.Content || p.calls != 0 {
		t.Fatalf("content=%q calls=%d", sent.Content, p.calls)
	}
}

func TestTranslationIncomingUsesWorkspaceLanguage(t *testing.T) {
	env, c, p := translationFixture(t)
	s := env.service.supportInboxService
	msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Hallo"}
	if err := env.messageRepo.Create(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	req := model.SupportTranslateRequest{MessageID: msg.ID, TargetLanguage: "fr"}
	result, err := s.TranslateSupport(context.Background(), c.WorkspaceID, c.ID, "agent", req)
	if err != nil {
		t.Fatal(err)
	}
	if result.TargetLanguage != "en" || result.TranslatedText != "Hello" {
		t.Fatalf("client language overrode workspace: %+v", result)
	}
	setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationIncomingEnabled = true })
	if _, err := s.TranslateSupport(context.Background(), c.WorkspaceID, c.ID, "agent", req); err != nil || p.calls != 1 {
		t.Fatalf("manual translation unavailable: err=%v calls=%d", err, p.calls)
	}
}

func TestTranslationWorkspacePolicyRevalidatedAfterGeneration(t *testing.T) {
	for _, change := range []string{"disable", "outgoing", "language"} {
		t.Run(change, func(t *testing.T) {
			env, c, p := translationFixture(t)
			p.onCall = func() {
				enabled, language := false, "de"
				if change == "language" {
					enabled = true
					language = "fr"
				}
				if _, err := env.service.supportInboxService.SetLiveTranslate(context.Background(), c.WorkspaceID, c.ID, enabled, language); err != nil {
					t.Fatal(err)
				}
			}
			req := explicitDeliveryRequest(t, "chat_only")
			req.Content = "Hello"
			if _, err := env.service.supportInboxService.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, nil); err == nil {
				t.Fatal("changed policy allowed send")
			}
			var count int64
			if err := env.messageRepo.DB().Model(&model.SupportMessage{}).Where("conversation_id = ?", c.ID).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("sent %d messages after policy changed", count)
			}
		})
	}
}

func TestTranslationSameLanguagePreservesOriginalWithoutJev(t *testing.T) {
	for _, responseText := range []string{"", "A model rewrite must never replace the original."} {
		t.Run(responseText, func(t *testing.T) {
			env, c, p := translationFixture(t)
			s := env.service.supportInboxService
			p.responseLanguage = "en"
			p.responseText = responseText
			setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationCustomerLanguage = "en" })
			jev, reviewer, _, jevDB := setupJevDecisionTest(t, "primary")
			if err := jevDB.Exec("INSERT INTO workspaces(id) VALUES (?)", c.WorkspaceID).Error; err != nil {
				t.Fatal(err)
			}
			jev.policies[JevTranslationReview] = decision.Policy{Mode: "primary", Threshold: .95, DailyLimit: 10}
			jev.workspaces = map[string]bool{c.WorkspaceID: true}
			reviewer.choices = map[string]string{"meaning": "no"}
			reviewer.err = errors.New("review provider unavailable")
			s.translations.jev = jev
			req := explicitDeliveryRequest(t, "chat_only")
			req.Content = "Hello, please check https://example.com/reset for ORDER-123."
			req.ClientMessageID = uuid.NewString()
			actor := strPtr("22222222-2222-2222-2222-222222222222")
			sent, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", actor, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if sent.Content != req.Content || reviewer.calls != 0 {
				t.Fatalf("unchanged reply=%q review calls=%d", sent.Content, reviewer.calls)
			}
			retry, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", actor, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if retry.ID != sent.ID || p.calls != 1 {
				t.Fatalf("duplicate same language send: %s/%s calls=%d", sent.ID, retry.ID, p.calls)
			}
		})
	}
}

func TestTranslationSameLanguageIncomingPreservesOriginal(t *testing.T) {
	env, c, p := translationFixture(t)
	p.responseLanguage = "en"
	msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Hello, I cannot log in to my account."}
	if err := env.messageRepo.Create(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	result, err := env.service.supportInboxService.TranslateSupport(context.Background(), c.WorkspaceID, c.ID, "agent", model.SupportTranslateRequest{MessageID: msg.ID, TargetLanguage: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ready" || result.TranslatedText != msg.Content || result.ReviewStatus != "not_requested" {
		t.Fatalf("same language=%+v", result)
	}
}

func TestTranslationWorkspaceSettingsValidateAndPersist(t *testing.T) {
	env, c, _ := translationFixture(t)
	s := env.service.supportInboxService
	disabled := false
	language := "fr"
	auto := ""
	_, settings, err := s.UpdateInstallationSettings(context.Background(), c.WorkspaceID, model.UpdateInstallationSettingsRequest{TranslationIncomingEnabled: &disabled, TranslationOutgoingEnabled: &disabled, DefaultAgentLanguage: &language, TranslationCustomerLanguage: &auto})
	if err != nil {
		t.Fatal(err)
	}
	if settings.TranslationIncomingEnabled || settings.TranslationOutgoingEnabled || settings.DefaultAgentLanguage != "fr" || settings.TranslationCustomerLanguage != "" {
		t.Fatalf("settings not saved: %+v", settings)
	}
	options, err := s.TranslationOptions(context.Background(), c.WorkspaceID, c.ID, "agent")
	if err != nil {
		t.Fatal(err)
	}
	if options.Preference.AutoTranslateIncoming || options.Preference.AutoTranslateOutgoing || options.Preference.ReadingLanguage != "fr" || options.Conversation.CustomerLanguage != "" {
		t.Fatalf("settings not applied: %+v", options)
	}
	invalid := "bogus"
	for _, req := range []model.UpdateInstallationSettingsRequest{{DefaultAgentLanguage: &invalid}, {TranslationCustomerLanguage: &invalid}} {
		if _, _, err := s.UpdateInstallationSettings(context.Background(), c.WorkspaceID, req); err == nil {
			t.Fatal("invalid language accepted")
		}
	}
}

func TestTranslationRetryAfterPipelineUpgrade(t *testing.T) {
	env, c, p := translationFixture(t)
	s := env.service.supportInboxService
	setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationCustomerLanguage = "en" })
	p.fail = true
	req := explicitDeliveryRequest(t, "chat_only")
	req.Content = "Hello, please check your inbox."
	req.ClientMessageID = uuid.NewString()
	actor := strPtr("22222222-2222-2222-2222-222222222222")
	if _, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", actor, nil, nil); err == nil {
		t.Fatal("failed generation sent a reply")
	}
	db := env.messageRepo.DB()
	if err := db.Model(&model.SupportTranslation{}).Where("conversation_id = ?", c.ID).Updates(map[string]any{"pipeline_version": "v1", "cache_key": "old-cache"}).Error; err != nil {
		t.Fatal(err)
	}
	p.fail = false
	p.responseLanguage = "en"
	sent, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", actor, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Content != req.Content || p.calls != 3 {
		t.Fatalf("retry content=%q calls=%d", sent.Content, p.calls)
	}
	if err := db.Model(&model.SupportTranslation{}).Where("conversation_id = ?", c.ID).Updates(map[string]any{"pipeline_version": "v1", "cache_key": "sent-old-cache"}).Error; err != nil {
		t.Fatal(err)
	}
	again, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", actor, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != sent.ID || p.calls != 3 {
		t.Fatalf("upgrade duplicated send: %s/%s calls=%d", sent.ID, again.ID, p.calls)
	}
}

func TestTranslationDetectionRecoversAfterRepeatedProviderFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		failure   error
		errorCode string
	}{
		{"provider unavailable", errors.New("provider unavailable"), "detection_unavailable"},
		{"provider timeout", context.DeadlineExceeded, "detection_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, c, p := translationFixture(t)
			setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) {
				settings.TranslationCustomerLanguage = ""
				settings.TranslationIncomingEnabled = true
			})
			s := env.service.supportInboxService
			ctx := context.Background()
			db := env.messageRepo.DB()
			msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Hallo"}
			if err := env.messageRepo.Create(ctx, msg); err != nil {
				t.Fatal(err)
			}
			req := explicitDeliveryRequest(t, "chat_only")
			req.Content = "Hello"
			req.ClientMessageID = uuid.NewString()
			actor := strPtr("22222222-2222-2222-2222-222222222222")
			p.failure = tc.failure
			var artifact model.SupportTranslation
			for attempt := 1; attempt <= 3; attempt++ {
				req.ClientMessageID = uuid.NewString()
				sent, err := s.CreateConversationMessage(ctx, c.WorkspaceID, c.ID, req, "user", actor, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if sent.Content != req.Content || sent.TranslationID != "" {
					t.Fatal("detection failure did not preserve original")
				}
				if err := db.Where("source_message_id = ?", msg.ID).First(&artifact).Error; err != nil {
					t.Fatal(err)
				}
				if artifact.Attempts != attempt || artifact.ErrorCode != tc.errorCode {
					t.Fatalf("attempt=%d code=%s", artifact.Attempts, artifact.ErrorCode)
				}
				if err := db.Model(&artifact).Update("updated_at", time.Now().Add(-2*time.Minute)).Error; err != nil {
					t.Fatal(err)
				}
			}
			p.failure = nil
			req.ClientMessageID = uuid.NewString()
			if sent, err := s.CreateConversationMessage(ctx, c.WorkspaceID, c.ID, req, "user", actor, nil, nil); err != nil || sent.Content != req.Content {
				t.Fatalf("cooldown blocked original: %v", err)
			}
			if p.calls != 6 {
				t.Fatalf("cooldown called provider: %d", p.calls)
			}
			rows, err := env.messageRepo.ListByConversation(ctx, c.WorkspaceID, c.ID, true)
			if err != nil {
				t.Fatal(err)
			}
			originalReplies := 0
			for _, row := range rows {
				if row.SenderType == "user" && row.MessageType == "reply" && row.Content == req.Content {
					originalReplies++
				}
			}
			if originalReplies != 4 {
				t.Fatalf("original replies=%d, want 4", originalReplies)
			}
			if err := db.Model(&artifact).Update("updated_at", time.Now().Add(-16*time.Minute)).Error; err != nil {
				t.Fatal(err)
			}
			req.ClientMessageID = uuid.NewString()
			sent, err := s.CreateConversationMessage(ctx, c.WorkspaceID, c.ID, req, "user", actor, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if sent.Content != "Hallo" || p.calls != 8 {
				t.Fatalf("recovery content=%q calls=%d", sent.Content, p.calls)
			}
			if err := db.First(&artifact, "id = ?", artifact.ID).Error; err != nil {
				t.Fatal(err)
			}
			if artifact.Attempts != 4 || artifact.Status != "ready" || artifact.ErrorCode != "" {
				t.Fatalf("recovered artifact: attempts=%d status=%s code=%s", artifact.Attempts, artifact.Status, artifact.ErrorCode)
			}
			again, err := s.CreateConversationMessage(ctx, c.WorkspaceID, c.ID, req, "user", actor, nil, nil)
			if err != nil || again.ID != sent.ID || p.calls != 8 {
				t.Fatalf("recovered send not deduplicated: err=%v calls=%d", err, p.calls)
			}
		})
	}
}

func TestTranslationSendWithoutCustomerEvidencePreservesDraft(t *testing.T) {
	for _, mode := range []string{"chat_only", "email_only", "chat_and_email"} {
		t.Run(mode, func(t *testing.T) {
			env, conv, provider := translationFixture(t)
			setTranslationWorkspaceSettings(t, env, conv.WorkspaceID, func(settings *model.SupportInboxSettings) {
				settings.TranslationCustomerLanguage = ""
				settings.TranslationOutgoingEnabled = true
			})
			provider.fail = true
			req := explicitDeliveryRequest(t, mode)
			req.Content = "Bonjour, voici votre mise à jour."
			sent, err := env.service.supportInboxService.CreateConversationMessage(context.Background(), conv.WorkspaceID, conv.ID, req, "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if sent.Content != req.Content || sent.TranslationID != "" || provider.calls != 0 {
				t.Fatalf("no-evidence send changed draft: %+v calls=%d", sent, provider.calls)
			}
		})
	}
}

func TestTranslationSendOriginalBypassesProviderAndDeduplicates(t *testing.T) {
	for _, mode := range []string{"chat_only", "email_only", "chat_and_email"} {
		t.Run(mode, func(t *testing.T) {
			env, c, provider := translationFixture(t)
			provider.fail = true
			req := explicitDeliveryRequest(t, mode)
			req.Content = "Hello — keep my original reply."
			req.ClientMessageID = uuid.NewString()
			actor := "22222222-2222-2222-2222-222222222222"
			s := env.service.supportInboxService
			if _, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", &actor, nil, nil); err == nil {
				t.Fatal("expected translation failure")
			}
			calls := provider.calls
			req.SendOriginal = true
			first, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", &actor, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			second, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", &actor, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if first.Content != req.Content || first.ID != second.ID || provider.calls != calls {
				t.Fatalf("original send changed or duplicated: %q %s/%s calls=%d", first.Content, first.ID, second.ID, provider.calls)
			}
			req.SendOriginal = false
			again, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", &actor, nil, nil)
			if err != nil || again.ID != first.ID || provider.calls != calls {
				t.Fatalf("translated retry after original duplicated send: %v", err)
			}
		})
	}
}

func TestTranslationLongEnglishEmailUsesLLMAndPreservesOriginal(t *testing.T) {
	env, c, provider := translationFixture(t)
	setTranslationWorkspaceSettings(t, env, c.WorkspaceID, func(settings *model.SupportInboxSettings) { settings.TranslationCustomerLanguage = "" })
	jev, classifier, _, db := setupJevDecisionTest(t, "primary")
	if err := db.Exec("INSERT INTO workspaces(id) VALUES (?)", c.WorkspaceID).Error; err != nil {
		t.Fatal(err)
	}
	jev.workspaces = map[string]bool{c.WorkspaceID: true}
	jev.policies[JevLanguageDetection] = decision.Policy{Mode: "primary", Threshold: .95, DailyLimit: 100}
	classifier.choices["language"] = "en"
	s := env.service.supportInboxService
	s.translations.jev = jev
	provider.responseLanguage = "en"
	msg := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: strings.Repeat("Our Instagram posts are failing. Please help.\n", 220)}
	if err := env.messageRepo.Create(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	req := explicitDeliveryRequest(t, "chat_only")
	req.Content = strings.Repeat("Thanks, we are investigating this issue. ", 250) + "We will update you."
	req.ClientMessageID = uuid.NewString()
	actor := "22222222-2222-2222-2222-222222222222"
	sent, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", &actor, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Content != req.Content || sent.TranslationID != "" || provider.calls == 0 || classifier.calls != 0 {
		t.Fatalf("same-language send translated: %q calls=%d", sent.Content, provider.calls)
	}
	options, err := s.TranslationOptions(context.Background(), c.WorkspaceID, c.ID, actor)
	if err != nil || options.DetectedCustomerLanguage != "en" {
		t.Fatalf("detection was not cached: %+v %v", options, err)
	}
}

func TestTranslationLongMessagePreservesAllContent(t *testing.T) {
	env, c, provider := translationFixture(t)
	s := env.service.supportInboxService
	original := strings.Repeat("Hello, please check my account.\n", 400)
	artifact, err := s.TranslateSupport(context.Background(), c.WorkspaceID, c.ID, "22222222-2222-2222-2222-222222222222", model.SupportTranslateRequest{Content: original, DraftID: uuid.NewString(), TargetLanguage: "de"})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Status != "ready" || artifact.TranslatedText != strings.ReplaceAll(strings.TrimSpace(original), "Hello", "Hallo") {
		t.Fatalf("long translation incomplete: status=%s bytes=%d", artifact.Status, len(artifact.TranslatedText))
	}
	if provider.calls < 2 {
		t.Fatal("expected bounded chunks")
	}
}

func TestTranslationLongReplyDoesNotCallJev(t *testing.T) {
	env, c, _ := translationFixture(t)
	jev, reviewer, _, db := setupJevDecisionTest(t, "primary")
	if err := db.Exec("INSERT INTO workspaces(id) VALUES (?)", c.WorkspaceID).Error; err != nil {
		t.Fatal(err)
	}
	jev.workspaces = map[string]bool{c.WorkspaceID: true}
	jev.policies[JevTranslationReview] = decision.Policy{Mode: "primary", Threshold: .95, DailyLimit: 100}
	s := env.service.supportInboxService
	s.translations.jev = jev
	req := explicitDeliveryRequest(t, "email_only")
	req.Content = strings.Repeat("Hello, please check the account details.\n", 300)
	req.ClientMessageID = uuid.NewString()
	sent, err := s.CreateConversationMessage(context.Background(), c.WorkspaceID, c.ID, req, "user", strPtr("22222222-2222-2222-2222-222222222222"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Content != strings.TrimSpace(strings.ReplaceAll(req.Content, "Hello", "Hallo")) || reviewer.calls != 0 {
		t.Fatalf("long reply/review incomplete: calls=%d", reviewer.calls)
	}
	for _, state := range reviewer.states {
		if len(state) > 16000 {
			t.Fatal("review exceeded Jev input limit")
		}
	}
}

func TestTranslationLanguageHintIgnoresOlderAndDeletedMessages(t *testing.T) {
	env, c, _ := translationFixture(t)
	ctx := context.Background()
	repo := env.service.supportInboxService.translations.repo
	db := env.messageRepo.DB()
	old := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Hello", CreatedAt: time.Now().Add(-time.Hour)}
	if err := env.messageRepo.Create(ctx, old); err != nil {
		t.Fatal(err)
	}
	artifact := &model.SupportTranslation{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, Purpose: "language_detection", SourceMessageID: &old.ID, SourceText: old.Content, SourceLanguage: "en", Status: "ready", CacheKey: uuid.NewString()}
	if err := db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	newer := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: c.WorkspaceID, ConversationID: c.ID, SenderType: "customer", MessageType: "reply", Content: "Hallo", CreatedAt: time.Now()}
	if err := env.messageRepo.Create(ctx, newer); err != nil {
		t.Fatal(err)
	}
	language, err := repo.DetectedLanguage(ctx, c.WorkspaceID, c.ID)
	if err != nil || language != "" {
		t.Fatalf("stale hint=%q %v", language, err)
	}
	if err := db.Model(newer).Update("deleted_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	latest, err := repo.LatestCustomerMessageID(ctx, c.WorkspaceID, c.ID)
	if err != nil || latest != old.ID {
		t.Fatalf("deleted message used for detection: %s %v", latest, err)
	}
}

func TestTranslationChunksPreserveUnicodeAndProtectedText(t *testing.T) {
	original := strings.Repeat("你好 Привет Hello\n", 500) + "https://example.com/" + strings.Repeat("x", 5000) + "\nThanks"
	chunks := translationChunks(original)
	if strings.Join(chunks, "") != original {
		t.Fatal("chunks changed original text")
	}
	for _, chunk := range chunks {
		if !utf8.ValidString(chunk) {
			t.Fatal("split UTF-8 sequence")
		}
	}
	url := "https://example.com/" + strings.Repeat("x", 5000)
	found := false
	for _, chunk := range chunks {
		if strings.Contains(chunk, url) {
			found = true
		}
	}
	if !found {
		t.Fatal("split protected URL between chunks")
	}
}
