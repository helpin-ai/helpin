package worker

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func toolAddTaskComment(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		TaskID  string `json:"task_id"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	content := strings.TrimSpace(params.Content)
	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	taskID := strings.TrimSpace(params.TaskID)
	if taskID == "" {
		taskID = strings.TrimSpace(ctx.TaskID)
	}
	if taskID == "" {
		return "", fmt.Errorf("task_id is required when no task is associated with this run")
	}

	commandInput, _ := json.Marshal(map[string]any{
		"task_id": taskID,
		"content": content,
	})
	if output, ok, err := executeInternalCommand(ctx, "task", taskID, "pm.add_task_comment", commandInput); ok {
		if err != nil {
			return "", fmt.Errorf("add comment: %w", err)
		}
		return string(output), nil
	}

	if err := ctx.Services.AddComment(ctx.Context, ctx.WorkspaceID, taskID, ctx.AgentID, content); err != nil {
		return "", fmt.Errorf("add comment: %w", err)
	}

	return "Comment added to task.", nil
}

func toolEnsureTaskLabel(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if output, ok, err := executeInternalCommand(ctx, "workspace", ctx.WorkspaceID, "pm.ensure_label", input); ok {
		if err != nil {
			return "", fmt.Errorf("ensure task label: %w", err)
		}
		return string(output), nil
	}
	return "", fmt.Errorf("ensure_task_label requires internal commands")
}

func toolListTasks(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if output, ok, err := executeInternalCommand(ctx, "workspace", ctx.WorkspaceID, "pm.list_tasks", input); ok {
		if err != nil {
			return "", fmt.Errorf("list tasks: %w", err)
		}
		return string(output), nil
	}
	return "", fmt.Errorf("list_tasks requires internal commands")
}

func toolUpdateTaskState(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		StateID string `json:"state_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if ctx.TaskID == "" {
		return "", fmt.Errorf("no task associated with this run")
	}

	commandInput, _ := json.Marshal(map[string]any{
		"task_id":  ctx.TaskID,
		"state_id": params.StateID,
	})
	if output, ok, err := executeInternalCommand(ctx, "task", ctx.TaskID, "pm.update_task_state", commandInput); ok {
		if err != nil {
			return "", fmt.Errorf("update task state: %w", err)
		}
		return string(output), nil
	}

	if err := ctx.Services.UpdateTaskState(ctx.Context, ctx.WorkspaceID, ctx.TaskID, params.StateID); err != nil {
		return "", fmt.Errorf("update task state: %w", err)
	}

	return fmt.Sprintf("Task state updated to %s.", params.StateID), nil
}

func toolListTaskChecklist(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.TaskID == "" {
		return "", fmt.Errorf("no task associated with this run")
	}

	items, err := ctx.Services.ListChecklist(ctx.Context, ctx.WorkspaceID, ctx.TaskID)
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

func toolCreateTask(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("execution context is required")
	}
	var params struct {
		Name          string   `json:"name"`
		Description   *string  `json:"description"`
		TaskType      string   `json:"task_type"`
		Estimate      *int     `json:"estimate"`
		Priority      *string  `json:"priority"`
		EpicID        *string  `json:"epic_id"`
		TeamID        string   `json:"team_id"`
		WorkflowID    *string  `json:"workflow_id"`
		StateID       *string  `json:"state_id"`
		OwnerMemberID *string  `json:"owner_member_id"`
		LabelIDs      []string `json:"label_ids"`
		Deadline      *string  `json:"deadline"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	params.Name = strings.TrimSpace(params.Name)
	params.TeamID = strings.TrimSpace(params.TeamID)
	if params.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	if params.TeamID == "" {
		return "", fmt.Errorf("team_id is required")
	}

	trimPtr := func(value **string) {
		if *value == nil {
			return
		}
		trimmed := strings.TrimSpace(**value)
		if trimmed == "" {
			*value = nil
			return
		}
		**value = trimmed
	}
	trimPtr(&params.Description)
	trimPtr(&params.Priority)
	trimPtr(&params.EpicID)
	trimPtr(&params.WorkflowID)
	trimPtr(&params.StateID)
	trimPtr(&params.OwnerMemberID)
	trimPtr(&params.Deadline)

	if params.EpicID == nil && strings.TrimSpace(ctx.TargetType) == "epic" && strings.TrimSpace(ctx.TargetID) != "" {
		epicID := strings.TrimSpace(ctx.TargetID)
		params.EpicID = &epicID
	}

	var deadline *time.Time
	if params.Deadline != nil {
		parsed, err := parseTaskToolDeadline(*params.Deadline)
		if err != nil {
			return "", err
		}
		deadline = parsed
	}

	commandInput, _ := json.Marshal(map[string]any{
		"name":            params.Name,
		"description":     params.Description,
		"task_type":       strings.TrimSpace(params.TaskType),
		"estimate":        params.Estimate,
		"priority":        params.Priority,
		"epic_id":         params.EpicID,
		"team_id":         params.TeamID,
		"workflow_id":     params.WorkflowID,
		"state_id":        params.StateID,
		"owner_member_id": params.OwnerMemberID,
		"label_ids":       params.LabelIDs,
		"deadline":        params.Deadline,
	})
	targetType, targetID := "workspace", ctx.WorkspaceID
	if strings.TrimSpace(ctx.TargetType) == "epic" && strings.TrimSpace(ctx.TargetID) != "" {
		targetType, targetID = "epic", strings.TrimSpace(ctx.TargetID)
	}
	if output, ok, err := executeInternalCommand(ctx, targetType, targetID, "pm.create_task", commandInput); ok {
		if err != nil {
			return "", fmt.Errorf("create task: %w", err)
		}
		return string(output), nil
	}

	if ctx.Services == nil || ctx.Services.CreateTask == nil {
		return "", fmt.Errorf("task creation is not available for this agent")
	}
	result, err := ctx.Services.CreateTask(ctx.Context, ctx.WorkspaceID, ctx.AgentID, CreateTaskToolRequest{
		Name:          params.Name,
		Description:   params.Description,
		TaskType:      strings.TrimSpace(params.TaskType),
		Estimate:      params.Estimate,
		Priority:      params.Priority,
		EpicID:        params.EpicID,
		TeamID:        params.TeamID,
		WorkflowID:    params.WorkflowID,
		StateID:       params.StateID,
		OwnerMemberID: params.OwnerMemberID,
		LabelIDs:      params.LabelIDs,
		Deadline:      deadline,
	})
	if err != nil {
		return "", fmt.Errorf("create task: %w", err)
	}
	return toCompactJSONString(result), nil
}

func parseTaskToolDeadline(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339, time.RFC3339Nano} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("deadline must be YYYY-MM-DD or RFC3339")
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
