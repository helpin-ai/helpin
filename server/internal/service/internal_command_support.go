package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

var supportCommandTargetTypes = []string{"conversation", "support_conversation"}
var supportReadCommandTargetTypes = []string{"workspace", "conversation", "support_conversation", "support_coverage_gap"}

// registerSupportCommands registers command-backed variants of the native
// support tools so delegated runtime runs can use them through the internal
// command executor.
func (s *InternalCommandService) registerSupportCommands() {
	s.register(InternalCommandDefinition{
		Name:                 "support.list_conversation_messages",
		Module:               "support",
		Mutating:             false,
		SupportedTargetTypes: supportReadCommandTargetTypes,
		Tool:                 mustCommandToolMetadata("support.list_conversation_messages"),
		Execute:              s.executeListConversationMessages,
	})
	s.register(InternalCommandDefinition{
		Name:                 "support.draft_reply",
		Module:               "support",
		Mutating:             true,
		SupportedTargetTypes: supportCommandTargetTypes,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "support.draft_reply",
			Alias:       "draft_support_reply",
			Category:    "Support",
			Description: "Draft a support reply for later human approval.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"content": map[string]any{
						"type":        "string",
						"description": "The reply content to send after approval",
					},
					"is_internal": map[string]any{
						"type":        "boolean",
						"description": "Whether this should be saved as an internal-only note",
					},
					"sender_display_name": map[string]any{
						"type":        "string",
						"description": "Optional display name for the drafted response",
					},
				},
				"required": []string{"content"},
			},
		},
		Execute: s.executeDraftSupportReply,
	})
	s.register(InternalCommandDefinition{
		Name:                 "support.update_conversation_status",
		Module:               "support",
		Mutating:             true,
		SupportedTargetTypes: supportCommandTargetTypes,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "support.update_conversation_status",
			Alias:       "update_conversation_status",
			Category:    "Support",
			Description: "Transition the current support conversation to a different status.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{
						"type":        "string",
						"description": "The target conversation status",
					},
					"conversation_id": map[string]any{
						"type":        "string",
						"description": "Optional conversation ID. Defaults to the current conversation target.",
					},
				},
				"required": []string{"status"},
			},
		},
		Execute: s.executeUpdateConversationStatus,
	})
}

func (s *InternalCommandService) executeListConversationMessages(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.supportMessageRepo == nil {
		return nil, fmt.Errorf("support message repository is not configured")
	}
	var req struct {
		ConversationID string `json:"conversation_id"`
		Limit          int    `json:"limit"`
		Offset         int    `json:"offset"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse list conversation messages input: %w", err)
		}
	}
	conversationID := firstNonEmptyCommand(req.ConversationID, commandConversationTargetID(meta))
	if conversationID == "" {
		return nil, fmt.Errorf("no support conversation associated with this run")
	}
	if req.Limit == 0 {
		req.Limit = 20
	}
	if req.Limit < 1 || req.Limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	if req.Offset < 0 {
		return nil, fmt.Errorf("offset must be zero or greater")
	}
	messages, total, err := s.supportMessageRepo.ListConversationPageFromNewest(ctx, meta.WorkspaceID, conversationID, true, req.Limit, req.Offset)
	if err != nil {
		return nil, fmt.Errorf("list ticket messages: %w", err)
	}
	if s.supportAttachmentRepo != nil && len(messages) > 0 {
		messageIDs := make([]string, len(messages))
		for i := range messages {
			messageIDs[i] = messages[i].ID
		}
		attachments, attachmentErr := s.supportAttachmentRepo.ListByMessageIDs(ctx, messageIDs)
		if attachmentErr != nil {
			return nil, fmt.Errorf("list ticket message attachments: %w", attachmentErr)
		}
		byMessage := make(map[string][]model.SupportAttachmentPayload)
		for _, attachment := range attachments {
			if attachment.MessageID == nil {
				continue
			}
			byMessage[*attachment.MessageID] = append(byMessage[*attachment.MessageID], model.SupportAttachmentPayload{
				ID: attachment.ID, FileName: attachment.FileName, FileType: attachment.ContentType,
				FileSize: attachment.FileSize, URL: attachment.PublicURL,
			})
		}
		for i := range messages {
			messages[i].Attachments = byMessage[messages[i].ID]
		}
	}
	type safeAttachment struct {
		ID       string `json:"id"`
		FileName string `json:"file_name"`
		FileType string `json:"file_type"`
		FileSize int64  `json:"file_size"`
		URL      string `json:"url,omitempty"`
	}
	type ticketMessageWithAttachments struct {
		SenderType  string           `json:"sender_type"`
		Content     string           `json:"content"`
		IsInternal  bool             `json:"is_internal"`
		CreatedAt   string           `json:"created_at"`
		Attachments []safeAttachment `json:"attachments,omitempty"`
	}
	resultWithAttachments := make([]ticketMessageWithAttachments, 0, len(messages))
	for _, message := range messages {
		attachments := make([]safeAttachment, 0, len(message.Attachments))
		for _, attachment := range message.Attachments {
			attachments = append(attachments, safeAttachment{
				ID: attachment.ID, FileName: attachment.FileName, FileType: attachment.FileType,
				FileSize: attachment.FileSize, URL: attachment.URL,
			})
		}
		resultWithAttachments = append(resultWithAttachments, ticketMessageWithAttachments{
			SenderType: message.SenderType, Content: message.Content, IsInternal: message.IsInternal,
			CreatedAt: message.CreatedAt.Format(time.RFC3339), Attachments: attachments,
		})
	}
	response := commandPaginationOutput(total, req.Offset, req.Limit, len(resultWithAttachments))
	response["messages"] = resultWithAttachments
	return mustJSON(response), nil
}

// executeDraftSupportReply stages a support draft on the run's output summary
// under the "draft_reply" key. This is the same product-safe staging contract
// the native executors use: the support finalizer reads the run output summary
// after approval and creates the outbound support message from it.
func (s *InternalCommandService) executeDraftSupportReply(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Content           string  `json:"content"`
		IsInternal        bool    `json:"is_internal"`
		SenderDisplayName *string `json:"sender_display_name"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse draft support reply input: %w", err)
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil {
		return nil, err
	}

	draft := map[string]any{
		"content":           content,
		"is_internal":       req.IsInternal,
		"approval_required": true,
	}
	if name := strings.TrimSpace(commandDerefString(req.SenderDisplayName)); name != "" {
		draft["sender_display_name"] = name
	}

	summary := map[string]any{}
	if len(run.OutputSummary) > 0 {
		if err := json.Unmarshal(run.OutputSummary, &summary); err != nil {
			summary = map[string]any{}
		}
	}
	summary["draft_reply"] = draft
	if err := s.agentRunRepo.UpdateOutputSummary(ctx, run.ID, mustJSON(summary)); err != nil {
		return nil, fmt.Errorf("stage support draft on run: %w", err)
	}

	return mustJSON(map[string]any{
		"status":      "drafted",
		"run_id":      run.ID,
		"draft_reply": draft,
		"next_action": "Finish. The drafted reply is queued for human approval; do not send it yourself.",
	}), nil
}

func (s *InternalCommandService) executeUpdateConversationStatus(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.supportConversationRepo == nil {
		return nil, fmt.Errorf("support conversation repository is not configured")
	}
	var req struct {
		Status         string `json:"status"`
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse update conversation status input: %w", err)
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		return nil, fmt.Errorf("status is required")
	}
	conversationID := firstNonEmptyCommand(req.ConversationID, commandConversationTargetID(meta))
	if conversationID == "" {
		return nil, fmt.Errorf("no support conversation associated with this run")
	}
	conversation, err := s.supportConversationRepo.GetByID(ctx, meta.WorkspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	conversation.Status = status
	if err := s.supportConversationRepo.Update(ctx, conversation); err != nil {
		return nil, err
	}
	if s.supportEventPublisher != nil {
		s.supportEventPublisher.Publish(websocket.Event{
			Action:      "updated",
			Entity:      "support_conversation",
			EntityID:    conversationID,
			WorkspaceID: meta.WorkspaceID,
		})
	}
	return mustJSON(map[string]any{
		"conversation_id": conversationID,
		"status":          status,
	}), nil
}

func commandConversationTargetID(meta model.InternalCommandContext) string {
	switch strings.TrimSpace(meta.TargetType) {
	case "conversation", "support_conversation":
		return strings.TrimSpace(meta.TargetID)
	default:
		return ""
	}
}
