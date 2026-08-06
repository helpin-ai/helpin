package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPMDeliveryCommandsAreRegisteredAndTargetAware(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	tests := []struct {
		command string
		alias   string
		target  string
	}{
		{"pm.update_task_delivery_target", "update_task_delivery_target", "task"},
		{"pm.update_epic_delivery_target", "update_epic_delivery_target", "epic"},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			def, ok := svc.Definition(tt.command)
			if !ok || def.Tool == nil {
				t.Fatalf("missing command definition %q", tt.command)
			}
			if def.Tool.Alias != tt.alias || !def.Mutating || !containsCommandTarget(def.SupportedTargetTypes, tt.target) {
				t.Fatalf("unexpected definition: %#v", def)
			}
		})
	}
}

func TestPMDeliveryCommandRejectsConflictingTaskTargetBeforeMutation(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	_, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "workspace-1", TargetType: "task", TargetID: "task-1",
	}, "pm.update_task_delivery_target", json.RawMessage(`{"task_id":"task-2","repository_id":"repo-1"}`))
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("expected conflicting target error, got %v", err)
	}
}

func TestPMDeliveryCommandReportsUnavailableGitService(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	_, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "workspace-1", TargetType: "task", TargetID: "task-1",
	}, "pm.update_task_delivery_target", json.RawMessage(`{"repository_id":"repo-1"}`))
	if err == nil || !strings.Contains(err.Error(), "git service") {
		t.Fatalf("expected unavailable git service error, got %v", err)
	}
}
