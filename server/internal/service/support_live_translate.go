package service

import (
	"context"
	"encoding/json"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"log/slog"
	"strings"
	"time"
)

func normalizeLiveLanguage(value string) string {
	value = normalizeSupportLanguage(value)
	if strings.HasPrefix(value, "zh") {
		if strings.Contains(value, "hant") || strings.Contains(value, "tw") || strings.Contains(value, "hk") {
			return "zh-TW"
		}
		return "zh-CN"
	}
	if value == "pt-br" {
		return "pt-BR"
	}
	base := strings.Split(value, "-")[0]
	if supportTranslationLanguages[base] != "" {
		return base
	}
	return ""
}
func meaningfulLanguageText(value string) bool {
	return model.MeaningfulSupportLanguageText(languageEvidence(value))
}
func (s *SupportInboxService) SetLiveTranslate(ctx context.Context, workspaceID, conversationID string, enabled bool, language string) (*model.SupportTranslationOptions, error) {
	if s.translations == nil {
		return nil, ErrSupportTranslation
	}
	if language != "" && normalizeLiveLanguage(language) == "" {
		return nil, ErrSupportTranslation
	}
	if _, err := s.TranslationOptions(ctx, workspaceID, conversationID, "settings"); err != nil {
		return nil, err
	}
	if err := s.translations.repo.SetLiveConversation(ctx, workspaceID, conversationID, enabled, normalizeLiveLanguage(language)); err != nil {
		return nil, err
	}
	s.publishLiveTranslate(workspaceID, conversationID)
	return s.TranslationOptions(ctx, workspaceID, conversationID, "settings")
}
func (s *SupportInboxService) CachedLiveTranslations(ctx context.Context, workspaceID, conversationID, userID string, ids []string) ([]model.SupportTranslation, error) {
	if len(ids) > 100 {
		return nil, ErrSupportTranslation
	}
	options, err := s.TranslationOptions(ctx, workspaceID, conversationID, userID)
	if err != nil {
		return nil, err
	}
	return s.translations.repo.CachedMessages(ctx, workspaceID, conversationID, options.Preference.ReadingLanguage, ids)
}
func (s *SupportInboxService) publishLiveTranslate(workspaceID, conversationID string) {
	if s.wsPublisher == nil {
		return
	}
	data, _ := json.Marshal(map[string]string{"conversation_id": conversationID})
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "support_translation", EntityID: conversationID, WorkspaceID: workspaceID, ParentType: "support_conversation", ParentID: conversationID, Data: data})
}

// RunLiveTranslate drains durable incoming work. Shutdown cancels all provider calls.
func (s *SupportInboxService) RunLiveTranslate(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.translations == nil || !s.translations.available {
				continue
			}
			for i := 0; i < 8; i++ {
				if ctx.Err() != nil {
					return
				}
				job, err := s.translations.repo.ClaimLiveMessage(ctx)
				if err != nil {
					slog.WarnContext(ctx, "claim live translation failed")
					break
				}
				if job == nil {
					break
				}
				status := "ready"
				if err := s.processLiveMessage(ctx, job); err != nil {
					status = "failed"
				}
				finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
				if err := s.translations.repo.FinishLiveMessage(finishCtx, job, status); err != nil {
					slog.WarnContext(ctx, "settle live translation failed")
				}
				cancel()
				s.publishLiveTranslate(job.WorkspaceID, job.ConversationID)
			}
		}
	}
}
func (s *SupportInboxService) processLiveMessage(ctx context.Context, job *model.SupportLiveMessage) error {
	options, err := s.TranslationOptions(ctx, job.WorkspaceID, job.ConversationID, "live-worker")
	if err != nil {
		return err
	}
	msg, err := s.messageRepo.GetByID(ctx, job.MessageID)
	if err != nil {
		return err
	}
	if msg == nil || msg.DeletedAt.Valid || (job.SourceHash != "" && translationHash(msg.Content) != job.SourceHash) {
		return nil
	}
	if !job.Enabled || !options.Preference.AutoTranslateIncoming || options.Conversation.Revision != job.Revision || options.Preference.ReadingLanguage != job.TargetLanguage {
		if meaningfulLanguageText(msg.Content) {
			_, err = s.detectCustomerLanguage(ctx, job.WorkspaceID, job.ConversationID, msg)
		}
		return err
	}
	result, err := s.TranslateSupport(ctx, job.WorkspaceID, job.ConversationID, "live-worker", model.SupportTranslateRequest{MessageID: job.MessageID, TargetLanguage: job.TargetLanguage, Live: true})
	if err == nil && (result == nil || result.Status != "ready") {
		return ErrSupportTranslation
	}
	return err
}

func (s *SupportInboxService) RefreshWidgetLocale(ctx context.Context, session *model.SupportWidgetSession, locale string) error {
	if session == nil || locale == "" || (session.Locale != nil && *session.Locale == locale) {
		return nil
	}
	normalized := normalizeSupportLanguage(locale)
	if normalized == "" || len(normalized) > 35 {
		return nil
	}
	return s.messageRepo.DB().WithContext(ctx).Model(&model.SupportWidgetSession{}).Where("id = ? AND workspace_id = ?", session.ID, session.WorkspaceID).Update("locale", normalized).Error
}
