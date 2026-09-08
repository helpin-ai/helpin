package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAgentRuntimeClientResumeProvenanceWire(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("app_id") != "helpin" || r.Header.Get("Authorization") != "Bearer test-token" || r.URL.Path != "/v1/runs/r/resume" {
			t.Errorf("invalid trusted routing")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["message_provenance"] != "system_notification" || body["resume_id"] != "child-1" || body["intent"] != "reply" {
			t.Errorf("wire payload: %+v", body)
		}
		_, _ = w.Write([]byte(`{"id":"r","status":"running"}`))
	}))
	defer server.Close()
	client, err := NewAgentRuntimeClient(server.URL, "helpin", "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ResumeRunWithProvenance(context.Background(), "r", AgentRuntimeResumeRunRequest{Intent: "reply", ResumeID: "child-1", Content: "Child completed"}, "system_notification"); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRuntimeClientHumanAndLegacyResumeProvenance(t *testing.T) {
	for _, test := range []struct {
		name, intent, actor, want string
	}{
		{name: "human reply", intent: "reply", actor: "user-1", want: "human"},
		{name: "human approval", intent: "approve", actor: "user-1", want: "human"},
		{name: "human correction", intent: "request_changes", actor: "user-1", want: "human"},
		{name: "legacy reply", intent: "reply"},
		{name: "auth event", intent: "auth_completed", actor: "user-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					MessageProvenance string          `json:"message_provenance"`
					ResumeID          string          `json:"resume_id"`
					InteractionID     string          `json:"interaction_id"`
					ResponsePayload   json.RawMessage `json:"response_payload"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if body.MessageProvenance != test.want || body.ResumeID != "turn-1" || body.InteractionID != "interaction-1" || string(body.ResponsePayload) != `{"answer":"draft only"}` {
					t.Errorf("resume contract changed: %+v", body)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("service authentication missing")
				}
				if err := json.NewEncoder(w).Encode(AgentRuntimeRun{ID: "r"}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := NewAgentRuntimeClient(server.URL, "helpin", "test-token", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.ResumeRun(context.Background(), "r", AgentRuntimeResumeRunRequest{
				Intent: test.intent, ExternalActorID: test.actor, ResumeID: "turn-1", InteractionID: "interaction-1",
				ResponsePayload: json.RawMessage(`{"answer":"draft only"}`),
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAgentRuntimeClientRejectsNotificationApprovalBeforeSending(t *testing.T) {
	client, err := NewAgentRuntimeClient("http://unused.invalid", "helpin", "", http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ResumeRunWithProvenance(context.Background(), "r", AgentRuntimeResumeRunRequest{Intent: model.AgentRunResumeIntentApprove}, "system_notification")
	if err == nil {
		t.Fatal("notification approval accepted")
	}
}
