package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestApproveEpicSpecToolReturnsApprovedSummary(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context:     context.Background(),
		WorkspaceID: "ws-1",
		TargetType:  "epic",
		TargetID:    "epic-1",
		AgentID:     "agent-1",
		AllowedTools: map[string]bool{
			"approve_epic_spec": true,
		},
		Services: &ServiceBridge{
			ApproveEpicSpec: func(ctx context.Context, workspaceID, epicID, actorID string, versionID *string) (*model.ApprovedSpecSummary, error) {
				if workspaceID != "ws-1" || epicID != "epic-1" || actorID != "agent-1" {
					t.Fatalf("unexpected tool context: workspace=%q epic=%q actor=%q", workspaceID, epicID, actorID)
				}
				if versionID == nil || *versionID != "ver-1" {
					t.Fatalf("expected version_id ver-1, got %#v", versionID)
				}
				return &model.ApprovedSpecSummary{
					Stage:          model.PlanningStageDraftSpec,
					SpecDocumentID: "doc-1",
					SpecVersionID:  "ver-1",
				}, nil
			},
		},
	}

	output, err := registry.ExecuteAllowed(ctx, "approve_epic_spec", json.RawMessage(`{"version_id":"ver-1"}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"spec_document_id": "doc-1"`) || !strings.Contains(output, `"spec_version_id": "ver-1"`) {
		t.Fatalf("expected approved spec summary JSON, got %s", output)
	}
}
