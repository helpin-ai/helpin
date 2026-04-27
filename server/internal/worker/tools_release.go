package worker

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func toolGetReleaseContext(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.GetReleaseContext == nil {
		return "", fmt.Errorf("release facts are not available for this agent")
	}
	var params model.GetReleaseContextRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return "", fmt.Errorf("parse input: %w", err)
		}
	}
	applyReleaseDefaults(ctx, &params)
	if strings.TrimSpace(params.TagName) == "" {
		return "", fmt.Errorf("tag_name is required")
	}
	if strings.TrimSpace(params.RepositoryID) == "" && strings.TrimSpace(params.RepoFullName) == "" {
		return "", fmt.Errorf("repository_id or repo_full_name is required")
	}
	result, err := ctx.Services.GetReleaseContext(ctx.Context, ctx.WorkspaceID, params)
	if err != nil {
		return "", fmt.Errorf("get release context: %w", err)
	}
	return toCompactJSONString(result), nil
}

func toolFindTasksForGitChanges(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.FindTasksForGitChanges == nil {
		return "", fmt.Errorf("release facts are not available for this agent")
	}
	var params model.FindTasksForGitChangesRequest
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(params.RepositoryID) == "" && strings.TrimSpace(params.RepoFullName) == "" {
		if ctx.TargetType == "repository" && strings.TrimSpace(ctx.TargetID) != "" {
			params.RepositoryID = ctx.TargetID
		}
		if strings.TrimSpace(params.RepoFullName) == "" && ctx.RunInput != nil && ctx.RunInput.Event != nil && ctx.RunInput.Event.GitHub != nil {
			params.RepoFullName = strings.TrimSpace(ctx.RunInput.Event.GitHub.RepoFullName)
		}
	}
	if strings.TrimSpace(params.RepositoryID) == "" && strings.TrimSpace(params.RepoFullName) == "" {
		return "", fmt.Errorf("repo_full_name or repository_id is required")
	}
	if len(params.PRNumbers) == 0 && len(params.CommitSHAs) == 0 && len(params.Branches) == 0 && len(params.Texts) == 0 {
		return "", fmt.Errorf("at least one evidence array is required")
	}
	result, err := ctx.Services.FindTasksForGitChanges(ctx.Context, ctx.WorkspaceID, params)
	if err != nil {
		return "", fmt.Errorf("find tasks for git changes: %w", err)
	}
	return toCompactJSONString(result), nil
}

func toolGetTaskContext(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx.Services == nil || ctx.Services.GetTaskContext == nil {
		return "", fmt.Errorf("task context is not available for this agent")
	}
	var params model.GetTaskContextRequest
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if len(params.TaskIDs) == 0 {
		return "", fmt.Errorf("task_ids is required")
	}
	result, err := ctx.Services.GetTaskContext(ctx.Context, ctx.WorkspaceID, params)
	if err != nil {
		return "", fmt.Errorf("get task context: %w", err)
	}
	return toCompactJSONString(result), nil
}

func applyReleaseDefaults(ctx *ExecutionContext, params *model.GetReleaseContextRequest) {
	if ctx == nil || params == nil {
		return
	}
	params.RepositoryID = strings.TrimSpace(params.RepositoryID)
	params.RepoFullName = strings.TrimSpace(params.RepoFullName)
	params.TagName = strings.TrimSpace(params.TagName)
	if params.RepositoryID == "" && ctx.TargetType == "repository" && strings.TrimSpace(ctx.TargetID) != "" {
		params.RepositoryID = strings.TrimSpace(ctx.TargetID)
	}
	if ctx.RunInput != nil && ctx.RunInput.Event != nil && ctx.RunInput.Event.GitHub != nil {
		if params.RepoFullName == "" {
			params.RepoFullName = strings.TrimSpace(ctx.RunInput.Event.GitHub.RepoFullName)
		}
		if params.RepositoryID == "" {
			params.RepositoryID = strings.TrimSpace(ctx.RunInput.Event.GitHub.RepositoryID)
		}
		if params.TagName == "" && ctx.RunInput.Event.GitHub.Release != nil {
			params.TagName = strings.TrimSpace(ctx.RunInput.Event.GitHub.Release.TagName)
		}
	}
}
