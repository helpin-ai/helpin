package service

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const supportContinueNextAction = "Acknowledgment delivered; this is not a final answer. Continue the investigation now using your assigned tools, then send_support_reply or escalate_to_human. Do not wait for another visitor message or repeat the acknowledgment. If you launch child research, end this runtime turn after the launch and resume when its result arrives."

func (s *InternalCommandService) registerSupportProgressCommand() {
	s.register(InternalCommandDefinition{
		Name: "support.send_progress", Module: "support", Mutating: true,
		SupportedTargetTypes: []string{"support_conversation"},
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "support.send_progress", Alias: "send_support_progress", Category: "Support",
			Description: "Send one brief acknowledgment before a longer investigation, then CONTINUE using tools in this same run. This is not terminal and does not answer or resolve the customer's request. Use only when a multi-step lookup or external investigation may take time, not for quick answers or immediate handovers. Mention the issue being checked, without factual conclusions, internal tools or promised completion times. Repeated calls in the same customer turn do not send another acknowledgment. A suppressed result means stop.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"content": map[string]any{"type": "string", "minLength": 1, "maxLength": 300, "description": "One short customer-facing acknowledgment of what you are checking. No findings, promises or internal process details."},
			}, "required": []string{"content"}, "additionalProperties": false},
		}, Execute: s.executeSupportProgress,
	})
}

func (s *InternalCommandService) executeSupportProgress(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, errCommandInput("invalid progress input")
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" || utf8.RuneCountInString(req.Content) > 300 {
		return nil, errCommandInput("acknowledgment must contain 1-300 characters")
	}
	if meta.TargetType != "support_conversation" || strings.TrimSpace(meta.RunID) == "" {
		return nil, errCommandInput("progress requires an active support chat run")
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil {
		return nil, err
	}
	if run == nil || runInputTriggerType(run) != supportChatTriggerType {
		return nil, errCommandInput("progress is only available during a live support chat turn")
	}
	payload := mustJSON(map[string]any{"content": req.Content, "reply_kind": supportReplyKindConversational, "confidence": 1})
	return s.executeSupportReply(ctx, meta, payload, true)
}
