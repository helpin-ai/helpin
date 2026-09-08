package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestFinalizerMarkerDoesNotReplaceSettledUsage(t *testing.T) {
	ctx := context.Background()
	durable := &model.AgentRun{ID: "run", WorkspaceID: "ws", OutputSummary: json.RawMessage(`{"ai_usage_checkpoint":{"turn":2,"input_tokens":200},"other_marker":true}`)}
	stale := *durable
	stale.OutputSummary = json.RawMessage(`{"ai_usage_checkpoint":{"turn":1,"input_tokens":100}}`)
	repo := &fakeAgentRuntimeProjectionRunRepo{byID: map[string]*model.AgentRun{"run": durable}}
	finalizers := &AgentRunFinalizerService{runRepo: repo}
	if err := finalizers.markRunOutputSummaryFlag(ctx, &stale, agentRuntimeFinalizerAgentIdleSummaryKey); err != nil {
		t.Fatal(err)
	}
	if checkpoint := agentRunUsageCheckpointFromSummary(durable.OutputSummary); checkpoint.Turn != 2 || checkpoint.InputTokens != 200 {
		t.Fatalf("finalizer overwrote durable usage: %+v", checkpoint)
	}
	if !runOutputSummaryFlag(durable.OutputSummary, agentRuntimeFinalizerAgentIdleSummaryKey) || !runOutputSummaryFlag(durable.OutputSummary, "other_marker") {
		t.Fatal("finalizer marker missing or unrelated marker lost")
	}
	// Do not refresh the stale local watermark from the marker write: the final
	// guarded projection save still needs to detect that this worker is stale.
	if agentRunUsageCheckpointFromSummary(stale.OutputSummary).Turn != 1 {
		t.Fatal("marker refreshed stale local watermark")
	}
}

func TestProjectionMarkerDoesNotReplaceSettledUsage(t *testing.T) {
	for _, test := range []struct{ name, key, value string }{
		{"replay", agentRuntimeV2ReplayThroughSummaryKey, "20"},
		{"transcript", agentRuntimeTranscriptReconciledVersionKey, `"2026-09-07T12:00:00Z"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			durable := &model.AgentRun{ID: "run", WorkspaceID: "ws", OutputSummary: json.RawMessage(`{"ai_usage_checkpoint":{"turn":2,"input_tokens":200}}`)}
			stale := *durable
			stale.OutputSummary = json.RawMessage(`{"ai_usage_checkpoint":{"turn":1,"input_tokens":100},"` + test.key + `":` + test.value + `}`)
			repo := &fakeAgentRuntimeProjectionRunRepo{byID: map[string]*model.AgentRun{"run": durable}}
			if err := persistRuntimeSummaryMarker(context.Background(), repo, &stale, test.key); err != nil {
				t.Fatal(err)
			}
			if checkpoint := agentRunUsageCheckpointFromSummary(durable.OutputSummary); checkpoint.Turn != 2 || checkpoint.InputTokens != 200 {
				t.Fatalf("projection marker overwrote usage: %+v", checkpoint)
			}
		})
	}
}
