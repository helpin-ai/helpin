package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// ChangeConversationAIControl explicitly pauses AI or returns human ownership to AI.
func (s *SupportInboxService) ChangeConversationAIControl(ctx context.Context, workspaceID, conversationID, actorID string, req model.SupportAIControlRequest) error {
	if strings.TrimSpace(actorID) == "" {
		return fmt.Errorf("a teammate is required")
	}
	conv, err := s.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation not found")
	}
	if req.Action != "pause" && req.Action != "return" {
		return fmt.Errorf("action must be pause or return")
	}
	var settings model.SupportInboxSettings
	if req.Action == "return" {
		inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
		if err != nil {
			return err
		}
		if inst == nil || !inst.Active {
			return fmt.Errorf("support AI is not enabled")
		}
		settings = parseSettings(inst.Settings)
		if !shouldAutomaticallyProcessSupportAI(settings) || strings.TrimSpace(derefString(settings.AIAgentID)) == "" {
			return fmt.Errorf("support AI is not enabled")
		}
		agent, err := s.agentRepo.GetByID(ctx, workspaceID, *settings.AIAgentID)
		if err != nil {
			return err
		}
		if agent == nil {
			return fmt.Errorf("support AI agent is unavailable")
		}
		if err := validateAgentTarget(agent, "support_conversation"); err != nil {
			return err
		}
	}
	return s.changeConversationAIControl(ctx, conv, actorID, &req, settings, nil, "paused_by_teammate")
}

// pauseForTeammate is also used by assignment and public reply. It preserves
// existing ownership and does not send an automated visitor-facing message.
func (s *SupportInboxService) pauseForTeammate(ctx context.Context, conv *model.SupportConversation, actorID, reason string, fields map[string]any) error {
	return s.changeConversationAIControl(ctx, conv, actorID, nil, model.SupportInboxSettings{}, fields, reason)
}

func (s *SupportInboxService) changeConversationAIControl(ctx context.Context, conv *model.SupportConversation, actorID string, req *model.SupportAIControlRequest, settings model.SupportInboxSettings, extra map[string]any, reason string) error {
	returning := req != nil && req.Action == "return"
	var history []model.SupportMessage
	if !returning && s.messageRepo != nil {
		var err error
		history, err = s.messageRepo.ListByConversation(ctx, conv.WorkspaceID, conv.ID, false)
		if err != nil {
			slog.WarnContext(ctx, "load AI handoff context", "error", err, "conversation_id", conv.ID)
		}
	}
	var note *model.SupportMessage
	previousRun, changed, err := s.conversationRepo.ChangeAIControl(ctx, conv.WorkspaceID, conv.ID, func(current *model.SupportConversation) (map[string]any, *model.SupportMessage, error) {
		now := time.Now().UTC()
		if current.AnonymizedAt != nil {
			return nil, nil, fmt.Errorf("this conversation is read-only because its customer was deleted")
		}
		if req != nil && current.AIControlVersion != req.ExpectedVersion {
			return nil, nil, repository.ErrSupportAIControlConflict
		}
		fields := make(map[string]any)
		for key, value := range extra {
			fields[key] = value
		}
		if returning {
			if !shouldAutomaticallyProcessSupportAI(settings) || strings.TrimSpace(derefString(settings.AIAgentID)) == "" {
				return nil, nil, fmt.Errorf("support AI is not enabled")
			}
			if current.Status == "resolved" || current.Status == "spam" || !supportAIConversationSupported(current) || !model.SupportAIReplyAllowed(settings, current, nil) {
				return nil, nil, fmt.Errorf("this conversation is not eligible for AI; reopen it and check the enabled reply channels")
			}
			if current.CustomerRequestedHumanAt != nil && !req.ConfirmHumanRequest {
				return nil, nil, fmt.Errorf("confirm returning to AI: the customer requested a human")
			}
			if !model.SupportAIConversationBlocked(current) && derefString(current.AssignedAgentID) == derefString(settings.AIAgentID) && derefString(current.AIState) == "pending" {
				return nil, nil, nil
			}
			fields["human_takeover"] = false
			fields["assigned_user_id"] = nil
			fields["opened_by_user_id"] = nil
			fields["customer_requested_human_at"] = nil
			fields["assigned_agent_id"] = settings.AIAgentID
			fields["ai_state"] = "pending"
			fields["ai_resolved_at"] = nil
			fields["ai_resolution_type"] = nil
			fields["handoff_state"] = nil
			fields["flow_state"] = model.SupportConversationFlowStateAIHandling
			fields["ai_resumed_at"] = now
			fields["ai_paused_at"] = nil
			fields["ai_paused_by_user_id"] = nil
			note = supportControlNote(current, actorID, "Returned to AI. AI will respond to the next customer message.", now)
			if current.CustomerRequestedHumanAt != nil {
				note.Content += "\n\nA teammate explicitly confirmed this return after the customer requested a human."
			}
		} else {
			if req != nil && (current.Status == "spam" || current.Status == "resolved") {
				return nil, nil, fmt.Errorf("reopen the conversation before changing AI control")
			}
			alreadyPaused := current.HumanTakeover != nil && *current.HumanTakeover
			if alreadyPaused && len(extra) == 0 {
				return nil, nil, nil
			}
			fields["human_takeover"] = true
			fields["assigned_agent_id"] = nil
			fields["flow_state"] = model.SupportConversationFlowStateWaitingForHuman
			assigned, opened := current.AssignedUserID, current.OpenedByUserID
			if value, ok := extra["assigned_user_id"]; ok {
				assigned, _ = value.(*string)
			}
			if value, ok := extra["opened_by_user_id"]; ok {
				opened, _ = value.(*string)
			}
			if derefString(assigned) != "" || derefString(opened) != "" {
				fields["flow_state"] = model.SupportConversationFlowStateAssignedToHuman
			}
			if !alreadyPaused {
				fields["ai_paused_at"] = now
				if actorID != "" {
					fields["ai_paused_by_user_id"] = actorID
				}
				// Ordinary human-only conversations do not need an AI briefing.
				if current.AIState != nil || current.AIActiveRunID != nil || req != nil {
					note = buildSupportHandoffNote(current, history, reason, SupportHandoffBrief{}, now)
					if actorID != "" {
						note.SenderUserID = &actorID
					}
				}
			}
		}
		return fields, note, nil
	})
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if s.supportAIService != nil {
		s.supportAIService.cancelControlledRun(ctx, conv.WorkspaceID, conv.ID, previousRun)
	}
	if s.wsPublisher != nil {
		if note != nil {
			s.wsPublisher.Publish(websocket.SupportMessageEvent(conv.WorkspaceID, note, actorID))
		}
		s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "support_conversation", EntityID: conv.ID, WorkspaceID: conv.WorkspaceID, ActorID: actorID})
	}
	return nil
}

func (s *SupportAIService) cancelControlledRun(ctx context.Context, workspaceID, conversationID, runID string) {
	if s.wsPublisher != nil {
		s.publishTypingIndicator(ctx, workspaceID, conversationID, false)
	}
	if s.runCloser == nil || runID == "" {
		return
	}
	cancelCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := s.runCloser.CancelRun(cancelCtx, workspaceID, runID, ""); err != nil {
		slog.WarnContext(ctx, "cancel run after AI control change", "run_id", runID, "error", err)
	}
}
