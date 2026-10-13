package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// TranslationOptionsForDisplay fills missing language evidence when a reader
// opens an older conversation. Internal settings reads never start detection.
func (s *SupportInboxService) TranslationOptionsForDisplay(ctx context.Context, workspaceID, conversationID, userID string) (*model.SupportTranslationOptions, error) {
	options, err := s.TranslationOptions(ctx, workspaceID, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !options.Available || options.Conversation.TranslationMode != "on" ||
		options.Conversation.CustomerLanguage != "" || options.DetectedCustomerLanguage != "" {
		return options, nil
	}
	// Reuse the existing bounded detector, artifact cache and retry backoff.
	// Save the first clear language without translating history or changing overrides.
	if _, err := s.detectCustomerLanguage(ctx, workspaceID, conversationID); err != nil && !errors.Is(err, errCustomerLanguageUnknown) {
		slog.WarnContext(ctx, "support display language detection unavailable", "conversation_id", conversationID, "error", err)
	}
	// A teammate may have paused translation or chosen a language during the
	// provider call. Return current settings rather than the earlier snapshot.
	return s.TranslationOptions(ctx, workspaceID, conversationID, userID)
}
