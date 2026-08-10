package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDockRunAttentionKind(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		pauseReason string
		want        string
	}{
		{name: "input", status: model.AgentRunStatusPaused, pauseReason: model.AgentRunPauseReasonHumanInput, want: "input"},
		{name: "approval", status: model.AgentRunStatusPaused, pauseReason: model.AgentRunPauseReasonHumanApproval, want: "approval"},
		{name: "authentication", status: model.AgentRunStatusPaused, pauseReason: model.AgentRunPauseReasonAuthentication, want: "authentication"},
		{name: "user message is not attention", status: model.AgentRunStatusPaused, pauseReason: model.AgentRunPauseReasonUserMessage},
		{name: "running is not attention", status: model.AgentRunStatusRunning, pauseReason: model.AgentRunPauseReasonHumanInput},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := dockRunAttentionKind(model.AgentRun{Status: test.status, PauseReason: test.pauseReason})
			if got != test.want {
				t.Fatalf("dockRunAttentionKind() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestGetDockRunForActorEnforcesOwnershipAndExcludesChatRuns(t *testing.T) {
	svc, _ := setupAgentRunActivityTest(t)
	ownerID := "user-1"
	otherID := "user-2"
	chatID := "chat-1"

	for _, run := range []*model.AgentRun{
		{ID: "owned", WorkspaceID: "ws-1", AgentID: "agent-1", TargetType: "task", TargetID: "task-1", RuntimeKind: "native_sdk", InvocationMode: model.InvocationModeInteractive, ApprovalState: "not_required", PauseReason: model.AgentRunPauseReasonNone, TriggeredByUserID: &ownerID, Status: model.AgentRunStatusRunning, Input: []byte(`{}`), OutputSummary: []byte(`{}`)},
		{ID: "other-user", WorkspaceID: "ws-1", AgentID: "agent-1", TargetType: "task", TargetID: "task-2", RuntimeKind: "native_sdk", InvocationMode: model.InvocationModeInteractive, ApprovalState: "not_required", PauseReason: model.AgentRunPauseReasonNone, TriggeredByUserID: &otherID, Status: model.AgentRunStatusRunning, Input: []byte(`{}`), OutputSummary: []byte(`{}`)},
		{ID: "chat-run", WorkspaceID: "ws-1", AgentID: "agent-1", TargetType: "workspace", TargetID: "ws-1", RuntimeKind: "native_sdk", InvocationMode: model.InvocationModeInteractive, ApprovalState: "not_required", PauseReason: model.AgentRunPauseReasonNone, TriggeredByUserID: &ownerID, DockChatID: &chatID, Status: model.AgentRunStatusRunning, Input: []byte(`{}`), OutputSummary: []byte(`{}`)},
	} {
		if err := svc.runRepo.Create(context.Background(), run); err != nil {
			t.Fatalf("create run %q: %v", run.ID, err)
		}
	}

	run, err := svc.GetDockRunForActor(context.Background(), "ws-1", ownerID, "owned")
	if err != nil || run == nil || run.ID != "owned" {
		t.Fatalf("owned run = %#v, err = %v", run, err)
	}
	for _, test := range []struct {
		name    string
		actorID string
		runID   string
	}{
		{name: "other user", actorID: ownerID, runID: "other-user"},
		{name: "chat backing run", actorID: ownerID, runID: "chat-run"},
		{name: "blank actor", actorID: "", runID: "owned"},
		{name: "missing run", actorID: ownerID, runID: "missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, gotErr := svc.GetDockRunForActor(context.Background(), "ws-1", test.actorID, test.runID)
			if !errors.Is(gotErr, ErrDockRunNotFound) {
				t.Fatalf("error = %v, want ErrDockRunNotFound", gotErr)
			}
		})
	}
}
