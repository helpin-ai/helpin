package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/observability"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const supportTranslationVersion = "v4"
const BillingFeatureSupportTranslation = "support_translation"

var ErrSupportTranslation = errors.New("Translation is unavailable. Your reply hasn’t been sent.")
var supportTranslationLanguages = map[string]string{"fa": "Persian", "en": "English", "de": "German", "fr": "French", "es": "Spanish", "it": "Italian", "pt": "Portuguese", "pt-BR": "Portuguese (Brazil)", "nl": "Dutch", "pl": "Polish", "uk": "Ukrainian", "ru": "Russian", "tr": "Turkish", "ar": "Arabic", "he": "Hebrew", "hi": "Hindi", "bn": "Bengali", "ur": "Urdu", "ja": "Japanese", "ko": "Korean", "zh-CN": "Chinese (Simplified)", "zh-TW": "Chinese (Traditional)", "vi": "Vietnamese", "th": "Thai", "id": "Indonesian", "sv": "Swedish", "da": "Danish", "no": "Norwegian", "fi": "Finnish", "cs": "Czech", "ro": "Romanian", "el": "Greek"}

type supportTranslationService struct {
	metrics   *observability.Metrics
	repo      *repository.SupportTranslationRepository
	provider  llm.Provider
	jev       *JevDecisionService
	route     AICompletionRoute
	available bool
	// unconfigured means this server has no translation provider at all (for
	// example a Community install without an OpenRouter key). Unlike an outage
	// (available=false), replies are then sent as written instead of blocked.
	unconfigured bool
}

func (s *SupportInboxService) SetTranslations(repo *repository.SupportTranslationRepository, provider llm.Provider, jev *JevDecisionService, route AICompletionRoute, available bool) *SupportInboxService {
	s.translations = &supportTranslationService{repo: repo, provider: provider, jev: jev, route: route, available: available}
	return s
}

// SetTranslationProviderConfigured records whether the server has a
// translation provider. Without one, translation is off rather than
// "temporarily unavailable", so teammate replies are never blocked by it.
func (s *SupportInboxService) SetTranslationProviderConfigured(configured bool) *SupportInboxService {
	if s.translations != nil {
		s.translations.unconfigured = !configured
	}
	return s
}

// translationConfigured reports whether translation can run on this server.
func (s *SupportInboxService) translationConfigured() bool {
	return s.translations != nil && !s.translations.unconfigured
}
func (s *SupportInboxService) SetTranslationMetrics(metrics *observability.Metrics) {
	if s.translations != nil {
		s.translations.metrics = metrics
	}
}
func translationHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

func (s *SupportInboxService) TranslationOptions(ctx context.Context, workspaceID, conversationID, userID string) (*model.SupportTranslationOptions, error) {
	return s.translationOptions(ctx, workspaceID, conversationID, userID, false)
}

func (s *SupportInboxService) translationOptions(ctx context.Context, workspaceID, conversationID, userID string, forSend bool) (*model.SupportTranslationOptions, error) {
	conv, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conv == nil || conv.AnonymizedAt != nil || userID == "" {
		return nil, ErrSupportTranslation
	}
	result := &model.SupportTranslationOptions{Languages: supportTranslationLanguages, Preference: model.SupportTranslationPreference{ReadingLanguage: "en", AutoTranslateIncoming: true, AutoTranslateOutgoing: true}, Conversation: model.SupportTranslationConversation{TranslationMode: "inherit"}}
	if s.translations == nil {
		result.UnavailableReason = "Translation is not configured."
		return result, nil
	}
	if s.translations.unconfigured {
		result.Preference.AutoTranslateIncoming = false
		result.Preference.AutoTranslateOutgoing = false
		result.Conversation.TranslationMode = "off"
		result.UnavailableReason = "Translation isn’t set up on this server. Add an OpenRouter key on the server to turn it on."
		return result, nil
	}
	settings := model.DefaultSupportInboxSettings()
	if s.installationRepo != nil {
		installation, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		if installation != nil {
			settings = parseSettings(installation.Settings)
		}
	}
	// Persisted conversation policy is independent of the default for new threads.
	policy, err := s.translations.repo.LiveConversation(ctx, workspaceID, conversationID, settings.TranslationEnabled && settings.TranslationIncomingEnabled && settings.TranslationOutgoingEnabled, settings.TranslationCustomerLanguage)
	if err != nil {
		return nil, err
	}
	enabled := policy.TranslationMode == "on"
	result.Preference = model.SupportTranslationPreference{ReadingLanguage: settings.DefaultAgentLanguage, AutoTranslateIncoming: enabled, AutoTranslateOutgoing: enabled}
	result.Conversation = *policy
	result.Available = s.translations.available
	if !result.Available {
		result.UnavailableReason = "Translation is temporarily unavailable."
	}
	if forSend && !enabled {
		return result, nil
	}
	detected, err := s.translations.repo.DetectedLanguage(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	result.DetectedCustomerLanguage = detected
	result.JevReview = false
	if result.DetectedCustomerLanguage == "" {
		result.DetectedCustomerLanguage = normalizeLiveLanguage(s.translations.repo.BrowserLanguage(ctx, workspaceID, conversationID))
	}
	return result, nil
}
func (s *SupportInboxService) TranslateSupport(ctx context.Context, workspaceID, conversationID, userID string, req model.SupportTranslateRequest) (*model.SupportTranslation, error) {
	options, err := s.TranslationOptions(ctx, workspaceID, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if req.MessageID != "" {
		if req.Live && !options.Preference.AutoTranslateIncoming {
			return nil, ErrSupportTranslation
		}
		req.TargetLanguage = options.Preference.ReadingLanguage
	} else if !options.Preference.AutoTranslateOutgoing {
		return nil, ErrSupportTranslation
	}
	if !options.Available || supportTranslationLanguages[req.TargetLanguage] == "" {
		return nil, ErrSupportTranslation
	}
	artifact := &model.SupportTranslation{ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: conversationID, Purpose: "outgoing_reply", CreatedByUserID: &userID, TargetLanguage: req.TargetLanguage, PipelineVersion: supportTranslationVersion, PolicyRevision: options.Conversation.Revision, Status: "pending", ReviewStatus: "not_requested", Attempts: 1}
	source := strings.TrimSpace(req.Content)
	scope := userID + ":" + req.DraftID
	if req.MessageID != "" {
		msg, err := s.messageRepo.GetByID(ctx, req.MessageID)
		if err != nil {
			return nil, err
		}
		if msg == nil || msg.WorkspaceID != workspaceID || msg.ConversationID != conversationID || msg.IsInternal || msg.SenderType != "customer" || msg.MessageType != "reply" || msg.DeletedAt.Valid {
			return nil, ErrSupportTranslation
		}
		artifact.Purpose = "manual_display"
		if req.Live {
			artifact.Purpose = "message_display"
		}
		artifact.SourceMessageID = &msg.ID
		artifact.CreatedByUserID = nil
		source = msg.Content
		scope = msg.ID
	} else {
		if _, err := uuid.Parse(req.DraftID); err != nil {
			return nil, ErrSupportTranslation
		}
		if options.Conversation.CustomerLanguage != "" && options.Conversation.CustomerLanguage != req.TargetLanguage {
			return nil, ErrSupportTranslation
		}
		artifact.SendKey = req.DraftID
		expires := time.Now().UTC().Add(24 * time.Hour)
		artifact.ExpiresAt = &expires
	}
	if source == "" || !utf8.ValidString(source) {
		return nil, ErrSupportTranslation
	}
	if estimatedTranslationTokens(source) > 32000 {
		return nil, fmt.Errorf("message is too long for automatic translation; send the original instead")
	}
	if job, ok := ctx.Value(supportSendGuardKey{}).(*model.SupportPendingSend); ok {
		artifact.RetryAttempt = job.Attempts
	}
	artifact.SourceText = source
	artifact.SourceHash = translationHash(source)
	reviewPolicy := "off"
	artifact.CacheKey = translationHash(strings.Join([]string{workspaceID, conversationID, artifact.Purpose, scope, artifact.SourceHash, req.TargetLanguage, reviewPolicy, supportTranslationVersion, s.translations.route.Provider, s.translations.route.Model}, "\x00"))
	artifact, owned, err := s.translations.repo.Reserve(ctx, artifact)
	if err != nil {
		return nil, err
	}
	if !owned {
		outcome := "pending"
		if artifact.Status == "ready" {
			outcome = "cache_hit"
		} else if artifact.Status == "failed" {
			outcome = "failed"
		}
		s.translations.metrics.TranslationEvent(artifact.Purpose, outcome)
		return artifact, nil
	}
	started := time.Now()
	defer func() { s.translations.metrics.TranslationWorkflow(artifact.Purpose, time.Since(started)) }()
	bounded, cancel := context.WithTimeout(ctx, 55*time.Second)
	defer cancel()
	callErr := s.generateSupportTranslation(bounded, artifact)
	if callErr != nil {
		artifact.Status = "failed"
		artifact.ErrorCode = "generation_failed"
		if errors.Is(callErr, context.DeadlineExceeded) || errors.Is(bounded.Err(), context.DeadlineExceeded) {
			artifact.ErrorCode = "generation_timeout"
		}
		slog.WarnContext(ctx, "support translation generation failed", "workspace_id", workspaceID, "translation_id", artifact.ID, "error_code", artifact.ErrorCode)
	} else {
		artifact.Status = "ready"
		if artifact.SourceLanguage == artifact.TargetLanguage && artifact.TranslatedText == artifact.SourceText {
			s.translations.metrics.TranslationEvent(artifact.Purpose, "same_language")
		}
	}
	s.translations.metrics.TranslationEvent(artifact.Purpose, artifact.Status)
	if artifact.Purpose == "outgoing_reply" {
		s.translations.metrics.TranslationEvent("review", artifact.ReviewStatus)
	}
	// Settle even when the requesting browser has disconnected, with a bounded context.
	settle, cancelSettle := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancelSettle()
	if err := s.translations.repo.Finish(settle, artifact); err != nil {
		return nil, err
	}
	// Revalidate authorization and source after the external call.
	current, err := s.TranslationOptions(ctx, workspaceID, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !current.Available || (artifact.Purpose == "message_display" &&
		((!current.Preference.AutoTranslateIncoming) || current.Conversation.Revision != options.Conversation.Revision || current.Preference.ReadingLanguage != artifact.TargetLanguage)) ||
		(artifact.Purpose == "outgoing_reply" && (!current.Preference.AutoTranslateOutgoing || current.Conversation.Revision != options.Conversation.Revision ||
			(current.Conversation.CustomerLanguage == "" && current.DetectedCustomerLanguage != "" && current.DetectedCustomerLanguage != artifact.TargetLanguage) ||
			(current.Conversation.CustomerLanguage != "" && current.Conversation.CustomerLanguage != artifact.TargetLanguage))) {
		return nil, ErrSupportTranslation
	}
	if artifact.SourceMessageID != nil {
		msg, err := s.messageRepo.GetByID(ctx, *artifact.SourceMessageID)
		if err != nil {
			return nil, err
		}
		if msg == nil || translationHash(msg.Content) != artifact.SourceHash {
			return nil, ErrSupportTranslation
		}
	}
	return artifact, nil
}

// prepareTranslatedReply returns no translation and no error when the customer
// language is unknown and there is no customer-authored text to detect it from.
// Detection failures offer an explicit send-original choice at the API boundary.
func (s *SupportInboxService) prepareTranslatedReply(ctx context.Context, workspaceID, conversationID, userID string, req model.CreateMessageRequest) (*model.SupportTranslation, error) {
	options, err := s.TranslationOptions(ctx, workspaceID, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !options.Available || !options.Preference.AutoTranslateOutgoing || req.IsInternal || (req.MessageType != "" && req.MessageType != "reply") {
		return nil, ErrSupportTranslation
	}
	target := options.Conversation.CustomerLanguage
	if target == "" {
		target = options.DetectedCustomerLanguage
	}
	if target == "" {
		messageID, lookupErr := s.translations.repo.LatestCustomerMessageID(ctx, workspaceID, conversationID)
		if lookupErr != nil {
			return nil, ErrSupportTranslation
		}
		if messageID == "" {
			return nil, nil
		}
		msg, loadErr := s.messageRepo.GetByID(ctx, messageID)
		if loadErr != nil || msg == nil {
			return nil, ErrSupportTranslation
		}
		target, err = s.detectCustomerLanguage(ctx, workspaceID, conversationID, msg)
		if err != nil {
			return nil, err
		}
	}
	if supportTranslationLanguages[target] == "" {
		return nil, fmt.Errorf("could not identify the customer’s language; send the original or retry")
	}
	// The client ID is reused by retries of the same send; namespace it per actor.
	if req.ClientMessageID == "" {
		return nil, ErrSupportTranslation
	}
	draftID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(workspaceID+":"+conversationID+":"+userID+":"+req.ClientMessageID)).String()
	t, err := s.TranslateSupport(ctx, workspaceID, conversationID, userID, model.SupportTranslateRequest{Content: req.Content, DraftID: draftID, TargetLanguage: target})
	if err != nil {
		slog.WarnContext(ctx, "support translated send preparation failed", "workspace_id", workspaceID)
		return nil, ErrSupportTranslation
	}
	if t.Status != "ready" {
		return nil, fmt.Errorf("translation failed; your reply was not sent. Please try again")
	}
	if t.ReviewStatus == "needs_review" {
		return nil, fmt.Errorf("translation could not be verified; your reply was not sent")
	}
	if t.SourceHash != translationHash(strings.TrimSpace(req.Content)) || t.TargetLanguage != target {
		return nil, ErrSupportTranslation
	}
	if t.SentMessageID == nil && (t.ExpiresAt == nil || !time.Now().Before(*t.ExpiresAt)) {
		return nil, ErrSupportTranslation
	}
	return t, nil
}
