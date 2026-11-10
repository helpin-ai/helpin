package service

import (
	"context"
	"encoding/json"
	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSteeringUsesWorkspaceMappingAndTrustedActor(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	repo := repository.NewAgentRunRepository(db)
	now := time.Now().UTC()
	seedAgentRuntimeSignalAgent(t, db, now)
	run := seedAgentRuntimeSignalRun(t, repo, model.AgentRunStatusRunning, model.AgentRunPauseReasonNone, "not_required", now)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/runs/run_runtime_1/steering" || r.URL.Query().Get("app_id") != "helpin" || r.Header.Get("Authorization") != "Bearer runtime-token" {
			t.Errorf("invalid runtime boundary: %s", r.URL)
		}
		var req sdk.SteerRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.ExternalActorID != "user-1" || req.CommandID != "command" || req.ExpectedRevision != 2 {
			t.Errorf("incorrect attribution: %+v", req)
		}
		if err := json.NewEncoder(w).Encode(sdk.RunSteeringCommand{SteerRunRequest: req, Revision: 3, State: "accepted"}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, err := NewAgentRuntimeClient(server.URL, "helpin", "runtime-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	service := &AgentService{runRepo: repo, agentRuntimeClient: client}
	request := model.SteerAgentRunRequest{CommandID: "command", ExpectedRevision: 2, Content: "Research only"}
	if _, err := service.SteerRun(context.Background(), "foreign", run.ID, "user-1", request); err == nil {
		t.Fatal("foreign workspace accepted")
	}
	if _, err := service.SteerRun(context.Background(), "ws-1", run.ID, "", request); err == nil {
		t.Fatal("anonymous actor accepted")
	}
	if calls != 0 {
		t.Fatal("invalid request reached runtime")
	}
	result, err := service.SteerRun(context.Background(), "ws-1", run.ID, "user-1", request)
	if err != nil || result.State != "accepted" || calls != 1 {
		t.Fatalf("steer: %+v %v calls=%d", result, err, calls)
	}
	after, err := repo.GetByID(context.Background(), "ws-1", run.ID)
	if err != nil || after.Status != model.AgentRunStatusRunning {
		t.Fatal("steering changed run lifecycle")
	}
}
