package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAgentRuntimeProjectionCanonicalAnswerSurvivesPauseAndReplay(t *testing.T) {
	const answer = "The editor supports **Mermaid**, `nwdiag`, and Excalidraw.\n\nAdd a diagrams section to 4.2 and link to a dedicated guide."
	for _, delivery := range []string{"live", "missed final event", "reconnect", "identical prose"} {
		t.Run(delivery, func(t *testing.T) {
			ctx := context.Background()
			preambleText := "All the evidence is in. Here's the complete picture:"
			if delivery == "identical prose" {
				preambleText = answer
			}
			at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
			run := &model.AgentRun{
				ID: "helpin-answer", WorkspaceID: "ws-1", AgentID: "agent-1", DockChatID: stringPointer("dock-1"),
				RuntimeKind: "native_sdk", Status: model.AgentRunStatusRunning,
				ExternalRuntime: stringPointer(agentRuntimeName), ExternalRuntimeID: stringPointer("runtime-answer"),
			}
			repo := &fakeAgentRuntimeProjectionMessageRepo{}
			blocks, err := json.Marshal([]map[string]string{{"type": "text", "text": answer}})
			if err != nil {
				t.Fatal(err)
			}
			client := &fakeAgentRuntimeSignalClient{messages: map[string][]AgentRuntimeMessage{
				"runtime-answer": {
					{ID: "store-preamble", RuntimeMessageID: "preamble", Role: "assistant", Content: preambleText, MessageType: "assistant_turn", CreatedAt: at},
					{ID: "store-answer", RuntimeMessageID: "canonical-answer", Role: "assistant", Content: answer, ContentBlocks: blocks, MessageType: "assistant_final", CreatedAt: at.Add(time.Second)},
				},
			}}
			svc := &AgentRuntimeProjectionService{
				runRepo:        &fakeAgentRuntimeProjectionRunRepo{byExternal: map[string]*model.AgentRun{agentRuntimeName + "|runtime-answer": run}},
				runMessageRepo: repo, agentRuntimeClient: client, now: func() time.Time { return at.Add(2 * time.Second) },
			}
			apply := func(event AgentRuntimeEventEnvelope) {
				t.Helper()
				if err := svc.ApplyEvent(ctx, event); err != nil {
					t.Fatal(err)
				}
			}
			preamble := AgentRuntimeEventEnvelope{RunID: "runtime-answer", Type: "assistant_message_completed", SentAt: at, Data: map[string]any{"message_id": "preamble", "content": preambleText}}
			final := AgentRuntimeEventEnvelope{RunID: "runtime-answer", Type: "assistant_message_completed", SentAt: at.Add(time.Second), Data: map[string]any{"message_id": "canonical-answer", "content": answer, "message_type": "assistant_final"}}
			pause := AgentRuntimeEventEnvelope{RunID: "runtime-answer", Type: "run.paused", SentAt: at.Add(2 * time.Second), Data: map[string]any{"pause_reason": model.AgentRunPauseReasonUserMessage}}
			apply(preamble)
			if delivery == "live" {
				apply(final)
				apply(final)
			}
			apply(pause)
			if delivery == "reconnect" {
				apply(final)
				apply(preamble)
				apply(pause)
			}
			if len(repo.messages) != 2 {
				t.Fatalf("expected distinct preamble and answer without duplicates: %+v", repo.messages)
			}
			got := repo.messages[1]
			if got.Content != answer || got.RuntimeMessageID != "canonical-answer" || got.MessageType != "assistant_final" {
				t.Fatalf("lost canonical answer: %+v", got)
			}
			if got.SequenceNo <= repo.messages[0].SequenceNo {
				t.Fatal("answer precedes preamble")
			}
			if !jsonRawContainsStringField(got.ContentBlocks, "text", answer) {
				t.Fatalf("answer blocks disagree: %s", got.ContentBlocks)
			}
			if run.Status != model.AgentRunStatusPaused || run.PauseReason != model.AgentRunPauseReasonUserMessage {
				t.Fatalf("unexpected pause: %+v", run)
			}
		})
	}
}
