package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type runChangePublisher struct{ events []Event }

func (p *runChangePublisher) Publish(event Event) { p.events = append(p.events, event) }

func TestRunNotifierPreservesChangeKindAcrossAliases(t *testing.T) {
	for _, kind := range []model.AgentRunChangeKind{model.AgentRunChangeState, model.AgentRunChangeMessage, model.AgentRunChangeInteraction, model.AgentRunChangeArtifact} {
		t.Run(string(kind), func(t *testing.T) {
			publisher := &runChangePublisher{}
			chatID, stage := "chat-1", "waiting"
			run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1", Status: "paused", PauseReason: "awaiting_user_message", DockChatID: &chatID, ExecutionStage: &stage}
			run.UpdatedAt = time.Date(2026, 9, 7, 12, 0, 0, 123, time.UTC)
			NewRunNotifier(publisher).PublishRunChange(context.Background(), run, kind)
			if len(publisher.events) != 2 {
				t.Fatalf("events = %d, want 2", len(publisher.events))
			}
			for i, entity := range []string{"agent_run", "coding_session"} {
				event := publisher.events[i]
				var data map[string]string
				if err := json.Unmarshal(event.Data, &data); err != nil {
					t.Fatal(err)
				}
				if event.Entity != entity || event.EntityID != run.ID || event.WorkspaceID != run.WorkspaceID || data["change_kind"] != string(kind) || data["dock_chat_id"] != chatID || data["execution_stage"] != stage {
					t.Fatalf("incorrect event routing/semantics: %+v, data=%v", event, data)
				}
				if kind == model.AgentRunChangeState && data["state_revision"] != run.UpdatedAt.Format(time.RFC3339Nano) {
					t.Fatalf("missing persisted state revision: %v", data)
				}
			}
		})
	}
}
