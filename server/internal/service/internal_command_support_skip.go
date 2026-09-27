package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type supportSkipReplyInput struct {
	Reason  string `json:"reason"`
	Summary string `json:"summary"`
}

func parseSupportSkipReply(input json.RawMessage) (supportSkipReplyInput, error) {
	var req supportSkipReplyInput
	if err := json.Unmarshal(input, &req); err != nil {
		return req, errCommandInput("invalid no-reply input")
	}
	req.Summary = strings.TrimSpace(req.Summary)
	switch req.Reason {
	case "spam", "automated_message", "needs_review":
	default:
		return req, errCommandInput("reason must be spam, automated_message or needs_review")
	}
	if req.Summary == "" || utf8.RuneCountInString(req.Summary) > 700 {
		return req, errCommandInput("summary must contain 1-700 characters")
	}
	return req, nil
}

func (s *InternalCommandService) registerSupportSkipReplyCommand() {
	s.register(InternalCommandDefinition{
		Name: "support.skip_reply", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "support.skip_reply", Alias: "skip_support_reply", Category: "Support",
			Description: "Finish without contacting the sender. Use spam only for clearly unsolicited spam/phishing with no genuine customer request; automated_message for notifications with no request; needs_review for suspicious mail that a teammate should review silently. Never use this for a genuine inquiry or a customer's report of phishing. Records a short internal reason, marks only clear spam as Spam, and sends no handoff reply. End your turn after success.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"reason":  map[string]any{"type": "string", "enum": []string{"spam", "automated_message", "needs_review"}},
				"summary": map[string]any{"type": "string", "maxLength": 700, "description": "One brief internal sentence explaining why no reply should be sent."},
			}, "required": []string{"reason", "summary"}, "additionalProperties": false},
		}, Execute: s.executeSupportSkipReply,
	})
}

func (s *InternalCommandService) executeSupportSkipReply(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	req, err := parseSupportSkipReply(input)
	if err != nil {
		return nil, err
	}
	ai := s.supportAIService
	if ai == nil || ai.messageRepo == nil || s.supportProcessingRepo == nil {
		return nil, fmt.Errorf("support reply service is not configured")
	}
	conversationID := commandConversationTargetID(meta)
	if conversationID == "" {
		return nil, errCommandInput("skip_support_reply requires a support conversation")
	}
	outcome := mustJSON(map[string]any{"status": "suppressed", "next_action": "No reply was sent. End your turn without further tools or messages."})
	var runID string
	if meta.RunID != "" {
		run, err := s.resolveCommandRun(ctx, meta)
		if err != nil {
			return nil, err
		}
		if run == nil {
			return outcome, nil
		}
		runID = run.ID
	}
	turn, err := s.supportProcessingRepo.LatestProcessingForConversation(ctx, meta.WorkspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if turn == nil {
		return outcome, nil
	}
	var note *model.SupportMessage
	changed := false
	err = ai.messageRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conv model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", meta.WorkspaceID, conversationID).First(&conv).Error; err != nil {
			return err
		}
		if model.SupportAIConversationBlocked(&conv) || (derefString(conv.AIActiveRunID) != runID && (conv.AIControlVersion > 0 || derefString(conv.AIActiveRunID) != "")) {
			return nil
		}
		var current model.AIMessageProcessing
		if err := tx.Where("id = ?", turn.ID).First(&current).Error; err != nil {
			return err
		}
		if current.Status != "processing" && current.Status != "waiting_for_result" {
			return nil
		}
		// A decision about old mail must not hide a newer customer's request.
		if conv.LastCustomerMessageID != nil && *conv.LastCustomerMessageID != turn.SourceMessageID {
			return errCommandInput("A newer customer message arrived. Review it before deciding no reply is needed.")
		}
		var source model.SupportMessage
		if err := tx.Where("workspace_id = ? AND conversation_id = ? AND id = ? AND sender_type = ? AND is_internal = ?", meta.WorkspaceID, conversationID, turn.SourceMessageID, "customer", false).First(&source).Error; err != nil {
			return err
		}
		metadata := map[string]any{}
		_ = json.Unmarshal([]byte(source.Metadata), &metadata)
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["email_ai_request"] = false
		metadata["email_ai_suppression_reason"] = req.Reason
		raw, _ := json.Marshal(metadata)
		if err := tx.Model(&source).Update("metadata", string(raw)).Error; err != nil {
			return err
		}
		fields := map[string]any{"updated_at": time.Now().UTC()}
		if req.Reason == "spam" || req.Reason == "needs_review" {
			fields["ai_active_run_id"] = nil
			fields["ai_control_version"] = conv.AIControlVersion + 1
			fields["assigned_agent_id"] = nil
			fields["ai_state"] = nil
			if req.Reason == "spam" {
				fields["status"] = "spam"
				fields["flow_state"] = nil
				fields["handoff_state"] = nil
			} else {
				fields["human_takeover"] = true
				fields["ai_paused_at"] = time.Now().UTC()
				fields["flow_state"] = model.SupportConversationFlowStateWaitingForHuman
			}
		}
		if err := tx.Model(&conv).Updates(fields).Error; err != nil {
			return err
		}
		title := "AI skipped reply"
		if req.Reason == "spam" {
			title = "AI marked as spam"
		} else if req.Reason == "needs_review" {
			title = "AI left for review without replying"
		}
		noteMetadata, _ := json.Marshal(map[string]any{"ai_no_reply": true, "reason": req.Reason, "source_message_ids": []string{source.ID}})
		note = &model.SupportMessage{WorkspaceID: meta.WorkspaceID, ConversationID: conversationID, SenderType: "agent", SenderDisplayName: strPtr(helpinAIDisplayName), MessageType: "note", IsInternal: true, Content: title + "\n\n" + req.Summary, Metadata: string(noteMetadata)}
		if err := repository.NewSupportMessageRepository(tx).Create(ctx, note); err != nil {
			return err
		}
		if err := repository.NewAIMessageProcessingRepository(tx).MarkCompleted(ctx, turn.ID, nil, 0); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if changed {
		if ai.wsPublisher != nil {
			ai.wsPublisher.Publish(websocket.SupportMessageEvent(meta.WorkspaceID, note, "ai:no_reply"))
			ai.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "support_conversation", EntityID: conversationID, WorkspaceID: meta.WorkspaceID})
		}
		ai.publishTypingIndicator(ctx, meta.WorkspaceID, conversationID, false)
		if req.Reason != "automated_message" {
			s.closeEscalatedSupportCommandRun(ctx, meta)
		}
	}
	return outcome, nil
}
