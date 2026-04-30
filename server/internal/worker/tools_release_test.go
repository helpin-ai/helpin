package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestToolGetReleaseContextUsesRunDefaults(t *testing.T) {
	var captured model.GetReleaseContextRequest
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		TargetType:  "repository",
		TargetID:    "repo-1",
		RunInput: &model.AgentRunInputPayload{
			Event: &model.AgentRunEventContext{
				GitHub: &model.AgentRunGitHubEventContext{
					RepoFullName: "acme/api",
					RepositoryID: "repo-1",
					Release: &model.AgentRunGitHubReleaseEventContext{
						TagName: "v1.4.0",
					},
				},
			},
		},
		Services: &ServiceBridge{
			GetReleaseContext: func(ctx context.Context, workspaceID string, req model.GetReleaseContextRequest) (*model.ReleaseContextResult, error) {
				captured = req
				return &model.ReleaseContextResult{
					RepositoryID: workspaceID,
					RepoFullName: req.RepoFullName,
					CurrentRelease: model.ReleaseSummary{
						TagName: req.TagName,
					},
				}, nil
			},
		},
	}

	output, err := toolGetReleaseContext(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("toolGetReleaseContext returned error: %v", err)
	}
	if captured.RepositoryID != "repo-1" || captured.RepoFullName != "acme/api" || captured.TagName != "v1.4.0" {
		t.Fatalf("unexpected request defaults %#v", captured)
	}
	if !strings.Contains(output, `"tag_name":"v1.4.0"`) {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestToolFindTasksForGitChangesRequiresEvidence(t *testing.T) {
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		Services: &ServiceBridge{
			FindTasksForGitChanges: func(ctx context.Context, workspaceID string, req model.FindTasksForGitChangesRequest) (*model.FindTasksForGitChangesResult, error) {
				return &model.FindTasksForGitChangesResult{}, nil
			},
		},
	}

	if _, err := toolFindTasksForGitChanges(ctx, json.RawMessage(`{"repo_full_name":"acme/api"}`)); err == nil || err.Error() != "at least one evidence array is required" {
		t.Fatalf("expected evidence validation error, got %v", err)
	}
}

func TestToolGetTaskContextRequiresTaskIDs(t *testing.T) {
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		Services: &ServiceBridge{
			GetTaskContext: func(ctx context.Context, workspaceID string, req model.GetTaskContextRequest) (*model.GetTaskContextResult, error) {
				return &model.GetTaskContextResult{}, nil
			},
		},
	}

	if _, err := toolGetTaskContext(ctx, json.RawMessage(`{}`)); err == nil || err.Error() != "task_ids is required" {
		t.Fatalf("expected task_ids validation error, got %v", err)
	}
}
