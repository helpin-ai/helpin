package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentRuntimeClientStartRunUsesV1AuthAndHostRunID(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody AgentRuntimeStartRunRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(AgentRuntimeRun{
			ID:        "run-runtime-1",
			AppID:     gotBody.AppID,
			HostRunID: gotBody.HostRunID,
			AgentID:   gotBody.AgentID,
			Status:    "queued",
		})
	}))
	defer server.Close()

	client, err := NewAgentRuntimeClient(server.URL, "helpin", "runtime-token", server.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	run, err := client.StartRun(context.Background(), AgentRuntimeStartRunRequest{
		HostRunID: "helpin-run-1",
		AgentID:   "agent-runtime-agent-1",
		Target:    AgentRuntimeTargetRef{Type: "task", ID: "task-1"},
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if gotPath != "/v1/runs" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
	if gotAuth != "Bearer runtime-token" {
		t.Fatalf("unexpected authorization header: %q", gotAuth)
	}
	if gotBody.AppID != "helpin" || gotBody.HostRunID != "helpin-run-1" {
		t.Fatalf("unexpected start body: %#v", gotBody)
	}
	if run.ID != "run-runtime-1" || run.HostRunID != "helpin-run-1" {
		t.Fatalf("unexpected run response: %#v", run)
	}
}

func TestAgentRuntimeClientRequiresAppID(t *testing.T) {
	if _, err := NewAgentRuntimeClient("http://runtime.test", " ", "runtime-token", nil); err == nil {
		t.Fatal("expected missing app ID error")
	}
}

func TestAgentRuntimeClientForwardsRunSignals(t *testing.T) {
	var paths []string
	var resumeBodies []AgentRuntimeResumeRunRequest
	var messageBodies []map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.RequestURI())
		switch r.URL.EscapedPath() {
		case "/v1/runs/run-runtime-1/resume":
			var body AgentRuntimeResumeRunRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode resume body: %v", err)
			}
			resumeBodies = append(resumeBodies, body)
		case "/v1/runs/run-runtime-1/messages":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode message body: %v", err)
			}
			messageBodies = append(messageBodies, body)
		}
		_ = json.NewEncoder(w).Encode(AgentRuntimeRun{
			ID:     "run-runtime-1",
			AppID:  "helpin",
			Status: "paused",
		})
	}))
	defer server.Close()

	client, err := NewAgentRuntimeClient(server.URL+"/", "helpin", "runtime-token", server.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if _, err := client.ResumeRun(context.Background(), "run-runtime-1", AgentRuntimeResumeRunRequest{Intent: "reply", Content: "continue"}); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if _, err := client.ApproveRun(context.Background(), "run-runtime-1", "user-1"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := client.RequestChanges(context.Background(), "run-runtime-1", "revise", "user-2"); err != nil {
		t.Fatalf("request changes: %v", err)
	}
	if err := client.AppendMessage(context.Background(), "run-runtime-1", "user", "hello", "user-3"); err != nil {
		t.Fatalf("append message: %v", err)
	}
	if _, err := client.CancelRun(context.Background(), "run-runtime-1"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	want := []string{
		"POST /v1/runs/run-runtime-1/resume?app_id=helpin",
		"POST /v1/runs/run-runtime-1/resume?app_id=helpin",
		"POST /v1/runs/run-runtime-1/resume?app_id=helpin",
		"POST /v1/runs/run-runtime-1/messages?app_id=helpin",
		"POST /v1/runs/run-runtime-1/cancel?app_id=helpin",
	}
	if len(paths) != len(want) {
		t.Fatalf("expected paths %#v, got %#v", want, paths)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("path %d: expected %q, got %q", i, want[i], paths[i])
		}
	}
	if len(resumeBodies) != 3 {
		t.Fatalf("expected three resume bodies, got %#v", resumeBodies)
	}
	if resumeBodies[1].Intent != "approve" || resumeBodies[1].ExternalActorID != "user-1" {
		t.Fatalf("expected attributed approve body, got %#v", resumeBodies[1])
	}
	if resumeBodies[2].Intent != "request_changes" || resumeBodies[2].Content != "revise" || resumeBodies[2].ExternalActorID != "user-2" {
		t.Fatalf("expected attributed request-changes body, got %#v", resumeBodies[2])
	}
	if len(messageBodies) != 1 || messageBodies[0]["external_actor_id"] != "user-3" {
		t.Fatalf("expected attributed message body, got %#v", messageBodies)
	}
}
