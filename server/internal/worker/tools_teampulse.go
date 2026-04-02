package worker

import (
	"encoding/json"
	"fmt"
	"strings"
)

func toolAddStoryComment(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if ctx.StoryID == "" {
		return "", fmt.Errorf("no task associated with this run")
	}

	if err := ctx.Services.AddComment(ctx.Context, ctx.WorkspaceID, ctx.StoryID, ctx.AgentID, params.Content); err != nil {
		return "", fmt.Errorf("add comment: %w", err)
	}

	return "Comment added to task.", nil
}

func toolUpdateStoryState(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		StateID string `json:"state_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if ctx.StoryID == "" {
		return "", fmt.Errorf("no task associated with this run")
	}

	commandInput, _ := json.Marshal(map[string]any{
		"task_id":  ctx.StoryID,
		"state_id": params.StateID,
	})
	if output, ok, err := executeInternalCommand(ctx, "task", ctx.StoryID, "pm.update_task_state", commandInput); ok {
		if err != nil {
			return "", fmt.Errorf("update task state: %w", err)
		}
		return string(output), nil
	}

	if err := ctx.Services.UpdateStoryState(ctx.Context, ctx.WorkspaceID, ctx.StoryID, params.StateID); err != nil {
		return "", fmt.Errorf("update task state: %w", err)
	}

	return fmt.Sprintf("Task state updated to %s.", params.StateID), nil
}

func toolListStoryChecklist(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.StoryID == "" {
		return "", fmt.Errorf("no task associated with this run")
	}

	items, err := ctx.Services.ListChecklist(ctx.Context, ctx.WorkspaceID, ctx.StoryID)
	if err != nil {
		return "", fmt.Errorf("list checklist: %w", err)
	}

	if len(items) == 0 {
		return "No checklist items.", nil
	}

	var lines []string
	for _, item := range items {
		status := "[ ]"
		if item.Completed {
			status = "[x]"
		}
		lines = append(lines, fmt.Sprintf("%s %s", status, item.Text))
	}

	return toCompactJSONString(lines), nil
}

func toolListConversationMessages(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.ConversationID == "" || ctx.Services == nil || ctx.Services.ListConversationMessages == nil {
		return "", fmt.Errorf("no support conversation associated with this run")
	}

	messages, err := ctx.Services.ListConversationMessages(ctx.Context, ctx.WorkspaceID, ctx.ConversationID)
	if err != nil {
		return "", fmt.Errorf("list ticket messages: %w", err)
	}
	if len(messages) == 0 {
		return "No ticket messages.", nil
	}

	type ticketMessage struct {
		SenderType string `json:"sender_type"`
		Content    string `json:"content"`
		IsInternal bool   `json:"is_internal"`
		CreatedAt  string `json:"created_at"`
	}

	result := make([]ticketMessage, 0, len(messages))
	for _, message := range messages {
		result = append(result, ticketMessage{
			SenderType: message.SenderType,
			Content:    message.Content,
			IsInternal: message.IsInternal,
			CreatedAt:  message.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return toCompactJSONString(result), nil
}

func toolDraftSupportReply(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Content           string  `json:"content"`
		IsInternal        bool    `json:"is_internal"`
		SenderDisplayName *string `json:"sender_display_name"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(params.Content) == "" {
		return "", fmt.Errorf("content is required")
	}

	ctx.PendingSupportDraft = &SupportDraftReply{
		Content:           strings.TrimSpace(params.Content),
		IsInternal:        params.IsInternal,
		SenderDisplayName: params.SenderDisplayName,
		ApprovalRequired:  true,
	}

	return "Support reply drafted and queued for approval.", nil
}

func toolUpdateConversationStatus(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if ctx.ConversationID == "" || ctx.Services == nil || ctx.Services.UpdateConversationStatus == nil {
		return "", fmt.Errorf("no support conversation associated with this run")
	}
	if strings.TrimSpace(params.Status) == "" {
		return "", fmt.Errorf("status is required")
	}

	if err := ctx.Services.UpdateConversationStatus(ctx.Context, ctx.WorkspaceID, ctx.ConversationID, params.Status); err != nil {
		return "", fmt.Errorf("update conversation status: %w", err)
	}
	return fmt.Sprintf("Conversation status updated to %s.", params.Status), nil
}
