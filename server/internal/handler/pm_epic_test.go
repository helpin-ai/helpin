package handler

import (
	"fmt"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestValidateLinkEpicTasksRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     model.LinkEpicTasksRequest
		wantErr string
	}{
		{name: "valid", req: model.LinkEpicTasksRequest{TaskIDs: []string{"task-a", "task-b"}}},
		{name: "empty", req: model.LinkEpicTasksRequest{}, wantErr: "at least one"},
		{name: "blank id", req: model.LinkEpicTasksRequest{TaskIDs: []string{"task-a", "  "}}, wantErr: "cannot be empty"},
		{name: "too many", req: model.LinkEpicTasksRequest{TaskIDs: makeTaskIDs(101)}, wantErr: "at most 100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLinkEpicTasksRequest(tt.req)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func makeTaskIDs(count int) []string {
	ids := make([]string, count)
	for index := range ids {
		ids[index] = fmt.Sprintf("task-%d", index)
	}
	return ids
}
