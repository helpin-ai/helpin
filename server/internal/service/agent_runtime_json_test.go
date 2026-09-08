package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAgentRuntimeProjectionJSONEquality(t *testing.T) {
	for _, tt := range []struct {
		name, left, right string
		want              bool
	}{
		{"jsonb object order and whitespace", `{"b":2,"a":{"x":1}}`, `{ "a": {"x": 1}, "b": 2 }`, true},
		{"escaped text", `{"text":"\u0041"}`, `{"text":"A"}`, true},
		{"numeric representation", `{"n":1e3}`, `{"n":1000.00}`, true},
		{"missing nullable value", ``, `null`, true},
		{"large distinct integers", `{"id":9007199254740992}`, `{"id":9007199254740993}`, false},
		{"changed value", `{"a":1}`, `{"a":2}`, false},
		{"array order matters", `[1,2]`, `[2,1]`, false},
		{"null is not an empty array", `null`, `[]`, false},
		{"number is not text", `1`, `"1"`, false},
		{"trailing invalid data", `{"a":1} false`, `{"a":1}`, false},
		{"different invalid data", `{a}`, `{b}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := agentRuntimeProjectionJSONRawEqual(json.RawMessage(tt.left), json.RawMessage(tt.right)); got != tt.want {
				t.Fatalf("JSON equality = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAgentRuntimeMessageReconciliationIgnoresJSONBFormatting(t *testing.T) {
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1", Status: model.AgentRunStatusPaused, PauseReason: model.AgentRunPauseReasonUserMessage}
	runs := &projectionChangeRepo{fakeAgentRuntimeProjectionRunRepo: &fakeAgentRuntimeProjectionRunRepo{}}
	messages := &fakeAgentRuntimeProjectionMessageRepo{}
	svc := &AgentRuntimeProjectionService{runRepo: runs, runMessageRepo: messages}
	runtimeMessage := AgentRuntimeMessage{ID: "message-1", Role: "assistant", Content: "Reply", ContentBlocks: json.RawMessage(`[{"type":"text","text":"Reply"}]`)}
	if err := svc.createRuntimeMessage(context.Background(), run, runtimeMessage); err != nil {
		t.Fatal(err)
	}
	runs.notifications = 0
	runs.changes = nil
	for attempt := 0; attempt < 3; attempt++ {
		for _, raw := range []*json.RawMessage{&messages.messages[0].ContentBlocks, &messages.messages[0].TurnSegments} {
			if len(*raw) == 0 {
				continue
			}
			var value any
			if err := json.Unmarshal(*raw, &value); err != nil {
				t.Fatal(err)
			}
			formatted, err := json.MarshalIndent(value, "", " ")
			if err != nil {
				t.Fatal(err)
			}
			*raw = formatted
		}
		if err := svc.createRuntimeMessage(context.Background(), run, runtimeMessage); err != nil {
			t.Fatal(err)
		}
	}
	if messages.updates != 0 || runs.notifications != 0 {
		t.Fatalf("unchanged JSONB message: updates=%d notifications=%d, want 0/0", messages.updates, runs.notifications)
	}
	runtimeMessage.Content = "An updated reply"
	if err := svc.createRuntimeMessage(context.Background(), run, runtimeMessage); err != nil {
		t.Fatal(err)
	}
	if messages.updates != 1 || runs.notifications != 1 {
		t.Fatalf("changed message: updates=%d notifications=%d, want 1/1", messages.updates, runs.notifications)
	}
	if len(runs.changes) != 1 || runs.changes[0] != model.AgentRunChangeMessage {
		t.Fatalf("changed message must preserve content notification: %v", runs.changes)
	}
}

type projectionChangeRepo struct {
	*fakeAgentRuntimeProjectionRunRepo
	changes []model.AgentRunChangeKind
}

func (r *projectionChangeRepo) NotifyChange(ctx context.Context, run *model.AgentRun, kind model.AgentRunChangeKind) {
	r.changes = append(r.changes, kind)
	r.Notify(ctx, run)
}
