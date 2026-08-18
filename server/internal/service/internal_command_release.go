package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// registerReleaseFactsCommands registers command-backed variants of the native
// release facts tools. These query Helpin's DB and GitHub App integration and
// are therefore product-owned and command-backed.
//
// Unlike the native tools, the command context does not carry the triggering
// GitHub event payload, so tag/repo defaults from release events are not
// applied here; only repository-target defaulting is preserved. Runtime agents
// receive the trigger event in their run input and pass explicit parameters.
func (s *InternalCommandService) registerReleaseFactsCommands() {
	s.register(InternalCommandDefinition{
		Name:     "release.get_release_context",
		Module:   "release",
		Mutating: false,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "release.get_release_context",
			Alias:       "get_release_context",
			Category:    "Release",
			Description: "Load release metadata, compare commits/files against the previous published release, and resolve related tasks. Defaults the repository from the current repository target when available.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repository_id": map[string]any{
						"type":        "string",
						"description": "Optional repository ID. Defaults from the current repository target when omitted.",
					},
					"repo_full_name": map[string]any{
						"type":        "string",
						"description": "Optional repository full name like owner/repo.",
					},
					"tag_name": map[string]any{
						"type":        "string",
						"description": "The release tag name. Use the tag from the run trigger event when present.",
					},
					"include_changed_files": map[string]any{
						"type":        "boolean",
						"description": "Whether to include changed files from the release comparison.",
					},
					"max_commits": map[string]any{
						"type":        "integer",
						"description": "Maximum number of commits to return. Default 100, max 200.",
					},
					"max_files": map[string]any{
						"type":        "integer",
						"description": "Maximum number of changed files to return when include_changed_files is true. Default 200, max 500.",
					},
				},
				"additionalProperties": false,
			},
		},
		Execute: s.executeGetReleaseContext,
	})
	s.register(InternalCommandDefinition{
		Name:     "release.find_tasks_for_git_changes",
		Module:   "release",
		Mutating: false,
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "release.find_tasks_for_git_changes",
			Alias:       "find_tasks_for_git_changes",
			Category:    "Release",
			Description: "Resolve tasks related to PRs, branches, commits, and text references for a repository. Returns evidence and confidence for each match.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repository_id": map[string]any{
						"type":        "string",
						"description": "Optional repository ID. Defaults from the current repository target when omitted.",
					},
					"repo_full_name": map[string]any{
						"type":        "string",
						"description": "Optional repository full name like owner/repo.",
					},
					"pr_numbers": map[string]any{
						"type":        "array",
						"description": "Pull request numbers to resolve. Max 50.",
						"items":       map[string]any{"type": "integer"},
					},
					"commit_shas": map[string]any{
						"type":        "array",
						"description": "Commit SHAs to resolve. Max 200.",
						"items":       map[string]any{"type": "string"},
					},
					"branches": map[string]any{
						"type":        "array",
						"description": "Branch names to resolve. Max 50.",
						"items":       map[string]any{"type": "string"},
					},
					"texts": map[string]any{
						"type":        "array",
						"description": "Free text to scan for task keys. Max 100.",
						"items":       map[string]any{"type": "string"},
					},
				},
				"additionalProperties": false,
			},
		},
		Execute: s.executeFindTasksForGitChanges,
	})
	s.register(InternalCommandDefinition{
		Name:     "release.get_task_context",
		Module:   "release",
		Mutating: false,
		Tool:     mustCommandToolMetadata("release.get_task_context"),
		Execute:  s.executeGetTaskContext,
	})
}

func (s *InternalCommandService) executeGetReleaseContext(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req model.GetReleaseContextRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse get release context input: %w", err)
		}
	}
	applyCommandRepositoryTargetDefault(meta, &req.RepositoryID)
	req.RepoFullName = strings.TrimSpace(req.RepoFullName)
	req.TagName = strings.TrimSpace(req.TagName)
	if req.TagName == "" {
		return nil, fmt.Errorf("tag_name is required")
	}
	if strings.TrimSpace(req.RepositoryID) == "" && req.RepoFullName == "" {
		return nil, fmt.Errorf("repository_id or repo_full_name is required")
	}
	if s.releaseFactsProvider == nil {
		return nil, fmt.Errorf("release facts are not available")
	}
	result, err := s.releaseFactsProvider.GetReleaseContext(ctx, meta.WorkspaceID, req)
	if err != nil {
		return nil, fmt.Errorf("get release context: %w", err)
	}
	return mustJSON(result), nil
}

func (s *InternalCommandService) executeFindTasksForGitChanges(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req model.FindTasksForGitChangesRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse find tasks for git changes input: %w", err)
		}
	}
	applyCommandRepositoryTargetDefault(meta, &req.RepositoryID)
	req.RepoFullName = strings.TrimSpace(req.RepoFullName)
	if strings.TrimSpace(req.RepositoryID) == "" && req.RepoFullName == "" {
		return nil, fmt.Errorf("repo_full_name or repository_id is required")
	}
	if len(req.PRNumbers) == 0 && len(req.CommitSHAs) == 0 && len(req.Branches) == 0 && len(req.Texts) == 0 {
		return nil, fmt.Errorf("at least one evidence array is required")
	}
	if s.releaseFactsProvider == nil {
		return nil, fmt.Errorf("release facts are not available")
	}
	result, err := s.releaseFactsProvider.FindTasksForGitChanges(ctx, meta.WorkspaceID, req)
	if err != nil {
		return nil, fmt.Errorf("find tasks for git changes: %w", err)
	}
	return mustJSON(result), nil
}

func (s *InternalCommandService) executeGetTaskContext(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req model.GetTaskContextRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse get task context input: %w", err)
		}
	}
	req.TaskIDs = commandTrimStringSlice(req.TaskIDs)
	req.TaskKeys = commandTrimStringSlice(req.TaskKeys)
	for _, taskKey := range req.TaskKeys {
		taskID, err := s.resolveCommandTaskKey(ctx, meta, taskKey)
		if err != nil {
			return nil, err
		}
		req.TaskIDs = append(req.TaskIDs, taskID)
	}
	req.TaskIDs = uniqueNonEmptyStringsWithLimit(req.TaskIDs, 51)
	if len(req.TaskIDs) == 0 && commandTaskTargetID(meta) != "" {
		req.TaskIDs = []string{commandTaskTargetID(meta)}
	}
	if len(req.TaskIDs) == 0 {
		return nil, fmt.Errorf("task_ids or task_keys is required")
	}
	if len(req.TaskIDs) > 50 {
		return nil, fmt.Errorf("at most 50 task_ids may be requested")
	}
	if s.taskService != nil {
		for _, taskID := range req.TaskIDs {
			detail, err := s.taskService.GetByID(ctx, taskID)
			if err != nil || detail.Task.WorkspaceID != meta.WorkspaceID {
				return nil, fmt.Errorf("task not found")
			}
			if err := s.validateTaskWithinTarget(ctx, meta, &detail.Task); err != nil {
				return nil, err
			}
		}
	}
	if s.releaseFactsProvider == nil {
		return nil, fmt.Errorf("task context is not available")
	}
	result, err := s.releaseFactsProvider.GetTaskContext(ctx, meta.WorkspaceID, req)
	if err != nil {
		return nil, fmt.Errorf("get task context: %w", err)
	}
	return mustJSON(result), nil
}

func applyCommandRepositoryTargetDefault(meta model.InternalCommandContext, repositoryID *string) {
	if repositoryID == nil {
		return
	}
	*repositoryID = strings.TrimSpace(*repositoryID)
	if *repositoryID == "" && strings.TrimSpace(meta.TargetType) == "repository" {
		*repositoryID = strings.TrimSpace(meta.TargetID)
	}
}
