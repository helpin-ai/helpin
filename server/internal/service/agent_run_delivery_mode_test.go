package service

import (
	"context"
	"encoding/json"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"slices"
	"strings"
	"testing"
)

func TestPreviewRunProjectionAndDelivery(t *testing.T) {
	run := &model.AgentRun{ID: "preview", TargetType: "task", Input: json.RawMessage(`{"delivery_mode":"preview","additional_context":"Test the fix"}`), OutputSummary: json.RawMessage(`{"repository":{"pushed":true,"branch":"should-not-publish"}}`)}
	req, err := buildRuntimeStartRunRequest(run, &model.Agent{}, AgentRuntimeAgent{AllowedTools: []string{"run_command", "edit_file", "commit_and_push", "open_pr"}})
	if err != nil {
		t.Fatal(err)
	}
	if req.Metadata["delivery_mode"] != "preview" || !strings.Contains(req.Instructions, "Automatic repository delivery is disabled") {
		t.Fatalf("missing preview policy: %#v", req)
	}
	if !slices.Equal(req.AllowedTools, []string{"run_command", "edit_file"}) {
		t.Fatalf("tools=%v", req.AllowedTools)
	}
	delivery := &fakeFinalizerRepositoryDelivery{}
	svc := &AgentRunFinalizerService{repositoryDelivery: delivery}
	if err := svc.finalizeRepositoryDelivery(context.Background(), run); err != nil || len(delivery.calls) != 0 {
		t.Fatalf("preview published: %v %v", delivery.calls, err)
	}
	// The lower-level service must also refuse delivery even with a misleading pushed summary.
	if result, err := (&GitService{}).FinalizeDelegatedRunDelivery(context.Background(), run, AgentRunRepositoryDelivery{Branch: "test"}); result != nil || err != nil {
		t.Fatalf("preview delivery=%v %v", result, err)
	}
}

func TestPreviewRunPersistedAndInherited(t *testing.T) {
	db := setupAutomationDelegationTestDB(t)
	repo := repository.NewAgentRunRepository(db)
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := (&AgentService{runRepo: repo, agentRepo: repository.NewAgentRepository(db)}).SetAgentRuntimeClient(runtimeClient).SetAgentRuntimeLaunchEnabled(true)
	agent := &model.Agent{ID: "agent", WorkspaceID: "ws", IsSystem: true, PresetKey: model.AgentPresetMarketer, RuntimeKind: "native_sdk", AllowedCommands: json.RawMessage(`[]`), AllowedTargets: json.RawMessage(`[]`), AllowedTools: json.RawMessage(`["run_command"]`)}
	parent, err := svc.createRun(context.Background(), createRunParams{workspaceID: "ws", agent: agent, targetType: "workspace", targetID: "ws", input: []byte(`{}`), deliveryMode: "preview"})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := repo.GetByID(context.Background(), "ws", parent.ID)
	if err != nil || !agentRunIsPreview(saved) {
		t.Fatalf("preview not persisted: %v", err)
	}
	if _, err := svc.createRun(context.Background(), createRunParams{workspaceID: "ws", agent: agent, targetType: "workspace", targetID: "ws", input: []byte(`{}`), deliveryMode: "publish"}); err == nil {
		t.Fatal("reused active preview as publish run")
	}
	child, err := svc.createRun(context.Background(), createRunParams{workspaceID: "ws", agent: agent, targetType: "workspace", targetID: "other", input: []byte(`{}`), parentRunID: &parent.ID, deliveryMode: "publish"})
	if err != nil || !agentRunIsPreview(child) {
		t.Fatalf("child lost preview policy: %v", err)
	}
	if _, err := svc.startTargetRunWithOptions(context.Background(), "ws", "task", "task", model.StartAgentRunRequest{AgentID: "agent", DeliveryMode: "typo"}, nil, nil, nil, nil, startTargetRunOptions{}); err == nil {
		t.Fatal("accepted invalid delivery mode")
	}
}
