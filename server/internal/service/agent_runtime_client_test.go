package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
		MCPServers: []ExternalMCPRunServer{{
			ServerID: "customer-io", ServerName: "customer_io", Transport: "streamable_http", URL: "https://mcp.customer.io/mcp",
			Tools:      []ExternalMCPRunTool{{Name: "cio_read_api", Access: "read"}},
			Credential: &ExternalMCPRunCredential{Type: "bearer_token", AccessToken: "run-secret"},
		}},
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
	if len(gotBody.MCPServers) != 1 || gotBody.MCPServers[0].Credential == nil || gotBody.MCPServers[0].Credential.AccessToken != "run-secret" {
		t.Fatalf("MCP attachment not forwarded: %#v", gotBody.MCPServers)
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

func TestAgentRuntimeClientListsBoundedV2EventPageWithoutSDKChange(t *testing.T) {
	var gotPath, gotQuery, gotAuth, gotProtocol string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		gotProtocol = r.Header.Get("X-Agent-Runtime-Event-Protocol")
		_ = json.NewEncoder(w).Encode(AgentRuntimeEventListResponse{
			Events:         []AgentRuntimeEventEnvelope{{RunID: "runtime/run", SequenceNo: 42, Type: "run.started"}},
			NextSequenceNo: 42,
		})
	}))
	defer server.Close()

	client, err := NewAgentRuntimeClient(server.URL, "helpin", "runtime-token", server.Client(), "v2")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	response, err := client.ListV2EventPage(context.Background(), "runtime/run", 41, 250)
	if err != nil {
		t.Fatalf("list event page: %v", err)
	}
	if gotPath != "/v2/runs/runtime%2Frun/events" || gotQuery != "after_sequence=41&app_id=helpin&page_size=250" {
		t.Fatalf("unexpected page request: %s?%s", gotPath, gotQuery)
	}
	if gotAuth != "Bearer runtime-token" || gotProtocol != "v2" {
		t.Fatalf("unexpected page headers: auth=%q protocol=%q", gotAuth, gotProtocol)
	}
	if response == nil || len(response.Events) != 1 || response.Events[0].SequenceNo != 42 || response.NextSequenceNo != 42 {
		t.Fatalf("unexpected page response: %#v", response)
	}
}

func TestAgentRuntimeClientRotatesOnlyCredential(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotProtocol string
	var gotBody struct {
		Credential ExternalMCPRunCredential `json:"credential"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotQuery = r.URL.RawQuery
		gotProtocol = r.Header.Get("X-Agent-Runtime-Event-Protocol")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode credential rotation: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"run_id": "runtime/run", "server_id": "server one"})
	}))
	defer server.Close()

	client, err := NewAgentRuntimeClient(server.URL, "helpin", "runtime-token", server.Client(), "v2")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.UpdateRunMCPCredential(context.Background(), "runtime/run", "server one", ExternalMCPRunCredential{Type: "bearer_token", AccessToken: "replacement"}); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPut || gotPath != "/v1/runs/runtime%2Frun/mcp-servers/server%20one/credential" || gotQuery != "app_id=helpin" {
		t.Fatalf("unexpected rotation request: %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	if gotProtocol != "v2" {
		t.Fatalf("event protocol header = %q", gotProtocol)
	}
	if gotBody.Credential.Type != "bearer_token" || gotBody.Credential.AccessToken != "replacement" {
		t.Fatalf("credential body = %#v", gotBody.Credential)
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
		case "/v1/runs/run-runtime-1/codex-auth/device-code/start",
			"/v1/runs/run-runtime-1/codex-auth/device-code/cancel":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"provider":  "openai",
				"auth_mode": "chatgpt_device_code",
				"state":     "pending",
			})
			return
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

}

func TestAgentRuntimeClientRequestsManualPauseWithV1Auth(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/v1/runs/run-runtime-1/pause" || r.URL.Query().Get("app_id") != "helpin" {
			t.Errorf("unexpected pause request: %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer runtime-token" || r.Header.Get("X-Agent-Runtime-Event-Protocol") != "v2" {
			t.Error("pause request omitted runtime authentication or protocol")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"run-runtime-1","status":"running"}`)), Request: r}, nil
	})}
	client, err := NewAgentRuntimeClient("https://runtime.example.test", "helpin", "runtime-token", httpClient, "v2")
	if err != nil {
		t.Fatal(err)
	}
	run, err := client.PauseRun(context.Background(), "run-runtime-1")
	if err != nil || run.ID != "run-runtime-1" {
		t.Fatalf("pause request failed: run=%#v err=%v", run, err)
	}
}
