package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// SetSupportFollowUpService wires the fenced assessment outcome command.
func (s *InternalCommandService) SetSupportFollowUpService(followUps *SupportFollowUpService) {
	s.supportFollowUpService = followUps
}

func (s *InternalCommandService) registerSupportFollowUpCommand() {
	s.register(InternalCommandDefinition{
		Name: "support.finish_follow_up", Module: "support", Mutating: true, SupportedTargetTypes: supportCommandTargetTypes,
		Tool: &commandtools.RuntimeToolMetadata{CommandName: "support.finish_follow_up", Alias: "finish_support_follow_up", Category: "Support", Description: "Complete a scheduled inactivity assessment. Only callable from its assigned follow-up run. The server validates current ownership and message history before sending or handing off.", InputSchema: map[string]any{
			"type": "object", "properties": map[string]any{
				"action":             map[string]any{"type": "string", "enum": []string{"follow_up", "handoff", "skip"}},
				"reason":             map[string]any{"type": "string"},
				"question":           map[string]any{"type": "string"},
				"closure_notice":     map[string]any{"type": "string", "description": "Separate customer-language SECOND reminder: closing shortly because there was no reply; reply anytime to reopen. Never state a duration, date or deadline. The first question must not mention closing."},
				"obligations_clear":  map[string]any{"type": "boolean"},
				"source_message_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			}, "required": []string{"action", "reason", "question", "closure_notice", "obligations_clear", "source_message_ids"}, "additionalProperties": false,
		}}, Execute: s.executeSupportFollowUp,
	})
}

func (s *InternalCommandService) executeSupportFollowUp(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.supportFollowUpService == nil {
		return nil, fmt.Errorf("support follow-up service unavailable")
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	if run == nil || runInputTriggerType(run) != supportFollowUpTriggerType || run.WorkspaceID != meta.WorkspaceID || run.TargetID != commandConversationTargetID(meta) {
		return nil, fmt.Errorf("not a scheduled support follow-up run")
	}
	var payload model.AgentRunInputPayload
	if err := json.Unmarshal(run.Input, &payload); err != nil {
		return nil, err
	}
	var trigger struct {
		FollowUpID string `json:"follow_up_id"`
	}
	if err := json.Unmarshal(payload.Trigger.Context, &trigger); err != nil {
		return nil, err
	}
	var decision supportFollowUpDecision
	if err := json.Unmarshal(input, &decision); err != nil {
		return nil, err
	}
	status, err := s.supportFollowUpService.Complete(ctx, run, trigger.FollowUpID, decision)
	if err != nil {
		return nil, err
	}
	if status == "waiting" {
		episode, err := s.supportFollowUpService.repo.Latest(ctx, run.WorkspaceID, run.TargetID)
		if err != nil {
			return nil, err
		}
		if episode != nil && episode.ID == trigger.FollowUpID && episode.SentMessageID != nil {
			s.consumeSupportReplyBilling(ctx, run.WorkspaceID, run.TargetID, *episode.SentMessageID)
		}
	}
	return mustJSON(map[string]any{"status": status, "next_action": "Assessment complete. End this run now without calling any more tools."}), nil
}
