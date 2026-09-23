package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestArchiveAndRestoreTask(t *testing.T) {
	for _, tt := range []struct {
		tool string
		want bool
	}{{tool: "archive_task", want: true}, {tool: "restore_task", want: false}} {
		t.Run(tt.tool, func(t *testing.T) {
			service, principal, actor, tasks, _ := setupMCPPMToolTest()
			if _, err := service.executeSpecialMCPTool(context.Background(), principal, actor, tt.tool,
				json.RawMessage(`{"task_id":"task-1"}`)); err != nil {
				t.Fatalf("%s error = %v", tt.tool, err)
			}
			if tasks.updateID != "task-1" || tasks.updateRequest.Archived == nil || *tasks.updateRequest.Archived != tt.want {
				t.Fatalf("update = %s %#v", tasks.updateID, tasks.updateRequest)
			}
		})
	}
}

func TestArchiveTaskHidesOtherWorkspaces(t *testing.T) {
	service, principal, actor, tasks, _ := setupMCPPMToolTest()
	tasks.tasks["task-1"].Task.WorkspaceID = "workspace-other"
	_, err := service.executeSpecialMCPTool(context.Background(), principal, actor, "archive_task",
		json.RawMessage(`{"task_id":"task-1"}`))
	if !errors.Is(err, ErrMCPNotFound) || tasks.updateID != "" {
		t.Fatalf("error = %v, update = %q", err, tasks.updateID)
	}
}

func TestListTaskCommentsExposesOnlyPublicFields(t *testing.T) {
	service, principal, actor, _, _ := setupMCPPMToolTest()
	service.pmComments = fakeMCPPMCommentLister{
		{
			Comment: model.PMComment{ID: "c-1", AuthorID: "user-2", Body: "Looks good"},
			Author:  model.User{ID: "user-2", FullName: "Sam Lee", Email: "sam@example.com"},
			Replies: []model.CommentWithAuthor{{
				Comment: model.PMComment{ID: "c-2", AuthorID: "user-1", Body: "Thanks"},
				Author:  model.User{ID: "user-1", FullName: "Ava"},
			}},
		},
	}
	result, err := service.executeSpecialMCPTool(context.Background(), principal, actor, "list_task_comments",
		json.RawMessage(`{"task_id":"task-1"}`))
	if err != nil {
		t.Fatalf("list_task_comments error = %v", err)
	}
	encoded, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !jsonContains(encoded, `"author_name":"Sam Lee"`) || !jsonContains(encoded, `"body":"Thanks"`) {
		t.Fatalf("comments = %s", encoded)
	}
	if jsonContains(encoded, "sam@example.com") {
		t.Fatalf("comments leaked author email: %s", encoded)
	}
}

type fakeMCPPMCommentLister []model.CommentWithAuthor

func (f fakeMCPPMCommentLister) List(context.Context, string, string) ([]model.CommentWithAuthor, error) {
	return f, nil
}

func jsonContains(encoded []byte, fragment string) bool {
	return strings.Contains(string(encoded), fragment)
}
