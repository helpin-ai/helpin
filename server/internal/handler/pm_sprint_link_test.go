package handler

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestValidateLinkSprintTasksRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     model.LinkSprintTasksRequest
		wantErr string
	}{
		{name: "valid", req: model.LinkSprintTasksRequest{TaskIDs: []string{"task-a", "task-b"}}},
		{name: "empty", req: model.LinkSprintTasksRequest{}, wantErr: "at least one"},
		{name: "blank id", req: model.LinkSprintTasksRequest{TaskIDs: []string{"task-a", "  "}}, wantErr: "cannot be empty"},
		{name: "too many", req: model.LinkSprintTasksRequest{TaskIDs: makeTaskIDs(101)}, wantErr: "at most 100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLinkSprintTasksRequest(tt.req)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
