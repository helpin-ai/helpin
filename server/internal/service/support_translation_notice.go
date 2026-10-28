package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// TranslateCustomerNotice puts a message the team wrote, such as a handoff
// message, into the customer's language. It works out the language the same way
// teammate replies do. The text comes back unchanged when that language is
// unknown or matches the team's, or when translation is unavailable or fails,
// so the message is never held back.
func (s *SupportInboxService) TranslateCustomerNotice(ctx context.Context, workspaceID, conversationID, text, teamLanguage string) string {
	if s == nil || s.translations == nil || s.translations.unconfigured || !s.translations.available || strings.TrimSpace(text) == "" {
		return text
	}
	target, err := s.detectCustomerLanguage(ctx, workspaceID, conversationID)
	if err != nil {
		if !errors.Is(err, errCustomerLanguageUnknown) {
			slog.WarnContext(ctx, "customer language lookup failed; sending the notice as written", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
		}
		return text
	}
	source := normalizeLiveLanguage(teamLanguage)
	if source == "" {
		source = "en"
	}
	if target == "" || target == source || supportTranslationLanguages[target] == "" {
		return text
	}
	// Not stored: the sent message itself is the record of what the customer got.
	artifact := &model.SupportTranslation{ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: conversationID, SourceText: text, TargetLanguage: target, Attempts: 1}
	if err := s.generateSupportTranslation(ctx, artifact); err != nil {
		slog.WarnContext(ctx, "customer notice translation failed; sending it as written", "workspace_id", workspaceID, "conversation_id", conversationID, "target_language", target, "error", err)
		return text
	}
	if strings.TrimSpace(artifact.TranslatedText) == "" {
		return text
	}
	return artifact.TranslatedText
}
