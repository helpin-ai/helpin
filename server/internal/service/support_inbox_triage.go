package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

const (
	supportTriageEventEvaluated = "evaluated"
	supportTriageEventAccepted  = "accepted"
	supportTriageEventDismissed = "dismissed"
	supportTriageEventCorrected = "corrected"
	supportTriageEventAutoMoved = "auto_moved"
	supportTriageProvider       = supportSmallTierProvider
	supportTriageModel          = supportSmallTierModel
)

type SupportInboxTriageService struct {
	supportService   *SupportInboxService
	triageRepo       *repository.SupportConversationTriageRepository
	triageEventRepo  *repository.SupportConversationTriageEventRepository
	ruleRepo         *repository.SupportTriageRuleRepository
	installRepo      *repository.SupportInboxInstallationRepository
	mailboxRepo      *repository.SupportMailboxRepository
	conversationRepo *repository.SupportConversationRepository
	messageRepo      *repository.SupportMessageRepository
	llmProvider      llm.Provider
	localDecision    *SupportDecisionClient
	jev              *SupportJevService
	entitlementSvc   EntitlementPolicy
}

type supportInboxTriageResult struct {
	Intent             *string
	Confidence         *float64
	Reason             *string
	ClassifierSource   string
	SuggestedMailboxID *string
	SuggestedHandle    string
	Cached             bool
}

type supportTriageMailboxOption struct {
	ID     *string
	Name   string
	Handle string
	Prompt string
}

type supportTriageLLMResponse struct {
	Intent              string  `json:"intent"`
	TargetMailboxHandle string  `json:"target_mailbox_handle"`
	Confidence          float64 `json:"confidence"`
	Reason              string  `json:"reason"`
}

func NewSupportInboxTriageService(
	supportService *SupportInboxService,
	triageRepo *repository.SupportConversationTriageRepository,
	triageEventRepo *repository.SupportConversationTriageEventRepository,
	ruleRepo *repository.SupportTriageRuleRepository,
	installRepo *repository.SupportInboxInstallationRepository,
	mailboxRepo *repository.SupportMailboxRepository,
	conversationRepo *repository.SupportConversationRepository,
	messageRepo *repository.SupportMessageRepository,
	llmProvider llm.Provider,
) *SupportInboxTriageService {
	return &SupportInboxTriageService{
		supportService:   supportService,
		triageRepo:       triageRepo,
		triageEventRepo:  triageEventRepo,
		ruleRepo:         ruleRepo,
		installRepo:      installRepo,
		mailboxRepo:      mailboxRepo,
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		llmProvider:      llmProvider,
	}
}

// SetJevService injects typed routing. Tagging runs in its durable worker.
func (s *SupportInboxTriageService) SetJevService(jev *SupportJevService) { s.jev = jev }

// SetLocalDecisionClient injects optional local-first routing. Configure before serving requests.
func (s *SupportInboxTriageService) SetLocalDecisionClient(client *SupportDecisionClient) *SupportInboxTriageService {
	s.localDecision = client
	return s
}

func (s *SupportInboxTriageService) SetEntitlementService(entitlementSvc EntitlementPolicy) *SupportInboxTriageService {
	s.entitlementSvc = entitlementSvc
	return s
}

func (s *SupportInboxTriageService) HydrateConversation(ctx context.Context, conversation *model.SupportConversation) error {
	if s == nil || conversation == nil || s.triageRepo == nil {
		return nil
	}
	triage, err := s.triageRepo.GetByConversation(ctx, conversation.WorkspaceID, conversation.ID)
	if err != nil {
		return err
	}
	conversation.Triage = triage
	return nil
}

func (s *SupportInboxTriageService) HydrateConversations(ctx context.Context, conversations []model.SupportConversation) error {
	if s == nil || len(conversations) == 0 || s.triageRepo == nil {
		return nil
	}

	workspaceID := conversations[0].WorkspaceID
	ids := make([]string, 0, len(conversations))
	for _, conversation := range conversations {
		ids = append(ids, conversation.ID)
	}

	triageByConversationID, err := s.triageRepo.ListByConversationIDs(ctx, workspaceID, ids)
	if err != nil {
		return err
	}

	for idx := range conversations {
		if triage, ok := triageByConversationID[conversations[idx].ID]; ok {
			triageCopy := triage
			conversations[idx].Triage = &triageCopy
		}
	}

	return nil
}

func (s *SupportInboxTriageService) ListRules(ctx context.Context, workspaceID string) ([]model.SupportTriageRule, error) {
	if s == nil || s.ruleRepo == nil {
		return []model.SupportTriageRule{}, nil
	}
	return s.ruleRepo.ListByWorkspace(ctx, workspaceID)
}

func (s *SupportInboxTriageService) CreateRule(ctx context.Context, workspaceID, actorID string, req model.CreateSupportTriageRuleRequest) (*model.SupportTriageRule, error) {
	if s == nil || s.ruleRepo == nil {
		return nil, fmt.Errorf("support triage rules are unavailable")
	}
	if s.entitlementSvc != nil {
		if err := s.entitlementSvc.RequireFeature(ctx, workspaceID, EntitlementFeatureAIConversationRouting); err != nil {
			return nil, err
		}
	}

	rule, err := s.buildRuleModel(ctx, workspaceID, actorID, nil, req)
	if err != nil {
		return nil, err
	}

	if err := s.ruleRepo.Create(ctx, rule); err != nil {
		return nil, err
	}
	return s.ruleRepo.GetByID(ctx, workspaceID, rule.ID)
}

func (s *SupportInboxTriageService) UpdateRule(ctx context.Context, workspaceID, ruleID string, req model.UpdateSupportTriageRuleRequest) (*model.SupportTriageRule, error) {
	if s == nil || s.ruleRepo == nil {
		return nil, fmt.Errorf("support triage rules are unavailable")
	}
	if s.entitlementSvc != nil {
		if err := s.entitlementSvc.RequireFeature(ctx, workspaceID, EntitlementFeatureAIConversationRouting); err != nil {
			return nil, err
		}
	}

	existing, err := s.ruleRepo.GetByID(ctx, workspaceID, ruleID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("triage rule not found")
	}

	mutated := *existing
	if req.Priority != nil {
		mutated.Priority = *req.Priority
	}
	if req.Active != nil {
		mutated.Active = *req.Active
	}
	if req.Name != nil {
		mutated.Name = strings.TrimSpace(*req.Name)
	}
	if req.Channels != nil {
		mutated.Channels = normalizeTriageChannels(req.Channels)
	}
	if req.Conditions != nil {
		mutated.Conditions = normalizeTriageConditions(*req.Conditions)
	}
	if req.TargetMailboxID != nil {
		mutated.TargetMailboxID = strings.TrimSpace(*req.TargetMailboxID)
	}

	if err := s.validateRuleModel(ctx, workspaceID, &mutated); err != nil {
		return nil, err
	}
	if err := s.ruleRepo.Update(ctx, &mutated); err != nil {
		return nil, err
	}
	return s.ruleRepo.GetByID(ctx, workspaceID, mutated.ID)
}

func (s *SupportInboxTriageService) DeleteRule(ctx context.Context, workspaceID, ruleID string) error {
	if s == nil || s.ruleRepo == nil {
		return nil
	}
	return s.ruleRepo.Delete(ctx, workspaceID, ruleID)
}

func (s *SupportInboxTriageService) EvaluateAndRoute(ctx context.Context, workspaceID, conversationID, messageID string) (*model.SupportConversationTriage, error) {
	if s == nil || s.triageRepo == nil || s.conversationRepo == nil || s.messageRepo == nil || s.installRepo == nil {
		return nil, nil
	}

	conversation, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil || conversation == nil {
		return nil, err
	}
	if supportConversationHumanOwned(conversation) {
		slog.InfoContext(ctx, "support triage skipped: conversation is human-owned", "workspace_id", workspaceID, "conversation_id", conversationID)
		return nil, nil
	}

	inst, err := s.installRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	settings := model.DefaultSupportInboxSettings()
	if inst != nil {
		settings = parseSettings(inst.Settings)
	}

	channel := supportConversationChannel(conversation)
	if !settings.TriageEnabled || !triageChannelEnabled(settings, channel) {
		slog.InfoContext(ctx, "support triage skipped",
			"workspace_id", workspaceID,
			"conversation_id", conversationID,
			"triage_enabled", settings.TriageEnabled,
			"channel", channel,
			"widget_enabled", settings.TriageWidgetEnabled,
			"email_enabled", settings.TriageEmailEnabled,
		)
		return nil, nil
	}
	if s.entitlementSvc != nil {
		if err := s.entitlementSvc.RequireFeature(ctx, workspaceID, EntitlementFeatureAIConversationRouting); err != nil {
			slog.InfoContext(ctx, "support triage skipped due to billing entitlement", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
			return nil, nil
		}
	}
	if settings.TriageSkipSpamConversations && strings.EqualFold(strings.TrimSpace(conversation.Status), "spam") {
		return nil, nil
	}

	existing, err := s.triageRepo.GetByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if existing != nil && !settings.TriageRerunOnMeaningChange {
		return existing, nil
	}

	message, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if message == nil || message.ConversationID != conversationID || message.WorkspaceID != workspaceID {
		return nil, nil
	}
	if message.IsInternal || message.SenderType != "customer" || strings.TrimSpace(message.MessageType) != "reply" {
		return nil, nil
	}

	messages, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, true)
	if err != nil {
		return nil, err
	}
	firstCustomerReply := firstCustomerReplyMessage(messages)
	if firstCustomerReply == nil {
		return nil, nil
	}
	if firstCustomerReply.ID != message.ID && !settings.TriageRerunOnMeaningChange {
		slog.DebugContext(ctx, "support triage skipped: not first customer reply", "workspace_id", workspaceID, "conversation_id", conversationID, "message_id", messageID, "first_reply_id", firstCustomerReply.ID)
		return existing, nil
	}

	slog.InfoContext(ctx, "support triage evaluating", "workspace_id", workspaceID, "conversation_id", conversationID, "message_id", messageID)

	inputContent := strings.TrimSpace(firstCustomerReply.Content)
	if settings.TriageRerunOnMeaningChange && firstCustomerReply.ID != message.ID {
		inputContent = strings.TrimSpace(message.Content)
	}
	inputHash := buildSupportTriageInputHash(conversation.Subject, inputContent, derefString(conversation.CustomerEmail), supportConversationChannel(conversation))

	result, err := s.evaluateRules(ctx, workspaceID, conversation, inputContent)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result, err = s.evaluateAI(ctx, workspaceID, settings, conversation, inputContent, inputHash)
		if err != nil {
			slog.ErrorContext(ctx, "support triage AI evaluation failed", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
		}
	}
	if result == nil || result.SuggestedMailboxID == nil {
		return nil, nil
	}

	now := time.Now().UTC()
	triage := &model.SupportConversationTriage{
		WorkspaceID:        workspaceID,
		ConversationID:     conversationID,
		Status:             model.SupportConversationTriageStatusSuggested,
		Intent:             result.Intent,
		Confidence:         result.Confidence,
		Reason:             result.Reason,
		ClassifierSource:   result.ClassifierSource,
		SuggestedMailboxID: result.SuggestedMailboxID,
		AutoMoved:          false,
		EvaluatedAt:        &now,
		InputHash:          inputHash,
	}
	if existing != nil {
		triage.ID = existing.ID
		triage.CreatedAt = existing.CreatedAt
	}

	if err := s.triageRepo.Upsert(ctx, triage); err != nil {
		return nil, err
	}
	evaluatedPayload := triageEventPayload(triage, map[string]any{
		"message_id": message.ID,
		"cached":     result.Cached,
	})
	if err := s.triageEventRepo.Create(ctx, &model.SupportConversationTriageEvent{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		TriageID:       &triage.ID,
		EventType:      supportTriageEventEvaluated,
		Source:         strPtr(result.ClassifierSource),
		ToMailboxID:    triage.SuggestedMailboxID,
		InputHash:      inputHash,
		Cached:         result.Cached,
		Payload:        evaluatedPayload,
	}); err != nil {
		return nil, err
	}

	if s.shouldAutoMove(settings, conversation, messages, triage) {
		if _, moveErr := s.supportService.moveConversationInternal(
			ctx,
			workspaceID,
			conversationID,
			triage.SuggestedMailboxID,
			"",
			supportConversationMoveOptions{
				EnforceMailboxAccess: false,
				UseAccessibleLoad:    false,
				RecordTriageFeedback: false,
			},
		); moveErr == nil {
			triage.Status = model.SupportConversationTriageStatusAutoMoved
			triage.AutoMoved = true
			if err := s.triageRepo.Upsert(ctx, triage); err != nil {
				return nil, err
			}
			if err := s.triageEventRepo.Create(ctx, &model.SupportConversationTriageEvent{
				WorkspaceID:    workspaceID,
				ConversationID: conversationID,
				TriageID:       &triage.ID,
				EventType:      supportTriageEventAutoMoved,
				Source:         strPtr(result.ClassifierSource),
				FromMailboxID:  conversation.MailboxID,
				ToMailboxID:    triage.SuggestedMailboxID,
				InputHash:      inputHash,
				Payload:        triageEventPayload(triage, nil),
			}); err != nil {
				return nil, err
			}
			s.createSystemMessage(ctx, workspaceID, conversationID, nil, "ai", "AI triage", autoMoveMessage(result.ClassifierSource, triage.SuggestedMailboxID, s.loadMailboxName(ctx, workspaceID, triage.SuggestedMailboxID)), true, model.SystemEventTriageRouted)
		} else {
			slog.ErrorContext(ctx, "support triage auto-move failed", "workspace_id", workspaceID, "conversation_id", conversationID, "error", moveErr)
		}
	}

	s.publishConversationUpdated(workspaceID, conversationID, "")
	return triage, nil
}

func (s *SupportInboxTriageService) DismissConversationTriage(ctx context.Context, workspaceID, conversationID, actorUserID string) (*model.SupportConversationTriage, error) {
	if s == nil || s.triageRepo == nil || s.triageEventRepo == nil {
		return nil, fmt.Errorf("support triage is unavailable")
	}

	triage, err := s.triageRepo.GetByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if triage == nil {
		return nil, fmt.Errorf("triage suggestion not found")
	}

	now := time.Now().UTC()
	triage.Status = model.SupportConversationTriageStatusDismissed
	triage.LockedAt = &now
	triage.FeedbackAction = strPtr(model.SupportConversationTriageFeedbackDismissed)
	if err := s.triageRepo.Upsert(ctx, triage); err != nil {
		return nil, err
	}
	if err := s.triageEventRepo.Create(ctx, &model.SupportConversationTriageEvent{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		TriageID:       &triage.ID,
		EventType:      supportTriageEventDismissed,
		Source:         strPtr(triage.ClassifierSource),
		ActorUserID:    &actorUserID,
		ToMailboxID:    triage.SuggestedMailboxID,
		InputHash:      triage.InputHash,
		Payload:        triageEventPayload(triage, nil),
	}); err != nil {
		return nil, err
	}

	s.createSystemMessage(ctx, workspaceID, conversationID, &actorUserID, "user", "", "Dismissed the routing suggestion.", true, model.SystemEventTriageDismissed)
	s.publishConversationUpdated(workspaceID, conversationID, actorUserID)
	return triage, nil
}

func (s *SupportInboxTriageService) RecordManualMoveFeedback(ctx context.Context, workspaceID string, conversation *model.SupportConversation, targetMailboxID *string, actorUserID string) {
	if s == nil || s.triageRepo == nil || s.triageEventRepo == nil || conversation == nil {
		return
	}

	triage, err := s.triageRepo.GetByConversation(ctx, workspaceID, conversation.ID)
	if err != nil {
		slog.ErrorContext(ctx, "load support triage for manual move feedback", "workspace_id", workspaceID, "conversation_id", conversation.ID, "error", err)
		return
	}

	now := time.Now().UTC()
	if triage == nil {
		payload := model.JSONB{
			"manual_move_without_suggestion": true,
		}
		if err := s.triageEventRepo.Create(ctx, &model.SupportConversationTriageEvent{
			WorkspaceID:    workspaceID,
			ConversationID: conversation.ID,
			EventType:      supportTriageEventCorrected,
			ActorUserID:    &actorUserID,
			FromMailboxID:  conversation.MailboxID,
			ToMailboxID:    normalizeMailboxIDPointer(targetMailboxID),
			Payload:        payload,
		}); err != nil {
			slog.ErrorContext(ctx, "record support triage manual move without suggestion", "workspace_id", workspaceID, "conversation_id", conversation.ID, "error", err)
		}
		return
	}

	var (
		eventType string
		changed   bool
	)

	switch {
	case triage.Status == model.SupportConversationTriageStatusSuggested && triage.LockedAt == nil && sameMailboxID(triage.SuggestedMailboxID, targetMailboxID):
		triage.LockedAt = &now
		triage.FeedbackAction = strPtr(model.SupportConversationTriageFeedbackAccepted)
		eventType = supportTriageEventAccepted
		changed = true
	case triage.Status == model.SupportConversationTriageStatusSuggested && triage.LockedAt == nil && !sameMailboxID(triage.SuggestedMailboxID, targetMailboxID):
		triage.Status = model.SupportConversationTriageStatusOverridden
		triage.LockedAt = &now
		triage.FeedbackAction = strPtr(model.SupportConversationTriageFeedbackCorrected)
		eventType = supportTriageEventCorrected
		changed = true
	case triage.Status == model.SupportConversationTriageStatusAutoMoved && !sameMailboxID(conversation.MailboxID, targetMailboxID):
		triage.Status = model.SupportConversationTriageStatusOverridden
		triage.LockedAt = &now
		triage.FeedbackAction = strPtr(model.SupportConversationTriageFeedbackCorrected)
		eventType = supportTriageEventCorrected
		changed = true
	}

	if !changed {
		return
	}

	if err := s.triageRepo.Upsert(ctx, triage); err != nil {
		slog.ErrorContext(ctx, "update support triage feedback", "workspace_id", workspaceID, "conversation_id", conversation.ID, "error", err)
		return
	}
	if err := s.triageEventRepo.Create(ctx, &model.SupportConversationTriageEvent{
		WorkspaceID:    workspaceID,
		ConversationID: conversation.ID,
		TriageID:       &triage.ID,
		EventType:      eventType,
		Source:         strPtr(triage.ClassifierSource),
		ActorUserID:    &actorUserID,
		FromMailboxID:  conversation.MailboxID,
		ToMailboxID:    normalizeMailboxIDPointer(targetMailboxID),
		InputHash:      triage.InputHash,
		Payload:        triageEventPayload(triage, nil),
	}); err != nil {
		slog.ErrorContext(ctx, "record support triage feedback event", "workspace_id", workspaceID, "conversation_id", conversation.ID, "error", err)
	}
}

func (s *SupportInboxTriageService) buildRuleModel(ctx context.Context, workspaceID, actorID string, existing *model.SupportTriageRule, req model.CreateSupportTriageRuleRequest) (*model.SupportTriageRule, error) {
	active := true
	if req.Active != nil {
		active = *req.Active
	}

	rule := &model.SupportTriageRule{
		WorkspaceID:     workspaceID,
		Priority:        req.Priority,
		Active:          active,
		Name:            strings.TrimSpace(req.Name),
		Channels:        normalizeTriageChannels(req.Channels),
		Conditions:      normalizeTriageConditions(req.Conditions),
		TargetMailboxID: strings.TrimSpace(req.TargetMailboxID),
		CreatedByID:     actorID,
	}
	if existing != nil {
		rule.ID = existing.ID
		rule.CreatedAt = existing.CreatedAt
		rule.CreatedByID = existing.CreatedByID
	}
	return rule, s.validateRuleModel(ctx, workspaceID, rule)
}

func (s *SupportInboxTriageService) validateRuleModel(ctx context.Context, workspaceID string, rule *model.SupportTriageRule) error {
	if rule == nil {
		return fmt.Errorf("triage rule is required")
	}
	if strings.TrimSpace(rule.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(rule.TargetMailboxID) == "" {
		return fmt.Errorf("target_mailbox_id is required")
	}
	if len(rule.Conditions.PhraseContains) == 0 && len(rule.Conditions.EmailDomainEquals) == 0 && len(rule.Conditions.SenderEmailContains) == 0 {
		return fmt.Errorf("at least one triage rule condition is required")
	}
	if rule.Priority < 0 {
		return fmt.Errorf("priority must be >= 0")
	}
	for _, channel := range rule.Channels {
		if !validSupportTriageChannel(channel) {
			return fmt.Errorf("invalid triage channel %q", channel)
		}
	}
	if s.mailboxRepo != nil {
		mailbox, err := s.mailboxRepo.GetByID(ctx, workspaceID, rule.TargetMailboxID)
		if err != nil {
			return fmt.Errorf("load target mailbox: %w", err)
		}
		if mailbox == nil || !mailbox.Active {
			return fmt.Errorf("target_mailbox_id must reference an active mailbox")
		}
	}
	return nil
}

func (s *SupportInboxTriageService) evaluateRules(ctx context.Context, workspaceID string, conversation *model.SupportConversation, inputContent string) (*supportInboxTriageResult, error) {
	if s == nil || s.ruleRepo == nil {
		return nil, nil
	}

	rules, err := s.ruleRepo.ListActiveByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	channel := supportConversationChannel(conversation)
	combinedText := strings.ToLower(strings.TrimSpace(strings.Join([]string{conversation.Subject, inputContent}, "\n")))
	senderEmail := strings.ToLower(strings.TrimSpace(derefString(conversation.CustomerEmail)))
	emailDomain := supportEmailDomain(conversation.CustomerEmail)
	slog.InfoContext(ctx, "support triage rules evaluating",
		"workspace_id", workspaceID,
		"conversation_id", derefString(func() *string {
			if conversation == nil {
				return nil
			}
			return &conversation.ID
		}()),
		"channel", channel,
		"active_rule_count", len(rules),
		"sender_email", senderEmail,
		"email_domain", emailDomain,
		"combined_text_preview", safeLogPreview(combinedText, 160),
	)

	for _, rule := range rules {
		if len(rule.Channels) > 0 && !containsTriageChannel(rule.Channels, channel) {
			continue
		}
		if !triageConditionsMatch(rule.Conditions, combinedText, senderEmail, emailDomain) {
			continue
		}

		intent := normalizeSupportTriageIntent(rule.Name)
		confidence := 1.0
		reason := "Routing rule matched."
		targetMailboxID := strings.TrimSpace(rule.TargetMailboxID)
		if s.mailboxRepo != nil {
			mailbox, err := s.mailboxRepo.GetByID(ctx, workspaceID, targetMailboxID)
			if err != nil {
				return nil, err
			}
			if mailbox == nil || !mailbox.Active {
				slog.InfoContext(ctx, "support triage rule skipped archived target",
					"workspace_id", workspaceID,
					"conversation_id", derefString(func() *string {
						if conversation == nil {
							return nil
						}
						return &conversation.ID
					}()),
					"rule_id", rule.ID,
					"target_mailbox_id", targetMailboxID,
				)
				continue
			}
		}
		slog.InfoContext(ctx, "support triage rule matched",
			"workspace_id", workspaceID,
			"conversation_id", derefString(func() *string {
				if conversation == nil {
					return nil
				}
				return &conversation.ID
			}()),
			"rule_id", rule.ID,
			"rule_name", rule.Name,
			"target_mailbox_id", targetMailboxID,
			"channels", rule.Channels,
		)
		return &supportInboxTriageResult{
			Intent:             &intent,
			Confidence:         &confidence,
			Reason:             &reason,
			ClassifierSource:   model.SupportConversationTriageSourceRule,
			SuggestedMailboxID: &targetMailboxID,
		}, nil
	}

	slog.InfoContext(ctx, "support triage rules no match",
		"workspace_id", workspaceID,
		"conversation_id", derefString(func() *string {
			if conversation == nil {
				return nil
			}
			return &conversation.ID
		}()),
		"channel", channel,
		"active_rule_count", len(rules),
	)
	return nil, nil
}

func (s *SupportInboxTriageService) evaluateAI(ctx context.Context, workspaceID string, settings model.SupportInboxSettings, conversation *model.SupportConversation, inputContent, inputHash string) (*supportInboxTriageResult, error) {
	if s == nil || s.llmProvider == nil || s.mailboxRepo == nil {
		return nil, nil
	}

	if settings.TriageDeduplicateFirstMessage && s.triageEventRepo != nil && !s.localDecision.enabled(workspaceID) && !(s.jev.enabled(workspaceID) && s.jev.config.RoutingMode != "off") {
		cachedEvent, err := s.triageEventRepo.FindLatestEvaluatedByInputHash(ctx, workspaceID, inputHash, time.Now().UTC().Add(-15*time.Minute))
		if err != nil {
			return nil, err
		}
		if cachedEvent != nil {
			if cached := triageResultFromPayload(cachedEvent.Payload); cached != nil {
				cached.Cached = true
				slog.InfoContext(ctx, "support triage AI cache hit",
					"workspace_id", workspaceID,
					"conversation_id", derefString(func() *string {
						if conversation == nil {
							return nil
						}
						return &conversation.ID
					}()),
					"input_hash", inputHash,
					"suggested_mailbox_id", derefString(cached.SuggestedMailboxID),
					"classifier_source", cached.ClassifierSource,
				)
				return cached, nil
			}
		}
	}

	if settings.TriageDailyBudget > 0 && s.triageEventRepo != nil {
		startOfDay := time.Now().UTC().Truncate(24 * time.Hour)
		count, err := s.triageEventRepo.CountAIEvaluationsSince(ctx, workspaceID, startOfDay)
		if err != nil {
			return nil, err
		}
		if count >= int64(settings.TriageDailyBudget) {
			slog.WarnContext(ctx, "support triage AI budget exhausted", "workspace_id", workspaceID, "daily_budget", settings.TriageDailyBudget)
			return nil, nil
		}
	}

	options, handleToID, err := s.aiMailboxOptions(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if len(options) == 0 {
		slog.InfoContext(ctx, "support triage AI skipped: no mailbox options",
			"workspace_id", workspaceID,
			"conversation_id", derefString(func() *string {
				if conversation == nil {
					return nil
				}
				return &conversation.ID
			}()),
		)
		return nil, nil
	}
	slog.InfoContext(ctx, "support triage AI mailbox options prepared",
		"workspace_id", workspaceID,
		"conversation_id", derefString(func() *string {
			if conversation == nil {
				return nil
			}
			return &conversation.ID
		}()),
		"option_count", len(options),
		"options", summarizeTriageMailboxOptions(options),
	)

	if s.jev.enabled(workspaceID) && s.jev.config.RoutingMode != "off" {
		candidate, accepted, jevErr := s.jev.route(ctx, workspaceID, conversation.ID, inputContent, options)
		if jevErr != nil {
			slog.WarnContext(ctx, "Jev routing falling back to LLM", "workspace_id", workspaceID, "conversation_id", conversation.ID, "error", jevErr)
		}
		if accepted {
			if candidate == nil {
				return nil, nil
			}
			if target, ok := handleToID[candidate.SuggestedHandle]; ok {
				candidate.SuggestedMailboxID = &target
				return candidate, nil
			}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	var shadowDecision *supportDecisionResponse
	if s.localDecision.enabled(workspaceID) && !(s.jev.enabled(workspaceID) && s.jev.config.RoutingMode != "off") {
		started := time.Now()
		local, accepted, reason, localErr := s.localDecision.decide(ctx, workspaceID, inputContent, options)
		if s.localDecision.config.Mode == "shadow" {
			shadowDecision = local
		}
		_, choicesHash := decisionChoices(options)
		slog.InfoContext(ctx, "support local decision evaluated", "workspace_id", workspaceID, "mode", s.localDecision.config.Mode, "accepted", accepted, "reason", reason, "choices_hash", choicesHash, "latency_ms", time.Since(started).Milliseconds())
		if localErr != nil {
			slog.WarnContext(ctx, "support local decision falling back", "workspace_id", workspaceID, "reason", reason)
		}
		if accepted && local != nil && local.Choice != nil {
			handle := *local.Choice
			if handle == "shared" {
				return nil, nil
			}
			if target, ok := handleToID[handle]; ok {
				explanation := "Local semantic decision; validated deployment policy " + local.DeploymentFingerprint
				intent := "local_semantic_routing"
				return &supportInboxTriageResult{Intent: &intent, Confidence: &local.Confidence, Reason: &explanation, ClassifierSource: model.SupportConversationTriageSourceAI, SuggestedMailboxID: &target, SuggestedHandle: handle}, nil
			}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	prompt := buildSupportTriagePrompt(conversation, inputContent, options)
	conversationID := ""
	if conversation != nil {
		conversationID = conversation.ID
	}
	resp, err := completeAI(ctx, s.llmProvider, AICompletionRequest{
		WorkspaceID:    workspaceID,
		FeatureKey:     BillingFeatureAIRouting,
		IdempotencyKey: aiUsageIdempotencyKey(workspaceID, BillingFeatureAIRouting, "triage", conversationID, aiUsageStableHash(inputContent)),
		Metadata: map[string]interface{}{
			"conversation_id": conversationID,
		},
		Chat: llm.ChatRequest{
			SystemPrompt: supportTriageSystemPrompt,
			Messages: []llm.Message{
				{Role: "user", Content: prompt},
			},
			Temperature: 0.1,
			MaxTokens:   400,
			JSONMode:    true,
			JSONSchema:  supportTriageJSONSchema(),
		},
	})
	if err != nil {
		return nil, err
	}

	var parsed supportTriageLLMResponse
	if err := llm.UnmarshalResponse(resp.Content, &parsed); err != nil {
		slog.WarnContext(ctx, "support triage AI response parse failed",
			"workspace_id", workspaceID,
			"conversation_id", derefString(func() *string {
				if conversation == nil {
					return nil
				}
				return &conversation.ID
			}()),
			"raw_response_preview", safeLogPreview(resp.Content, 240),
			"error", err,
		)
		return nil, err
	}

	handle := strings.ToLower(strings.TrimSpace(parsed.TargetMailboxHandle))
	if shadowDecision != nil && shadowDecision.Choice != nil {
		slog.InfoContext(ctx, "support local shadow comparison", "workspace_id", workspaceID, "agrees_with_llm", *shadowDecision.Choice == handle, "local_confidence", shadowDecision.Confidence, "local_abstained", shadowDecision.Abstained, "deployment_fingerprint", shadowDecision.DeploymentFingerprint)
	}

	slog.InfoContext(ctx, "support triage AI response received",
		"workspace_id", workspaceID,
		"conversation_id", derefString(func() *string {
			if conversation == nil {
				return nil
			}
			return &conversation.ID
		}()),
		"target_mailbox_handle", handle,
		"intent", parsed.Intent,
		"confidence", parsed.Confidence,
		"reason", strings.TrimSpace(parsed.Reason),
	)
	if handle == "" || handle == "shared" {
		slog.InfoContext(ctx, "support triage AI selected shared inbox",
			"workspace_id", workspaceID,
			"conversation_id", derefString(func() *string {
				if conversation == nil {
					return nil
				}
				return &conversation.ID
			}()),
			"target_mailbox_handle", handle,
		)
		return nil, nil
	}
	targetMailboxID, ok := handleToID[handle]
	if !ok {
		slog.WarnContext(ctx, "support triage AI returned unknown mailbox handle",
			"workspace_id", workspaceID,
			"conversation_id", derefString(func() *string {
				if conversation == nil {
					return nil
				}
				return &conversation.ID
			}()),
			"target_mailbox_handle", handle,
			"available_handles", sortedMapKeys(handleToID),
		)
		return nil, nil
	}

	intent := normalizeSupportTriageIntent(parsed.Intent)
	reason := strings.TrimSpace(parsed.Reason)
	confidence := normalizedSupportTriageConfidence(parsed.Confidence)
	if confidence != parsed.Confidence {
		slog.WarnContext(ctx, "support triage AI returned invalid confidence",
			"workspace_id", workspaceID,
			"conversation_id", derefString(func() *string {
				if conversation == nil {
					return nil
				}
				return &conversation.ID
			}()),
			"target_mailbox_handle", handle,
			"raw_confidence", parsed.Confidence,
			"normalized_confidence", confidence,
		)
	}
	slog.InfoContext(ctx, "support triage AI mailbox resolved",
		"workspace_id", workspaceID,
		"conversation_id", derefString(func() *string {
			if conversation == nil {
				return nil
			}
			return &conversation.ID
		}()),
		"target_mailbox_handle", handle,
		"target_mailbox_id", targetMailboxID,
	)
	return &supportInboxTriageResult{
		Intent:             &intent,
		Confidence:         &confidence,
		Reason:             strPtr(reason),
		ClassifierSource:   model.SupportConversationTriageSourceAI,
		SuggestedMailboxID: &targetMailboxID,
		SuggestedHandle:    handle,
	}, nil
}

func (s *SupportInboxTriageService) aiMailboxOptions(ctx context.Context, workspaceID string) ([]supportTriageMailboxOption, map[string]string, error) {
	mailboxes, err := s.mailboxRepo.ListByWorkspace(ctx, workspaceID, false)
	if err != nil {
		return nil, nil, err
	}

	options := []supportTriageMailboxOption{
		{
			ID:     nil,
			Name:   "Shared Inbox",
			Handle: "shared",
			Prompt: "General support and uncategorized conversations that do not clearly belong in another inbox.",
		},
	}
	handleToID := make(map[string]string, len(mailboxes))
	for _, mailbox := range mailboxes {
		if !mailbox.Active || !mailbox.TriageEligible {
			continue
		}
		mailboxID := mailbox.ID
		prompt := strings.TrimSpace(derefString(mailbox.RoutingPrompt))
		if prompt == "" {
			prompt = strings.TrimSpace(derefString(mailbox.Description))
		}
		if prompt == "" {
			prompt = mailbox.Name
		}
		options = append(options, supportTriageMailboxOption{
			ID:     &mailboxID,
			Name:   mailbox.Name,
			Handle: mailbox.Handle,
			Prompt: prompt,
		})
		handleToID[strings.ToLower(strings.TrimSpace(mailbox.Handle))] = mailbox.ID
	}
	return options, handleToID, nil
}

func summarizeTriageMailboxOptions(options []supportTriageMailboxOption) []string {
	if len(options) == 0 {
		return nil
	}
	summary := make([]string, 0, len(options))
	for _, option := range options {
		summary = append(summary, strings.TrimSpace(option.Handle)+":"+safeLogPreview(strings.TrimSpace(option.Prompt), 120))
	}
	return summary
}

func sortedMapKeys(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *SupportInboxTriageService) shouldAutoMove(settings model.SupportInboxSettings, conversation *model.SupportConversation, messages []model.SupportMessage, triage *model.SupportConversationTriage) bool {
	if triage == nil || triage.SuggestedMailboxID == nil {
		return false
	}
	if !supportTriageSourceCanAutoMove(triage.ClassifierSource) {
		return false
	}
	if !settings.TriageAutoMoveEnabled {
		return false
	}
	if supportConversationChannel(conversation) == "internal" || supportConversationChannel(conversation) == "api" {
		return false
	}
	if sameMailboxID(conversation.MailboxID, triage.SuggestedMailboxID) {
		return false
	}
	if supportConversationHumanOwned(conversation) {
		return false
	}
	if !mailboxIsSharedOrDefault(conversation.MailboxID, settings.DefaultMailboxID) {
		return false
	}
	if hasHumanReply(messages) {
		return false
	}
	confidence := 0.0
	if triage.Confidence != nil {
		confidence = *triage.Confidence
	}
	return confidence >= settings.TriageConfidenceThreshold
}

func supportTriageSourceCanAutoMove(source string) bool {
	switch strings.TrimSpace(source) {
	case model.SupportConversationTriageSourceRule, model.SupportConversationTriageSourceAI:
		return true
	default:
		return false
	}
}

func (s *SupportInboxTriageService) publishConversationUpdated(workspaceID, conversationID, actorID string) {
	if s == nil || s.supportService == nil || s.supportService.wsPublisher == nil {
		return
	}
	s.supportService.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})
}

func (s *SupportInboxTriageService) createSystemMessage(ctx context.Context, workspaceID, conversationID string, actorUserID *string, senderType, fallbackDisplayName, content string, isInternal bool, eventType model.SupportSystemEventType) {
	if s == nil || s.supportService == nil || s.supportService.messageRepo == nil || strings.TrimSpace(content) == "" {
		return
	}

	displayName := strings.TrimSpace(fallbackDisplayName)
	var avatarURL *string
	if actorUserID != nil && strings.TrimSpace(*actorUserID) != "" && s.supportService.userRepo != nil {
		user, err := s.supportService.userRepo.GetByID(ctx, strings.TrimSpace(*actorUserID))
		if err == nil && user != nil {
			displayName = strings.TrimSpace(user.FullName)
			avatarURL = user.AvatarURL
		}
	}
	if displayName == "" {
		displayName = "Routing"
	}
	messageContent := strings.TrimSpace(content)
	if eventType == model.SystemEventTriageDismissed && actorUserID != nil {
		if firstName := supportSystemFirstName(displayName); firstName != "" {
			messageContent = fmt.Sprintf("%s dismissed the routing suggestion.", firstName)
		}
	}

	msg := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        senderType,
		SenderUserID:      actorUserID,
		SenderDisplayName: &displayName,
		SenderAvatarURL:   avatarURL,
		Content:           messageContent,
		IsInternal:        isInternal,
		MessageType:       "system",
		SystemEventType:   model.SupportSystemEventTypeStrPtr(eventType),
	}
	if err := s.supportService.messageRepo.Create(ctx, msg); err != nil {
		slog.ErrorContext(ctx, "create support routing system message", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
		return
	}
	if s.supportService.wsPublisher != nil {
		s.supportService.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, msg, derefString(actorUserID)))
	}
}

func (s *SupportInboxTriageService) loadMailboxName(ctx context.Context, workspaceID string, mailboxID *string) string {
	if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" || s.mailboxRepo == nil {
		return "Shared Inbox"
	}
	mailbox, err := s.mailboxRepo.GetByID(ctx, workspaceID, strings.TrimSpace(*mailboxID))
	if err != nil || mailbox == nil {
		return "Selected Inbox"
	}
	return mailbox.Name
}

func triageChannelEnabled(settings model.SupportInboxSettings, channel string) bool {
	switch channel {
	case "email":
		return settings.TriageEmailEnabled
	case "internal", "api":
		return settings.TriageInternalEnabled
	default:
		return settings.TriageWidgetEnabled
	}
}

func supportConversationChannel(conversation *model.SupportConversation) string {
	if conversation == nil {
		return "widget"
	}
	if trimmed := strings.ToLower(strings.TrimSpace(conversation.Source)); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.ToLower(strings.TrimSpace(conversation.Channel)); trimmed != "" {
		return trimmed
	}
	return "widget"
}

func supportEmailDomain(email *string) string {
	value := strings.ToLower(strings.TrimSpace(derefString(email)))
	parts := strings.Split(value, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

func firstCustomerReplyMessage(messages []model.SupportMessage) *model.SupportMessage {
	for idx := range messages {
		message := messages[idx]
		if message.IsInternal || strings.TrimSpace(message.MessageType) != "reply" || message.SenderType != "customer" {
			continue
		}
		return &messages[idx]
	}
	return nil
}

func hasHumanReply(messages []model.SupportMessage) bool {
	for _, message := range messages {
		if message.IsInternal || strings.TrimSpace(message.MessageType) != "reply" {
			continue
		}
		if message.SenderType == "user" || message.SenderType == "agent" {
			return true
		}
	}
	return false
}

func mailboxIsSharedOrDefault(currentMailboxID, defaultMailboxID *string) bool {
	if currentMailboxID == nil || strings.TrimSpace(*currentMailboxID) == "" {
		return true
	}
	if defaultMailboxID == nil {
		return false
	}
	return strings.TrimSpace(*currentMailboxID) == strings.TrimSpace(*defaultMailboxID)
}

func triageConditionsMatch(conditions model.SupportTriageRuleConditions, combinedText, senderEmail, emailDomain string) bool {
	groupCount := 0
	matchedGroups := 0

	if len(conditions.PhraseContains) > 0 {
		groupCount++
		phraseMatched := false
		for _, phrase := range conditions.PhraseContains {
			trimmed := strings.ToLower(strings.TrimSpace(phrase))
			if trimmed != "" && strings.Contains(combinedText, trimmed) {
				phraseMatched = true
				break
			}
		}
		if phraseMatched {
			matchedGroups++
		} else if normalizeTriageConditionLogic(conditions.ConditionLogic) == "all" {
			return false
		}
	}

	if len(conditions.EmailDomainEquals) > 0 {
		groupCount++
		domainMatched := false
		for _, domain := range conditions.EmailDomainEquals {
			if strings.EqualFold(strings.TrimSpace(domain), emailDomain) {
				domainMatched = true
				break
			}
		}
		if domainMatched {
			matchedGroups++
		} else if normalizeTriageConditionLogic(conditions.ConditionLogic) == "all" {
			return false
		}
	}

	if len(conditions.SenderEmailContains) > 0 {
		groupCount++
		emailMatched := false
		for _, value := range conditions.SenderEmailContains {
			trimmed := strings.ToLower(strings.TrimSpace(value))
			if trimmed != "" && strings.Contains(senderEmail, trimmed) {
				emailMatched = true
				break
			}
		}
		if emailMatched {
			matchedGroups++
		} else if normalizeTriageConditionLogic(conditions.ConditionLogic) == "all" {
			return false
		}
	}

	if groupCount == 0 {
		return true
	}
	if normalizeTriageConditionLogic(conditions.ConditionLogic) == "any" {
		return matchedGroups > 0
	}
	return matchedGroups == groupCount
}

func normalizeTriageConditionLogic(logic string) string {
	if strings.EqualFold(strings.TrimSpace(logic), "any") {
		return "any"
	}
	return "all"
}

func normalizeTriageChannels(channels []string) model.DocsStringArray {
	if channels == nil {
		return nil
	}
	normalized := make([]string, 0, len(channels))
	seen := map[string]struct{}{}
	for _, channel := range channels {
		trimmed := strings.ToLower(strings.TrimSpace(channel))
		if trimmed == "" || !validSupportTriageChannel(trimmed) {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return model.DocsStringArray(normalized)
}

func normalizeTriageConditions(conditions model.SupportTriageRuleConditions) model.SupportTriageRuleConditions {
	return model.SupportTriageRuleConditions{
		ConditionLogic:      normalizeTriageConditionLogic(conditions.ConditionLogic),
		PhraseContains:      normalizeTriageStringList(conditions.PhraseContains, false),
		EmailDomainEquals:   normalizeTriageStringList(conditions.EmailDomainEquals, true),
		SenderEmailContains: normalizeTriageStringList(conditions.SenderEmailContains, true),
	}
}

func normalizeTriageStringList(values []string, lower bool) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if lower {
			trimmed = strings.ToLower(trimmed)
		}
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func normalizeSupportTriageIntent(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return "uncategorized"
	}
	replacer := strings.NewReplacer(" ", "_", "-", "_", "/", "_")
	return replacer.Replace(trimmed)
}

func normalizedSupportTriageConfidence(value float64) float64 {
	if value < 0 || value > 1 {
		return 0
	}
	return value
}

func validSupportTriageChannel(channel string) bool {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "widget", "email", "internal", "api":
		return true
	default:
		return false
	}
}

func containsTriageChannel(channels []string, target string) bool {
	for _, channel := range channels {
		if strings.EqualFold(strings.TrimSpace(channel), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func buildSupportTriageInputHash(subject, content, customerEmail, channel string) string {
	normalized := strings.Join([]string{
		strings.ToLower(strings.Join(strings.Fields(subject), " ")),
		strings.ToLower(strings.Join(strings.Fields(content), " ")),
		strings.ToLower(strings.TrimSpace(customerEmail)),
		strings.ToLower(strings.TrimSpace(channel)),
	}, "\n")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func triageResultFromPayload(payload model.JSONB) *supportInboxTriageResult {
	if payload == nil {
		return nil
	}

	source := strings.TrimSpace(asString(payload["classifier_source"]))
	targetMailboxID := strings.TrimSpace(asString(payload["suggested_mailbox_id"]))
	if source == "" || targetMailboxID == "" {
		return nil
	}

	intent := normalizeSupportTriageIntent(asString(payload["intent"]))
	reason := strings.TrimSpace(asString(payload["reason"]))
	confidenceValue, _ := payload["confidence"].(float64)
	return &supportInboxTriageResult{
		Intent:             &intent,
		Confidence:         &confidenceValue,
		Reason:             strPtr(reason),
		ClassifierSource:   source,
		SuggestedMailboxID: &targetMailboxID,
	}
}

func triageEventPayload(triage *model.SupportConversationTriage, extra map[string]any) model.JSONB {
	payload := model.JSONB{}
	if triage != nil {
		payload["status"] = triage.Status
		payload["intent"] = derefString(triage.Intent)
		payload["confidence"] = derefFloat64(triage.Confidence)
		payload["reason"] = derefString(triage.Reason)
		payload["classifier_source"] = triage.ClassifierSource
		payload["suggested_mailbox_id"] = derefString(triage.SuggestedMailboxID)
		payload["feedback_action"] = derefString(triage.FeedbackAction)
		payload["auto_moved"] = triage.AutoMoved
	}
	for key, value := range extra {
		payload[key] = value
	}
	return payload
}

func derefFloat64(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func normalizeMailboxIDPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func sameMailboxID(left, right *string) bool {
	return strings.TrimSpace(derefString(left)) == strings.TrimSpace(derefString(right))
}

func autoMoveMessage(source string, mailboxID *string, mailboxName string) string {
	switch source {
	case model.SupportConversationTriageSourceRule:
		return fmt.Sprintf("Routing rule moved to inbox '%s'.", mailboxName)
	default:
		return fmt.Sprintf("AI routing moved to inbox '%s'.", mailboxName)
	}
}

const supportTriageSystemPrompt = `You classify inbound support conversations into the most relevant existing inbox.

Return JSON only.

Rules:
- Choose only from the provided mailbox handles.
- Do not invent mailbox handles.
- If none of the provided mailboxes is a clear fit, return "shared".
- Choose "shared" unless the customer message contains direct evidence for a specialized inbox.
- Confidence must be between 0 and 1.
- Reason must be one short, concrete sentence.`

func supportTriageJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"intent": map[string]any{
				"type": "string",
			},
			"target_mailbox_handle": map[string]any{
				"type": "string",
			},
			"confidence": map[string]any{
				"type": "number",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "One short, concrete sentence.",
			},
		},
		"required":             []string{"intent", "target_mailbox_handle", "confidence", "reason"},
		"additionalProperties": false,
	}
}

func buildSupportTriagePrompt(conversation *model.SupportConversation, inputContent string, options []supportTriageMailboxOption) string {
	var mailboxLines []string
	for _, option := range options {
		mailboxLines = append(mailboxLines, fmt.Sprintf("- %s (%s): %s", option.Name, option.Handle, option.Prompt))
	}
	emailValue := strings.TrimSpace(derefString(conversation.CustomerEmail))
	domain := supportEmailDomain(conversation.CustomerEmail)
	return fmt.Sprintf(`Conversation subject:
%s

First customer message:
%s

Channel: %s
Customer email: %s
Customer domain: %s

Mailbox options:
%s

Return this JSON shape:
{
  "intent": "short_snake_case_label",
  "target_mailbox_handle": "one_of_the_handles_or_shared",
  "confidence": 0.0,
  "reason": "one short, concrete sentence"
}`, strings.TrimSpace(conversation.Subject), strings.TrimSpace(inputContent), supportConversationChannel(conversation), emailValue, domain, strings.Join(mailboxLines, "\n"))
}
