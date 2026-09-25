package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type mcpPMCommentLister interface {
	List(context.Context, string, string) ([]model.CommentWithAuthor, error)
}

// SetPMComments wires task comment reads for list_task_comments.
func (s *MCPService) SetPMComments(comments mcpPMCommentLister) {
	if comments != nil {
		s.pmComments = comments
	}
}

func pmParityMCPToolDefinitions() []MCPToolDefinition {
	taskOnly := map[string]any{
		"type":                 "object",
		"properties":           map[string]any{"task_id": map[string]any{"type": "string", "minLength": 1}},
		"required":             []string{"task_id"},
		"additionalProperties": false,
	}
	pmWrite := func(definition MCPToolDefinition) MCPToolDefinition {
		definition.Toolset, definition.Scope = MCPToolsetPM, MCPScopePMWrite
		definition.Permission, definition.Module = authorization.PermPMEdit, model.ModulePM
		definition.Mutating, definition.IdempotentHint = true, true
		definition.InputSchema = withMCPIdempotencyKey(taskOnly)
		return definition
	}
	return []MCPToolDefinition{
		pmWrite(MCPToolDefinition{
			Name: "archive_task", Title: "Archive task", Destructive: true,
			Description: "Archive a task so it leaves boards and lists. Reversible with restore_task.",
		}),
		pmWrite(MCPToolDefinition{
			Name: "restore_task", Title: "Restore task",
			Description: "Restore an archived task.",
		}),
		{
			Name: "list_task_comments", Title: "List task comments",
			Description: "List a task's comments with replies, newest threads last. Authors are returned by ID and name.",
			InputSchema: taskOnly,
			Toolset:     MCPToolsetPM, Scope: MCPScopePMRead, Permission: authorization.PermPMRead, Module: model.ModulePM,
		},
	}
}

func (s *MCPService) executePMParityMCPTool(
	ctx context.Context,
	principal *model.MCPPrincipal,
	name string,
	arguments json.RawMessage,
) (*MCPToolResult, bool, error) {
	switch name {
	case "archive_task", "restore_task", "list_task_comments":
	default:
		return nil, false, nil
	}
	var input struct {
		TaskID string `json:"task_id"`
	}
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, true, err
	}
	task, err := s.accessibleMCPTask(ctx, principal, input.TaskID)
	if err != nil {
		return nil, true, err
	}
	if task == nil {
		return nil, true, ErrMCPNotFound
	}
	if name == "list_task_comments" {
		result, err := s.listMCPTaskComments(ctx, task.Task.ID)
		return result, true, err
	}
	archived := name == "archive_task"
	updated, err := s.tasks.Update(ctx, task.Task.ID, model.UpdateTaskRequest{Archived: &archived}, principal.UserID)
	if err != nil {
		return nil, true, err
	}
	verb := "restored"
	if archived {
		verb = "archived"
	}
	return &MCPToolResult{
		Summary: "Task " + updated.Task.Name + " " + verb + ".",
		Data:    map[string]any{"task_id": updated.Task.ID, "name": updated.Task.Name, "archived": updated.Task.Archived},
	}, true, nil
}

func (s *MCPService) listMCPTaskComments(ctx context.Context, taskID string) (*MCPToolResult, error) {
	if s.pmComments == nil {
		return nil, ErrMCPForbidden
	}
	threads, err := s.pmComments.List(ctx, "task", taskID)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(threads))
	for _, thread := range threads {
		items = append(items, mcpCommentView(thread))
	}
	return &MCPToolResult{Summary: "Returned " + strconv.Itoa(len(items)) + " comment threads.", Data: map[string]any{"items": items}}, nil
}

// mcpCommentView exposes only public comment fields and the author's ID and name.
func mcpCommentView(comment model.CommentWithAuthor) map[string]any {
	view := map[string]any{
		"id":          comment.Comment.ID,
		"body":        comment.Comment.Body,
		"author_id":   comment.Comment.AuthorID,
		"author_name": strings.TrimSpace(comment.Author.FullName),
		"parent_id":   comment.Comment.ParentID,
		"resolved_at": comment.Comment.ResolvedAt,
		"created_at":  comment.Comment.CreatedAt,
	}
	if comment.Comment.AgentName != "" {
		view["agent_name"] = comment.Comment.AgentName
	}
	if len(comment.Replies) > 0 {
		replies := make([]map[string]any, 0, len(comment.Replies))
		for _, reply := range comment.Replies {
			replies = append(replies, mcpCommentView(reply))
		}
		view["replies"] = replies
	}
	return view
}
