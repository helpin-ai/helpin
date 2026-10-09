package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type supportFollowUpDecision struct {
	Action           string   `json:"action"`
	Reason           string   `json:"reason"`
	Question         string   `json:"question"`
	ClosureNotice    string   `json:"closure_notice"`
	ObligationsClear bool     `json:"obligations_clear"`
	SourceMessageIDs []string `json:"source_message_ids"`
}

func validateSupportFollowUpDecision(d supportFollowUpDecision) error {
	if d.Action != "follow_up" && d.Action != "handoff" && d.Action != "skip" {
		return fmt.Errorf("invalid follow-up action")
	}
	if strings.TrimSpace(d.Reason) == "" || len(d.Reason) > 1000 {
		return fmt.Errorf("a short assessment reason is required")
	}
	if d.Action != "follow_up" {
		return nil
	}
	if !d.ObligationsClear || strings.TrimSpace(d.Question) == "" || strings.TrimSpace(d.ClosureNotice) == "" || len(d.Question) > 1000 || len(d.ClosureNotice) > 600 || len(d.SourceMessageIDs) == 0 || len(d.SourceMessageIDs) > 30 {
		return fmt.Errorf("follow-up requires grounded question, closure notice and no outstanding obligations")
	}
	if len(supportReplyInternalProcessDisclosures(d.Question+" "+d.ClosureNotice)) > 0 {
		return fmt.Errorf("rewrite follow-up in customer-facing language")
	}
	return nil
}

// Keep timing out of the final notice even if an assessment ignores the prompt.
var followUpDurationWords = regexp.MustCompile(`(?i)\b(hours?|minutes?|days?|weeks?|tomorrow|tonight|today|noon|midnight|monday|tuesday|wednesday|thursday|friday|saturday|sunday|january|february|march|april|june|july|august|september|october|november|december|horas?|minutos?|días?|heures?|jours?|stunden?|tage?|ore|giorni|uur|dagen|horas|dias|час|часа|часов|дня|дней)\b|小時|小时|分鐘|分钟|ساع|گھنٹ|घंट`)

var followUpPrematureClosure = regexp.MustCompile(`(?i)\b(close|closes|closing|closed|closure)\b`)

func validateDeadlineFreeFollowUp(d supportFollowUpDecision) error {
	if strings.IndexFunc(d.ClosureNotice, unicode.IsDigit) >= 0 || followUpDurationWords.MatchString(d.ClosureNotice) || followUpDurationWords.MatchString(d.Question) || followUpPrematureClosure.MatchString(d.Question) {
		return fmt.Errorf("final reminder must say closing shortly and reply anytime, without a time, date or duration")
	}
	return nil
}

// Complete applies one Runtime assessment under conversation and episode locks.
// Duplicate callbacks return the persisted outcome without publishing again.
func (s *SupportFollowUpService) Complete(ctx context.Context, run *model.AgentRun, id string, d supportFollowUpDecision) (string, error) {
	if err := validateSupportFollowUpDecision(d); err != nil {
		return "", err
	}
	now := s.now().UTC()
	status := "suppressed"
	var sent *model.SupportMessage
	err := s.repo.WithEpisode(ctx, run.WorkspaceID, id, func(tx *gorm.DB, inst *model.SupportWidgetInstallation, conv *model.SupportConversation, e *model.SupportAIFollowUp) error {
		if e.RunID != run.ID || e.ConversationID != run.TargetID {
			return fmt.Errorf("follow-up run does not match episode")
		}
		if e.Status != "assessing" {
			status = e.Status
			return nil
		}
		if !model.IsAgentRunActiveStatus(run.Status) {
			return fmt.Errorf("assessment run is no longer active")
		}
		settings := parseSettings(inst.Settings)
		channel, err := supportFollowUpReplyChannel(ctx, tx, conv)
		if err != nil {
			return err
		}
		if !inst.Active || run.AgentID != derefString(settings.AIAgentID) || !supportFollowUpEligible(conv, e, settings, channel) {
			return finishFollowUpRow(tx, e, "cancelled", "conversation_changed", now)
		}
		if now.Before(e.DueAt) || (e.StartedAt != nil && now.Sub(*e.StartedAt) > time.Hour) {
			var err error
			sent, err = s.stopWithFailure(ctx, tx, conv, e, "assessment_expired", now)
			status = "failed"
			return err
		}
		busy, err := repository.SupportConversationBusy(ctx, tx, conv.WorkspaceID, conv.ID, e.RunID)
		if err != nil {
			return err
		}
		if busy {
			return finishFollowUpRow(tx, e, "cancelled", "conversation_busy", now)
		}
		if d.Action == "skip" {
			status = "skipped"
			return finishFollowUpRow(tx, e, status, d.Reason, now)
		}
		if d.Action == "handoff" {
			status = "handoff"
			var err error
			sent, err = s.finishHandoff(ctx, tx, conv, e, settings, d.Reason, now)
			return err
		}
		if e.SequenceVersion >= 2 {
			if err := validateDeadlineFreeFollowUp(d); err != nil {
				return err
			}
		}
		seenSource := false
		for _, id := range d.SourceMessageIDs {
			if id == e.SourceMessageID {
				seenSource = true
			}
		}
		if !seenSource {
			return fmt.Errorf("assessment must cite the triggering AI message")
		}
		var messages []model.SupportMessage
		if err := tx.Where("workspace_id = ? AND conversation_id = ? AND id IN ? AND is_internal = false AND message_type = 'reply'", conv.WorkspaceID, conv.ID, d.SourceMessageIDs).Find(&messages).Error; err != nil {
			return err
		}
		if len(messages) != len(d.SourceMessageIDs) {
			return fmt.Errorf("assessment citations must be public messages in this conversation")
		}
		metadata, _ := json.Marshal(map[string]any{"ai_auto_reply": true, "ai_agent_id": run.AgentID, "ai_model": "agent-runtime", "ai_reply_kind": "inactivity_follow_up", "support_follow_up_id": e.ID})
		content := strings.TrimSpace(d.Question)
		if e.SequenceVersion < 2 {
			content += "\n\n" + strings.TrimSpace(d.ClosureNotice)
		}
		content = stripConversationPII(content, conv.CustomerEmail, conv.CustomerPhone)
		messageID := uuid.NewString()
		closeAt := now.Add(time.Duration(e.CloseHours) * time.Hour)
		// Persist the sent ID before insertion so the message projection's
		// cancellation trigger recognizes this episode's own follow-up.
		fields := map[string]any{"status": "waiting", "reason": d.Reason, "sent_message_id": messageID, "sent_at": now, "close_at": closeAt, "due_at": closeAt, "lease_until": nil, "updated_at": now}
		if e.SequenceVersion >= 2 {
			fields["close_at"] = nil
			fields["due_at"] = now.Add(time.Duration(e.SecondDelayHours) * time.Hour)
			fields["closing_notice"] = stripConversationPII(strings.TrimSpace(d.ClosureNotice), conv.CustomerEmail, conv.CustomerPhone)
		}
		if err := tx.Model(e).Updates(fields).Error; err != nil {
			return err
		}
		sent = &model.SupportMessage{ID: messageID, WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "ai", SenderAgentID: &run.AgentID, SenderDisplayName: strPtr(helpinAIDisplayName), Content: content, MessageType: "reply", Metadata: string(metadata), CreatedAt: now}
		setSupportFollowUpEmailDelivery(sent, channel)
		if err := s.chat.messageRepo.WithTx(tx).Create(ctx, sent); err != nil {
			return err
		}
		if err := repository.NewSupportConversationRepository(tx).UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{"ai_turn_count": gorm.Expr("ai_turn_count + 1")}); err != nil {
			return err
		}
		status = "waiting"
		return nil
	})
	if err != nil {
		return "", err
	}
	if sent != nil {
		s.publishFollowUpMessage(ctx, run.WorkspaceID, sent)
	}
	if s.chat.supportAIService.wsPublisher != nil {
		s.chat.supportAIService.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "support_conversation", EntityID: run.TargetID, WorkspaceID: run.WorkspaceID})
	}
	return status, nil
}

func (s *SupportFollowUpService) closeIfDue(ctx context.Context, tx *gorm.DB, conv *model.SupportConversation, e *model.SupportAIFollowUp, now time.Time) error {
	if e.CloseAt == nil || e.SentAt == nil || e.SentMessageID == nil || now.Before(*e.CloseAt) {
		return nil
	}
	busy, err := repository.SupportConversationBusy(ctx, tx, conv.WorkspaceID, conv.ID, "")
	if err != nil {
		return err
	}
	if busy {
		return nil
	}

	// Email delivery uses the existing outbound log. Never close an email
	// thread whose message has not been accepted, or any known failed delivery.
	var logs []model.SupportEmailLog
	if tx.Dialector.Name() == "postgres" {
		if err := tx.Where("workspace_id = ? AND conversation_id = ? AND direction = 'outbound' AND ? = ANY(message_ids)", conv.WorkspaceID, conv.ID, *e.SentMessageID).Find(&logs).Error; err != nil {
			return err
		}
	}
	message, err := repository.NewSupportMessageRepository(tx).GetByID(ctx, *e.SentMessageID)
	if err != nil {
		return err
	}
	emailDelivery := model.SupportAIReplyChannel(conv, message) == "email" || (message != nil && message.DeliveryMode() == model.SupportDeliveryEmailOnly)
	delivered := message != nil && !emailDelivery
	acceptedAt := *e.SentAt
	for _, log := range logs {
		if log.Status == "failed" || log.Status == "spam_complaint" || log.Status == "bounced" || log.BouncedAt != nil {
			return finishFollowUpRow(tx, e, "failed", "follow_up_delivery_failed", now)
		}
		if log.Status == "sent" || log.Status == "delivered" || log.Status == "opened" {
			delivered = true
			if log.CreatedAt.After(acceptedAt) {
				acceptedAt = log.CreatedAt
			}
			if log.DeliveredAt != nil && log.DeliveredAt.After(acceptedAt) {
				acceptedAt = *log.DeliveredAt
			}
		}
	}
	if !delivered {
		return finishFollowUpRow(tx, e, "failed", "follow_up_delivery_unconfirmed", now)
	}
	if emailDelivery {
		deadline := acceptedAt.Add(time.Duration(e.CloseHours) * time.Hour)
		if now.Before(deadline) {
			return tx.Model(e).Updates(map[string]any{"close_at": deadline, "due_at": deadline, "lease_until": nil}).Error
		}
	}
	if err := finishFollowUpRow(tx, e, "resolved", "no_reply_after_follow_up", now); err != nil {
		return err
	}
	return resolveSupportAIConversation(ctx, tx, conv, "assumed", now)
}

func resolveSupportAIConversation(ctx context.Context, tx *gorm.DB, conv *model.SupportConversation, resolutionType string, now time.Time) error {
	// An answer accepted before takeover must not resolve the human-owned
	// conversation if its post-publication finalizer runs later.
	scoped := tx.Where("ai_control_version = ? AND NOT coalesce(human_takeover,false) AND assigned_user_id IS NULL AND opened_by_user_id IS NULL AND customer_requested_human_at IS NULL AND anonymized_at IS NULL", conv.AIControlVersion)
	return repository.NewSupportConversationRepository(scoped).UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{
		"status": model.SupportConversationStatusResolved, "flow_state": model.SupportConversationFlowStateResolvedByAI,
		"ai_state": "resolved", "ai_resolution_type": resolutionType, "ai_resolved_at": now, "resolved_at": now,
	})
}

func (s *SupportFollowUpService) handoff(ctx context.Context, tx *gorm.DB, conv *model.SupportConversation, settings model.SupportInboxSettings, now time.Time) error {
	ai := s.chat.supportAIService
	mailboxID := ai.resolveActiveMailboxID(ctx, conv.WorkspaceID, conv.MailboxID)
	if mailboxID == nil {
		mailboxID = ai.resolveConfiguredHandoffMailbox(ctx, conv.WorkspaceID, settings)
	}
	fields := map[string]any{"human_takeover": true, "status": model.SupportConversationStatusOpen, "resolved_at": nil, "closed_at": nil, "customer_awaiting_response": true, "ai_state": "escalated", "ai_escalated_at": now, "flow_state": escalatedConversationFlowState(settings, now), "assigned_agent_id": nil, "assigned_user_id": nil, "mailbox_id": mailboxID}
	if settings.HandoffBehavior != "unassigned" && ai.workspaceRepo != nil {
		selection, err := selectSupportConversationRecipient(ctx, ai.workspaceRepo, ai.mailboxRepo, ai.installationRepo, nil, ai.presence, ai.statusOverrideRepo, supportRecipientSelectorInput{WorkspaceID: conv.WorkspaceID, MailboxID: mailboxID, HandoffBehavior: settings.HandoffBehavior, HandoffTeamID: settings.HandoffTeamID, RequireAvailability: true, UseMailboxAssignment: true, Now: now})
		if err != nil {
			return err
		}
		if selection != nil {
			fields["assigned_user_id"] = selection.UserID
			fields["flow_state"] = model.SupportConversationFlowStateAssignedToHuman
		}
	}
	return repository.NewSupportConversationRepository(tx).UpdateFields(ctx, conv.WorkspaceID, conv.ID, fields)
}

// finishHandoff applies the existing follow-up handoff and its internal briefing.
func (s *SupportFollowUpService) finishHandoff(ctx context.Context, tx *gorm.DB, conv *model.SupportConversation, e *model.SupportAIFollowUp, settings model.SupportInboxSettings, reason string, now time.Time) (*model.SupportMessage, error) {
	if err := finishFollowUpRow(tx, e, "handoff", reason, now); err != nil {
		return nil, err
	}
	if err := s.handoff(ctx, tx, conv, settings, now); err != nil {
		return nil, err
	}
	history, err := s.chat.messageRepo.WithTx(tx).ListByConversation(ctx, conv.WorkspaceID, conv.ID, false)
	if err != nil {
		return nil, err
	}
	// Separate the note from the event at browser timestamp precision too.
	sent := buildSupportHandoffNote(conv, history, "Follow-up assessment: "+reason, SupportHandoffBrief{}, now.Add(time.Millisecond))
	metadata, _ := json.Marshal(map[string]any{"reason": reason, "support_follow_up_id": e.ID, "ai_handoff_brief": true})
	sent.Metadata = string(metadata)
	event := &model.SupportMessage{WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "ai", SenderDisplayName: strPtr(helpinAIDisplayName), MessageType: "system", SystemEventType: strPtr(model.SystemEventAIEscalated), IsInternal: true, Content: "Follow-up assessment requested a teammate: " + reason, Metadata: string(metadata), CreatedAt: now}
	if err := s.chat.messageRepo.WithTx(tx).Create(ctx, event); err != nil {
		return nil, err
	}
	if err := s.chat.messageRepo.WithTx(tx).Create(ctx, sent); err != nil {
		return nil, err
	}
	return sent, nil
}
